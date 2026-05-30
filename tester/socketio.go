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

	"github.com/mockarty/mockarty-go/protocols/socketio"
)

// SocketIOFacet is the Socket.IO entry point reached via
// Tester.SocketIO(url). Distinct from WebSocket — it speaks the
// Engine.IO/Socket.IO framing (handshake, namespace connect, named
// events) rather than raw frames.
type SocketIOFacet struct {
	t   *Tester
	url string
}

// SocketIO returns the Socket.IO facet bound to a ws://, wss://, http://
// or https:// URL. The /socket.io/ path and EIO=4&transport=websocket
// query are appended automatically when absent.
//
//	t.SocketIO("http://localhost:18770").
//	  Connect().
//	  Emit("greet", "World").
//	  Collect(2 * time.Second).
//	  ExpectConnected().
//	  ExpectEvent("greeting").
//	  ExpectEventJSONPath("greeting", "$.msg", "hello World")
func (t *Tester) SocketIO(url string) *SocketIOFacet {
	t.flushPending()
	return &SocketIOFacet{t: t, url: url}
}

// Connect starts a Socket.IO chain. The actual dial + namespace connect
// fires lazily on the first Collect / Expect / Extract (or on commit).
func (f *SocketIOFacet) Connect() *SocketIOStep {
	f.t.flushPending()
	step := &SocketIOStep{
		t:         f.t,
		url:       interpolate(f.url, f.t.snapshotVars()),
		namespace: "/",
		window:    3 * time.Second,
		connWait:  3 * time.Second,
	}
	f.t.setPending(step)
	return step
}

// SocketIOStep is one Socket.IO interaction window.
type SocketIOStep struct {
	t         *Tester
	url       string
	namespace string
	window    time.Duration
	connWait  time.Duration

	headers  [][2]string
	outbound []sioEmit

	client     *socketio.Client
	received   []socketio.Event
	connectErr error
	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	failures   []string
}

type sioEmit struct {
	event string
	args  []any
}

// Namespace sets the Socket.IO namespace to connect (default "/").
func (s *SocketIOStep) Namespace(ns string) *SocketIOStep {
	if s.guardSent("Namespace") {
		return s
	}
	s.namespace = interpolate(ns, s.t.snapshotVars())
	return s
}

// Header sets a handshake header (e.g. Authorization), {{var}}-interpolated.
func (s *SocketIOStep) Header(k, v string) *SocketIOStep {
	if s.guardSent("Header") {
		return s
	}
	s.headers = append(s.headers, [2]string{k, interpolate(v, s.t.snapshotVars())})
	return s
}

// ConnectTimeout overrides the namespace-connect wait (default 3s).
func (s *SocketIOStep) ConnectTimeout(d time.Duration) *SocketIOStep {
	if s.guardSent("ConnectTimeout") {
		return s
	}
	if d > 0 {
		s.connWait = d
	}
	return s
}

// Emit queues an event to emit after connect, before the collect window.
// String args are {{var}}-interpolated; other args pass through.
func (s *SocketIOStep) Emit(event string, args ...any) *SocketIOStep {
	if s.guardSent("Emit") {
		return s
	}
	vars := s.t.snapshotVars()
	interpolated := make([]any, len(args))
	for i, a := range args {
		if str, ok := a.(string); ok {
			interpolated[i] = interpolate(str, vars)
		} else {
			interpolated[i] = a
		}
	}
	s.outbound = append(s.outbound, sioEmit{event: interpolate(event, vars), args: interpolated})
	return s
}

// Collect sets the receive window (default 3s). Calling it triggers the
// dial/connect/emit/collect sequence.
func (s *SocketIOStep) Collect(d time.Duration) *SocketIOStep {
	if !s.sent && d > 0 {
		s.window = d
	}
	s.ensureSent()
	return s
}

// ── assertions ────────────────────────────────────────────────────────────

// ExpectConnected asserts the handshake + namespace connect succeeded.
func (s *SocketIOStep) ExpectConnected() *SocketIOStep {
	s.ensureSent()
	if s.connectErr != nil {
		s.fail("ExpectConnected: " + s.connectErr.Error())
	}
	return s
}

// ExpectEvent asserts an event with the given name was received.
func (s *SocketIOStep) ExpectEvent(name string) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	if s.findEvent(name) < 0 {
		s.fail(fmt.Sprintf("ExpectEvent: %q not received", name))
	}
	return s
}

