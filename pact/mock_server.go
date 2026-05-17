// Copyright (c) 2026 Mockarty.  All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
)

// MockServer is the in-process consumer-side mock backed by
// httptest.Server. Lifetime is tied to one test (or sub-test) — Start
// returns a fresh server, the test points its HTTP client at URL(),
// then calls Verify() + Close() at teardown.
//
// MockServer is safe for concurrent use by multiple goroutines so
// table-driven and parallel sub-tests can share a single mock.
type MockServer struct {
	consumer     *Consumer
	server       *httptest.Server
	interactions []*recordedInteraction
	// unmatched accumulates the structured failure reports for every
	// request that did not match any declared interaction. The slice is
	// the source of truth for Verify() — it returns one error per
	// unmatched request, with the full per-matcher mismatch breakdown.
	unmatched []UnmatchedRequest
	mu        sync.Mutex
	closeOnce sync.Once
	closed    atomic.Bool
}

// UnmatchedRequest is one wire-side request that the mock server could
// not pair with any declared interaction. Mismatches is the per-
// interaction strict-mode failure breakdown collected while trying
// each candidate.
type UnmatchedRequest struct {
	Method     string                     `json:"method"`
	Path       string                     `json:"path"`
	Body       string                     `json:"body,omitempty"`
	Mismatches map[string][]MatchMismatch `json:"mismatches,omitempty"`
}

// recordedInteraction is the mock-server-side view of an interaction:
// the declared shape plus an atomic call counter. The counter sits at
// the top of the struct so the 64-bit atomic is naturally aligned even
// on 32-bit GOARCH targets.
type recordedInteraction struct {
	bodyJSON any
	ix       Interaction
	called   int64
}

// newMockServer wires the http.Handler that serves declared
// interactions, starts the underlying httptest.Server, and returns a
// MockServer ready to use.
func newMockServer(c *Consumer, snapshot []Interaction) (*MockServer, error) {
	ms := &MockServer{consumer: c}
	for i := range snapshot {
		rec := &recordedInteraction{ix: snapshot[i]}
		// Parse declared body once so the request matcher can compare
		// JSON shapes without re-parsing on every call.
		if snapshot[i].Request.Body != nil {
			rec.bodyJSON = snapshot[i].Request.Body
		}
		ms.interactions = append(ms.interactions, rec)
	}
	handler := http.HandlerFunc(ms.serve)
	ms.server = httptest.NewServer(handler)
	return ms, nil
}

// URL returns the base URL of the running mock server.
//
// Panics after Close — callers should capture the URL before tearing
// the server down.
func (s *MockServer) URL() string {
	if s.closed.Load() {
		panic("pact: MockServer.URL called after Close")
	}
	return s.server.URL
}

// Verify returns an error if any declared interaction was never hit,
// or if the server saw any unmatched requests. It does NOT close the
// server — call Close separately.
//
// The error message lists every uncalled interaction AND every wire-
// side request that produced a 404, including the per-matcher
// mismatch breakdown so the test author can see exactly which JSON
// path failed which matcher.
func (s *MockServer) Verify() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var missed []string
	for _, rec := range s.interactions {
		if atomic.LoadInt64(&rec.called) == 0 {
			missed = append(missed, fmt.Sprintf("%s %s (desc=%q)",
				rec.ix.Request.Method, rec.ix.Request.Path, rec.ix.Description))
		}
	}
	parts := []string{}
	if len(missed) > 0 {
		parts = append(parts, fmt.Sprintf("%d interaction(s) declared but never invoked:\n  - %s",
			len(missed), strings.Join(missed, "\n  - ")))
	}
	if len(s.unmatched) > 0 {
		var lines []string
		for _, u := range s.unmatched {
			var detail []string
			for desc, mms := range u.Mismatches {
				for _, mm := range mms {
					detail = append(detail, fmt.Sprintf("    [%s] %s", desc, mm.Error()))
				}
			}
			lines = append(lines, fmt.Sprintf("  - %s %s\n%s", u.Method, u.Path, strings.Join(detail, "\n")))
		}
		parts = append(parts, fmt.Sprintf("%d unmatched request(s):\n%s",
			len(s.unmatched), strings.Join(lines, "\n")))
	}
	if len(parts) > 0 {
		return fmt.Errorf("pact: %s", strings.Join(parts, "\n"))
	}
	return nil
}

