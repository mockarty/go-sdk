// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

func TestNewClient_EmptyBrokers(t *testing.T) {
	_, err := NewClient(nil)
	if err == nil {
		t.Fatal("expected error for empty brokers")
	}
}

func TestNewClient_DefaultsApplied(t *testing.T) {
	c, err := NewClient([]string{"localhost:9092"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	if c.cfg.recorder == nil {
		t.Fatal("default recorder must be non-nil")
	}
	if c.cfg.balancer == nil {
		t.Fatal("default balancer must be non-nil")
	}
	if c.cfg.requiredAcks != kafka.RequireOne {
		t.Fatalf("requiredAcks = %v, want RequireOne", c.cfg.requiredAcks)
	}
	if c.cfg.writeTimeout <= 0 {
		t.Fatalf("writeTimeout = %v, must be positive", c.cfg.writeTimeout)
	}
	if c.cfg.payloadCap <= 0 {
		t.Fatalf("payloadCap = %v, must be positive", c.cfg.payloadCap)
	}
}

func TestOptions_Coverage(t *testing.T) {
	rec := &countingRecorder{}
	c, err := NewClient([]string{"b:9092"},
		WithRecorder(rec),
		WithRecorder(nil), // must coerce to NopRecorder, not nil pointer
		WithBalancer(&kafka.Hash{}),
		WithBalancer(nil), // must keep previous
		WithWriteTimeout(5*time.Second),
		WithWriteTimeout(0), // must ignore zero
		WithRequiredAcks(kafka.RequireAll),
		WithAutoTopicCreation(true),
		WithPayloadCaptureBytes(-1), // must clamp to 0
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	// recorder: WithRecorder(nil) should have coerced to NopRecorder
	if _, ok := c.cfg.recorder.(telemetry.NopRecorder); !ok {
		t.Fatalf("recorder after nil-option = %T, want NopRecorder", c.cfg.recorder)
	}
	// Balancer survived the nil call.
	if _, ok := c.cfg.balancer.(*kafka.Hash); !ok {
		t.Fatalf("balancer = %T, want *kafka.Hash", c.cfg.balancer)
	}
	if c.cfg.writeTimeout != 5*time.Second {
		t.Fatalf("writeTimeout = %v, want 5s", c.cfg.writeTimeout)
	}
	if c.cfg.requiredAcks != kafka.RequireAll {
		t.Fatalf("requiredAcks = %v, want RequireAll", c.cfg.requiredAcks)
	}
	if !c.cfg.allowAutoTopic {
		t.Fatal("allowAutoTopic must be true")
	}
	if c.cfg.payloadCap != 0 {
		t.Fatalf("payloadCap = %d, want 0 (negative clamped)", c.cfg.payloadCap)
	}
}

func TestMarshalPayload_AllShapes(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{[]byte("raw bytes"), "raw bytes"},
		{"string body", "string body"},
		{json.RawMessage(`{"a":1}`), `{"a":1}`},
		{map[string]any{"k": "v"}, `{"k":"v"}`},
		{struct {
			A int `json:"a"`
		}{A: 7}, `{"a":7}`},
	}
	for _, tc := range cases {
		got, err := marshalPayload(tc.in)
		if err != nil {
			t.Fatalf("marshalPayload(%T): %v", tc.in, err)
		}
		if string(got) != tc.want {
			t.Fatalf("marshalPayload(%T) = %q, want %q", tc.in, string(got), tc.want)
		}
	}
}

func TestMarshalPayload_UnmarshalableValue(t *testing.T) {
	// channel can't be JSON-marshalled — confirms error path.
	_, err := marshalPayload(make(chan int))
	if err == nil {
		t.Fatal("expected error marshalling chan")
	}
	if !strings.Contains(err.Error(), "marshal payload") {
		t.Fatalf("error message lacks context: %v", err)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "passed"},
		{context.Canceled, "broken"},
		{context.DeadlineExceeded, "broken"},
		{errors.New("auth: rejected"), "failed"},
	}
	for _, tc := range cases {
		got := classify(tc.err)
		if got != tc.want {
			t.Fatalf("classify(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestCapPreview_Boundaries(t *testing.T) {
	body := []byte(strings.Repeat("a", 100))
	if got := capPreview(body, 0); got != "" {
		t.Fatalf("cap=0 should return empty, got %q", got)
	}
	if got := capPreview(body, 200); got != strings.Repeat("a", 100) {
		t.Fatalf("cap larger than body returned wrong slice")
	}
	got := capPreview(body, 10)
	if !strings.HasPrefix(got, strings.Repeat("a", 10)) || !strings.Contains(got, "truncated 90B") {
		t.Fatalf("truncation marker missing: %q", got)
	}
}

func TestProduce_EmptyTopicRejected(t *testing.T) {
	c, _ := NewClient([]string{"localhost:9092"})
	defer c.Close()
	if err := c.Produce(context.Background(), "", "k", "v", nil); err == nil {
		t.Fatal("expected error for empty topic")
	}
}

func TestProduce_MarshalErrorRecordsFailedStep(t *testing.T) {
	rec := &countingRecorder{}
	c, _ := NewClient([]string{"localhost:9092"}, WithRecorder(rec))
	defer c.Close()
	err := c.Produce(context.Background(), "t", "k", make(chan int), nil)
	if err == nil {
		t.Fatal("expected marshal error to propagate")
	}
	if rec.count() != 1 {
		t.Fatalf("recorder saw %d steps, want 1", rec.count())
	}
	got := rec.last()
	if got.Status != "failed" {
		t.Fatalf("step.Status = %q, want failed", got.Status)
	}
	if !strings.Contains(got.Message, "marshal payload") {
		t.Fatalf("step.Message = %q, want context", got.Message)
	}
}

func TestConsume_EmptyTopicRejected(t *testing.T) {
	c, _ := NewClient([]string{"localhost:9092"})
	defer c.Close()
	_, err := c.Consume(context.Background(), ConsumeOptions{})
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
}

func TestClose_Idempotent(t *testing.T) {
	c, _ := NewClient([]string{"localhost:9092"})
	if err := c.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	// Second close should not panic / return inconsistent error.
	_ = c.Close()
}

// --- helpers ---

type countingRecorder struct {
	steps []telemetry.Step
}

func (r *countingRecorder) Record(_ context.Context, s telemetry.Step) {
	r.steps = append(r.steps, s)
}
func (r *countingRecorder) count() int           { return len(r.steps) }
func (r *countingRecorder) last() telemetry.Step { return r.steps[len(r.steps)-1] }