// ExpectEventCount asserts exactly n events with the given name arrived.
func (s *SocketIOStep) ExpectEventCount(name string, n int) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	count := 0
	for _, e := range s.received {
		if e.Name == name {
			count++
		}
	}
	if count != n {
		s.fail(fmt.Sprintf("ExpectEventCount[%s]: want %d, got %d", name, n, count))
	}
	return s
}

// ExpectReceivedCount asserts exactly n events (any name) arrived.
func (s *SocketIOStep) ExpectReceivedCount(n int) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.received) != n {
		s.fail(fmt.Sprintf("ExpectReceivedCount: want %d, got %d", n, len(s.received)))
	}
	return s
}

// ExpectEventArgContains asserts the first argument of the first event
// named name (as raw JSON text) contains sub.
func (s *SocketIOStep) ExpectEventArgContains(name, sub string) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	idx := s.findEvent(name)
	if idx < 0 {
		s.fail(fmt.Sprintf("ExpectEventArgContains: %q not received", name))
		return s
	}
	if len(s.received[idx].Args) == 0 {
		s.fail(fmt.Sprintf("ExpectEventArgContains[%s]: event has no args", name))
		return s
	}
	if !strings.Contains(string(s.received[idx].Args[0]), sub) {
		s.fail(fmt.Sprintf("ExpectEventArgContains[%s]: %q not found", name, sub))
	}
	return s
}

// ExpectEventJSONPath asserts a JSONPath value inside the first argument
// of the first event named name.
func (s *SocketIOStep) ExpectEventJSONPath(name, path string, want any) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalEventArg(name, path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectEventJSONPath[%s] %s: %v", name, path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectEventJSONPath[%s] %s: want %v, got %v", name, path, want, got))
	}
	return s
}

// Extract resolves a JSONPath in the first argument of the named event
// and stores it under varName.
func (s *SocketIOStep) Extract(name, path, varName string) *SocketIOStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalEventArg(name, path)
	if err != nil {
		s.fail(fmt.Sprintf("Extract[%s] %s: %v", name, path, err))
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
	s.t.SetVar(varName, str)
	return s
}

// Received returns a copy of the collected events (escape hatch).
func (s *SocketIOStep) Received() []socketio.Event {
	s.ensureSent()
	out := make([]socketio.Event, len(s.received))
	copy(out, s.received)
	return out
}

// Done finalises the step (closes the connection).
func (s *SocketIOStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

// ── internals ───────────────────────────────────────────────────────────

func (s *SocketIOStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *SocketIOStep) guardSent(method string) bool {
	if s.sent {
		s.fail(method + "() called after connect")
		return true
	}
	return false
}

func (s *SocketIOStep) findEvent(name string) int {
	for i, e := range s.received {
		if e.Name == name {
			return i
		}
	}
	return -1
}

func (s *SocketIOStep) evalEventArg(name, path string) (any, error) {
	idx := s.findEvent(name)
	if idx < 0 {
		return nil, fmt.Errorf("event %q not received", name)
	}
	if len(s.received[idx].Args) == 0 {
		return nil, fmt.Errorf("event %q has no args", name)
	}
	var doc any
	if err := json.Unmarshal(s.received[idx].Args[0], &doc); err != nil {
		return nil, fmt.Errorf("event %q arg is not JSON: %w", name, err)
	}
	return resolveJSONPath(doc, path)
}

func (s *SocketIOStep) ensureSent() bool {
	if s.sent {
		return s.connectErr == nil && !s.abortChain
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

	opts := []socketio.Option{}
	for _, h := range s.headers {
		opts = append(opts, socketio.WithHeader(h[0], h[1]))
	}
	client, err := socketio.Dial(ctx, s.url, opts...)
	if err != nil {
		s.connectErr = err
		s.fail("socket.io dial: " + err.Error())
		s.abortChain = true
		s.endedAt = time.Now()
		return false
	}
	s.client = client

	if cerr := client.Connect(s.namespace, s.connWait); cerr != nil {
		s.connectErr = cerr
		s.fail("socket.io connect: " + cerr.Error())
		_ = client.Close()
		s.abortChain = true
		s.endedAt = time.Now()
		return false
	}

	for _, e := range s.outbound {
		if eerr := client.Emit(s.namespace, e.event, e.args...); eerr != nil {
			s.fail("socket.io emit: " + eerr.Error())
		}
	}

	events, _ := client.Collect(s.window)
	s.received = events
	_ = client.Close()
	s.endedAt = time.Now()
	return true
}

func (s *SocketIOStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:     "socketio",
		Method:       "event",
		Name:         "socket.io " + s.url,
		URL:          s.url,
		StatusOrCode: len(s.received),
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
