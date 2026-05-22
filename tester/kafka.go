// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mockarty/mockarty-go/allure"
	"github.com/mockarty/mockarty-go/protocols/kafka"
)

// KafkaBroker is the minimal contract the Tester needs from a Kafka
// client. `*kafka.Client` from `protocols/kafka` satisfies it directly;
// tests pass an in-memory fake instead of standing up a real broker.
type KafkaBroker interface {
	Produce(ctx context.Context, topic, key string, payload any, headers map[string]string) error
	Consume(ctx context.Context, opts kafka.ConsumeOptions) ([]kafka.ConsumedMessage, error)
}

// KafkaFacet is the Kafka entry point reached via Tester.Kafka(broker).
type KafkaFacet struct {
	t      *Tester
	broker KafkaBroker
}

// Kafka returns the Kafka facet bound to the supplied broker. The broker
// is typically a `*kafka.Client` from `protocols/kafka`; pass any
// implementation of KafkaBroker for testing.
func (t *Tester) Kafka(broker KafkaBroker) *KafkaFacet {
	t.flushPending()
	return &KafkaFacet{t: t, broker: broker}
}

// ── Produce ───────────────────────────────────────────────────────────

// Produce starts a Kafka produce step. The chain reads:
//
//	t.Kafka(client).Produce("orders", "user-42").
//	  Header("trace", "{{trace}}").
//	  JSON(map[string]any{"id": 1}).
//	  ExpectOK()
func (k *KafkaFacet) Produce(topic, key string) *KafkaProduceStep {
	vars := k.t.snapshotVars()
	step := &KafkaProduceStep{
		t:       k.t,
		broker:  k.broker,
		topic:   interpolate(topic, vars),
		key:     interpolate(key, vars),
		headers: map[string]string{},
	}
	k.t.setPending(step)
	return step
}

// KafkaProduceStep is one Kafka publish call.
type KafkaProduceStep struct {
	t       *Tester
	broker  KafkaBroker
	topic   string
	key     string
	headers map[string]string
	payload []byte

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	err        error
	failures   []string
}

// Header sets a message header, {{var}}-interpolated.
func (s *KafkaProduceStep) Header(k, v string) *KafkaProduceStep {
	if s.sent {
		s.fail("Header() called after send")
		return s
	}
	s.headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// JSON marshals v and uses it as the message payload, {{var}}-interpolated.
func (s *KafkaProduceStep) JSON(v any) *KafkaProduceStep {
	if s.sent {
		s.fail("JSON() called after send")
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		s.fail(fmt.Sprintf("marshal payload: %v", err))
		s.abortChain = true
		return s
	}
	s.payload = []byte(interpolate(string(b), s.t.snapshotVars()))
	return s
}

// Bytes sets a raw byte payload without interpolation. Use when the
// payload is binary (protobuf / avro / etc).
func (s *KafkaProduceStep) Bytes(b []byte) *KafkaProduceStep {
	if s.sent {
		s.fail("Bytes() called after send")
		return s
	}
	s.payload = append([]byte(nil), b...)
	return s
}

// ExpectOK asserts the produce call succeeded. Most callers reach this
// at the end of a Produce chain — without an ExpectOK the produce still
// fires (on commit) but its outcome is only visible via Tester.Errors.
func (s *KafkaProduceStep) ExpectOK() *KafkaProduceStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectOK: %v", s.err))
	}
	return s
}

// Done finalises the step. Most callers don't need this — next chain
// start or Tester.Finish auto-commits.
func (s *KafkaProduceStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *KafkaProduceStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *KafkaProduceStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}
	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	s.startedAt = time.Now()
	s.err = s.broker.Produce(ctx, s.topic, s.key, s.payload, s.headers)
	s.endedAt = time.Now()
	if s.err != nil {
		s.fail(fmt.Sprintf("produce: %v", s.err))
		s.abortChain = true
		return false
	}
	return true
}

func (s *KafkaProduceStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:  "kafka",
		Method:    "produce",
		Name:      "produce " + s.topic,
		URL:       s.topic,
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}

// ── Consume ───────────────────────────────────────────────────────────

// Consume starts a Kafka consume step. The chain reads:
//
//	t.Kafka(client).Consume("orders").
//	  Group("test-runner").
//	  Max(5).
//	  ExpectCount(5).
//	  ExpectFirstOffsetAtLeast(0).
//	  ExpectMessageContains(0, "created").
//	  Extract(0, "$.id", "orderID")
func (k *KafkaFacet) Consume(topic string) *KafkaConsumeStep {
	k.t.flushPending()
	step := &KafkaConsumeStep{
		t:      k.t,
		broker: k.broker,
		opts: kafka.ConsumeOptions{
			Topic:       interpolate(topic, k.t.snapshotVars()),
			MaxMessages: 1,
		},
	}
	k.t.setPending(step)
	return step
}

// KafkaConsumeStep is one Kafka consume call.
type KafkaConsumeStep struct {
	t      *Tester
	broker KafkaBroker
	opts   kafka.ConsumeOptions

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	msgs       []kafka.ConsumedMessage
	err        error
	failures   []string
}

// Group sets the consumer group ID. Empty (default) = anonymous consumer
// starting at opts.StartOffset.
func (s *KafkaConsumeStep) Group(g string) *KafkaConsumeStep {
	if s.sent {
		s.fail("Group() called after send")
		return s
	}
	s.opts.GroupID = g
	return s
}

