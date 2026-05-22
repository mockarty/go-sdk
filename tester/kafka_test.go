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

	"github.com/mockarty/mockarty-go/protocols/kafka"
)

// fakeBroker is an in-memory KafkaBroker used by the test suite — no
// real Kafka cluster required. Maintains per-topic message lists and
// monotonic offsets.
type fakeBroker struct {
	mu       sync.Mutex
	topics   map[string][]kafka.ConsumedMessage
	produceN int
	consumeN int

	// produceErr / consumeErr let tests force error paths.
	produceErr error
	consumeErr error
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{topics: map[string][]kafka.ConsumedMessage{}}
}

func (f *fakeBroker) Produce(ctx context.Context, topic, key string, payload any, headers map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.produceN++
	if f.produceErr != nil {
		return f.produceErr
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
	offset := int64(len(f.topics[topic]))
	headersCopy := map[string]string{}
	for k, v := range headers {
		headersCopy[k] = v
	}
	f.topics[topic] = append(f.topics[topic], kafka.ConsumedMessage{
		Topic:   topic,
		Key:     key,
		Value:   body,
		Offset:  offset,
		Headers: headersCopy,
	})
	return nil
}

func (f *fakeBroker) Consume(ctx context.Context, opts kafka.ConsumeOptions) ([]kafka.ConsumedMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consumeN++
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	all := f.topics[opts.Topic]
	start := int(opts.StartOffset)
	if start < 0 || start > len(all) {
		start = 0
	}
	end := start + opts.MaxMessages
	if end > len(all) {
		end = len(all)
	}
	out := make([]kafka.ConsumedMessage, end-start)
	copy(out, all[start:end])
	return out, nil
}

func TestKafkaProduceConsumeRoundTrip(t *testing.T) {
	b := newFakeBroker()
	tt := New()

	tt.Kafka(b).Produce("orders", "user-42").
		JSON(map[string]any{"id": 1, "status": "created"}).
		ExpectOK()

	tt.Kafka(b).Consume("orders").
		Max(5).
		ExpectCount(1).
		ExpectFirstOffsetAtLeast(0).
		ExpectMessageContains(0, "created").
		ExpectJSONPath(0, "$.id", 1).
		Extract(0, "$.status", "lastStatus")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got errors: %v", tt.Errors())
	}
	if tt.Vars()["lastStatus"] != "created" {
		t.Fatalf("Extract did not store: %+v", tt.Vars())
	}
	if got := len(tt.Report()); got != 2 {
		t.Fatalf("want 2 steps, got %d", got)
	}
}

func TestKafkaProduceErrorPropagates(t *testing.T) {
	b := newFakeBroker()
	b.produceErr = errors.New("broker unreachable")

	tt := New()
	tt.Kafka(b).Produce("orders", "k").JSON(map[string]any{"x": 1}).ExpectOK()
	if tt.OK() {
		t.Fatal("expected failure from broker error")
	}
}

func TestKafkaConsumeCountMismatch(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Produce("orders", "k1").JSON(map[string]any{"i": 1}).ExpectOK()
	tt.Kafka(b).Produce("orders", "k2").JSON(map[string]any{"i": 2}).ExpectOK()

	tt.Kafka(b).Consume("orders").Max(5).ExpectCount(3)
	tt.Finish()

	if tt.OK() {
		t.Fatal("expected count mismatch failure")
	}
}

func TestKafkaConsumeHeader(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Produce("events", "k").
		Header("trace", "abc-123").
		JSON(map[string]any{"x": 1}).
		ExpectOK()

	tt.Kafka(b).Consume("events").Max(1).
		ExpectCount(1).
		ExpectHeader(0, "trace", "abc-123")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
}

func TestKafkaInterpolationAcrossChains(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.SetVar("user", "42")

	tt.Kafka(b).Produce("orders", "k-{{user}}").
		Header("X-User", "{{user}}").
		JSON(map[string]any{"userID": "{{user}}"}).
		ExpectOK()

	tt.Kafka(b).Consume("orders").Max(1).
		ExpectCount(1).
		ExpectHeader(0, "X-User", "42").
		ExpectJSONPath(0, "$.userID", "42")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	// Key should also be interpolated.
	msgs, _ := b.Consume(context.Background(), kafka.ConsumeOptions{Topic: "orders", MaxMessages: 1})
	if len(msgs) != 1 || msgs[0].Key != "k-42" {
		t.Fatalf("key interpolation failed: %+v", msgs)
	}
}

func TestKafkaStartOffsetSkipsHistory(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	for i := 0; i < 5; i++ {
		tt.Kafka(b).Produce("topic", "k").JSON(map[string]any{"i": i}).ExpectOK()
	}
	tt.Kafka(b).Consume("topic").
		StartOffset(3).
		Max(10).
		ExpectCount(2).
		ExpectFirstOffsetAtLeast(3)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
}

func TestKafkaConsumeErrorRecorded(t *testing.T) {
	b := newFakeBroker()
	b.consumeErr = errors.New("disconnected")
	tt := New()
	tt.Kafka(b).Consume("topic").Max(1).ExpectCount(0)
	tt.Finish()
	if tt.OK() {
		t.Fatal("consume error should produce a failure")
	}
}

func TestKafkaConsumerGroupAndDone(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Produce("topic", "k").JSON(map[string]any{"x": 1}).ExpectOK().Done()

	step := tt.Kafka(b).Consume("topic").
		Group("test-runner").
		StartOffset(0).
		Max(10).
		ExpectAtLeast(1)
	step.Done()

	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestKafkaIndexOutOfRange(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Consume("empty").Max(1).
		ExpectMessageContains(0, "anything").
		ExpectHeader(0, "x", "y").
		ExpectJSONPath(0, "$.a", 1).
		Extract(0, "$.a", "x")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failures from out-of-range index")
	}
	// Each Expect should have logged its own failure.
	if got := len(tt.Errors()); got < 4 {
		t.Fatalf("want >=4 errors, got %d: %v", got, tt.Errors())
	}
}

func TestKafkaConsumeNonJSONMessage(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Produce("topic", "k").Bytes([]byte("not-json")).ExpectOK()
	tt.Kafka(b).Consume("topic").Max(1).
		ExpectJSONPath(0, "$.x", 1) // should fail with parse error
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected JSON parse failure")
	}
}

func TestKafkaMessagesEscapeHatch(t *testing.T) {
	b := newFakeBroker()
	tt := New()
	tt.Kafka(b).Produce("topic", "k1").Bytes([]byte("raw-1")).ExpectOK()
	tt.Kafka(b).Produce("topic", "k2").Bytes([]byte("raw-2")).ExpectOK()

	consume := tt.Kafka(b).Consume("topic").Max(2)
	msgs := consume.Messages()
	if len(msgs) != 2 {
		t.Fatalf("want 2 raw messages, got %d", len(msgs))
	}
	if string(msgs[0].Value) != "raw-1" || string(msgs[1].Value) != "raw-2" {
		t.Fatalf("unexpected payloads: %q / %q", msgs[0].Value, msgs[1].Value)
	}
	tt.Finish()
}
