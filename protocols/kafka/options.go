// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package kafka

import (
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

// Option mutates dial-time configuration. Functional-options pattern,
// same shape as the rest of mockarty-go.
type Option func(*config)

type config struct {
	recorder       telemetry.StepRecorder
	balancer       kafka.Balancer
	brokers        []string
	writeTimeout   time.Duration
	requiredAcks   kafka.RequiredAcks
	payloadCap     int
	allowAutoTopic bool
}

func defaultConfig() *config {
	return &config{
		recorder:     telemetry.NopRecorder{},
		balancer:     &kafka.LeastBytes{},
		writeTimeout: 10 * time.Second,
		requiredAcks: kafka.RequireOne,
		payloadCap:   1024,
	}
}

// WithRecorder wires a step recorder. Nil = drop steps (default).
// Typical use:
//
//	rec := telemetry.NewExternalRunsRecorder(runs, runID)
//	cli, _ := kafka.NewClient([]string{"kafka:9092"}, kafka.WithRecorder(rec))
func WithRecorder(r telemetry.StepRecorder) Option {
	return func(c *config) {
		if r == nil {
			c.recorder = telemetry.NopRecorder{}
			return
		}
		c.recorder = r
	}
}

// WithBalancer pins the partitioner. Default is kafka.LeastBytes (most
// even distribution under varied message sizes). Use kafka.Hash for
// key-based partitioning when ordering-per-key matters.
func WithBalancer(b kafka.Balancer) Option {
	return func(c *config) {
		if b != nil {
			c.balancer = b
		}
	}
}

// WithWriteTimeout caps how long Produce will wait for the broker to
// ack a batch. Default 10s.
func WithWriteTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.writeTimeout = d
		}
	}
}

// WithRequiredAcks sets the acks level: RequireOne (default — leader
// only), RequireAll (full ISR), or RequireNone (fire-and-forget).
func WithRequiredAcks(a kafka.RequiredAcks) Option {
	return func(c *config) { c.requiredAcks = a }
}

// WithAutoTopicCreation toggles AllowAutoTopicCreation on the writer.
// Convenient in CI scratch namespaces where the test creates the
// topic on first publish. Default false.
func WithAutoTopicCreation(enabled bool) Option {
	return func(c *config) { c.allowAutoTopic = enabled }
}

// WithPayloadCaptureBytes caps the body preview recorded in step
// Parameters. Default 1024. Set to 0 to disable payload capture
// (topic / partition / status / duration still recorded).
func WithPayloadCaptureBytes(n int) Option {
	return func(c *config) {
		if n < 0 {
			n = 0
		}
		c.payloadCap = n
	}
}

// atomicMap is the per-client mutex guarding writer creation. Defined
// here as a thin wrapper around sync.Mutex so client.go doesn't have
// to mention sync.Mutex inline (keeps the Client struct cohesive).
type atomicMap struct {
	mu sync.Mutex
}

func (a *atomicMap) do(fn func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	fn()
}
