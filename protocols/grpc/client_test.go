// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package grpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
)

// --- splitMethod ---

func TestSplitMethod(t *testing.T) {
	cases := []struct {
		in      string
		service string
		method  string
		wantErr bool
	}{
		{"acme.UserService/GetUser", "acme.UserService", "GetUser", false},
		{"/acme.UserService/GetUser", "acme.UserService", "GetUser", false},
		{"  acme.UserService/GetUser  ", "acme.UserService", "GetUser", false},
		{"pkg.With.Dots.Service/MethodName", "pkg.With.Dots.Service", "MethodName", false},
		{"NoSlash", "", "", true},
		{"", "", "", true},
		{"/", "", "", true},
		{"acme.Service/", "", "", true},
		{"/acme.Service/", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			s, m, err := splitMethod(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if s != tc.service || m != tc.method {
				t.Fatalf("got (%q,%q), want (%q,%q)", s, m, tc.service, tc.method)
			}
		})
	}
}

// --- recorder semantics ---

func TestNopRecorder_NeverPanics(t *testing.T) {
	var r nopRecorder
	r.Record(context.Background(), Step{Key: "any", Name: "x"})
}

func TestExternalRunsRecorder_NilClient_DropsSilently(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	defer r.Close()
	// Must not panic / block.
	r.Record(context.Background(), Step{Key: "k", Name: "n"})
}

func TestExternalRunsRecorder_CloseIdempotent(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	r.Close()
	r.Close()
	// Second Close must not block or panic.
}

func TestExternalRunsRecorder_AfterCloseDropsNotPanics(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	r.Close()
	// Record after Close must noop, not panic on a closed channel.
	r.Record(context.Background(), Step{Key: "k", Name: "n"})
}

func TestToExternalStep_StatusDefault(t *testing.T) {
	out := toExternalStep(Step{Key: "k", Name: "n"})
	if out.Status != externalruns.StatusPassed {
		t.Fatalf("empty status must default to passed, got %q", out.Status)
	}
	if out.StepKey != "k" || out.Name != "n" {
		t.Fatalf("field mapping wrong: %+v", out)
	}
}

func TestToExternalStep_EndedAtDerivedFromDuration(t *testing.T) {
	start := time.Now()
	out := toExternalStep(Step{
		StartedAt: start,
		Duration:  150 * time.Millisecond,
		Key:       "k",
		Name:      "n",
	})
	if out.FinishedAt == nil {
		t.Fatal("FinishedAt must not be nil when StartedAt + Duration known")
	}
	want := start.Add(150 * time.Millisecond)
	if !out.FinishedAt.Equal(want) {
		t.Fatalf("FinishedAt = %v, want %v", *out.FinishedAt, want)
	}
}

func TestStepKey_StableShape(t *testing.T) {
	got := NewStepKey("acme.Svc/Method", 7)
	want := "acme.Svc/Method#7"
	if got != want {
		t.Fatalf("NewStepKey = %q, want %q", got, want)
	}
}

// --- options pattern ---

func TestDefaultConfig_HasNopRecorderAndReasonableDefaults(t *testing.T) {
	c := defaultConfig()
	if c.recorder == nil {
		t.Fatal("default recorder must be non-nil (nopRecorder)")
	}
	if c.timeout <= 0 {
		t.Fatal("default timeout must be positive")
	}
	if !c.useReflection {
		t.Fatal("default useReflection must be true (matches docstring)")
	}
	if c.payloadCap <= 0 {
		t.Fatal("default payloadCap must be positive")
	}
}

func TestWithRecorder_NilCoercesToNop(t *testing.T) {
	c := defaultConfig()
	WithRecorder(nil)(c)
	// recorder must be assignable and never produce a nil panic.
	c.recorder.Record(context.Background(), Step{})
}

func TestWithPayloadCaptureBytes_NegativeClampsToZero(t *testing.T) {
	c := defaultConfig()
	WithPayloadCaptureBytes(-1)(c)
	if c.payloadCap != 0 {
		t.Fatalf("payloadCap = %d, want 0 (negative clamped)", c.payloadCap)
	}
}

func TestWithTimeout_ZeroIgnored(t *testing.T) {
	c := defaultConfig()
	originalTimeout := c.timeout
	WithTimeout(0)(c)
	if c.timeout != originalTimeout {
		t.Fatalf("timeout = %v, expected unchanged %v", c.timeout, originalTimeout)
	}
}

func TestWithMetadata_AccumulatesAcrossCalls(t *testing.T) {
	c := defaultConfig()
	WithMetadata(map[string]string{"a": "1"})(c)
	WithMetadata(map[string]string{"b": "2"})(c)
	if c.metadata["a"] != "1" || c.metadata["b"] != "2" {
		t.Fatalf("metadata = %v, want {a:1, b:2}", c.metadata)
	}
}

// --- fileSource ---

const sampleProto = `syntax = "proto3";
package acme;

service UserService {
    rpc GetUser(GetUserRequest) returns (GetUserResponse);
    rpc Ping(Empty) returns (Empty);
}

message GetUserRequest {
    string id = 1;
}

message GetUserResponse {
    string id = 1;
    string name = 2;
}

message Empty {}
`

func writeSampleProto(t *testing.T) (path string, importDir string) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "user.proto")
	if err := os.WriteFile(path, []byte(sampleProto), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	return path, dir
}

