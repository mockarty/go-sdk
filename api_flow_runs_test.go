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

func TestFlowRunsExecuteHappyPath(t *testing.T) {
	var capturedPath string
	var capturedBody FlowRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &capturedBody)
		_ = json.NewEncoder(w).Encode(FlowRunResponse{
			Status:     "passed",
			DurationMs: 42,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	flow := []byte(`{"ir_version":1,"name":"smoke","steps":[]}`)
	resp, err := c.FlowRuns().Execute(context.Background(), flow,
		WithBaseURL("http://api.test"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Status != "passed" || resp.DurationMs != 42 {
		t.Fatalf("response wrong: %+v", resp)
	}
	if capturedPath != "/api/v1/api-tester/flow-runs" {
		t.Fatalf("path: %q", capturedPath)
	}
	if capturedBody.BaseURL != "http://api.test" {
		t.Fatalf("base_url not sent: %+v", capturedBody)
	}
	if string(capturedBody.Flow) != string(flow) {
		t.Fatalf("flow shape lost: %s vs %s", capturedBody.Flow, flow)
	}
}

func TestFlowRunsExecuteEmptyFlowRejected(t *testing.T) {
	c := NewClient("http://unused")
	if _, err := c.FlowRuns().Execute(context.Background(), nil); err == nil {
		t.Fatal("want error on empty flow")
	}
}

func TestFlowRunsExecuteNonJSONRejected(t *testing.T) {
	c := NewClient("http://unused")
	if _, err := c.FlowRuns().Execute(context.Background(), []byte(`not json`)); err == nil {
		t.Fatal("want error on non-json")
	}
}

func TestFlowRunsExecuteServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.FlowRuns().Execute(context.Background(),
		[]byte(`{"ir_version":1,"steps":[]}`))
	if err == nil {
		t.Fatal("want error on 500")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "500") &&
		!strings.Contains(strings.ToLower(err.Error()), "boom") {
		t.Fatalf("error message should mention status or body: %v", err)
	}
}

func TestFlowRunsExecuteOptionsNilSafe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FlowRunResponse{Status: "passed"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	// A nil option must not panic — the variadic loop has a guard.
	_, err := c.FlowRuns().Execute(context.Background(),
		[]byte(`{"ir_version":1,"steps":[]}`), nil)
	if err != nil {
		t.Fatalf("nil option must be tolerated: %v", err)
	}
}

func TestFlowRunsAccessorWiredOnClient(t *testing.T) {
	c := NewClient("http://example")
	if c.FlowRuns() == nil {
		t.Fatal("Client.FlowRuns() must return a wired API")
	}
}
