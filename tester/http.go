// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mockarty/mockarty-go/allure"
)

// HTTPFacet is the HTTP entry point reached via Tester.HTTP().
type HTTPFacet struct{ t *Tester }

// GET starts a GET request chain. path may be an absolute URL or a path
// relative to the Tester's BaseURL.
func (h *HTTPFacet) GET(path string) *HTTPStep { return h.req("GET", path) }

// POST starts a POST request chain.
func (h *HTTPFacet) POST(path string) *HTTPStep { return h.req("POST", path) }

// PUT starts a PUT request chain.
func (h *HTTPFacet) PUT(path string) *HTTPStep { return h.req("PUT", path) }

// PATCH starts a PATCH request chain.
func (h *HTTPFacet) PATCH(path string) *HTTPStep { return h.req("PATCH", path) }

// DELETE starts a DELETE request chain.
func (h *HTTPFacet) DELETE(path string) *HTTPStep { return h.req("DELETE", path) }

// HEAD starts a HEAD request chain.
func (h *HTTPFacet) HEAD(path string) *HTTPStep { return h.req("HEAD", path) }

func (h *HTTPFacet) req(method, path string) *HTTPStep {
	// Commit any pending step from a previous chain before starting a
	// fresh one. This lets users write back-to-back chains without an
	// explicit .Done().
	h.t.flushPending()
	step := &HTTPStep{
		t:       h.t,
		method:  method,
		path:    path,
		headers: http.Header{},
	}
	h.t.setPending(step)
	return step
}

// HTTPStep is one HTTP call. Builder and assertion methods chain.
// The first Expect / Extract triggers the actual request; subsequent
// assertions operate on the captured response. The step record is
// committed when the next chain starts or when Tester.Finish / .Done
// is called.
type HTTPStep struct {
	t        *Tester
	method   string
	path     string
	headers  http.Header
	body []byte

	sent       bool
	committed  bool
	abortChain bool
	resp       *http.Response
	respBody   []byte
	startedAt  time.Time
	endedAt    time.Time
	failures   []string
}

// Header sets a request header (replaces any previous value for k).
// Header value supports {{var}} interpolation.
func (s *HTTPStep) Header(k, v string) *HTTPStep {
	if s.sent {
		s.fail("Header() called after send")
		return s
	}
	s.headers.Set(k, interpolate(v, s.t.snapshotVars()))
	return s
}

// JSON marshals v and sets it as the body with content-type
// application/json. Use Body for raw bytes.
func (s *HTTPStep) JSON(v any) *HTTPStep {
	if s.sent {
		s.fail("JSON() called after send")
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		s.fail(fmt.Sprintf("marshal body: %v", err))
		s.abortChain = true
		return s
	}
	s.body = b
	if s.headers.Get("Content-Type") == "" {
		s.headers.Set("Content-Type", "application/json")
	}
	return s
}

// Body sets a raw byte body with the given content type. When the
// content type starts with "text/" or equals
// "application/x-www-form-urlencoded" the body is {{var}}-interpolated.
func (s *HTTPStep) Body(b []byte, contentType string) *HTTPStep {
	if s.sent {
		s.fail("Body() called after send")
		return s
	}
	if shouldInterpolateBody(contentType) {
		b = []byte(interpolate(string(b), s.t.snapshotVars()))
	}
	s.body = b
	if contentType != "" {
		s.headers.Set("Content-Type", contentType)
	}
	return s
}

func shouldInterpolateBody(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.HasPrefix(ct, "text/") || ct == "application/x-www-form-urlencoded"
}

// Send forces the request to fire if it hasn't already. Most callers
// don't need this — the first Expect/Extract triggers the send.
func (s *HTTPStep) Send() *HTTPStep { s.ensureSent(); return s }

// ExpectStatus asserts that the response status code matches.
func (s *HTTPStep) ExpectStatus(code int) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	if s.resp.StatusCode != code {
		s.fail(fmt.Sprintf("ExpectStatus: want %d, got %d", code, s.resp.StatusCode))
	}
	return s
}

// ExpectHeader asserts that the response header k equals v.
func (s *HTTPStep) ExpectHeader(k, v string) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	got := s.resp.Header.Get(k)
	if got != v {
		s.fail(fmt.Sprintf("ExpectHeader %s: want %q, got %q", k, v, got))
	}
	return s
}

// ExpectBodyContains asserts that the response body contains sub.
func (s *HTTPStep) ExpectBodyContains(sub string) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	if !bytes.Contains(s.respBody, []byte(sub)) {
		s.fail(fmt.Sprintf("ExpectBodyContains: %q not found", sub))
	}
	return s
}

