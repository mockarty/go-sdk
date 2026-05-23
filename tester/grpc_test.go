// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// fakeGRPC is an in-memory GRPCInvoker. responder picks a response
// per fullMethod; errs returns the configured error per method.
type fakeGRPC struct {
	responder map[string]any
	errs      map[string]error
	calls     []recordedGRPCCall
}

type recordedGRPCCall struct {
	method string
	req    any
}

func (f *fakeGRPC) InvokeJSON(ctx context.Context, fullMethod string, req, resp any) error {
	f.calls = append(f.calls, recordedGRPCCall{method: fullMethod, req: req})
	if err := f.errs[fullMethod]; err != nil {
		return err
	}
	r, ok := f.responder[fullMethod]
	if !ok {
		return nil // empty response
	}
	b, _ := json.Marshal(r)
	return json.Unmarshal(b, resp)
}

func TestGRPCCanonicalChain(t *testing.T) {
	g := &fakeGRPC{
		responder: map[string]any{
			"user.UserService/Get": map[string]any{
				"id": 42, "name": "Alice", "roles": []string{"admin"},
			},
		},
	}
	tt := New()
	tt.GRPC(g).
		Call("user.UserService/Get", map[string]any{"id": 42}).
		ExpectOK().
		ExpectField("$.name", "Alice").
		ExpectField("$.id", 42).
		Extract("$.name", "user")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	if tt.Vars()["user"] != "Alice" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
	if len(g.calls) != 1 || g.calls[0].method != "user.UserService/Get" {
		t.Fatalf("call inspector wrong: %+v", g.calls)
	}
}

func TestGRPCExpectErrorPath(t *testing.T) {
	g := &fakeGRPC{
		errs: map[string]error{"X/Y": errors.New("not found")},
	}
	tt := New()
	tt.GRPC(g).Call("X/Y", nil).ExpectError()
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("ExpectError should swallow the failure: %v", tt.Errors())
	}
}

func TestGRPCExpectOKOnErrorFails(t *testing.T) {
	g := &fakeGRPC{errs: map[string]error{"X/Y": errors.New("boom")}}
	tt := New()
	tt.GRPC(g).Call("X/Y", nil).ExpectOK()
	tt.Finish()
	if tt.OK() {
		t.Fatal("ExpectOK should fail when the call errored")
	}
}

func TestGRPCInterpolationInMethodAndRequest(t *testing.T) {
	g := &fakeGRPC{responder: map[string]any{"v1.Echo/Echo": map[string]any{"ok": true}}}
	tt := New()
	tt.SetVar("svc", "v1.Echo/Echo")
	tt.SetVar("user", "alice")

	tt.GRPC(g).
		Call("{{svc}}", map[string]any{"name": "{{user}}"}).
		ExpectOK()
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("got %v", tt.Errors())
	}
	if g.calls[0].method != "v1.Echo/Echo" {
		t.Fatalf("method not interpolated: %q", g.calls[0].method)
	}
	// Request body went through Marshal → interpolate → RawMessage,
	// so the fake will have received the substituted JSON.
	raw, ok := g.calls[0].req.(json.RawMessage)
	if !ok {
		t.Fatalf("req should be json.RawMessage after interpolation, got %T", g.calls[0].req)
	}
	if string(raw) != `{"name":"alice"}` {
		t.Fatalf("request not interpolated: %s", raw)
	}
}

func TestGRPCExpectFieldMissing(t *testing.T) {
	g := &fakeGRPC{responder: map[string]any{"X/Y": map[string]any{"a": 1}}}
	tt := New()
	tt.GRPC(g).Call("X/Y", nil).ExpectField("$.missing", "anything")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected field-not-found failure")
	}
}

func TestGRPCResponseEscapeHatch(t *testing.T) {
	g := &fakeGRPC{responder: map[string]any{"X/Y": map[string]any{"a": "b", "c": float64(3)}}}
	tt := New()
	step := tt.GRPC(g).Call("X/Y", nil)
	resp := step.Response()
	step.Done()
	if resp["a"] != "b" || resp["c"].(float64) != 3 {
		t.Fatalf("Response() returned wrong shape: %+v", resp)
	}
}

func TestGRPCExtractOnErrorAlsoFails(t *testing.T) {
	g := &fakeGRPC{errs: map[string]error{"X/Y": errors.New("nope")}}
	tt := New()
	tt.GRPC(g).Call("X/Y", nil).Extract("$.foo", "bar")
	tt.Finish()
	if tt.OK() {
		t.Fatal("Extract should fail when call errored")
	}
}
