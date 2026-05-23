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

	"github.com/mockarty/mockarty-go/protocols/rabbitmq"
)

// RabbitMQBroker is the minimal contract the Tester needs from a
// RabbitMQ client. `*rabbitmq.Client` from `protocols/rabbitmq`
// satisfies it; tests pass an in-memory fake.
type RabbitMQBroker interface {
	Publish(ctx context.Context, exchange, routingKey string, payload any, headers map[string]string) error
	Consume(ctx context.Context, opts rabbitmq.ConsumeOptions) ([]rabbitmq.ConsumedMessage, error)
}

// RabbitMQFacet is the RabbitMQ entry point reached via Tester.RabbitMQ(broker).
type RabbitMQFacet struct {
	t      *Tester
	broker RabbitMQBroker
}

// RabbitMQ returns the RabbitMQ facet bound to the supplied broker.
func (t *Tester) RabbitMQ(broker RabbitMQBroker) *RabbitMQFacet {
	t.flushPending()
	return &RabbitMQFacet{t: t, broker: broker}
}

// ── Publish ───────────────────────────────────────────────────────────

// Publish starts a RabbitMQ publish step. Exchange + routing key are
// {{var}}-interpolated. Example:
//
//	t.RabbitMQ(client).
//	  Publish("events", "user.updated").
//	  Header("trace", "{{trace}}").
//	  JSON(map[string]any{"id": 1}).
//	  ExpectOK()
func (r *RabbitMQFacet) Publish(exchange, routingKey string) *RabbitMQPublishStep {
	vars := r.t.snapshotVars()
	step := &RabbitMQPublishStep{
		t:          r.t,
		broker:     r.broker,
		exchange:   interpolate(exchange, vars),
		routingKey: interpolate(routingKey, vars),
		headers:    map[string]string{},
	}
	r.t.setPending(step)
	return step
}

// RabbitMQPublishStep is one publish call.
type RabbitMQPublishStep struct {
	t          *Tester
	broker     RabbitMQBroker
	exchange   string
	routingKey string
	headers    map[string]string
	payload    []byte

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	err        error
	failures   []string
}

// Header sets a message header, {{var}}-interpolated.
func (s *RabbitMQPublishStep) Header(k, v string) *RabbitMQPublishStep {
	if s.sent {
		s.fail("Header() called after send")
		return s
	}
	s.headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// JSON marshals v and uses it as the payload, {{var}}-interpolated.
func (s *RabbitMQPublishStep) JSON(v any) *RabbitMQPublishStep {
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

// Bytes sets a raw byte payload without interpolation.
func (s *RabbitMQPublishStep) Bytes(b []byte) *RabbitMQPublishStep {
	if s.sent {
		s.fail("Bytes() called after send")
		return s
	}
	s.payload = append([]byte(nil), b...)
	return s
}

// ExpectOK asserts the publish call succeeded.
func (s *RabbitMQPublishStep) ExpectOK() *RabbitMQPublishStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectOK: %v", s.err))
	}
	return s
}

// Done finalises the step.
func (s *RabbitMQPublishStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *RabbitMQPublishStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *RabbitMQPublishStep) ensureSent() bool {
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
	s.err = s.broker.Publish(ctx, s.exchange, s.routingKey, s.payload, s.headers)
	s.endedAt = time.Now()
	if s.err != nil {
		s.fail(fmt.Sprintf("publish: %v", s.err))
		s.abortChain = true
		return false
	}
	return true
}

func (s *RabbitMQPublishStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:  "rabbitmq",
		Method:    "publish",
		Name:      "publish " + s.exchange + "/" + s.routingKey,
		URL:       s.exchange + "/" + s.routingKey,
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}

// ── Consume ───────────────────────────────────────────────────────────

// Consume starts a RabbitMQ consume step.
func (r *RabbitMQFacet) Consume(queue string) *RabbitMQConsumeStep {
	r.t.flushPending()
	step := &RabbitMQConsumeStep{
		t:      r.t,
		broker: r.broker,
		opts: rabbitmq.ConsumeOptions{
			Queue:       interpolate(queue, r.t.snapshotVars()),
			MaxMessages: 1,
		},
	}
	r.t.setPending(step)
	return step
}

