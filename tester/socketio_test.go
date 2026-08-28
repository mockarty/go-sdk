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

// sioTestServer is a minimal in-process Socket.IO v4 server used to test
// the SocketIO facet end-to-end without the testbackend binary. It mirrors
// the testbackend handler: open handshake, namespace CONNECT, EVENT
// echo/greet.
func sioTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	mux := http.NewServeMux()
	mux.HandleFunc("/socket.io/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`0{"sid":"abc","upgrades":[],"pingInterval":25000,"pingTimeout":20000}`))
		for {
			_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			_, data, rerr := conn.ReadMessage()
			if rerr != nil {
				return
			}
			if len(data) == 0 || data[0] != '4' {
				continue
			}
			sio := string(data[1:])
			switch {
			case strings.HasPrefix(sio, "0"):
				// CONNECT — ack, preserving namespace prefix.
				ns := ""
				tail := sio[1:]
				if strings.HasPrefix(tail, "/") {
					if c := strings.IndexByte(tail, ','); c >= 0 {
						ns = tail[:c]
					}
				}
				if ns == "" {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`40{"sid":"s1"}`))
				} else {
					_ = conn.WriteMessage(websocket.TextMessage, []byte(`40`+ns+`,{"sid":"s1"}`))
				}
			case strings.HasPrefix(sio, "2"):
				body := sio[1:]
				ns := ""
				if strings.HasPrefix(body, "/") {
					if c := strings.IndexByte(body, ','); c >= 0 {
						ns = body[:c]
						body = body[c+1:]
					}
				}
				var arr []json.RawMessage
				if json.Unmarshal([]byte(body), &arr) != nil || len(arr) == 0 {
					continue
				}
				var name string
				_ = json.Unmarshal(arr[0], &name)
				prefix := "42"
				if ns != "" {
					prefix = "42" + ns + ","
				}
				switch name {
				case "echo":
					out, _ := json.Marshal([]any{"echo", json.RawMessage(arr[1])})
					_ = conn.WriteMessage(websocket.TextMessage, append([]byte(prefix), out...))
				case "greet":
					var who string
					if len(arr) > 1 {
						_ = json.Unmarshal(arr[1], &who)
					}
					out, _ := json.Marshal([]any{"greeting", map[string]string{"msg": "hello " + who}})
					_ = conn.WriteMessage(websocket.TextMessage, append([]byte(prefix), out...))
				}
			}
		}
	})
	return httptest.NewServer(mux)
}

func TestSocketIOEmitEcho(t *testing.T) {
	ts := sioTestServer(t)
	defer ts.Close()

	tst := New()
	tst.SocketIO(ts.URL).
		Connect().
		Emit("echo", map[string]any{"n": 1}).
		Emit("greet", "World").
		Collect(2*time.Second).
		ExpectConnected().
		ExpectEvent("echo").
		ExpectEvent("greeting").
		ExpectEventArgContains("echo", `"n":1`).
		ExpectEventJSONPath("greeting", "$.msg", "hello World").
		Extract("greeting", "$.msg", "greetMsg")
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
	if got := tst.Vars()["greetMsg"]; got != "hello World" {
		t.Fatalf("Extract: want 'hello World', got %q", got)
	}
}

func TestSocketIONamespace(t *testing.T) {
	ts := sioTestServer(t)
	defer ts.Close()

	tst := New()
	tst.SocketIO(ts.URL).
		Connect().
		Namespace("/admin").
		Emit("echo", "ns-payload").
		Collect(2*time.Second).
		ExpectConnected().
		ExpectEvent("echo").
		ExpectEventArgContains("echo", "ns-payload")
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
}

func TestSocketIONegative(t *testing.T) {
	ts := sioTestServer(t)
	defer ts.Close()

	cases := []struct {
		name    string
		run     func(tst *Tester)
		wantErr bool
	}{
		{
			name: "dial-failure-fails",
			run: func(tst *Tester) {
				tst.SocketIO("ws://127.0.0.1:1/socket.io/").
					Connect().Collect(500 * time.Millisecond).ExpectConnected()
			},
			wantErr: true,
		},
		{
			name: "missing-event-fails",
			run: func(tst *Tester) {
				tst.SocketIO(ts.URL).Connect().
					Emit("echo", "x").Collect(time.Second).
					ExpectEvent("never-emitted")
			},
			wantErr: true,
		},
		{
			name: "wrong-jsonpath-fails",
			run: func(tst *Tester) {
				tst.SocketIO(ts.URL).Connect().
					Emit("greet", "Bob").Collect(time.Second).
					ExpectEventJSONPath("greeting", "$.msg", "hello Alice")
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tst := New()
			tc.run(tst)
			tst.Finish()
			if tst.OK() == tc.wantErr {
				t.Fatalf("OK()=%v wantErr=%v errs=%v", tst.OK(), tc.wantErr, tst.Errors())
			}
		})
	}
}
