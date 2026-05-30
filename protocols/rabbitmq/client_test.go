// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

func TestNewClient_EmptyURL(t *testing.T) {
	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestNewClient_DefaultsApplied(t *testing.T) {
	c, err := NewClient("amqp://localhost:5672/")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	if c.cfg.recorder == nil {
		t.Fatal("default recorder must be non-nil")
	}
	if c.cfg.payloadCap <= 0 {
		t.Fatalf("payloadCap = %v, must be positive", c.cfg.payloadCap)
	}
}

func TestOptions_Coverage(t *testing.T) {
	rec := &countingRecorder{}
	c, err := NewClient("amqp://x/",
		WithRecorder(rec),
		WithRecorder(nil),
		WithPayloadCaptureBytes(-1),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer c.Close()
	if _, ok := c.cfg.recorder.(telemetry.NopRecorder); !ok {
		t.Fatalf("recorder after nil-option = %T, want NopRecorder", c.cfg.recorder)
	}
	if c.cfg.payloadCap != 0 {
		t.Fatalf("payloadCap = %d, want 0 (clamped)", c.cfg.payloadCap)
	}
}

func TestMarshalPayload_AllShapes(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{[]byte("raw"), "raw"},
		{"str", "str"},
		{json.RawMessage(`{"x":1}`), `{"x":1}`},
		{map[string]any{"k": "v"}, `{"k":"v"}`},
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
	_, err := marshalPayload(make(chan int))
	if err == nil {
		t.Fatal("expected error marshalling chan")
	}
}

func TestClassify(t *testing.T) {
	amqpErr := &amqp.Error{Code: 403, Reason: "access-refused", Server: true}
	cases := []struct {
		err  error
		want string
	}{
		{nil, "passed"},
		{context.Canceled, "broken"},
		{context.DeadlineExceeded, "broken"},
		{amqpErr, "failed"},
		{errors.New("read tcp 127.0.0.1:5672: connection reset"), "broken"},
	}
	for _, tc := range cases {
		got := classify(tc.err)
		if got != tc.want {
			t.Fatalf("classify(%T %v) = %q, want %q", tc.err, tc.err, got, tc.want)
		}
	}
}

func TestCapPreview_Boundaries(t *testing.T) {
	body := []byte(strings.Repeat("x", 200))
	if got := capPreview(body, 0); got != "" {
		t.Fatalf("cap=0 should return empty")
	}
	if got := capPreview(body, 500); got != string(body) {
		t.Fatalf("cap larger than body should return whole body")
	}
	got := capPreview(body, 50)
	if !strings.HasPrefix(got, strings.Repeat("x", 50)) || !strings.Contains(got, "truncated 150B") {
		t.Fatalf("truncation marker missing: %q", got)
	}
}

func TestPublish_NilClient(t *testing.T) {
	var c *Client
	if err := c.Publish(context.Background(), "ex", "rk", "x", nil); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestConsume_EmptyQueueRejected(t *testing.T) {
	c, _ := NewClient("amqp://x/")
	defer c.Close()
	_, err := c.Consume(context.Background(), ConsumeOptions{})
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
}

func TestDeclareQueue_EmptyName(t *testing.T) {
	c, _ := NewClient("amqp://x/")
	defer c.Close()
	if err := c.DeclareQueue(context.Background(), "", DeclareQueueOptions{}); err == nil {
		t.Fatal("expected error for empty queue name")
	}
}

func TestClose_Idempotent(t *testing.T) {
	c, _ := NewClient("amqp://x/")
	_ = c.Close()
	_ = c.Close()
}

// --- helpers ---

type countingRecorder struct {
	steps []telemetry.Step
}

func (r *countingRecorder) Record(_ context.Context, s telemetry.Step) {
	r.steps = append(r.steps, s)
}