// RabbitMQConsumeStep is one consume call.
type RabbitMQConsumeStep struct {
	t      *Tester
	broker RabbitMQBroker
	opts   rabbitmq.ConsumeOptions

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	msgs       []rabbitmq.ConsumedMessage
	err        error
	failures   []string
}

// Max sets the maximum number of messages to fetch (default 1).
func (s *RabbitMQConsumeStep) Max(n int) *RabbitMQConsumeStep {
	if s.sent {
		s.fail("Max() called after send")
		return s
	}
	if n > 0 {
		s.opts.MaxMessages = n
	}
	return s
}

// AutoAck toggles server-side auto-acknowledgement. Default false:
// each consumed message is explicitly ack'd before Consume returns.
func (s *RabbitMQConsumeStep) AutoAck(b bool) *RabbitMQConsumeStep {
	if s.sent {
		s.fail("AutoAck() called after send")
		return s
	}
	s.opts.AutoAck = b
	return s
}

// ExpectCount asserts the consumer received exactly n messages.
func (s *RabbitMQConsumeStep) ExpectCount(n int) *RabbitMQConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.msgs) != n {
		s.fail(fmt.Sprintf("ExpectCount: want %d, got %d", n, len(s.msgs)))
	}
	return s
}

// ExpectAtLeast asserts at least n messages were received.
func (s *RabbitMQConsumeStep) ExpectAtLeast(n int) *RabbitMQConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.msgs) < n {
		s.fail(fmt.Sprintf("ExpectAtLeast: want >=%d, got %d", n, len(s.msgs)))
	}
	return s
}

// ExpectMessageContains asserts the idx-th message body contains sub.
func (s *RabbitMQConsumeStep) ExpectMessageContains(idx int, sub string) *RabbitMQConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if idx < 0 || idx >= len(s.msgs) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: index out of range (len=%d)", idx, len(s.msgs)))
		return s
	}
	if !strings.Contains(string(s.msgs[idx].Body), sub) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: %q not found", idx, sub))
	}
	return s
}

// ExpectHeader asserts the idx-th message carries header k with value v.
func (s *RabbitMQConsumeStep) ExpectHeader(idx int, k, v string) *RabbitMQConsumeStep {
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

// ExpectRoutingKey asserts the idx-th message's routing key.
func (s *RabbitMQConsumeStep) ExpectRoutingKey(idx int, want string) *RabbitMQConsumeStep {
	if !s.ensureSent() {
		return s
	}
	if idx < 0 || idx >= len(s.msgs) {
		s.fail(fmt.Sprintf("ExpectRoutingKey[%d]: index out of range (len=%d)", idx, len(s.msgs)))
		return s
	}
	if got := s.msgs[idx].RoutingKey; got != want {
		s.fail(fmt.Sprintf("ExpectRoutingKey[%d]: want %q, got %q", idx, want, got))
	}
	return s
}

// ExpectJSONPath asserts a JSONPath value inside the idx-th message body.
func (s *RabbitMQConsumeStep) ExpectJSONPath(idx int, path string, want any) *RabbitMQConsumeStep {
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

// Extract resolves a JSONPath and stores it in the var store.
func (s *RabbitMQConsumeStep) Extract(idx int, path, name string) *RabbitMQConsumeStep {
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

// Messages returns the consumed messages — escape hatch for custom
// assertion logic.
func (s *RabbitMQConsumeStep) Messages() []rabbitmq.ConsumedMessage {
	s.ensureSent()
	out := make([]rabbitmq.ConsumedMessage, len(s.msgs))
	copy(out, s.msgs)
	return out
}

// Done finalises the step.
func (s *RabbitMQConsumeStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *RabbitMQConsumeStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *RabbitMQConsumeStep) evalAt(idx int, path string) (any, error) {
	if idx < 0 || idx >= len(s.msgs) {
		return nil, fmt.Errorf("index out of range (len=%d)", len(s.msgs))
	}
	var doc any
	if err := json.Unmarshal(s.msgs[idx].Body, &doc); err != nil {
		return nil, fmt.Errorf("message[%d] is not JSON: %w", idx, err)
	}
	return resolveJSONPath(doc, path)
}

func (s *RabbitMQConsumeStep) ensureSent() bool {
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
	}
	return true
}

func (s *RabbitMQConsumeStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:     "rabbitmq",
		Method:       "consume",
		Name:         "consume " + s.opts.Queue,
		URL:          s.opts.Queue,
		StatusOrCode: len(s.msgs),
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
