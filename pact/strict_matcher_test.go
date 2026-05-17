// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// doPOST helps drive strict-matcher tests with one-liner round-trips.
func doPOST(t *testing.T, url, body string) (int, string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// newServer is a tiny helper that builds a Consumer + one interaction.
func newServer(t *testing.T, body any) *pact.MockServer {
	t.Helper()
	c := pact.NewConsumer("strict",
		pact.WithProvider("strict"),
		pact.WithOutputDir(t.TempDir()),
	)
	c.AddInteraction().
		UponReceiving("strict body").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(body).
		WillRespondWith(200)
	srv, err := c.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv
}

func TestStrictEqualityMatcherRejectsDifferentValue(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"id": pact.Equality(42)})
	if code, _ := doPOST(t, srv.URL()+"/x", `{"id": 99}`); code != http.StatusNotFound {
		t.Fatalf("strict equality with wrong value should 404; got %d", code)
	}
	if code, _ := doPOST(t, srv.URL()+"/x", `{"id": 42}`); code != http.StatusOK {
		t.Fatalf("strict equality with same value should 200; got %d", code)
	}
}

func TestStrictTermMatcherEvaluatesRegex(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"name": pact.Term("anything", `^[a-z]+$`)})
	if code, _ := doPOST(t, srv.URL()+"/x", `{"name":"ABC"}`); code != http.StatusNotFound {
		t.Fatalf("regex mismatch should 404; got %d", code)
	}
	if code, _ := doPOST(t, srv.URL()+"/x", `{"name":"abc"}`); code != http.StatusOK {
		t.Fatalf("regex match should 200; got %d", code)
	}
}

func TestStrictIntegerDecimalBoolean(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{
		"i": pact.Integer(1),
		"d": pact.Decimal(1.5),
		"b": pact.Boolean(true),
	})
	cases := []struct {
		body string
		code int
	}{
		{`{"i":3,"d":2.5,"b":false}`, http.StatusOK},
		{`{"i":1.5,"d":2.5,"b":false}`, http.StatusNotFound}, // i is decimal
		{`{"i":1,"d":"oops","b":false}`, http.StatusNotFound},
		{`{"i":1,"d":1.0,"b":"oops"}`, http.StatusNotFound},
	}
	for _, c := range cases {
		got, _ := doPOST(t, srv.URL()+"/x", c.body)
		if got != c.code {
			t.Fatalf("%s -> %d want %d", c.body, got, c.code)
		}
	}
}

func TestStrictEachLikeRespectsMinMax(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("bounded").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"items": pact.MinMaxType(map[string]any{"id": 1}, 2, 4)}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()

	cases := []struct {
		body string
		code int
		why  string
	}{
		{`{"items":[{"id":1}]}`, 404, "below min"},
		{`{"items":[{"id":1},{"id":2}]}`, 200, "at min"},
		{`{"items":[{"id":1},{"id":2},{"id":3},{"id":4}]}`, 200, "at max"},
		{`{"items":[{"id":1},{"id":2},{"id":3},{"id":4},{"id":5}]}`, 404, "above max"},
	}
	for _, c := range cases {
		got, _ := doPOST(t, srv.URL()+"/x", c.body)
		if got != c.code {
			t.Fatalf("%s (%s): %d want %d", c.why, c.body, got, c.code)
		}
	}
}

