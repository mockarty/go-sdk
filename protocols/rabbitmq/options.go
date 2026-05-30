// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package rabbitmq

import "github.com/mockarty/mockarty-go/protocols/telemetry"

// Option mutates dial-time configuration.
type Option func(*config)

type config struct {
	recorder   telemetry.StepRecorder
	payloadCap int
}

func defaultConfig() *config {
	return &config{
		recorder:   telemetry.NopRecorder{},
		payloadCap: 1024,
	}
}

// WithRecorder wires a step recorder. Nil = drop (default).
//
//	rec := telemetry.NewExternalRunsRecorder(runs, runID)
//	cli, _ := rabbitmq.NewClient("amqp://…", rabbitmq.WithRecorder(rec))
func WithRecorder(r telemetry.StepRecorder) Option {
	return func(c *config) {
		if r == nil {
			c.recorder = telemetry.NopRecorder{}
			return
		}
		c.recorder = r
	}
}

// WithPayloadCaptureBytes caps the body preview recorded in step
// Parameters. Default 1024. Set to 0 to disable payload capture.
func WithPayloadCaptureBytes(n int) Option {
	return func(c *config) {
		if n < 0 {
			n = 0
		}
		c.payloadCap = n
	}
}