// UnmatchedRequests returns a defensive copy of every wire-side
// request that did not match any declared interaction, including the
// strict-mode failure breakdown per candidate. Useful for tests that
// want to assert mismatches without parsing the error string.
func (s *MockServer) UnmatchedRequests() []UnmatchedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]UnmatchedRequest, len(s.unmatched))
	copy(out, s.unmatched)
	return out
}

// Close shuts the underlying httptest.Server down and writes the
// pact.json file. Close is idempotent. The first call performs both
// teardown steps; subsequent calls are no-ops and return nil.
//
// If writing the pact.json file fails the server is still torn down —
// the caller gets the file-write error back. Failing to write is
// considered a hard error because the test loses its contract artefact.
func (s *MockServer) Close() error {
	var firstErr error
	s.closeOnce.Do(func() {
		s.server.Close()
		s.closed.Store(true)
		s.consumer.finalize()
		if _, err := WritePactFile(s.consumer); err != nil {
			firstErr = err
		}
	})
	return firstErr
}

// Calls returns per-interaction invocation counts in declaration
// order. Useful for tests that want to assert call multiplicity
// without parsing the on-wire pact.
func (s *MockServer) Calls() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]int, len(s.interactions))
	for i, rec := range s.interactions {
		out[i] = int(atomic.LoadInt64(&rec.called))
	}
	return out
}

// serve is the http.Handler that picks the first declared interaction
// matching the incoming request, increments its call counter, and
// writes the declared response. Unmatched requests get a 404 with a
// debug body identifying the closest miss AND the per-matcher
// mismatch breakdown for every candidate.
func (s *MockServer) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "pact: cannot read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	// Walk every candidate; collect per-interaction mismatches so the
	// 404 explains why each candidate was rejected.
	perCandidate := map[string][]MatchMismatch{}
	for _, rec := range s.interactions {
		mismatches := matchRequestStrict(rec, r, body)
		if len(mismatches) == 0 {
			// JSON-shape match passed. Run any registered plugin
			// matcher on top — a plugin can refuse a payload that the
			// generic engine waved through (e.g. malformed protobuf
			// inside a `application/x-protobuf` body).
			if pluginErrs := s.runPluginMatchers(r, body); len(pluginErrs) > 0 {
				key := fmt.Sprintf("%s %s %q", rec.ix.Request.Method, rec.ix.Request.Path, rec.ix.Description)
				perCandidate[key] = pluginErrs
				continue
			}
			atomic.AddInt64(&rec.called, 1)
			writeResponse(w, rec.ix.Response)
			return
		}
		key := fmt.Sprintf("%s %s %q", rec.ix.Request.Method, rec.ix.Request.Path, rec.ix.Description)
		perCandidate[key] = mismatches
	}

	// Record the unmatched request for Verify() and the test report.
	s.mu.Lock()
	s.unmatched = append(s.unmatched, UnmatchedRequest{
		Method:     r.Method,
		Path:       r.URL.Path,
		Body:       string(body),
		Mismatches: perCandidate,
	})
	s.mu.Unlock()

	// Build a useful 404 with the closest declared interaction and the
	// reason it didn't match. Helps the user spot a typo in their
	// real client without consulting logs.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	debug := map[string]any{
		"error":       "pact: no interaction matched",
		"method":      r.Method,
		"path":        r.URL.Path,
		"declared":    summariseDeclared(s.interactions),
		"requestBody": string(body),
		"mismatches":  perCandidate,
	}
	_ = json.NewEncoder(w).Encode(debug)
}

