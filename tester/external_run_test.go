// Copyright (c) 2026 Mockarty. All rights reserved.

package tester

import (
	"net/http"
	"net/http/httptest"
	"testing"

	mockarty "github.com/mockarty/mockarty-go"
)

func TestToExternalRunHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"name":"Alice"}`))
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	tt.HTTP().GET("/users/42").
		ExpectStatus(200).
		ExpectJSONPath("$.name", "Alice")
	tt.Finish()

	req := tt.ToExternalRun(ExternalRunOptions{
		CaseName:        "users/get",
		TestDisplayName: "GET /users/42",
		Framework:       "custom-runner",
		Labels:          map[string]string{"suite": "smoke"},
		AutoCreate:      true,
	})

	if req.Status != mockarty.ExternalStatusPassed {
		t.Fatalf("run status: %q", req.Status)
	}
	if req.CaseName != "users/get" {
		t.Fatalf("case name lost: %q", req.CaseName)
	}
	if req.Framework != "custom-runner" {
		t.Fatalf("framework override lost: %q", req.Framework)
	}
	if req.SchemaVersion != mockarty.ExternalRunSchemaVersion {
		t.Fatalf("schema version: %d", req.SchemaVersion)
	}
	if len(req.Steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(req.Steps))
	}
	step := req.Steps[0]
	if step.Status != mockarty.ExternalStatusPassed {
		t.Fatalf("step status: %q", step.Status)
	}
	if step.Metadata["protocol"] != "http" {
		t.Fatalf("protocol metadata lost: %+v", step.Metadata)
	}
	if step.Metadata["method"] != "GET" {
		t.Fatalf("method metadata lost: %+v", step.Metadata)
	}
	if step.Metadata["statusOrCode"] != 200 {
		t.Fatalf("statusOrCode metadata lost: %+v", step.Metadata)
	}
	if req.StartedAt == nil || req.FinishedAt == nil {
		t.Fatal("run timestamps missing")
	}
	if req.DurationMs < 0 {
		t.Fatalf("negative duration: %d", req.DurationMs)
	}
}

func TestToExternalRunFailureCarriesErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	tt.HTTP().GET("/").ExpectStatus(200)
	tt.Finish()

	req := tt.ToExternalRun(ExternalRunOptions{CaseName: "x"})
	if req.Status != mockarty.ExternalStatusFailed {
		t.Fatalf("want failed, got %q", req.Status)
	}
	if req.Error == "" {
		t.Fatal("run error message missing")
	}
	if len(req.Steps) != 1 || req.Steps[0].Status != mockarty.ExternalStatusFailed {
		t.Fatalf("step status mapping wrong: %+v", req.Steps)
	}
	if req.Steps[0].Error == "" {
		t.Fatal("step error missing")
	}
}

func TestToExternalRunEmptyTesterEmitsRunOnly(t *testing.T) {
	tt := New()
	tt.Finish()
	req := tt.ToExternalRun(ExternalRunOptions{CaseName: "empty"})
	if req.Status != mockarty.ExternalStatusPassed {
		t.Fatalf("empty tester should be 'passed', got %q", req.Status)
	}
	if len(req.Steps) != 0 {
		t.Fatalf("want 0 steps, got %d", len(req.Steps))
	}
	if req.StartedAt != nil || req.FinishedAt != nil {
		t.Fatal("no steps → no timestamps")
	}
	if req.Framework != "mockarty-tester-go" {
		t.Fatalf("default framework lost: %q", req.Framework)
	}
}

func TestToExternalRunMultipleFailuresJoined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(srv.Close)

	tt := New(WithBaseURL(srv.URL))
	tt.HTTP().GET("/").
		ExpectStatus(204).
		ExpectJSONPath("$.id", 99).
		ExpectJSONPath("$.missing", "x")
	tt.Finish()

	req := tt.ToExternalRun(ExternalRunOptions{CaseName: "multi"})
	if req.Status != mockarty.ExternalStatusFailed {
		t.Fatal("want failed")
	}
	if len(req.Steps) != 1 {
		t.Fatalf("want 1 step, got %d", len(req.Steps))
	}
	step := req.Steps[0]
	if step.Status != mockarty.ExternalStatusFailed {
		t.Fatalf("step status: %q", step.Status)
	}
	// "; " separator between failures so single-line UIs show first.
	if !contains(step.Error, "ExpectStatus") || !contains(step.Error, ";") {
		t.Fatalf("failures not joined: %q", step.Error)
	}
}

func TestExternalRunOptionsLabelsMetadataPassThrough(t *testing.T) {
	tt := New()
	tt.Finish()
	req := tt.ToExternalRun(ExternalRunOptions{
		CaseID: "case-uuid",
		Labels: map[string]string{"feature": "auth", "severity": "critical"},
		Metadata: map[string]any{
			"git_sha": "abc123",
			"ci_url":  "https://ci/build/42",
		},
		ClaimCaseOwnership: true,
	})
	if req.CaseID != "case-uuid" {
		t.Fatal("CaseID lost")
	}
	if req.Labels["feature"] != "auth" {
		t.Fatalf("labels lost: %+v", req.Labels)
	}
	if req.Metadata["git_sha"] != "abc123" {
		t.Fatalf("metadata lost: %+v", req.Metadata)
	}
	if !req.ClaimCaseOwnership {
		t.Fatal("ClaimCaseOwnership lost")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