func TestFileSource_FindMethod(t *testing.T) {
	path, dir := writeSampleProto(t)
	fs, err := newFileSource([]string{filepath.Base(path)}, []string{dir})
	if err != nil {
		t.Fatalf("newFileSource: %v", err)
	}
	defer fs.Close()
	md, err := fs.FindMethod(context.Background(), "acme.UserService/GetUser")
	if err != nil {
		t.Fatalf("FindMethod: %v", err)
	}
	if md.GetName() != "GetUser" {
		t.Fatalf("GetName = %q", md.GetName())
	}
	if md.GetInputType().GetFullyQualifiedName() != "acme.GetUserRequest" {
		t.Fatalf("input type = %q", md.GetInputType().GetFullyQualifiedName())
	}
}

func TestFileSource_FindMethod_NotFound(t *testing.T) {
	path, dir := writeSampleProto(t)
	fs, err := newFileSource([]string{filepath.Base(path)}, []string{dir})
	if err != nil {
		t.Fatalf("newFileSource: %v", err)
	}
	defer fs.Close()
	_, err = fs.FindMethod(context.Background(), "acme.UserService/UnknownMethod")
	if err == nil {
		t.Fatal("expected errMethodNotFound")
	}
}

func TestFileSource_ListServices(t *testing.T) {
	path, dir := writeSampleProto(t)
	fs, err := newFileSource([]string{filepath.Base(path)}, []string{dir})
	if err != nil {
		t.Fatalf("newFileSource: %v", err)
	}
	defer fs.Close()
	names, err := fs.ListServices(context.Background())
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(names) != 1 || names[0] != "acme.UserService" {
		t.Fatalf("services = %v, want [acme.UserService]", names)
	}
}

func TestFileSource_NoPathsReturnsNilSource(t *testing.T) {
	fs, err := newFileSource(nil, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if fs != nil {
		t.Fatal("expected nil source for empty path list")
	}
}

// --- combinedSource fallback semantics ---
//
// We can't easily synthesize a *desc.MethodDescriptor outside the jhump
// library, so the combined source's file-first + reflection-fallback
// path is exercised indirectly through TestFileSource_FindMethod_NotFound
// (file source surfaces errMethodNotFound; combined would then try
// reflection — covered by integration tests against a real testbackend).

// --- Client.Close idempotency (review B3) ---
//
// Dial with a never-connectable target so we get a valid *Client
// without a live server, then Close twice. The fixed sync.Once gate
// must return the cached err on the second call and NOT call
// cc.Close again (which would surface "grpc: the client connection
// is closing" from grpc-go).

func TestClient_Close_Idempotent(t *testing.T) {
	// Use an obviously-unreachable target — dial is non-blocking in
	// grpc-go by default (no WithBlock), so Dial succeeds and Close
	// owns the only path to surface errors. If you ever pass
	// WithBlock here, this test will need to dial a bufconn server.
	ctx := context.Background()
	c, err := Dial(ctx, "127.0.0.1:1")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	first := c.Close()
	second := c.Close()
	if first != nil && second != nil && first.Error() != second.Error() {
		t.Fatalf("expected cached error: first=%v, second=%v", first, second)
	}
	// Third + fourth call must also be safe.
	_ = c.Close()
	_ = c.Close()
}

// --- gRPC streaming-method rejection (review GAP) ---
//
// Direct test of the rejection path: a fileSource with a
// server-streaming method declared, then InvokeJSON returns the
// "use NewServerStream" error. No live gRPC server needed.

const streamingProto = `syntax = "proto3";
package acme;

service NotifyService {
    rpc Watch(WatchRequest) returns (stream WatchResponse);
}

message WatchRequest {}
message WatchResponse {}
`

func TestClient_InvokeJSON_RejectsStreamingMethod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notify.proto")
	if err := os.WriteFile(path, []byte(streamingProto), 0o644); err != nil {
		t.Fatalf("write proto: %v", err)
	}
	ctx := context.Background()
	c, err := Dial(ctx, "127.0.0.1:1",
		WithProtoFile(filepath.Base(path)),
		WithImportDir(dir),
		WithReflection(false),
	)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	var resp any
	err = c.InvokeJSON(ctx, "acme.NotifyService/Watch", map[string]any{}, &resp)
	if err == nil {
		t.Fatal("expected streaming-method rejection error")
	}
	if !strings.Contains(err.Error(), "streaming method") {
		t.Fatalf("error message lacks 'streaming method': %v", err)
	}
}

// --- gRPC method-name parsing (review CROSS-LANGUAGE INVARIANT) ---
//
// Already covered by TestSplitMethod above; this test pins the
// gRPC-style leading-slash form (often produced by the gRPC core
// libraries) round-trips through splitMethod to the same parts as
// the bare form.

func TestSplitMethod_LeadingSlashRoundTrip(t *testing.T) {
	bareS, bareM, err1 := splitMethod("acme.UserService/GetUser")
	slashS, slashM, err2 := splitMethod("/acme.UserService/GetUser")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if bareS != slashS || bareM != slashM {
		t.Fatalf("leading-slash form parses differently: bare=(%q,%q) slash=(%q,%q)",
			bareS, bareM, slashS, slashM)
	}
}
