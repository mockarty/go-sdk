// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// WSMessage is one frame received from the WebSocket server. Type is
// websocket.TextMessage or BinaryMessage; Data is the raw payload.
type WSMessage struct {
	Type int
	Data []byte
}

// WebSocketFacet is the WS entry point reached via Tester.WebSocket(url).
type WebSocketFacet struct {
	t   *Tester
	url string
}

// WebSocket returns the WebSocket facet bound to a ws://... or wss://...
// URL. If the URL is relative and Tester.BaseURL is http(s)://, the
// scheme is auto-rewritten (http→ws, https→wss).
func (t *Tester) WebSocket(url string) *WebSocketFacet {
	t.flushPending()
	return &WebSocketFacet{t: t, url: url}
}

// Connect starts a WebSocket chain. The actual dial fires lazily on
// the first Send / Listen / Expect / Extract.
func (w *WebSocketFacet) Connect() *WSStep {
	w.t.flushPending()
	step := &WSStep{
		t:       w.t,
		url:     interpolate(w.url, w.t.snapshotVars()),
		headers: http.Header{},
		listen:  5 * time.Second,
	}
	w.t.setPending(step)
	return step
}

// WSStep is one WebSocket interaction window.
type WSStep struct {
	t       *Tester
	url     string
	headers http.Header
	listen  time.Duration

	// Queued outbound frames — sent after dial, before the listen window.
	outbound []wsOutbound

	conn       *websocket.Conn
	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	received   []WSMessage
	dialErr    error
	failures   []string
}

type wsOutbound struct {
	kind    int // websocket.TextMessage | BinaryMessage
	payload []byte
}

// Header sets a handshake header (Authorization etc), {{var}}-interpolated.
func (s *WSStep) Header(k, v string) *WSStep {
	if s.sent {
		s.fail("Header() called after dial")
		return s
	}
	s.headers.Set(k, interpolate(v, s.t.snapshotVars()))
	return s
}

// Listen sets the receive window (default 5s).
func (s *WSStep) Listen(d time.Duration) *WSStep {
	if s.sent {
		s.fail("Listen() called after dial")
		return s
	}
	if d > 0 {
		s.listen = d
	}
	return s
}

// Send queues a text frame to be sent after the dial completes,
// {{var}}-interpolated.
func (s *WSStep) Send(text string) *WSStep {
	if s.sent {
		s.fail("Send() called after dial")
		return s
	}
	s.outbound = append(s.outbound, wsOutbound{
		kind:    websocket.TextMessage,
		payload: []byte(interpolate(text, s.t.snapshotVars())),
	})
	return s
}

// SendJSON marshals v and queues it as a text frame, {{var}}-interpolated.
func (s *WSStep) SendJSON(v any) *WSStep {
	if s.sent {
		s.fail("SendJSON() called after dial")
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		s.fail(fmt.Sprintf("marshal payload: %v", err))
		s.abortChain = true
		return s
	}
	s.outbound = append(s.outbound, wsOutbound{
		kind:    websocket.TextMessage,
		payload: []byte(interpolate(string(b), s.t.snapshotVars())),
	})
	return s
}

// SendBinary queues a raw binary frame.
func (s *WSStep) SendBinary(b []byte) *WSStep {
	if s.sent {
		s.fail("SendBinary() called after dial")
		return s
	}
	s.outbound = append(s.outbound, wsOutbound{
		kind:    websocket.BinaryMessage,
		payload: append([]byte(nil), b...),
	})
	return s
}

// ExpectConnected asserts the WS handshake succeeded.
func (s *WSStep) ExpectConnected() *WSStep {
	_ = s.ensureSent() // a dial error is recorded below as the assertion failure
	if s.dialErr != nil {
		s.fail(fmt.Sprintf("ExpectConnected: %v", s.dialErr))
	}
	return s
}

// ExpectReceivedCount asserts that the Listen window collected exactly n
// frames.
func (s *WSStep) ExpectReceivedCount(n int) *WSStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.received) != n {
		s.fail(fmt.Sprintf("ExpectReceivedCount: want %d, got %d", n, len(s.received)))
	}
	return s
}

