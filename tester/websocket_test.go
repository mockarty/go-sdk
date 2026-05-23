// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// echoWSServer upgrades the connection and immediately sends `serverFrames`
// to the client, then optionally echoes back anything the client sends
// (echo=true). Closes after 50ms of idle so tests stay fast.
func echoWSServer(t *testing.T, serverFrames []string, echo bool) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		defer c.Close()
		for _, f := range serverFrames {
			_ = c.WriteMessage(websocket.TextMessage, []byte(f))
		}
		if echo {
			_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
			for {
				mt, msg, err := c.ReadMessage()
				if err != nil {
					return
				}
				_ = c.WriteMessage(mt, msg)
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWSConnectReceiveFrames(t *testing.T) {
	srv := echoWSServer(t, []string{`{"event":"hi"}`, `{"event":"bye"}`}, false)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().
		Listen(500*time.Millisecond).
		ExpectConnected().
		ExpectReceivedCount(2).
		ExpectMessageContains(0, "hi").
		ExpectJSONPath(0, "$.event", "hi").
		ExpectJSONPath(1, "$.event", "bye").
		Extract(1, "$.event", "lastEvent")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	if tt.Vars()["lastEvent"] != "bye" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
}

func TestWSSendAndEcho(t *testing.T) {
	srv := echoWSServer(t, nil, true)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().
		Send("ping").
		SendJSON(map[string]any{"id": 42}).
		Listen(500*time.Millisecond).
		ExpectReceivedCount(2).
		ExpectMessageContains(0, "ping").
		ExpectJSONPath(1, "$.id", 42)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
}

func TestWSSendInterpolation(t *testing.T) {
	srv := echoWSServer(t, nil, true)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("user", "alice")
	tt.WebSocket("/ws").Connect().
		Send("hello {{user}}").
		SendJSON(map[string]any{"name": "{{user}}"}).
		Listen(500*time.Millisecond).
		ExpectMessageContains(0, "hello alice").
		ExpectJSONPath(1, "$.name", "alice")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestWSConnectFailureRecorded(t *testing.T) {
	tt := New()
	tt.WebSocket("ws://127.0.0.1:1/ws").Connect().
		Listen(100 * time.Millisecond).
		ExpectConnected()
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failure for unreachable WS endpoint")
	}
}

func TestWSReceivedEscapeHatch(t *testing.T) {
	srv := echoWSServer(t, []string{"raw-1", "raw-2"}, false)
	tt := New(WithBaseURL(srv.URL))
	step := tt.WebSocket("/ws").Connect().Listen(500 * time.Millisecond)
	frames := step.Received()
	step.Done()
	if len(frames) != 2 {
		t.Fatalf("want 2 frames, got %d", len(frames))
	}
	if !strings.Contains(string(frames[0].Data), "raw-1") {
		t.Fatalf("frame[0] wrong: %s", frames[0].Data)
	}
}

func TestWSBinaryFrame(t *testing.T) {
	srv := echoWSServer(t, nil, true)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().
		SendBinary([]byte{0x00, 0x01, 0x02}).
		Listen(500 * time.Millisecond).
		ExpectReceivedCount(1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestWSExpectReceivedAtLeast(t *testing.T) {
	srv := echoWSServer(t, []string{"a", "b", "c"}, false)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().Listen(500 * time.Millisecond).
		ExpectReceivedAtLeast(2)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestWSIndexOutOfRange(t *testing.T) {
	srv := echoWSServer(t, nil, false)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().Listen(150*time.Millisecond).
		ExpectMessageContains(0, "x").
		ExpectJSONPath(0, "$.a", 1).
		Extract(0, "$.a", "v")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failures from out-of-range index")
	}
}

func TestWSHeaderInterpolation(t *testing.T) {
	var seen string
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		c, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			c.Close()
		}
	}))
	t.Cleanup(srv.Close)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("tok", "abc")
	tt.WebSocket("/ws").Connect().
		Header("Authorization", "Bearer {{tok}}").
		Listen(150 * time.Millisecond).
		ExpectConnected()
	tt.Finish()
	if seen != "Bearer abc" {
		t.Fatalf("interpolation failed: %q", seen)
	}
}

func TestWSAbsoluteURLPassthrough(t *testing.T) {
	srv := echoWSServer(t, []string{"x"}, false)
	wsURL := "ws://" + strings.TrimPrefix(srv.URL, "http://") + "/ws"
	tt := New()
	tt.WebSocket(wsURL).Connect().Listen(500 * time.Millisecond).ExpectReceivedCount(1)
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("got: %v", tt.Errors())
	}
}

func TestWSJSONUnmarshalFailureSurfaces(t *testing.T) {
	srv := echoWSServer(t, []string{"not-json"}, false)
	tt := New(WithBaseURL(srv.URL))
	tt.WebSocket("/ws").Connect().Listen(500*time.Millisecond).
		ExpectJSONPath(0, "$.x", 1)
	tt.Finish()
	if tt.OK() {
		t.Fatal("non-JSON frame should fail ExpectJSONPath")
	}
}

func TestWSURLRewriteScheme(t *testing.T) {
	// Verify the wsURL helper directly without standing up a server.
	cases := []struct{ in, base, want string }{
		{"ws://x/y", "", "ws://x/y"},
		{"wss://x/y", "", "wss://x/y"},
		{"http://x/y", "", "ws://x/y"},
		{"https://x/y", "", "wss://x/y"},
		{"/y", "http://x", "ws://x/y"},
		{"/y", "https://x", "wss://x/y"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			tt := New(WithBaseURL(c.base))
			st := tt.WebSocket(c.in).Connect()
			defer tt.clearPending(st) // don't actually try to dial
			if got := st.wsURL(); got != c.want {
				t.Fatalf("wsURL(%q,base=%q)=%q, want %q", c.in, c.base, got, c.want)
			}
		})
	}
}

// Compile-time check: the example payload type matches what tests want.
var _ = json.Marshal
