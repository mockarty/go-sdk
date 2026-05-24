// Copyright (c) 2026 Mockarty. All rights reserved.

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

func TestExternalRunsSubmit(t *testing.T) {
	var capturedPath string
	var capturedBody ExternalRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)
		_ = json.NewEncoder(w).Encode(ExternalRunResponse{
			RunID: "run-uuid", CaseID: "case-uuid", Status: "passed",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("test-ns"))
	resp, err := c.ExternalRuns().Submit(context.Background(), "", ExternalRunRequest{
		CaseName: "smoke",
		Status:   ExternalStatusPassed,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.RunID != "run-uuid" {
		t.Fatalf("RunID: %q", resp.RunID)
	}
	if capturedPath != "/api/v1/namespaces/test-ns/tcm/external-runs" {
		t.Fatalf("path: %q", capturedPath)
	}
	if capturedBody.SchemaVersion != ExternalRunSchemaVersion {
		t.Fatalf("schemaVersion auto-fill failed: %d", capturedBody.SchemaVersion)
	}
}

func TestExternalRunsSubmitNamespaceOverride(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, WithNamespace("default-ns"))
	_, _ = c.ExternalRuns().Submit(context.Background(), "override-ns", ExternalRunRequest{
		CaseName: "x", Status: ExternalStatusPassed,
	})
	if !strings.Contains(capturedPath, "/override-ns/") {
		t.Fatalf("override ignored: %q", capturedPath)
	}
}

func TestExternalRunsSubmitRequiresNamespace(t *testing.T) {
	c := NewClient("http://unreachable.test")
	// No client namespace, no override → error.
	c.namespace = ""
	_, err := c.ExternalRuns().Submit(context.Background(), "", ExternalRunRequest{
		CaseName: "x", Status: ExternalStatusPassed,
	})
	if err == nil {
		t.Fatal("want error when namespace missing")
	}
}

func TestExternalRunsSubmitBatch(t *testing.T) {
	calls := 0
	var rows []ExternalRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Runs []ExternalRunRequest `json:"runs"`
		}
		dec := json.NewDecoder(r.Body)
		_ = dec.Decode(&body)
		rows = append(rows, body.Runs...)
		// Echo back per-row results.
		results := make([]struct {
			RunID  string `json:"runId,omitempty"`
			CaseID string `json:"caseId,omitempty"`
			Status string `json:"status,omitempty"`
			Error  string `json:"error,omitempty"`
		}, len(body.Runs))
		for i := range results {
			results[i].RunID = "r-" + body.Runs[i].CaseName
			results[i].Status = "passed"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("ns"))
	// 250 runs → 3 chunks of 100/100/50.
	in := make([]ExternalRunRequest, 250)
	for i := range in {
		in[i] = ExternalRunRequest{
			CaseName: "case-" + strings.Repeat("x", 1) + ":" + intToStr(i),
			Status:   ExternalStatusPassed,
		}
	}
	resp, err := c.ExternalRuns().SubmitBatch(context.Background(), "", in)
	if err != nil {
		t.Fatalf("SubmitBatch: %v", err)
	}
	if calls != 3 {
		t.Fatalf("want 3 batch posts, got %d", calls)
	}
	if got := len(resp.Results); got != 250 {
		t.Fatalf("want 250 results, got %d", got)
	}
	// Ordering preserved.
	if resp.Results[0].RunID != "r-"+in[0].CaseName {
		t.Fatalf("merge order wrong: %v", resp.Results[0])
	}
	if resp.Results[249].RunID != "r-"+in[249].CaseName {
		t.Fatalf("last result wrong: %v", resp.Results[249])
	}
	// Every row got schemaVersion auto-fill.
	for i, r := range rows {
		if r.SchemaVersion != ExternalRunSchemaVersion {
			t.Fatalf("row %d schemaVersion missing", i)
		}
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
