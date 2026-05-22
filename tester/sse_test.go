// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sseEcho serves a fixed event stream then closes — deterministic
// for unit tests (no flush + sleep loops needed).
func sseEcho(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		if status == 0 {
			status = 200
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
		// Implicit close on handler return — the SDK reader sees EOF.
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSSEParseBasic(t *testing.T) {
	body := "event: updated\ndata: {\"id\":42}\nid: e1\n\n" +
		"event: created\ndata: {\"id\":43}\n\n"
	srv := sseEcho(t, body, 200)

	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/stream").Subscribe().
		Listen(2*time.Second).
		ExpectExactEvents(2).
		ExpectEvent("updated").
		ExpectEvent("created").
		ExpectJSONPath("updated", "$.id", 42).
		Extract("created", "$.id", "created_id")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	if tt.Vars()["created_id"] != "43" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
}

func TestSSEMultilineData(t *testing.T) {
	// Spec: consecutive data: lines join with \n.
	body := "event: log\ndata: line1\ndata: line2\ndata: line3\n\n"
	srv := sseEcho(t, body, 200)

	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(time.Second).
		ExpectEvent("log").
		ExpectEventData("log", "line1\nline2\nline3")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("multiline failed: %v", tt.Errors())
	}
}

func TestSSEDefaultEventName(t *testing.T) {
	// No event: prefix → spec default of "message".
	body := "data: {\"x\":1}\n\n"
	srv := sseEcho(t, body, 200)
	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(time.Second).
		ExpectEvent("message").
		ExpectJSONPath("message", "$.x", 1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("default event name failed: %v", tt.Errors())
	}
}

func TestSSECommentLinesIgnored(t *testing.T) {
	body := ": heartbeat\n\n" +
		":\nevent: tick\ndata: 1\n\n"
	srv := sseEcho(t, body, 200)
	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(time.Second).
		ExpectExactEvents(1).
		ExpectEvent("tick")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("comments not skipped: %v", tt.Errors())
	}
}

func TestSSEListenTimesOutWithNoEvents(t *testing.T) {
	// Server hangs the connection open without sending anything. Use
	// a very short Listen so the test stays fast.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		// Block until the client cancels.
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(150 * time.Millisecond).ExpectMinEvents(0)
	tt.Finish()
	// 0 events is fine. The step records a normal record, not a failure.
	if !tt.OK() {
		t.Fatalf("timeout with no events should be OK, got: %v", tt.Errors())
	}
}

func TestSSEServerError(t *testing.T) {
	srv := sseEcho(t, "", 500)
	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(500 * time.Millisecond)
	tt.Finish()
	if tt.OK() {
		t.Fatal("HTTP 500 should fail the SSE step")
	}
}

func TestSSELastEventIDHeader(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Last-Event-ID")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	t.Cleanup(srv.Close)
	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().LastEventID("e-42").Listen(500 * time.Millisecond).ExpectEvent("message")
	tt.Finish()
	if seen != "e-42" {
		t.Fatalf("Last-Event-ID header not sent: %q", seen)
	}
}

func TestSSEHeaderInterpolation(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: ok\n\n"))
	}))
	t.Cleanup(srv.Close)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("token", "abc-123")
	tt.SSE("/").Subscribe().
		Header("Authorization", "Bearer {{token}}").
		Listen(500 * time.Millisecond).
		ExpectEvent("message")
	tt.Finish()
	if auth != "Bearer abc-123" {
		t.Fatalf("interpolation failed: %q", auth)
	}
}

func TestSSEEventDataPathOnNonJSONFails(t *testing.T) {
	body := "event: tick\ndata: not-json\n\n"
	srv := sseEcho(t, body, 200)
	tt := New(WithBaseURL(srv.URL))
	tt.SSE("/").Subscribe().Listen(500*time.Millisecond).
		ExpectJSONPath("tick", "$.x", 1)
	tt.Finish()
	if tt.OK() {
		t.Fatal("non-JSON data should fail ExpectJSONPath")
	}
}

func TestSSEEventsEscapeHatch(t *testing.T) {
	body := "data: a\n\ndata: b\n\n"
	srv := sseEcho(t, body, 200)
	tt := New(WithBaseURL(srv.URL))
	step := tt.SSE("/").Subscribe().Listen(time.Second)
	evs := step.Events()
	step.Done()
	if len(evs) != 2 {
		t.Fatalf("want 2 events, got %d: %+v", len(evs), evs)
	}
	if !strings.Contains(strings.Join([]string{evs[0].Data, evs[1].Data}, ","), "a,b") {
		t.Fatalf("payloads wrong: %+v", evs)
	}
}

func TestSSEParseRetryAndID(t *testing.T) {
	body := "retry: 5000\nid: msg-1\ndata: ok\n\n"
	evs := parseSSE(strings.NewReader(body))
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	if evs[0].Retry != 5000 || evs[0].ID != "msg-1" || evs[0].Data != "ok" {
		t.Fatalf("parse fields wrong: %+v", evs[0])
	}
}
