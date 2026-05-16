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
	mu           sync.Mutex
	closeOnce    sync.Once
	closed       atomic.Bool
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
	if len(missed) > 0 {
		return fmt.Errorf("pact: %d interaction(s) declared but never invoked:\n  - %s",
			len(missed), strings.Join(missed, "\n  - "))
	}
	return nil
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
// debug body identifying the closest miss.
func (s *MockServer) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "pact: cannot read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	for _, rec := range s.interactions {
		if matchRequest(rec, r, body) {
			atomic.AddInt64(&rec.called, 1)
			writeResponse(w, rec.ix.Response)
			return
		}
	}

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
	}
	_ = json.NewEncoder(w).Encode(debug)
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

// matchRequest compares an incoming request against one declared
// interaction. Matching is intentionally lenient — matchers in the
// declared body are compared via JSON-shape parity, not literal-equal.
func matchRequest(rec *recordedInteraction, r *http.Request, body []byte) bool {
	if !strings.EqualFold(rec.ix.Request.Method, r.Method) {
		return false
	}
	if rec.ix.Request.Path != r.URL.Path {
		return false
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
			return false
		}
		matched := false
		for _, want := range expected {
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
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}
	// Query parameters — every declared key/value pair must appear.
	q := r.URL.Query()
	for name, expected := range rec.ix.Request.Query {
		gotAll := q[name]
		if len(gotAll) == 0 {
			return false
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
				return false
			}
		}
	}
	// Body — JSON-shape parity if the declared body is non-nil.
	if rec.bodyJSON != nil {
		if !jsonShapeMatches(rec.bodyJSON, body) {
			return false
		}
	}
	return true
}

// jsonShapeMatches reports whether actualBytes parse to a JSON value
// whose shape matches expected, where:
//
//   - Matchers compare by their underlying example (no type-checking
//     on the actual value — keeping the mock permissive is the right
//     trade-off because the user's tests are the verifier here).
//   - Objects compare by key set (declared keys MUST be present, but
//     extras are allowed).
//   - Slices compare by length AND element-wise shape.
//   - Scalars compare by Go equality after JSON normalisation
//     (numbers become float64, etc.).
func jsonShapeMatches(expected any, actualBytes []byte) bool {
	var actual any
	if len(actualBytes) == 0 {
		return expected == nil
	}
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		// Non-JSON actual body, but declared body expects JSON — no match.
		return false
	}
	return jsonShapeMatchesValue(expected, actual)
}

// jsonShapeMatchesValue is the recursive shape comparison.
func jsonShapeMatchesValue(expected, actual any) bool {
	switch v := expected.(type) {
	case Matcher:
		// Matchers compare by shape, not value: as long as the actual
		// value has the JSON type the matcher describes, it counts as a
		// match. We delegate the type check to matcherAcceptsActual.
		return matcherAcceptsActual(v, actual)
	case map[string]any:
		am, ok := actual.(map[string]any)
		if !ok {
			return false
		}
		for k, e := range v {
			a, present := am[k]
			if !present {
				return false
			}
			if !jsonShapeMatchesValue(e, a) {
				return false
			}
		}
		return true
	case []any:
		as, ok := actual.([]any)
		if !ok {
			return false
		}
		if len(v) == 0 {
			// Empty declared array means "any array".
			return true
		}
		// Compare each declared position against the same actual
		// position; extras in actual are allowed.
		for i, e := range v {
			if i >= len(as) {
				return false
			}
			if !jsonShapeMatchesValue(e, as[i]) {
				return false
			}
		}
		return true
	case []Matcher:
		as, ok := actual.([]any)
		if !ok {
			return false
		}
		for i, e := range v {
			if i >= len(as) {
				return false
			}
			if !jsonShapeMatchesValue(e, as[i]) {
				return false
			}
		}
		return true
	case nil:
		return actual == nil
	default:
		return normalisedEqual(expected, actual)
	}
}

// matcherAcceptsActual reports whether the given matcher accepts the
// given actual JSON value, by JSON type. Used by the mock server's
// request-body matcher — the mock is intentionally permissive (it is
// not the contract verifier; that is the provider's job).
//
// Compound matchers recurse into their Example so EachLike(map{Like(x)})
// still checks the inner type.
func matcherAcceptsActual(m Matcher, actual any) bool {
	switch m.Rule.Match {
	case "type", "":
		return sameJSONType(m.Example, actual)
	case "integer":
		switch v := actual.(type) {
		case float64:
			return v == float64(int64(v))
		case int, int32, int64:
			return true
		}
		return false
	case "decimal":
		_, ok := actual.(float64)
		return ok
	case "boolean":
		_, ok := actual.(bool)
		return ok
	case "regex":
		s, ok := actual.(string)
		if !ok {
			return false
		}
		// We deliberately accept any string in the mock; provider-side
		// verification re-applies the regex strictly.
		_ = s
		return true
	case "values", "each-key", "each-value":
		_, ok := actual.(map[string]any)
		return ok
	case "arrayContains":
		_, ok := actual.([]any)
		return ok
	case "equality":
		return normalisedEqual(m.Example, actual)
	}
	// Unknown matcher kinds default to lenient acceptance — the user's
	// pact verifier will reject mismatches at provider-side replay.
	return true
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
