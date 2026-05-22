// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// newFakeBackend stands up a tiny in-process backend covering the
// shapes the test suite needs. Endpoints:
//
//	GET  /users/42        → 200 {"id":42,"name":"Alice","roles":["admin","ops"]}
//	GET  /login           → 200 {"token":"tok-123"}
//	POST /orders          → 201 {"id":99,"status":"created"} + Echoes X-Auth header
//	GET  /text            → 200 text/plain body "plain"
//	GET  /broken-json     → 200 garbage body (not JSON)
//	*    /404             → 404
func newFakeBackend(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/users/42", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 42, "name": "Alice", "roles": []string{"admin", "ops"},
		})
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok-123"})
	})
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Echo-Auth", r.Header.Get("X-Auth"))
		w.Header().Set("X-Echo-Body", string(body))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 99, "status": "created"})
	})
	mux.HandleFunc("/text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain"))
	})
	mux.HandleFunc("/broken-json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("this is not json"))
	})
	mux.HandleFunc("/404", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHTTPGetExpectStatusJSON(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))

	tt.HTTP().GET("/users/42").
		ExpectStatus(200).
		ExpectJSONPath("$.id", 42).
		ExpectJSONPath("$.name", "Alice").
		ExpectJSONArrayLen("$.roles", 2).
		ExpectJSONPath("$.roles[0]", "admin")

	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got errors: %v", tt.Errors())
	}
	if got := len(tt.Report()); got != 1 {
		t.Fatalf("want 1 step, got %d", got)
	}
}

func TestHTTPChainExtractAndInterpolate(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))

	tt.HTTP().GET("/login").
		ExpectStatus(200).
		Extract("$.token", "token")

	tt.HTTP().POST("/orders").
		Header("X-Auth", "Bearer {{token}}").
		JSON(map[string]any{"userId": 42}).
		ExpectStatus(201).
		ExpectHeader("X-Echo-Auth", "Bearer tok-123").
		ExpectJSONPath("$.id", 99)

	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got errors: %v", tt.Errors())
	}
	report := tt.Report()
	if len(report) != 2 {
		t.Fatalf("want 2 steps, got %d", len(report))
	}
	if report[0].StatusOrCode != 200 || report[1].StatusOrCode != 201 {
		t.Fatalf("unexpected codes: %+v", report)
	}
}

func TestHTTPFailingAssertionsAccumulate(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))

	tt.HTTP().GET("/users/42").
		ExpectStatus(204).
		ExpectJSONPath("$.id", 99).
		ExpectJSONArrayLen("$.roles", 5)

	tt.Finish()

	if tt.OK() {
		t.Fatalf("expected failures")
	}
	if got := len(tt.Errors()); got != 3 {
		t.Fatalf("want 3 errors, got %d: %v", got, tt.Errors())
	}
}

func TestHTTPFailFastShortCircuits(t *testing.T) {
	srv := newFakeBackend(t)
	var second atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/first", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	mux.HandleFunc("/second", func(w http.ResponseWriter, r *http.Request) {
		second.Store(true)
	})
	override := httptest.NewServer(mux)
	t.Cleanup(override.Close)
	_ = srv // backend ref kept for parity with helper

	tt := New(WithBaseURL(override.URL), WithFailFast())
	tt.HTTP().GET("/first").ExpectStatus(200)
	tt.HTTP().GET("/second").ExpectStatus(200)
	tt.Finish()

	if second.Load() {
		t.Fatal("/second should not have been called under fail-fast")
	}
	if tt.OK() {
		t.Fatal("expected failure")
	}
}

func TestHTTPBodyInterpolationTextOnly(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(body)
	}))
	t.Cleanup(echo.Close)

	tt := New(WithBaseURL(echo.URL))
	tt.SetVar("user", "alice")
	tt.HTTP().POST("/").
		Body([]byte("hello {{user}}"), "text/plain").
		ExpectStatus(200).
		ExpectBodyContains("hello alice")
	tt.Finish()

	if !tt.OK() {
		t.Fatalf("expected OK, got %v", tt.Errors())
	}
}

func TestHTTPInvalidJSONResponseFailsExpectJSONPath(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))
	tt.HTTP().GET("/broken-json").ExpectJSONPath("$.foo", "bar")
	tt.Finish()
	if tt.OK() {
		t.Fatal("expected failure")
	}
}