// Max sets the maximum number of messages to fetch. Default 1.
func (s *KafkaConsumeStep) Max(n int) *KafkaConsumeStep {
	if s.sent {
		s.fail("Max() called after send")
		return s
	}
	if n > 0 {
		s.opts.MaxMessages = n
	}
	return s
}

// StartOffset sets the initial offset (kafka.FirstOffset / kafka.LastOffset
// from segmentio/kafka-go, or a specific positive value).
func (s *KafkaConsumeStep) StartOffset(o int64) *KafkaConsumeStep {
	if s.sent {
		s.fail("StartOffset() called after send")
		return s
	}
	s.opts.StartOffset = o
	return s
}

// ExpectCount asserts the consumer received exactly n messages.
func (s *KafkaConsumeStep) ExpectCount(n int) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.msgs) != n {
		s.fail(fmt.Sprintf("ExpectCount: want %d, got %d", n, len(s.msgs)))
	}
	return s
}

// ExpectAtLeast asserts the consumer received at least n messages.
func (s *KafkaConsumeStep) ExpectAtLeast(n int) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.msgs) < n {
		s.fail(fmt.Sprintf("ExpectAtLeast: want >=%d, got %d", n, len(s.msgs)))
	}
	return s
}

// ExpectFirstOffsetAtLeast asserts the first consumed message's offset is
// at least o — useful when seeking past historical retention markers.
func (s *KafkaConsumeStep) ExpectFirstOffsetAtLeast(o int64) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.msgs) == 0 {
		s.fail("ExpectFirstOffsetAtLeast: no messages")
		return s
	}
	if s.msgs[0].Offset < o {
		s.fail(fmt.Sprintf("ExpectFirstOffsetAtLeast: want >=%d, got %d", o, s.msgs[0].Offset))
	}
	return s
}

// ExpectMessageContains asserts that the idx-th message's payload contains sub.
func (s *KafkaConsumeStep) ExpectMessageContains(idx int, sub string) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if idx < 0 || idx >= len(s.msgs) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: index out of range (len=%d)", idx, len(s.msgs)))
		return s
	}
	if !strings.Contains(string(s.msgs[idx].Value), sub) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: %q not found", idx, sub))
	}
	return s
}

// ExpectJSONPath asserts a JSONPath value inside the idx-th message body.
func (s *KafkaConsumeStep) ExpectJSONPath(idx int, path string, want any) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalAt(idx, path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectJSONPath[%d] %s: %v", idx, path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectJSONPath[%d] %s: want %v, got %v", idx, path, want, got))
	}
	return s
}

// ExpectHeader asserts the idx-th message carries header k with value v.
func (s *KafkaConsumeStep) ExpectHeader(idx int, k, v string) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if idx < 0 || idx >= len(s.msgs) {
		s.fail(fmt.Sprintf("ExpectHeader[%d]: index out of range (len=%d)", idx, len(s.msgs)))
		return s
	}
	if got := s.msgs[idx].Headers[k]; got != v {
		s.fail(fmt.Sprintf("ExpectHeader[%d] %s: want %q, got %q", idx, k, v, got))
	}
	return s
}

// Extract resolves a JSONPath in the idx-th message body and stores it
// under name for use in subsequent {{name}} substitutions.
func (s *KafkaConsumeStep) Extract(idx int, path, name string) *KafkaConsumeStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalAt(idx, path)
	if err != nil {
		s.fail(fmt.Sprintf("Extract[%d] %s: %v", idx, path, err))
		return s
	}
	var str string
	switch v := got.(type) {
	case string:
		str = v
	case float64:
		str = formatNumber(v)
	case bool:
		str = fmt.Sprintf("%t", v)
	case nil:
		str = ""
	default:
		b, _ := json.Marshal(v)
		str = string(b)
	}
	s.t.SetVar(name, str)
	return s
}

// Messages returns the consumed messages (after the step has fired) so
// callers can drop down to their own assertion logic when the built-in
// vocabulary isn't enough.
func (s *KafkaConsumeStep) Messages() []kafka.ConsumedMessage {
	s.ensureSent()
	out := make([]kafka.ConsumedMessage, len(s.msgs))
	copy(out, s.msgs)
	return out
}

// Done finalises the step explicitly.
func (s *KafkaConsumeStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *KafkaConsumeStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *KafkaConsumeStep) evalAt(idx int, path string) (any, error) {
	if idx < 0 || idx >= len(s.msgs) {
		return nil, fmt.Errorf("index out of range (len=%d)", len(s.msgs))
	}
	var doc any
	if err := json.Unmarshal(s.msgs[idx].Value, &doc); err != nil {
		return nil, fmt.Errorf("message[%d] is not JSON: %w", idx, err)
	}
	return resolveJSONPath(doc, path)
}

func (s *KafkaConsumeStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}
	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	s.startedAt = time.Now()
	s.msgs, s.err = s.broker.Consume(ctx, s.opts)
	s.endedAt = time.Now()
	if s.err != nil {
		s.fail(fmt.Sprintf("consume: %v", s.err))
		// Partial reads are still recorded so assertions on what DID
		// arrive can fire — keep abortChain off.
	}
	return true
}

func (s *KafkaConsumeStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:     "kafka",
		Method:       "consume",
		Name:         "consume " + s.opts.Topic,
		URL:          s.opts.Topic,
		StatusOrCode: len(s.msgs),
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}

// emitAllureStep is the shared Allure emitter for protocol facets.
func emitAllureStep(ctx context.Context, rec StepRecord) {
	handle := allure.BeginStep(ctx, rec.Name)
	if len(rec.Failures) == 0 {
		handle.End()
		return
	}
	handle.Fail(strings.Join(rec.Failures, "; "))
}
