// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/mockarty/mockarty-go/protocols/rabbitmq"
)

// fakeRabbit is an in-memory RabbitMQBroker keyed by routing key. Each
// publish lands in the matching queue (we treat queue == routing key
// for simplicity — real RabbitMQ has bindings, irrelevant here).
type fakeRabbit struct {
	mu         sync.Mutex
	queues     map[string][]rabbitmq.ConsumedMessage
	publishErr error
	consumeErr error
}

func newFakeRabbit() *fakeRabbit {
	return &fakeRabbit{queues: map[string][]rabbitmq.ConsumedMessage{}}
}

func (f *fakeRabbit) Publish(ctx context.Context, exchange, routingKey string, payload any, headers map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return f.publishErr
	}
	var body []byte
	switch v := payload.(type) {
	case nil:
		body = nil
	case []byte:
		body = append([]byte(nil), v...)
	case string:
		body = []byte(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		body = b
	}
	headersCopy := map[string]string{}
	for k, v := range headers {
		headersCopy[k] = v
	}
	f.queues[routingKey] = append(f.queues[routingKey], rabbitmq.ConsumedMessage{
		Exchange:    exchange,
		RoutingKey:  routingKey,
		Body:        body,
		Headers:     headersCopy,
		ContentType: "application/json",
	})
	return nil
}

func (f *fakeRabbit) Consume(ctx context.Context, opts rabbitmq.ConsumeOptions) ([]rabbitmq.ConsumedMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	all := f.queues[opts.Queue]
	n := opts.MaxMessages
	if n > len(all) {
		n = len(all)
	}
	out := append([]rabbitmq.ConsumedMessage(nil), all[:n]...)
	// In real RabbitMQ a non-AutoAck consume removes the messages from
	// the queue once Ack'd. Mirror that here so re-runs don't double-read.
	f.queues[opts.Queue] = all[n:]
	return out, nil
}

func TestRabbitMQPublishConsumeRoundTrip(t *testing.T) {
	b := newFakeRabbit()
	tt := New()

	tt.RabbitMQ(b).Publish("events", "user.updated").
		JSON(map[string]any{"id": 1, "status": "ok"}).
		ExpectOK()

	tt.RabbitMQ(b).Consume("user.updated").
		Max(5).
		ExpectCount(1).
		ExpectRoutingKey(0, "user.updated").
		ExpectMessageContains(0, "ok").
		ExpectJSONPath(0, "$.id", 1).
		Extract(0, "$.status", "lastStatus")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	if tt.Vars()["lastStatus"] != "ok" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
}

func TestRabbitMQPublishErrorPropagates(t *testing.T) {
	b := newFakeRabbit()
	b.publishErr = errors.New("connection refused")
	tt := New()
	tt.RabbitMQ(b).Publish("ex", "rk").JSON(map[string]any{"x": 1}).ExpectOK()
	if tt.OK() {
		t.Fatal("expected failure from publish error")
	}
}

func TestRabbitMQExpectAtLeast(t *testing.T) {
	b := newFakeRabbit()
	tt := New()
	for i := 0; i < 3; i++ {
		tt.RabbitMQ(b).Publish("ex", "q").JSON(map[string]any{"i": i}).ExpectOK()
	}
	tt.RabbitMQ(b).Consume("q").Max(10).ExpectAtLeast(2)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestRabbitMQConsumeAutoAckAndHeader(t *testing.T) {
	b := newFakeRabbit()
	tt := New()
	tt.RabbitMQ(b).Publish("ex", "q").
		Header("trace", "abc").
		JSON(map[string]any{"x": 1}).ExpectOK()

	tt.RabbitMQ(b).Consume("q").AutoAck(true).Max(1).
		ExpectCount(1).
		ExpectHeader(0, "trace", "abc")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestRabbitMQInterpolation(t *testing.T) {
	b := newFakeRabbit()
	tt := New()
	tt.SetVar("user", "alice")

	tt.RabbitMQ(b).Publish("ex-{{user}}", "rk-{{user}}").
		Header("X-User", "{{user}}").
		JSON(map[string]any{"name": "{{user}}"}).
		ExpectOK()

	tt.RabbitMQ(b).Consume("rk-alice").Max(1).
		ExpectCount(1).
		ExpectHeader(0, "X-User", "alice").
		ExpectJSONPath(0, "$.name", "alice")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestRabbitMQConsumeErrorRecorded(t *testing.T) {
	b := newFakeRabbit()
	b.consumeErr = errors.New("disconnected")
	tt := New()
	tt.RabbitMQ(b).Consume("q").Max(1).ExpectCount(0)
	tt.Finish()
	if tt.OK() {
		t.Fatal("consume error should produce a failure")
	}
}

func TestRabbitMQMessagesEscapeHatch(t *testing.T) {
	b := newFakeRabbit()
	tt := New()
	tt.RabbitMQ(b).Publish("ex", "q").Bytes([]byte("raw-1")).ExpectOK().Done()

	step := tt.RabbitMQ(b).Consume("q").Max(2)
	msgs := step.Messages()
	step.Done()
	if len(msgs) != 1 || string(msgs[0].Body) != "raw-1" {
		t.Fatalf("escape hatch wrong: %+v", msgs)
	}
	tt.Finish()
}

func TestRabbitMQIndexOutOfRange(t *testing.T) {
	b := newFakeRabbit()
	tt := New()
	tt.RabbitMQ(b).Consume("empty").Max(1).
		ExpectMessageContains(0, "x").
		ExpectHeader(0, "x", "y").
		ExpectRoutingKey(0, "z").
		ExpectJSONPath(0, "$.x", 1).
		Extract(0, "$.x", "v")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failures from out-of-range index")
	}
	if got := len(tt.Errors()); got < 5 {
		t.Fatalf("want >=5 errors, got %d: %v", got, tt.Errors())
	}
}