func TestHTTPMissingVarRendersLiteral(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo-Path", r.URL.Path)
	}))
	t.Cleanup(echo.Close)

	tt := New(WithBaseURL(echo.URL))
	tt.HTTP().GET("/u/{{missing}}").ExpectHeader("X-Echo-Path", "/u/{{missing}}")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("unexpected: %v", tt.Errors())
	}
}

func TestHTTPJSONBodyInterpolation(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(echo.Close)

	tt := New(WithBaseURL(echo.URL))
	tt.SetVar("user", "alice")
	tt.SetVar("id", "42")
	tt.HTTP().POST("/").
		JSON(map[string]any{"name": "{{user}}", "id": "{{id}}"}).
		ExpectStatus(200).
		ExpectJSONPath("$.name", "alice").
		ExpectJSONPath("$.id", "42")
	if !tt.OK() {
		t.Fatalf("expected JSON interpolation OK, got %v", tt.Errors())
	}
}

func TestHTTPRawBodyJSONContentTypeSkipsInterpolation(t *testing.T) {
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(echo.Close)

	tt := New(WithBaseURL(echo.URL))
	tt.SetVar("user", "alice")
	raw := []byte(`{"name":"{{user}}"}`)
	tt.HTTP().POST("/").
		Body(raw, "application/json").
		ExpectStatus(200).
		ExpectJSONPath("$.name", "{{user}}") // literal, not substituted

	if !tt.OK() {
		t.Fatalf("raw .Body should bypass interpolation: %v", tt.Errors())
	}
}

func TestTesterAutoFlushOnInspect(t *testing.T) {
	srv := newFakeBackend(t)
	tt := New(WithBaseURL(srv.URL))

	// Chain with failing assertion — NO Finish() call.
	tt.HTTP().GET("/users/42").ExpectStatus(204)

	// OK() must auto-flush so the assertion failure is observable.
	if tt.OK() {
		t.Fatal("OK() should auto-flush pending step and return false")
	}
	if got := len(tt.Errors()); got != 1 {
		t.Fatalf("want 1 error, got %d: %v", got, tt.Errors())
	}
	if got := len(tt.Report()); got != 1 {
		t.Fatalf("Report should contain the auto-flushed step, got %d", got)
	}
}

func TestInterpolate(t *testing.T) {
	cases := []struct {
		name, in, want string
		vars           map[string]string
	}{
		{"no tokens", "plain", "plain", map[string]string{"a": "b"}},
		{"single", "hi {{n}}", "hi ann", map[string]string{"n": "ann"}},
		{"trim spaces", "x {{ n }} y", "x ann y", map[string]string{"n": "ann"}},
		{"missing", "x {{n}} y", "x {{n}} y", nil},
		{"unterminated", "x {{n y", "x {{n y", map[string]string{"n": "ann"}},
		{"multiple", "{{a}}/{{b}}", "1/2", map[string]string{"a": "1", "b": "2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := interpolate(c.in, c.vars)
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestResolveJSONPath(t *testing.T) {
	doc := map[string]any{
		"a": map[string]any{
			"b": []any{"x", "y", float64(3)},
		},
		"name": "alice",
	}
	cases := []struct {
		name string
		path string
		want any
		err  bool
	}{
		{"root", "$", doc, false},
		{"key", "$.name", "alice", false},
		{"nested", "$.a.b[0]", "x", false},
		{"neg index", "$.a.b[-1]", float64(3), false},
		{"star", "$.a.b[*]", []any{"x", "y", float64(3)}, false},
		{"missing key", "$.a.z", nil, true},
		{"out of range", "$.a.b[99]", nil, true},
		{"bad prefix", "a.b", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveJSONPath(doc, c.path)
			if c.err {
				if err == nil {
					t.Fatalf("want error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !equalJSONScalarLoose(got, c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func equalJSONScalarLoose(got, want any) bool {
	switch w := want.(type) {
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !equalJSONScalarLoose(g[i], w[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for k, v := range w {
			if !equalJSONScalarLoose(g[k], v) {
				return false
			}
		}
		return true
	default:
		return equalJSONScalar(got, want)
	}
}
