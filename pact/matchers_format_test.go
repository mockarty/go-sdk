// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"net/http"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// TestFormatMatchers exercises the matcher catalogue added for parity with
// the server-side engine: each format matcher must accept a valid example
// (200) and reject a malformed one (404) through the strict mock server.
func TestFormatMatchers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		matcher pact.Matcher
		good    string // JSON value
		bad     string // JSON value
	}{
		{"date", pact.Date("2026-06-12"), `"2026-06-12"`, `"12/06/2026"`},
		{"time", pact.Time("10:30:00"), `"10:30:00"`, `"10:30"`},
		{"datetime", pact.DateTime("2026-06-12T10:30:00Z"), `"2026-06-12T10:30:00Z"`, `"yesterday"`},
		{"timestamp_alias", pact.Timestamp("2026-06-12T10:30:00Z"), `"2026-06-12T10:30:00Z"`, `"nope"`},
		{"uuid", pact.UUID("550e8400-e29b-41d4-a716-446655440000"), `"550e8400-e29b-41d4-a716-446655440000"`, `"not-a-uuid"`},
		{"semver", pact.Semver("1.2.3"), `"1.2.3"`, `"1.2"`},
		{"ipv4", pact.IPv4("192.168.0.1"), `"192.168.0.1"`, `"::1"`},
		{"notNull", pact.NotNull("x"), `"anything"`, `null`},
		{"contentType", pact.ContentType("application/json", "application/json"), `"application/json; charset=utf-8"`, `"text/plain"`},
		{"date_regex_override", pact.DateFormat("12/06/2026", `^\d{2}/\d{2}/\d{4}$`), `"31/12/2026"`, `"2026-12-31"`},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newServer(t, map[string]any{"v": tc.matcher})
			if code, body := doPOST(t, srv.URL()+"/x", `{"v": `+tc.good+`}`); code != http.StatusOK {
				t.Fatalf("%s: valid value should be 200, got %d (%s)", tc.name, code, body)
			}
			if code, _ := doPOST(t, srv.URL()+"/x", `{"v": `+tc.bad+`}`); code != http.StatusNotFound {
				t.Fatalf("%s: malformed value should 404, got %d", tc.name, code)
			}
		})
	}
}

// TestAtLeastOneMatcher checks the array-non-empty matcher independently
// (array body, not scalar).
func TestAtLeastOneMatcher(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"items": pact.AtLeastOne(map[string]any{"id": pact.Integer(1)})})
	if code, body := doPOST(t, srv.URL()+"/x", `{"items": [{"id": 7}]}`); code != http.StatusOK {
		t.Fatalf("non-empty array should be 200, got %d (%s)", code, body)
	}
	if code, _ := doPOST(t, srv.URL()+"/x", `{"items": []}`); code != http.StatusNotFound {
		t.Fatalf("empty array should 404, got %d", code)
	}
}
