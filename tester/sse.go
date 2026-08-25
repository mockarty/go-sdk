// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SSEEvent is one parsed Server-Sent-Events record.
type SSEEvent struct {
	Event string // "" → spec default of "message"
	Data  string // concatenated data: lines, \n-joined
	ID    string // last-event-id field
	Retry int    // retry hint in ms, 0 if absent
}

// SSEFacet is the SSE entry point reached via Tester.SSE(endpoint).
type SSEFacet struct {
	t        *Tester
	endpoint string
}

// SSE returns the SSE facet bound to a stream endpoint URL. Relative
// paths resolve against Tester.BaseURL.
func (t *Tester) SSE(endpoint string) *SSEFacet {
	t.flushPending()
	return &SSEFacet{t: t, endpoint: endpoint}
}

// Subscribe starts the SSE chain. The actual GET fires lazily on the
// first Expect / Extract — by then the user has had a chance to set
// .Listen / .Header / .LastEventID.
func (s *SSEFacet) Subscribe() *SSEStep {
	s.t.flushPending()
	step := &SSEStep{
		t:        s.t,
		endpoint: interpolate(s.endpoint, s.t.snapshotVars()),
		listen:   5 * time.Second,
		headers:  map[string]string{},
	}
	s.t.setPending(step)
	return step
}

// SSEStep is one SSE subscription window.
type SSEStep struct {
	t           *Tester
	endpoint    string
	listen      time.Duration
	headers     map[string]string
	lastEventID string

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	events     []SSEEvent
	statusCode int
	err        error
	failures   []string
}

// Listen sets the maximum collection window (default 5s). The stream
// keeps reading until the duration elapses, the context cancels, or
// the server closes the connection — whichever comes first.
func (s *SSEStep) Listen(d time.Duration) *SSEStep {
	if s.sent {
		s.fail("Listen() called after subscribe")
		return s
	}
	if d > 0 {
		s.listen = d
	}
	return s
}

// Header sets a request header on the underlying GET, {{var}}-interpolated.
func (s *SSEStep) Header(k, v string) *SSEStep {
	if s.sent {
		s.fail("Header() called after subscribe")
		return s
	}
	s.headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// LastEventID is shorthand for Header("Last-Event-ID", id). Used for
// resuming a stream from a known point.
func (s *SSEStep) LastEventID(id string) *SSEStep {
	if s.sent {
		s.fail("LastEventID() called after subscribe")
		return s
	}
	s.lastEventID = id
	return s
}

// ExpectMinEvents asserts at least n events were collected during Listen.
func (s *SSEStep) ExpectMinEvents(n int) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.events) < n {
		s.fail(fmt.Sprintf("ExpectMinEvents: want >=%d, got %d", n, len(s.events)))
	}
	return s
}

// ExpectExactEvents asserts that exactly n events were collected.
func (s *SSEStep) ExpectExactEvents(n int) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.events) != n {
		s.fail(fmt.Sprintf("ExpectExactEvents: want %d, got %d", n, len(s.events)))
	}
	return s
}

// ExpectEvent asserts that an event with the given name was received.
// Pass an empty name to match the spec default ("message").
func (s *SSEStep) ExpectEvent(name string) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	if s.findEvent(name) == nil {
		s.fail(fmt.Sprintf("ExpectEvent %q: not received (%d events)", name, len(s.events)))
	}
	return s
}

// ExpectJSONPath asserts a JSONPath value inside the data of the FIRST
// event matching `eventName`. Empty eventName = "message".
func (s *SSEStep) ExpectJSONPath(eventName, path string, want any) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalEvent(eventName, path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectJSONPath[%s] %s: %v", eventName, path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectJSONPath[%s] %s: want %v, got %v", eventName, path, want, got))
	}
	return s
}

// ExpectEventData asserts an event with the given name carries the exact
// data string.
func (s *SSEStep) ExpectEventData(name, data string) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	ev := s.findEvent(name)
	if ev == nil {
		s.fail(fmt.Sprintf("ExpectEventData %q: event not received", name))
		return s
	}
	if ev.Data != data {
		s.fail(fmt.Sprintf("ExpectEventData %q: want %q, got %q", name, data, ev.Data))
	}
	return s
}

