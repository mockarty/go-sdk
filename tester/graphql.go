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
)

// GraphQLFacet is the GraphQL entry point reached via
// Tester.GraphQL(endpoint). Under the hood every operation is a POST
// to `endpoint` carrying {"query": ..., "variables": ...}; the facet
// adds GraphQL-specific assertions (errors[] / data.* path).
type GraphQLFacet struct {
	t        *Tester
	endpoint string
}

// GraphQL returns the GraphQL facet bound to a GraphQL endpoint URL.
// Relative paths resolve against Tester.BaseURL.
func (t *Tester) GraphQL(endpoint string) *GraphQLFacet {
	t.flushPending()
	return &GraphQLFacet{t: t, endpoint: endpoint}
}

// Query starts a GraphQL operation. `operation` is the raw GraphQL
// source (query / mutation / subscription); `variables` is the
// variables map (nil = none). Variable VALUES that are strings
// containing "{{name}}" tokens are interpolated against the Tester's
// var store before send.
func (g *GraphQLFacet) Query(operation string, variables map[string]any) *GraphQLStep {
	g.t.flushPending()
	step := &GraphQLStep{
		t:         g.t,
		endpoint:  g.endpoint,
		operation: interpolate(operation, g.t.snapshotVars()),
		variables: variables,
	}
	g.t.setPending(step)
	return step
}

// GraphQLStep is one GraphQL operation. The underlying transport is
// an HTTP POST issued directly (not via Tester.HTTP) so the step does
// not interleave with the chain pending-step machinery.
type GraphQLStep struct {
	t         *Tester
	endpoint  string
	operation string
	variables map[string]any
	headers   map[string]string

	resp       *http.Response
	respBody   []byte
	startedAtT time.Time
	endedAtT   time.Time

	sent       bool
	committed  bool
	abortChain bool
	parsed     graphQLResponse
	failures   []string
}

// graphQLResponse models the spec response envelope.
type graphQLResponse struct {
	Data       any              `json:"data"`
	Extensions map[string]any   `json:"extensions,omitempty"`
	Errors     []graphQLErrItem `json:"errors,omitempty"`
}

type graphQLErrItem struct {
	Message string `json:"message"`
}

// Header sets a request header (Authorization etc), {{var}}-interpolated.
func (s *GraphQLStep) Header(k, v string) *GraphQLStep {
	if s.sent {
		s.fail("Header() called after send")
		return s
	}
	if s.headers == nil {
		s.headers = map[string]string{}
	}
	s.headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// ExpectStatus asserts the HTTP status (GraphQL spec keeps 200 even on
// errors, so this is mostly a transport sanity check).
func (s *GraphQLStep) ExpectStatus(code int) *GraphQLStep {
	if !s.ensureSent() {
		return s
	}
	if s.resp != nil && s.resp.StatusCode != code {
		s.fail(fmt.Sprintf("ExpectStatus: want %d, got %d", code, s.resp.StatusCode))
	}
	return s
}

// ExpectNoErrors asserts the response has no entries in errors[].
func (s *GraphQLStep) ExpectNoErrors() *GraphQLStep {
	if !s.ensureSent() {
		return s
	}
	if n := len(s.parsed.Errors); n > 0 {
		msgs := make([]string, 0, n)
		for _, e := range s.parsed.Errors {
			msgs = append(msgs, e.Message)
		}
		s.fail(fmt.Sprintf("ExpectNoErrors: %d error(s): %v", n, msgs))
	}
	return s
}

// ExpectErrors asserts the response carries at least n errors[]. Pass
// n=1 for "any error" semantics.
func (s *GraphQLStep) ExpectErrors(n int) *GraphQLStep {
	if !s.ensureSent() {
		return s
	}
	if len(s.parsed.Errors) < n {
		s.fail(fmt.Sprintf("ExpectErrors: want >=%d, got %d", n, len(s.parsed.Errors)))
	}
	return s
}

// ExpectField asserts a value inside the response. Paths are evaluated
// against the WHOLE response envelope ($.data.* / $.errors[*].message
// / $.extensions.*); the most common usage is $.data.<field>.
func (s *GraphQLStep) ExpectField(path string, want any) *GraphQLStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalPath(path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectField %s: %v", path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectField %s: want %v, got %v", path, want, got))
	}
	return s
}

// Extract stores a response value (by JSONPath) into the var store.
func (s *GraphQLStep) Extract(path, name string) *GraphQLStep {
	if !s.ensureSent() {
		return s
	}
	got, err := s.evalPath(path)
	if err != nil {
		s.fail(fmt.Sprintf("Extract %s: %v", path, err))
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

// Done finalises the step.
func (s *GraphQLStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *GraphQLStep) fail(msg string) { s.failures = append(s.failures, msg) }

// evalPath builds the response envelope as a map and resolves the path
// against it. Callers pass $.data.foo / $.errors[0].message / etc.
func (s *GraphQLStep) evalPath(path string) (any, error) {
	envelope := map[string]any{
		"data": s.parsed.Data,
	}
	if len(s.parsed.Errors) > 0 {
		errs := make([]any, 0, len(s.parsed.Errors))
		for _, e := range s.parsed.Errors {
			errs = append(errs, map[string]any{"message": e.Message})
		}
		envelope["errors"] = errs
	}
	if len(s.parsed.Extensions) > 0 {
		envelope["extensions"] = any(s.parsed.Extensions)
	}
	return resolveJSONPath(any(envelope), path)
}

func (s *GraphQLStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}

	body := map[string]any{"query": s.operation}
	if s.variables != nil {
		body["variables"] = s.variables
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		s.fail(fmt.Sprintf("marshal body: %v", err))
		s.abortChain = true
		return false
	}
	bodyBytes = []byte(interpolate(string(bodyBytes), s.t.snapshotVars()))

	url := s.endpoint
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		if s.t.baseURL != "" {
			if !strings.HasPrefix(url, "/") {
				url = "/" + url
			}
			url = s.t.baseURL + url
		}
	}
	ctx := s.t.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		s.fail(fmt.Sprintf("build request: %v", err))
		s.abortChain = true
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range s.t.headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	s.startedAtT = time.Now()
	resp, err := s.t.http.Do(req)
	s.endedAtT = time.Now()
	if err != nil {
		s.fail(fmt.Sprintf("graphql: %v", err))
		s.abortChain = true
		return false
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	s.resp = resp
	s.respBody = respBody

	if err := json.Unmarshal(respBody, &s.parsed); err != nil {
		s.fail(fmt.Sprintf("graphql: parse response: %v", err))
		s.abortChain = true
		return false
	}
	return true
}

func (s *GraphQLStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	// Also commit the underlying HTTP step so the transport call shows
	// up in the report — but as a sub-record with its own row. We use
	// the GraphQL name so the timeline is readable.
	rec := StepRecord{
		Protocol:  "graphql",
		Method:    "POST",
		Name:      "graphql " + s.endpoint,
		URL:       s.endpoint,
		StartedAt: s.startedAtT,
		EndedAt:   s.endedAtT,
		Failures:  append([]string(nil), s.failures...),
	}
	if s.resp != nil {
		rec.StatusOrCode = s.resp.StatusCode
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