// ExpectReceivedAtLeast asserts >= n frames collected.
func (s *WSStep) ExpectReceivedAtLeast(n int) *WSStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.received) < n {
		s.fail(fmt.Sprintf("ExpectReceivedAtLeast: want >=%d, got %d", n, len(s.received)))
	}
	return s
}

// ExpectMessageContains asserts the idx-th frame payload contains sub.
func (s *WSStep) ExpectMessageContains(idx int, sub string) *WSStep {
	if !s.ensureSent() {
		return s
	}
	if idx < 0 || idx >= len(s.received) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: index out of range (len=%d)", idx, len(s.received)))
		return s
	}
	if !strings.Contains(string(s.received[idx].Data), sub) {
		s.fail(fmt.Sprintf("ExpectMessageContains[%d]: %q not found", idx, sub))
	}
	return s
}

// ExpectJSONPath asserts a JSONPath value inside the idx-th frame.
func (s *WSStep) ExpectJSONPath(idx int, path string, want any) *WSStep {
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

// Extract stores a JSONPath value from the idx-th frame into the var store.
func (s *WSStep) Extract(idx int, path, name string) *WSStep {
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

// Received returns a copy of the collected frames — escape hatch for
// custom matching logic.
func (s *WSStep) Received() []WSMessage {
	s.ensureSent()
	out := make([]WSMessage, len(s.received))
	copy(out, s.received)
	return out
}

// Done finalises the step (closes the connection if still open).
func (s *WSStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *WSStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *WSStep) evalAt(idx int, path string) (any, error) {
	if idx < 0 || idx >= len(s.received) {
		return nil, fmt.Errorf("index out of range (len=%d)", len(s.received))
	}
	var doc any
	if err := json.Unmarshal(s.received[idx].Data, &doc); err != nil {
		return nil, fmt.Errorf("frame[%d] is not JSON: %w", idx, err)
	}
	return resolveJSONPath(doc, path)
}

func (s *WSStep) ensureSent() bool {
	if s.sent {
		return s.dialErr == nil && !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}

	url := s.wsURL()
	parent := s.t.ctx
	if parent == nil {
		parent = context.Background()
	}

	s.startedAt = time.Now()
	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.DialContext(parent, url, s.headers)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		s.dialErr = err
		s.fail(fmt.Sprintf("ws dial: %v", err))
		s.abortChain = true
		s.endedAt = time.Now()
		return false
	}
	s.conn = conn

	// Flush outbound queue before opening the receive window.
	for _, frame := range s.outbound {
		if werr := conn.WriteMessage(frame.kind, frame.payload); werr != nil {
			s.fail(fmt.Sprintf("ws send: %v", werr))
		}
	}

	// Set the read deadline to the listen window and pull frames until
	// the deadline elapses or the peer closes.
	_ = conn.SetReadDeadline(time.Now().Add(s.listen))
	for {
		mt, data, rerr := conn.ReadMessage()
		if rerr != nil {
			// Timeouts / clean close both end the loop without recording
			// a failure — consistent with SSE's "listen window over"
			// semantics.
			break
		}
		s.received = append(s.received, WSMessage{Type: mt, Data: append([]byte(nil), data...)})
	}
	_ = conn.Close()
	s.endedAt = time.Now()
	return true
}

// wsURL rewrites http://host → ws://host (and https:// → wss://) when
// the user passed a Tester BaseURL of the HTTP variant. Absolute ws://
// URLs pass through.
func (s *WSStep) wsURL() string {
	u := s.url
	if strings.HasPrefix(u, "ws://") || strings.HasPrefix(u, "wss://") {
		return u
	}
	if strings.HasPrefix(u, "http://") {
		return "ws://" + strings.TrimPrefix(u, "http://")
	}
	if strings.HasPrefix(u, "https://") {
		return "wss://" + strings.TrimPrefix(u, "https://")
	}
	base := s.t.baseURL
	if strings.HasPrefix(base, "http://") {
		base = "ws://" + strings.TrimPrefix(base, "http://")
	} else if strings.HasPrefix(base, "https://") {
		base = "wss://" + strings.TrimPrefix(base, "https://")
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return base + u
}

func (s *WSStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:     "ws",
		Method:       "frame",
		Name:         "ws " + s.url,
		URL:          s.url,
		StatusOrCode: len(s.received),
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