// ExpectJSONPath asserts that the resolved JSONPath value equals want.
func (s *HTTPStep) ExpectJSONPath(path string, want any) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalJSONPath(path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectJSONPath %s: %v", path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectJSONPath %s: want %v, got %v", path, want, got))
	}
	return s
}

// ExpectJSONArrayLen asserts the resolved JSONPath value is a length-n array.
func (s *HTTPStep) ExpectJSONArrayLen(path string, n int) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalJSONPath(path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectJSONArrayLen %s: %v", path, err))
		return s
	}
	arr, ok := got.([]any)
	if !ok {
		s.fail(fmt.Sprintf("ExpectJSONArrayLen %s: not an array (%T)", path, got))
		return s
	}
	if len(arr) != n {
		s.fail(fmt.Sprintf("ExpectJSONArrayLen %s: want %d, got %d", path, n, len(arr)))
	}
	return s
}

// Extract resolves a JSONPath value and stores its string form under
// name. Subsequent steps reference it via "{{name}}".
func (s *HTTPStep) Extract(path, name string) *HTTPStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalJSONPath(path)
	if err != nil {
		s.fail(fmt.Sprintf("Extract %s into %q: %v", path, name, err))
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

func formatNumber(f float64) string {
	if f == float64(int64(f)) {
		return fmt.Sprintf("%d", int64(f))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", f), "0"), ".")
}

func (s *HTTPStep) evalJSONPath(path string) (any, error) {
	var doc any
	if err := json.Unmarshal(s.respBody, &doc); err != nil {
		return nil, fmt.Errorf("response is not JSON: %w", err)
	}
	return resolveJSONPath(doc, path)
}

func (s *HTTPStep) fail(msg string) { s.failures = append(s.failures, msg) }

// ensureSent runs the HTTP request the first time it is called.
// Subsequent calls are no-ops. Returns false when the step is aborted.
func (s *HTTPStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true

	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}

	s.startedAt = time.Now()
	url := s.buildURL()
	var body io.Reader
	if len(s.body) > 0 {
		body = bytes.NewReader(s.body)
	}
	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, s.method, url, body)
	if err != nil {
		s.fail(fmt.Sprintf("build request: %v", err))
		s.abortChain = true
		s.endedAt = time.Now()
		return false
	}
	for k, vs := range s.t.headers {
		if _, set := s.headers[k]; set {
			continue
		}
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, vs := range s.headers {
		req.Header[k] = append([]string(nil), vs...)
	}

	resp, err := s.t.http.Do(req)
	s.endedAt = time.Now()
	if err != nil {
		s.fail(fmt.Sprintf("http: %v", err))
		s.abortChain = true
		return false
	}
	defer resp.Body.Close()
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		s.fail(fmt.Sprintf("read body: %v", readErr))
	}
	s.resp = resp
	s.respBody = bodyBytes
	return true
}

// buildURL resolves the path against BaseURL with {{var}} interpolation.
func (s *HTTPStep) buildURL() string {
	p := interpolate(s.path, s.t.snapshotVars())
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if s.t.baseURL == "" {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return s.t.baseURL + p
}

// commit records the step on the Tester and emits an Allure step.
// Idempotent: subsequent calls are no-ops.
func (s *HTTPStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		// Builder methods called but no Expect/Extract ever triggered
		// send — fire it now so we still produce a step record.
		s.ensureSent()
	}
	rec := s.snapshot()
	s.t.recordStep(rec)
	s.emitAllure(rec)
}

func (s *HTTPStep) snapshot() StepRecord {
	rec := StepRecord{
		Protocol:  "http",
		Method:    s.method,
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	rec.URL = s.buildURL()
	rec.Name = fmt.Sprintf("%s %s", s.method, rec.URL)
	if s.resp != nil {
		rec.StatusOrCode = s.resp.StatusCode
	}
	return rec
}

func (s *HTTPStep) emitAllure(rec StepRecord) {
	handle := allure.BeginStep(s.t.ctx, rec.Name)
	if len(rec.Failures) == 0 {
		handle.End()
		return
	}
	handle.Fail(strings.Join(rec.Failures, "; "))
}

// Done finalises the step explicitly. Most callers don't need this — the
// next chain start or Tester.Finish auto-commits. Use Done when you want
// to read OK / Errors immediately after the chain.
func (s *HTTPStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}
