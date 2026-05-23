// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// graphqlEcho writes back the queried fields against a small in-memory
// graph. Receives `{query, variables}`; for "user(id:42)" returns
// {data: {user: {id: 42, name: "Alice"}}}. For "errIt" returns
// {data: null, errors: [{message: "boom"}]}.
func graphqlEcho(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo-Vars", string(raw))
		switch {
		case body.Query == "errIt":
			_, _ = w.Write([]byte(`{"data":null,"errors":[{"message":"boom"},{"message":"again"}]}`))
		default:
			id := 42
			if v, ok := body.Variables["id"]; ok {
				if f, ok := v.(float64); ok {
					id = int(f)
				} else if s, ok := v.(string); ok {
					_ = s // accept string vars too
				}
			}
			resp := map[string]any{
				"data": map[string]any{
					"user": map[string]any{"id": id, "name": "Alice"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGraphQLHappyPath(t *testing.T) {
	srv := graphqlEcho(t)
	tt := New(WithBaseURL(srv.URL))
	tt.GraphQL("/graphql").
		Query("{ user(id: 42) { id name } }", map[string]any{"id": 42}).
		ExpectStatus(200).
		ExpectNoErrors().
		ExpectField("$.data.user.name", "Alice").
		ExpectField("$.data.user.id", 42).
		Extract("$.data.user.name", "user")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
	if tt.Vars()["user"] != "Alice" {
		t.Fatalf("Extract failed: %+v", tt.Vars())
	}
}

func TestGraphQLErrorsArray(t *testing.T) {
	srv := graphqlEcho(t)
	tt := New(WithBaseURL(srv.URL))
	tt.GraphQL("/graphql").
		Query("errIt", nil).
		ExpectErrors(2).
		ExpectField("$.errors[0].message", "boom").
		ExpectField("$.errors[1].message", "again")
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
}

func TestGraphQLExpectNoErrorsFails(t *testing.T) {
	srv := graphqlEcho(t)
	tt := New(WithBaseURL(srv.URL))
	tt.GraphQL("/graphql").Query("errIt", nil).ExpectNoErrors()
	tt.Finish()
	if tt.OK() {
		t.Fatal("ExpectNoErrors should fail on errors[]")
	}
}

func TestGraphQLVariableInterpolation(t *testing.T) {
	srv := graphqlEcho(t)
	tt := New(WithBaseURL(srv.URL))
	tt.SetVar("uid", "42")
	tt.GraphQL("/graphql").
		Query("{ user(id: {{uid}}) { id } }", map[string]any{"id": "{{uid}}"}).
		Header("X-Auth", "Bearer {{uid}}").
		ExpectStatus(200).
		ExpectNoErrors()
	tt.Finish()
	if !tt.OK() {
		t.Fatalf("expected OK, got: %v", tt.Errors())
	}
}

func TestGraphQLDoneAndReport(t *testing.T) {
	srv := graphqlEcho(t)
	tt := New(WithBaseURL(srv.URL))
	tt.GraphQL("/graphql").Query("{x}", nil).Done()
	if got := len(tt.Report()); got != 1 {
		t.Fatalf("want 1 step in report, got %d", got)
	}
}