// runPluginMatchers dispatches the incoming request through every
// registered V4 plugin whose SupportedContentTypes claims the
// request's Content-Type. Plugin failures surface as MatchMismatch
// entries with a `$.plugins.<name>` path so the 404 debug envelope
// distinguishes them from JSON-shape failures.
//
// Returns nil when every plugin accepts (or no plugin claims the
// content type, which is the common case for plain HTTP/JSON pacts).
func (s *MockServer) runPluginMatchers(r *http.Request, body []byte) []MatchMismatch {
	if len(s.consumer.pluginRuntimes) == 0 {
		return nil
	}
	ct := r.Header.Get("Content-Type")
	var out []MatchMismatch
	for _, b := range s.consumer.pluginRuntimes {
		if !pluginClaimsContentType(b.plugin, ct) {
			continue
		}
		if err := b.plugin.MatchRequest(r.Context(), b.config, body, ct); err != nil {
			out = append(out, MatchMismatch{
				Path:     "$.plugins." + b.plugin.Name(),
				Reason:   err.Error(),
				Expected: b.config,
				Actual:   "<binary>",
			})
		}
	}
	return out
}

// pluginClaimsContentType returns true when the plugin's declared
// content-type list covers the supplied MIME (with `; charset=...`
// stripping and `type/*` wildcard support).
func pluginClaimsContentType(p pluginRuntime, mime string) bool {
	if mime == "" {
		return false
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	for _, ct := range p.SupportedContentTypes() {
		ct = strings.ToLower(strings.TrimSpace(ct))
		if ct == mime {
			return true
		}
		if strings.HasSuffix(ct, "/*") {
			prefix := strings.TrimSuffix(ct, "/*")
			if idx := strings.Index(mime, "/"); idx > 0 && mime[:idx] == prefix {
				return true
			}
		}
		if ct == "*/*" {
			return true
		}
	}
	return false
}

// summariseDeclared returns a compact view of declared interactions
// for the 404 debug payload.
func summariseDeclared(in []*recordedInteraction) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, rec := range in {
		out = append(out, map[string]any{
			"method":      rec.ix.Request.Method,
			"path":        rec.ix.Request.Path,
			"description": rec.ix.Description,
			"called":      atomic.LoadInt64(&rec.called),
		})
	}
	return out
}

// matchRequestStrict compares an incoming request against one
// declared interaction. Unlike the original lenient matcher, it
// returns a structured per-path mismatch list so the 404 debug
// envelope can explain exactly which expectation failed.
//
// An empty return slice means full match.
func matchRequestStrict(rec *recordedInteraction, r *http.Request, body []byte) []MatchMismatch {
	var mm []MatchMismatch
	if !strings.EqualFold(rec.ix.Request.Method, r.Method) {
		mm = append(mm, MatchMismatch{
			Path:     "$.method",
			Reason:   "method mismatch",
			Expected: rec.ix.Request.Method,
			Actual:   r.Method,
		})
		// Method/path differences make it pointless to compare bodies —
		// return early so the failure list stays terse.
		return mm
	}
	if rec.ix.Request.Path != r.URL.Path {
		mm = append(mm, MatchMismatch{
			Path:     "$.path",
			Reason:   "path mismatch",
			Expected: rec.ix.Request.Path,
			Actual:   r.URL.Path,
		})
		return mm
	}
	// Headers — every declared header value must appear in the request
	// (case-insensitive name).
	for name, expected := range rec.ix.Request.Headers {
		gotAll := []string(nil)
		for k, v := range r.Header {
			if strings.EqualFold(k, name) {
				gotAll = v
				break
			}
		}
		if len(gotAll) == 0 {
			mm = append(mm, MatchMismatch{
				Path:     "$.header." + name,
				Reason:   "header missing",
				Expected: expected,
				Actual:   nil,
			})
			continue
		}
		for _, want := range expected {
			matched := false
			for _, got := range gotAll {
				if strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
					matched = true
					break
				}
				// Allow `application/json; charset=utf-8` to satisfy
				// `application/json` (very common content-type drift).
				if idx := strings.Index(got, ";"); idx >= 0 &&
					strings.EqualFold(strings.TrimSpace(got[:idx]), want) {
					matched = true
					break
				}
			}
			if !matched {
				mm = append(mm, MatchMismatch{
					Path:     "$.header." + name,
					Reason:   "header value mismatch",
					Expected: want,
					Actual:   gotAll,
				})
			}
		}
	}
	// Query parameters — every declared key/value pair must appear.
	q := r.URL.Query()
	for name, expected := range rec.ix.Request.Query {
		gotAll := q[name]
		if len(gotAll) == 0 {
			mm = append(mm, MatchMismatch{
				Path:     "$.query." + name,
				Reason:   "query missing",
				Expected: expected,
				Actual:   nil,
			})
			continue
		}
		for _, want := range expected {
			found := false
			for _, got := range gotAll {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				mm = append(mm, MatchMismatch{
					Path:     "$.query." + name,
					Reason:   "query value mismatch",
					Expected: want,
					Actual:   gotAll,
				})
			}
		}
	}
	// Body — strict shape & matcher rules.
	if rec.bodyJSON != nil {
		bodyMM := bodyMismatches(rec.bodyJSON, body)
		mm = append(mm, bodyMM...)
	}
	return mm
}