// Extract resolves a JSONPath inside the FIRST event matching eventName
// and stores it under varName for use in subsequent {{varName}}.
func (s *SSEStep) Extract(eventName, path, varName string) *SSEStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalEvent(eventName, path)
	if err != nil {
		s.fail(fmt.Sprintf("Extract[%s] %s: %v", eventName, path, err))
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

// Events returns a copy of the collected events — escape hatch for
// custom matching logic.
func (s *SSEStep) Events() []SSEEvent {
	s.ensureSent()
	out := make([]SSEEvent, len(s.events))
	copy(out, s.events)
	return out
}

// Done finalises the step.
func (s *SSEStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *SSEStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *SSEStep) findEvent(name string) *SSEEvent {
	if name == "" {
		name = "message"
	}
	for i := range s.events {
		ev := s.events[i].Event
		if ev == "" {
			ev = "message"
		}
		if ev == name {
			return &s.events[i]
		}
	}
	return nil
}

func (s *SSEStep) evalEvent(eventName, path string) (any, error) {
	ev := s.findEvent(eventName)
	if ev == nil {
		return nil, fmt.Errorf("event %q not received", eventName)
	}
	var doc any
	if err := json.Unmarshal([]byte(ev.Data), &doc); err != nil {
		return nil, fmt.Errorf("data is not JSON: %w", err)
	}
	return resolveJSONPath(doc, path)
}

func (s *SSEStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}

	url := s.endpoint
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		if s.t.baseURL != "" {
			if !strings.HasPrefix(url, "/") {
				url = "/" + url
			}
			url = s.t.baseURL + url
		}
	}

	parent := s.t.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, s.listen)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		s.fail(fmt.Sprintf("build request: %v", err))
		s.abortChain = true
		return false
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if s.lastEventID != "" {
		req.Header.Set("Last-Event-ID", s.lastEventID)
	}
	for k, vs := range s.t.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	s.startedAt = time.Now()
	resp, err := s.t.http.Do(req)
	if err != nil {
		s.endedAt = time.Now()
		// Deadline-exceeded means we listened for the full window with
		// no events — that's a legitimate outcome, NOT a step failure.
		// Empty events are visible via .ExpectMinEvents.
		if isDeadlineErr(err) {
			s.err = err
			return true
		}
		s.err = err
		s.fail(fmt.Sprintf("sse: %v", err))
		s.abortChain = true
		return false
	}
	defer resp.Body.Close()
	s.statusCode = resp.StatusCode
	if resp.StatusCode/100 != 2 {
		s.endedAt = time.Now()
		s.fail(fmt.Sprintf("sse: HTTP %d", resp.StatusCode))
		s.abortChain = true
		return false
	}
	s.events = parseSSE(resp.Body)
	s.endedAt = time.Now()
	return true
}

func isDeadlineErr(err error) bool {
	if err == nil {
		return false
	}
	// Both context.DeadlineExceeded and net.Error.Timeout flavours
	// surface this way; cover them with a simple substring check that
	// stays robust across Go versions.
	if e, ok := err.(interface{ Timeout() bool }); ok && e.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "deadline exceeded") ||
		strings.Contains(err.Error(), "context canceled")
}

func (s *SSEStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:     "sse",
		Method:       "GET",
		Name:         "sse " + s.endpoint,
		URL:          s.endpoint,
		StatusOrCode: s.statusCode,
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}

// parseSSE reads an SSE stream and returns its events. Blocks until
// the reader is exhausted or the underlying context times out.
//
// Implements the WHATWG Server-Sent Events parser:
//   - lines split by \n / \r\n / \r
//   - lines starting with ":" are comments → ignored
//   - "field: value" — value may have a leading space (per spec, stripped)
//   - blank line dispatches the buffered event (if any data was set)
//   - data: lines are concatenated with \n inside the same record
func parseSSE(r io.Reader) []SSEEvent {
	scanner := bufio.NewScanner(r)
	// Allow up to 1 MiB per line — SSE frames in practice are small but
	// some servers emit large JSON payloads as a single data: line.
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1<<20)

	var (
		out   []SSEEvent
		cur   SSEEvent
		dataB strings.Builder
		seen  bool
	)
	dispatch := func() {
		if !seen {
			return
		}
		cur.Data = dataB.String()
		out = append(out, cur)
		cur = SSEEvent{}
		dataB.Reset()
		seen = false
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			dispatch()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, ok := strings.Cut(line, ":")
		if !ok {
			field = line
			value = ""
		}
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			cur.Event = value
			seen = true
		case "data":
			if dataB.Len() > 0 {
				dataB.WriteByte('\n')
			}
			dataB.WriteString(value)
			seen = true
		case "id":
			cur.ID = value
			seen = true
		case "retry":
			// retry: <integer> — keep best-effort; ignore parse errors.
			n := 0
			for _, c := range value {
				if c < '0' || c > '9' {
					n = 0
					break
				}
				n = n*10 + int(c-'0')
			}
			cur.Retry = n
			seen = true
		}
	}
	dispatch()
	return out
}
