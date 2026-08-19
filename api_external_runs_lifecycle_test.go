// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExternalRunsLifecycle_Flow(t *testing.T) {
	var steps int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/namespaces/sandbox/tcm/external-runs/lifecycle"
		switch {
		case r.Method == http.MethodPost && r.URL.Path == base:
			_ = json.NewEncoder(w).Encode(LifecycleRun{ID: "run-1", Status: "running", Name: "checkout"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/steps"):
			body, _ := io.ReadAll(r.Body)
			var in struct {
				Steps []LifecycleStep `json:"steps"`
			}
			_ = json.Unmarshal(body, &in)
			steps += len(in.Steps)
			_ = json.NewEncoder(w).Encode(LifecycleRun{ID: "run-1", Status: "running", StepCount: steps})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/finish"):
			_ = json.NewEncoder(w).Encode(LifecycleRun{ID: "run-1", Status: "passed",
				ResolvedCaseID: "case-9", ResolvedRunID: "crun-9", StepCount: steps})
		case r.Method == http.MethodGet && r.URL.Path == base+"/run-1":
			_ = json.NewEncoder(w).Encode(LifecycleRun{ID: "run-1", Status: "running", StepCount: steps})
		case r.Method == http.MethodGet && r.URL.Path == base:
			_ = json.NewEncoder(w).Encode(map[string]any{"runs": []LifecycleRun{{ID: "run-1"}}, "total": 1})
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("sandbox"))
	ctx := context.Background()

	run, err := c.ExternalRuns().StartRun(ctx, "", StartRunRequest{Name: "checkout", Framework: "custom"})
	if err != nil || run.ID != "run-1" || run.Status != "running" {
		t.Fatalf("StartRun: %v %+v", err, run)
	}

	run, err = c.ExternalRuns().AppendSteps(ctx, "", run.ID, []LifecycleStep{
		{StepKey: "s1", Name: "login", Status: "passed"},
		{StepKey: "s2", Name: "pay", Status: "passed"},
	})
	if err != nil || run.StepCount != 2 {
		t.Fatalf("AppendSteps: %v %+v", err, run)
	}

	got, err := c.ExternalRuns().GetRun(ctx, "", "run-1")
	if err != nil || got.StepCount != 2 {
		t.Fatalf("GetRun: %v %+v", err, got)
	}

	fin, err := c.ExternalRuns().FinishRun(ctx, "", "run-1", FinishRunRequest{Status: "passed", Summary: "ok"})
	if err != nil || fin.Status != "passed" || fin.ResolvedCaseID != "case-9" || fin.ResolvedRunID != "crun-9" {
		t.Fatalf("FinishRun: %v %+v", err, fin)
	}

	runs, err := c.ExternalRuns().ListRuns(ctx, "")
	if err != nil || len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("ListRuns: %v %+v", err, runs)
	}
}

func TestExternalRunsLifecycle_NamespaceRequired(t *testing.T) {
	c := NewClient("http://x") // no namespace
	c.namespace = ""
	_, err := c.ExternalRuns().StartRun(context.Background(), "", StartRunRequest{Name: "x"})
	if err == nil || !strings.Contains(err.Error(), "namespace required") {
		t.Fatalf("expected namespace-required error, got %v", err)
	}
}