// bodyMismatches runs the strict-mode walker against the request body
// and returns every mismatch. An empty actual body where one was
// declared still counts as a mismatch — declaring a body asserts the
// caller will send one.
func bodyMismatches(expected any, actualBytes []byte) []MatchMismatch {
	var actual any
	if len(actualBytes) == 0 {
		if expected == nil {
			return nil
		}
		return []MatchMismatch{{
			Path:     "$.body",
			Reason:   "empty body where one was expected",
			Expected: expected,
			Actual:   nil,
		}}
	}
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		return []MatchMismatch{{
			Path:     "$.body",
			Reason:   "actual body is not valid JSON: " + err.Error(),
			Expected: expected,
			Actual:   string(actualBytes),
		}}
	}
	result := MatchResult{Root: actual}
	strictMatch(expected, actual, "$.body", &result)
	return result.Mismatches
}

// sameJSONType compares the JSON-type of two Go values after JSON
// normalisation (numbers all collapse to float64).
func sameJSONType(a, b any) bool {
	an := jsonNormalise(a)
	bn := jsonNormalise(b)
	switch an.(type) {
	case float64:
		_, ok := bn.(float64)
		return ok
	case string:
		_, ok := bn.(string)
		return ok
	case bool:
		_, ok := bn.(bool)
		return ok
	case map[string]any:
		_, ok := bn.(map[string]any)
		return ok
	case []any:
		_, ok := bn.([]any)
		return ok
	case nil:
		return bn == nil
	}
	return false
}

// normalisedEqual compares two scalar JSON values after normalising
// Go-side number types to float64 (the JSON unmarshal default).
func normalisedEqual(a, b any) bool {
	an := jsonNormalise(a)
	bn := jsonNormalise(b)
	return an == bn
}

func jsonNormalise(v any) any {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	}
	return v
}

// writeResponse serialises the declared response onto the wire.
func writeResponse(w http.ResponseWriter, resp Response) {
	for k, vals := range resp.Headers {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	// Default to status 200 when the caller forgot WillRespondWith.
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if resp.Body == nil {
		return
	}
	cleaned := stripMatchersForBody(resp.Body)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(cleaned); err != nil {
		// Best-effort fallback for unencodable bodies.
		_, _ = io.WriteString(w, errors.New("pact: response body marshal failed: "+err.Error()).Error())
		return
	}
	_, _ = w.Write(bytes.TrimRight(buf.Bytes(), "\n"))
}

// stripMatchersForBody replaces every embedded Matcher with its
// example, producing a plain JSON value safe for stdlib encoding.
func stripMatchersForBody(v any) any {
	switch t := v.(type) {
	case Matcher:
		return stripMatchersForBody(t.Example)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, child := range t {
			out[k] = stripMatchersForBody(child)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = stripMatchersForBody(child)
		}
		return out
	case []Matcher:
		out := make([]any, len(t))
		for i, child := range t {
			out[i] = stripMatchersForBody(child)
		}
		return out
	}
	return v
}
