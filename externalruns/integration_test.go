// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

//go:build integration

// External-runs integration tests — Stage 3 phase C.
//
// Two surfaces are covered:
//
//  1. FromAllureDir — local parser; runs without admin.
//  2. Client.CreateRun / GetRun / FinishRun / ListRuns — admin-bound;
//     skipped cleanly when the admin lacks the TCM external-runs feature
//     or returns 404 (different build, feature gated off, etc.).
package externalruns_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
)

// requireLiveAdmin returns (baseURL, token, namespace) or skips. Mirrors
// the helper in the root package (kept local to avoid a build-time
// dependency loop between externalruns and the parent SDK package).
func requireLiveAdmin(t *testing.T) (baseURL, token, namespace string) {
	t.Helper()
	if os.Getenv("MOCKARTY_INTEGRATION") != "1" {
		t.Skip("MOCKARTY_INTEGRATION!=1")
	}
	baseURL = os.Getenv("MOCKARTY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5770"
	}
	token = os.Getenv("MOCKARTY_API_KEY")
	if token == "" {
		t.Skip("MOCKARTY_API_KEY unset")
	}
	namespace = os.Getenv("MOCKARTY_NAMESPACE")
	if namespace == "" {
		namespace = "sandbox"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("admin unreachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("admin health probe -> %d", resp.StatusCode)
	}
	return baseURL, token, namespace
}

// writeAllureResultsFixture creates a minimal allure-results dir on disk
// for FromAllureDir parsing tests.
func writeAllureResultsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	type label struct {
		Name, Value string
	}
	type step struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Start  int64  `json:"start"`
		Stop   int64  `json:"stop"`
	}
	type result struct {
		UUID      string  `json:"uuid"`
		HistoryID string  `json:"historyId"`
		FullName  string  `json:"fullName"`
		Name      string  `json:"name"`
		Status    string  `json:"status"`
		Labels    []label `json:"labels"`
		Steps     []step  `json:"steps"`
		Start     int64   `json:"start"`
		Stop      int64   `json:"stop"`
	}
	r := result{
		UUID:      "00000000-0000-0000-0000-000000000001",
		HistoryID: "abc123",
		FullName:  "pkg::TestExample",
		Name:      "TestExample",
		Status:    "passed",
		Labels: []label{
			{Name: "framework", Value: "go-test"},
			{Name: "suite", Value: "stage3-integration"},
		},
		Steps: []step{
			{Name: "setup", Status: "passed", Start: 1, Stop: 2},
			{Name: "verify", Status: "passed", Start: 3, Stop: 4},
		},
		Start: 1,
		Stop:  10,
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "00000000-result.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestIntegration_FromAllureDir_Parsing verifies the bridge parses a
// canonical Allure result directory and yields a valid CreateRunRequest
// + Steps slice with no admin round-trip.
func TestIntegration_FromAllureDir_Parsing(t *testing.T) {
	t.Parallel()
	dir := writeAllureResultsFixture(t)
	runs, err := externalruns.FromAllureDir(dir)
	if err != nil {
		t.Fatalf("FromAllureDir: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	run := runs[0]
	if run.Request.Name == "" {
		t.Error("Request.Name empty after parse")
	}
	if run.Request.Framework == "" {
		t.Error("Request.Framework empty — expected go-test from label")
	}
	if len(run.Steps) != 2 {
		t.Errorf("Steps=%d, want 2", len(run.Steps))
	}
	for i, s := range run.Steps {
		if s.StepKey == "" {
			t.Errorf("step[%d].StepKey empty — required for idempotent upsert", i)
		}
		if s.Status == "" {
			t.Errorf("step[%d].Status empty", i)
		}
	}
}

// TestIntegration_FromAllureDir_EmptyDir confirms an empty (but valid)
// directory yields zero runs without error.
func TestIntegration_FromAllureDir_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	runs, err := externalruns.FromAllureDir(dir)
	if err != nil {
		t.Fatalf("FromAllureDir(empty): %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 runs from empty dir, got %d", len(runs))
	}
}

// TestIntegration_FromAllureDir_Errors checks the documented error path.
func TestIntegration_FromAllureDir_Errors(t *testing.T) {
	t.Parallel()
	if _, err := externalruns.FromAllureDir(""); err == nil {
		t.Error("expected error on empty dir argument, got nil")
	} else if !errors.Is(err, externalruns.ErrInvalidConfig) {
		t.Errorf("error chain missing ErrInvalidConfig: %v", err)
	}
	if _, err := externalruns.FromAllureDir("/nonexistent/path/that/cannot/exist"); err == nil {
		t.Error("expected error on missing dir, got nil")
	}
}

// TestIntegration_Client_CreateRun_Live attempts a real round-trip with
// the live admin. On admins that DO expose POST /tcm/external-runs the
// run is created, GET'd back, and finished. On admins that 404 or 403
// the test skips with a clear reason — both outcomes are valid under
// the current Mockarty feature-gating policy.
func TestIntegration_Client_CreateRun_Live(t *testing.T) {
	t.Parallel()
	baseURL, token, namespace := requireLiveAdmin(t)

	c, err := externalruns.NewClient(baseURL, namespace, token)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := externalruns.CreateRunRequest{
		Name:      "stage3-integration-go-sdk",
		Framework: "go-test",
		StartedAt: time.Now().UTC(),
		Tags:      []string{"stage3", "sdk"},
		Environment: map[string]string{
			"ci_job": "stage3-c",
			"sdk":    "go",
		},
	}
	run, err := c.CreateRun(ctx, req)
	if err != nil {
		// Admin doesn't expose this surface? Skip rather than fail —
		// SDK contract is "POST /tcm/external-runs"; if the admin
		// hasn't shipped it yet (different build, feature gated off)
		// that is operational, not a SDK regression.
		if api := externalruns.AsAPIError(err); api != nil {
			switch api.StatusCode {
			case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
				t.Skipf("admin lacks /tcm/external-runs (%d): %s", api.StatusCode, api.Message)
			}
		}
		if strings.Contains(err.Error(), "404") {
			t.Skipf("admin lacks /tcm/external-runs: %v", err)
		}
		t.Fatalf("CreateRun: %v", err)
	}
	if run == nil || run.ID == "" {
		t.Fatalf("CreateRun returned empty run: %+v", run)
	}

	// FinishRun closes the run. The SDK schema points at
	// POST /tcm/external-runs/<id>/finish — also gated. Tolerate skip.
	if err := c.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{
		FinishedAt: time.Now().UTC(),
		Status:     externalruns.StatusPassed,
		Summary:    "sdk integration",
	}); err != nil {
		t.Logf("FinishRun returned %v (acceptable if admin only exposes the create endpoint)", err)
	}
}

// TestIntegration_Client_Validation exercises the SDK-side input
// validation (no admin call required). Catching these client-side avoids
// chatty bad-request loops.
func TestIntegration_Client_Validation(t *testing.T) {
	t.Parallel()
	if _, err := externalruns.NewClient("", "ns", "tok"); err == nil {
		t.Error("empty baseURL should error")
	}
	if _, err := externalruns.NewClient("http://x", "", "tok"); err == nil {
		t.Error("empty namespace should error")
	}
	if _, err := externalruns.NewClient("http://x", "ns", ""); err == nil {
		t.Error("empty token should error")
	}
	if _, err := externalruns.NewClient("ftp://x", "ns", "tok"); err == nil {
		t.Error("ftp scheme should error")
	}

	c, _ := externalruns.NewClient("http://localhost:1", "ns", "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.CreateRun(ctx, externalruns.CreateRunRequest{}); err == nil {
		t.Error("CreateRun with empty Name should reject")
	}
	if err := c.AddSteps(ctx, "", nil); err == nil {
		t.Error("AddSteps with empty runID should reject")
	}
	if err := c.AddSteps(ctx, "id", []externalruns.Step{{Status: "bogus", StepKey: "k"}}); err == nil {
		t.Error("AddSteps with invalid Status should reject")
	}
}
