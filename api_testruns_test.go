// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The persistent merge surface (POST/GET/DELETE /test-runs/merges*) was
// removed server-side in migration 100. The stateless replacement is
// POST /test-runs/reports/aggregate — recomputed per call, no parent row.

func TestTestRuns_AggregateRunsReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/test-runs/reports/aggregate" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("format"); got != "markdown" {
			t.Fatalf("expected format=markdown, got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"run_ids":["r1","r2"]`) {
			t.Fatalf("expected run_ids in body, got %s", body)
		}
		if !strings.Contains(string(body), `"name":"nightly"`) {
			t.Fatalf("expected name in body, got %s", body)
		}
		_, _ = w.Write([]byte("# Aggregate test run: nightly\n"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	out, err := c.TestRuns().AggregateRunsReport(context.Background(),
		AggregateRunsReportRequest{Name: "nightly", RunIDs: []string{"r1", "r2"}},
		AggregateReportFormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Aggregate test run: nightly") {
		t.Fatalf("unexpected report body: %s", out)
	}
}

func TestTestRuns_AggregateRunsReport_DefaultsToUnified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("format"); got != "unified" {
			t.Fatalf("empty format must default to unified, got %q", got)
		}
		_, _ = w.Write([]byte(`{"totals":{"sources":0,"passed":0,"failed":0}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.TestRuns().AggregateRunsReport(context.Background(),
		AggregateRunsReportRequest{RunIDs: []string{"r1"}}, ""); err != nil {
		t.Fatal(err)
	}
}
