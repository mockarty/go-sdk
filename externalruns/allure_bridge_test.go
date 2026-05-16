// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package externalruns

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestFromAllureDir_Conversion writes a synthetic Allure result file to a
// temp dir and asserts the bridge converts it into a CreateRunRequest +
// steps that match the Mockarty external-runs API shape.
func TestFromAllureDir_Conversion(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`{
	  "uuid": "00000000-0000-0000-0000-000000000001",
	  "historyId": "abc123",
	  "fullName": "pkg.TestLogin",
	  "name": "happy-path",
	  "status": "failed",
	  "stage": "finished",
	  "start": 1700000000000,
	  "stop": 1700000005000,
	  "labels": [
	    {"name": "framework", "value": "mockarty-go-sdk"},
	    {"name": "tag", "value": "smoke"},
	    {"name": "tag", "value": "auth"},
	    {"name": "host", "value": "ci-1"},
	    {"name": "testClass", "value": "TestLogin"},
	    {"name": "testMethod", "value": "happy-path"}
	  ],
	  "statusDetails": {"message": "assert failed", "trace": "stack"},
	  "steps": [
	    {
	      "name": "submit",
	      "status": "passed",
	      "stage": "finished",
	      "start": 1700000000100,
	      "stop":  1700000001100,
	      "parameters": [{"name": "user", "value": "alice"}],
	      "steps": [
	        {
	          "name": "validate",
	          "status": "failed",
	          "stage": "finished",
	          "start": 1700000000200,
	          "stop":  1700000001000,
	          "statusDetails": {"message": "wrong code", "trace": "..."}
	        }
	      ]
	    }
	  ]
	}`)
	if err := os.WriteFile(filepath.Join(dir, "first-result.json"), body, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Extra non-result file to verify the bridge ignores noise.
	_ = os.WriteFile(filepath.Join(dir, "executor.json"), []byte("{}"), 0o644)

	runs, err := FromAllureDir(dir)
	if err != nil {
		t.Fatalf("FromAllureDir: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	r := runs[0]

	if r.Request.Name != "pkg.TestLogin" {
		t.Errorf("name = %q", r.Request.Name)
	}
	if r.Request.Framework != "mockarty-go-sdk" {
		t.Errorf("framework = %q", r.Request.Framework)
	}
	if r.Request.ExternalID != "abc123" {
		t.Errorf("external_id = %q", r.Request.ExternalID)
	}
	if len(r.Request.Tags) != 2 || r.Request.Tags[0] != "smoke" {
		t.Errorf("tags = %v", r.Request.Tags)
	}
	if r.Request.Environment["host"] != "ci-1" {
		t.Errorf("env.host = %q", r.Request.Environment["host"])
	}
	if r.FinishRequest.Status != StatusFailed {
		t.Errorf("finish status = %s", r.FinishRequest.Status)
	}
	if r.FinishRequest.Summary != "assert failed" {
		t.Errorf("summary = %q", r.FinishRequest.Summary)
	}

	if len(r.Steps) != 2 {
		t.Fatalf("steps = %d, want 2 (parent+child)", len(r.Steps))
	}
	if r.Steps[0].Name != "submit" || r.Steps[0].Status != StatusPassed {
		t.Errorf("step[0] wrong: %+v", r.Steps[0])
	}
	if r.Steps[0].Parameters["user"] != "alice" {
		t.Errorf("step[0] param: %v", r.Steps[0].Parameters)
	}
	if r.Steps[1].Name != "validate" || r.Steps[1].Status != StatusFailed {
		t.Errorf("step[1] wrong: %+v", r.Steps[1])
	}
	if r.Steps[1].ParentKey != r.Steps[0].StepKey {
		t.Errorf("step[1] parent_key = %q, want %q", r.Steps[1].ParentKey, r.Steps[0].StepKey)
	}
	if r.Steps[1].Message != "wrong code" {
		t.Errorf("step[1] message = %q", r.Steps[1].Message)
	}
	if r.Steps[0].DurationMS != 1000 {
		t.Errorf("step[0] duration = %d, want 1000", r.Steps[0].DurationMS)
	}
}

// TestFromAllureDir_EmptyDir verifies a directory with no result files
// returns no error and an empty slice.
func TestFromAllureDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	runs, err := FromAllureDir(dir)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("runs = %d, want 0", len(runs))
	}
}

// TestFromAllureDir_InvalidConfig covers the empty-dir argument case.
func TestFromAllureDir_InvalidConfig(t *testing.T) {
	if _, err := FromAllureDir(""); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
	if _, err := FromAllureDir("/non/existent/path"); err == nil {
		t.Errorf("expected error for missing dir")
	}
}

// TestFromAllureDir_MalformedJSON verifies a corrupt file yields a
// decoded-error, not a panic.
func TestFromAllureDir_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "bad-result.json"), []byte("{not json"), 0o644)
	if _, err := FromAllureDir(dir); err == nil {
		t.Errorf("expected error for malformed json")
	}
}

// TestStatusOf_AllValues covers every Allure -> ExternalRuns status mapping.
func TestStatusOf_AllValues(t *testing.T) {
	cases := []struct {
		in   string
		want Status
	}{
		{"passed", StatusPassed},
		{"failed", StatusFailed},
		{"broken", StatusBroken},
		{"skipped", StatusSkipped},
		{"running", StatusRunning},
		{"unknown", StatusUnknown},
		{"  PASSED  ", StatusPassed},
		{"", StatusUnknown},
	}
	for _, c := range cases {
		if got := statusOf(c.in); got != c.want {
			t.Errorf("statusOf(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}
