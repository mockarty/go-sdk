// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// GRPCInvoker is the minimal contract the Tester needs from a gRPC
// client. `*grpc.Client` from `protocols/grpc` satisfies it; tests pass
// an in-memory fake to stay offline.
//
// fullMethod is the canonical "Package.Service/Method" form. req / resp
// are JSON-shaped (the underlying client transcodes to/from protobuf
// via descriptor reflection).
type GRPCInvoker interface {
	InvokeJSON(ctx context.Context, fullMethod string, req, resp any) error
}

// GRPCFacet is the gRPC entry point reached via Tester.GRPC(client).
type GRPCFacet struct {
	t      *Tester
	client GRPCInvoker
}

// GRPC returns the gRPC facet bound to the supplied client.
func (t *Tester) GRPC(client GRPCInvoker) *GRPCFacet {
	t.flushPending()
	return &GRPCFacet{t: t, client: client}
}

// Call starts a unary gRPC call.
//
// Owner's canonical chain:
//
//	t.GRPC(client).
//	  Call("user.UserService/Get", map[string]any{"id": 42}).
//	  ExpectOK().
//	  ExpectField("$.name", "Alice").
//	  Extract("$.token", "token")
//
// Pass req=nil for empty-request calls (e.g. health.Check).
func (g *GRPCFacet) Call(fullMethod string, req any) *GRPCStep {
	step := &GRPCStep{
		t:          g.t,
		client:     g.client,
		fullMethod: interpolate(fullMethod, g.t.snapshotVars()),
		req:        req,
	}
	g.t.setPending(step)
	return step
}

// GRPCStep is one gRPC call.
type GRPCStep struct {
	t          *Tester
	client     GRPCInvoker
	fullMethod string
	req        any

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	resp       map[string]any
	err        error
	failures   []string
}

// ExpectOK asserts the call returned no error.
func (s *GRPCStep) ExpectOK() *GRPCStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectOK: %v", s.err))
	}
	return s
}

// ExpectError asserts the call returned an error. Useful for testing
// server-side validation paths.
func (s *GRPCStep) ExpectError() *GRPCStep {
	if !s.ensureSent() {
		// ensureSent records its own failure on transport issues; allow
		// ExpectError to swallow it since the user EXPECTS a failure.
	}
	if s.err == nil {
		s.fail("ExpectError: call succeeded")
		return s
	}
	// Clear any earlier transport-failure noise so the chain reports
	// the expected error as a pass.
	s.failures = nil
	return s
}

// ExpectField is an alias for ExpectJSONPath — kept because it matches
// owner's preferred verb in the canonical chain example.
func (s *GRPCStep) ExpectField(path string, want any) *GRPCStep {
	return s.ExpectJSONPath(path, want)
}

// ExpectJSONPath asserts the JSONPath value in the response body.
func (s *GRPCStep) ExpectJSONPath(path string, want any) *GRPCStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("ExpectJSONPath %s: call errored: %v", path, s.err))
		return s
	}
	got, err := resolveJSONPath(any(s.resp), path)
	if err != nil {
		s.fail(fmt.Sprintf("ExpectJSONPath %s: %v", path, err))
		return s
	}
	if !equalJSONScalar(got, want) {
		s.fail(fmt.Sprintf("ExpectJSONPath %s: want %v, got %v", path, want, got))
	}
	return s
}

// Extract resolves a JSONPath value in the response body and stores its
// string form under name for use in subsequent {{name}} substitutions.
func (s *GRPCStep) Extract(path, name string) *GRPCStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail(fmt.Sprintf("Extract %s: call errored: %v", path, s.err))
		return s
	}
	got, err := resolveJSONPath(any(s.resp), path)
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

// Response returns the decoded response body. Useful as an escape hatch
// when the built-in assertion vocabulary isn't enough.
func (s *GRPCStep) Response() map[string]any {
	s.ensureSent()
	cp := make(map[string]any, len(s.resp))
	for k, v := range s.resp {
		cp[k] = v
	}
	return cp
}

// Done finalises the step explicitly.
func (s *GRPCStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *GRPCStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *GRPCStep) ensureSent() bool {
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
	// Per-step request body interpolation — gRPC requests are JSON
	// at the SDK boundary, so we round-trip through Marshal/Unmarshal
	// to apply {{var}} substitution consistently with HTTP / Kafka.
	if s.req != nil {
		if b, err := json.Marshal(s.req); err == nil {
			s.req = json.RawMessage(interpolate(string(b), s.t.snapshotVars()))
		}
	}
	s.resp = map[string]any{}
	s.startedAt = time.Now()
	s.err = s.client.InvokeJSON(ctx, s.fullMethod, s.req, &s.resp)
	s.endedAt = time.Now()
	return true
}

func (s *GRPCStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	rec := StepRecord{
		Protocol:  "grpc",
		Method:    "unary",
		Name:      "grpc " + s.fullMethod,
		URL:       s.fullMethod,
		StartedAt: s.startedAt,
		EndedAt:   s.endedAt,
		Failures:  append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