func TestStrictEachKeyEachValue(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("typed map").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"keys": pact.EachKey(pact.Term("k", `^k\d+$`)),
			"vals": pact.EachValue(pact.Integer(0)),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()

	good := `{"keys":{"k1":"x","k2":"y"},"vals":{"a":1,"b":2}}`
	bad := `{"keys":{"k1":"x","NOPE":"y"},"vals":{"a":1,"b":2}}`
	if c, _ := doPOST(t, srv.URL()+"/x", good); c != http.StatusOK {
		t.Fatalf("good body: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", bad); c != http.StatusNotFound {
		t.Fatalf("bad key: %d", c)
	}
}

func TestStrictArrayContains(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("variants").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"tags": pact.ArrayContains(pact.Term("admin", `^admin$`), pact.Like("user")),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/x", `{"tags":["admin","user","root"]}`); c != http.StatusOK {
		t.Fatalf("good: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"tags":["bob","alice"]}`); c != http.StatusNotFound {
		t.Fatalf("missing admin: %d", c)
	}
}

func TestStrictJSONPathDottedSubset(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("jsonpath").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"_pp": pact.JSONPath("$.user.profile.age", pact.Integer(0)),
			// Also drop a normal expected field to anchor request.
			"user": map[string]any{"profile": map[string]any{"age": pact.Like(30)}},
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	good := `{"_pp":"$.user.profile.age","user":{"profile":{"age":42}}}`
	if c, _ := doPOST(t, srv.URL()+"/x", good); c != http.StatusOK {
		t.Fatalf("jsonpath good: %d", c)
	}
	bad := `{"_pp":"$.user.profile.age","user":{"profile":{"age":"forty"}}}`
	if c, _ := doPOST(t, srv.URL()+"/x", bad); c != http.StatusNotFound {
		t.Fatalf("jsonpath bad: %d", c)
	}
}

func TestStrictUnknownMatcherFallsBackToTypeCheck(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	odd := pact.Matcher{
		Example: "string-value",
		Type:    "future-v5",
		Rule:    pact.MatcherRule{Match: "future-v5"},
	}
	c.AddInteraction().
		UponReceiving("future").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{"x": odd}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"any string is fine"}`); c != http.StatusOK {
		t.Fatalf("unknown matcher should accept matching type: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":123}`); c != http.StatusNotFound {
		t.Fatalf("unknown matcher should reject different JSON type: %d", c)
	}
}

func TestStrictUnmatchedRequestExposesMismatchTrail(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"id": pact.Integer(1)})
	resp, err := http.Post(srv.URL()+"/x", "application/json", strings.NewReader(`{"id":"oops"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("body not json: %v", err)
	}
	if _, ok := doc["mismatches"]; !ok {
		t.Fatalf("mismatches missing from debug: %s", body)
	}
	// Public accessor returns same data.
	if len(srv.UnmatchedRequests()) != 1 {
		t.Fatalf("UnmatchedRequests = %d", len(srv.UnmatchedRequests()))
	}
}

func TestStrictNestedEachLikeWithLike(t *testing.T) {
	t.Parallel()
	c := pact.NewConsumer("strict", pact.WithOutputDir(t.TempDir()))
	c.AddInteraction().
		UponReceiving("nested").
		WithRequest(http.MethodPost, "/x").
		WithJSONBody(map[string]any{
			"users": pact.EachLike(map[string]any{
				"id":   pact.Integer(0),
				"name": pact.Like("alice"),
			}, 1),
		}).
		WillRespondWith(200)
	srv, _ := c.Start(context.Background())
	defer srv.Close()
	good := `{"users":[{"id":1,"name":"bob"},{"id":2,"name":"carol"}]}`
	if c, _ := doPOST(t, srv.URL()+"/x", good); c != http.StatusOK {
		t.Fatalf("nested EachLike accept: %d", c)
	}
	bad := `{"users":[]}`
	if c, _ := doPOST(t, srv.URL()+"/x", bad); c != http.StatusNotFound {
		t.Fatalf("nested EachLike empty array should fail min=1: %d", c)
	}
}

func TestStrictUnicodeAndLongStrings(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("ы", 10_000)
	srv := newServer(t, map[string]any{"s": pact.Like("привет")})
	body, _ := json.Marshal(map[string]any{"s": long})
	if c, _ := doPOST(t, srv.URL()+"/x", string(body)); c != http.StatusOK {
		t.Fatalf("unicode long string rejected: %d", c)
	}
}

func TestStrictNullExpectedRejectsNonNullActual(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"x": nil})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":"v"}`); c != http.StatusNotFound {
		t.Fatalf("null expected vs non-null actual should 404: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"x":null}`); c != http.StatusOK {
		t.Fatalf("null matches null: %d", c)
	}
}

func TestStrictIncludeMatcher(t *testing.T) {
	t.Parallel()
	srv := newServer(t, map[string]any{"log": pact.Include("ERROR")})
	if c, _ := doPOST(t, srv.URL()+"/x", `{"log":"info: ERROR happened"}`); c != http.StatusOK {
		t.Fatalf("substring match: %d", c)
	}
	if c, _ := doPOST(t, srv.URL()+"/x", `{"log":"no problem"}`); c != http.StatusNotFound {
		t.Fatalf("substring miss: %d", c)
	}
}
