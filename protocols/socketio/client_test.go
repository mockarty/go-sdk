// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package socketio

import "testing"

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "http://h:8080", want: "ws://h:8080/socket.io/?EIO=4&transport=websocket"},
		{in: "https://h", want: "wss://h/socket.io/?EIO=4&transport=websocket"},
		{in: "ws://h/socket.io/", want: "ws://h/socket.io/?EIO=4&transport=websocket"},
		{in: "ws://h/socket.io/?EIO=4&transport=websocket", want: "ws://h/socket.io/?EIO=4&transport=websocket"},
		{in: "http://h/socket.io/?token=x", want: "ws://h/socket.io/?token=x&EIO=4&transport=websocket"},
	}
	for _, tc := range cases {
		if got := normalizeURL(tc.in); got != tc.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseEvent(t *testing.T) {
	ev, ok := parseEvent("/admin", `["greeting",{"msg":"hi"}]`)
	if !ok || ev.Name != "greeting" || ev.Namespace != "/admin" || len(ev.Args) != 1 {
		t.Fatalf("parseEvent: ok=%v ev=%+v", ok, ev)
	}
	if _, ok := parseEvent("/", `not json`); ok {
		t.Fatalf("parseEvent should fail on non-json")
	}
	if _, ok := parseEvent("/", `[]`); ok {
		t.Fatalf("parseEvent should fail on empty array")
	}
}

func TestNsMatch(t *testing.T) {
	if !nsMatch("", "/") || !nsMatch("/", "") || !nsMatch("/admin", "/admin") {
		t.Fatalf("nsMatch positive cases failed")
	}
	if nsMatch("/admin", "/user") {
		t.Fatalf("nsMatch should reject mismatched namespaces")
	}
}
