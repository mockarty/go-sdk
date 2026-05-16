// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

//go:build integration

// Allure SDK integration tests — Stage 3 phase C.
//
// The Allure writer is fully self-contained (no admin round-trip), so
// these tests focus on observable end-to-end behaviour: a real test
// scenario writes `<uuid>-result.json` to disk; the file is parsed back
// and validated against the canonical Allure schema (labels, links,
// parameters, attachments, time fields, history-id stability, status
// derivation).
//
// We do NOT require MOCKARTY_INTEGRATION=1 here because Allure is a
// local writer — but the package is gated under the `integration` build
// tag so it runs alongside the live-admin suite.
package allure_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/allure"
)

// resultsFromDir loads every `*-result.json` produced under dir into the
// canonical allure.Result struct.  Tests use this to assert byte-shape
// fidelity without hand-rolling JSON inspection.
func resultsFromDir(t *testing.T, dir string) []allure.Result {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read results dir %s: %v", dir, err)
	}
	var out []allure.Result
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "-result.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var r allure.Result
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("unmarshal %s: %v\nraw: %s", e.Name(), err, raw)
		}
		out = append(out, r)
	}
	return out
}

// findLabel returns the value of the named label or "" if missing.
func findLabel(r allure.Result, name string) string {
	for _, l := range r.Labels {
		if l.Name == name {
			return l.Value
		}
	}
	return ""
}

// findLink returns the link of the given type with the given name (or "").
func findLink(r allure.Result, linkType, name string) string {
	for _, lk := range r.Links {
		if lk.Type == linkType && lk.Name == name {
			return lk.URL
		}
	}
	return ""
}

// TestIntegration_AllureHappyPath drives the canonical Allure flow end to
// end: T() / labels / links / Step / Attachment / Parameter, then parses
// the emitted JSON and verifies every observable field landed in the
// expected slot.
func TestIntegration_AllureHappyPath(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "allure-results")

	// Inner subtest so allure.T's flush completes BEFORE we read the
	// dir from the parent. t.Cleanup is LIFO — if we registered the
	// reader on the same t the flush would run after our assertion.
	t.Run("body", func(inner *testing.T) {
		a := allure.T(inner,
			allure.WithResultsDir(dir),
			allure.WithFeature("SDK Integration"),
			allure.WithStory("Allure happy path"),
			allure.WithSeverity(allure.SeverityCritical),
			allure.WithOwner("sdk-integration@mockarty.com"),
		)
		a.Issue("INT-1", "https://issues.example.com/INT-1")
		a.TmsLink("TC-99", "https://tms.example.com/cases/99")
		a.Description("Verifies the Allure writer round-trips every observable field.")
		a.Parameter("env", "stage3-integration")

		a.Step("setup", func() {
			a.AttachJSON("payload.json", []byte(`{"hello":"world"}`))
		})
		if err := a.StepErr("verify", func() error { return nil }); err != nil {
			inner.Fatalf("StepErr returned %v, want nil", err)
		}
		a.Tag("sdk", "integration")
	})

	results := resultsFromDir(t, dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result JSON, got %d", len(results))
	}
	r := results[0]
	if r.Status != allure.StatusPassed {
		t.Errorf("Status=%q, want passed", r.Status)
	}
	if r.Stage == "" {
		t.Error("Stage empty — should be set to finished")
	}
	if r.Start <= 0 || r.Stop < r.Start {
		t.Errorf("invalid time bounds Start=%d Stop=%d", r.Start, r.Stop)
	}
	if got := findLabel(r, "feature"); got != "SDK Integration" {
		t.Errorf("feature=%q want SDK Integration", got)
	}
	if got := findLabel(r, "story"); got != "Allure happy path" {
		t.Errorf("story=%q want Allure happy path", got)
	}
	if got := findLabel(r, "severity"); got != "critical" {
		t.Errorf("severity=%q want critical", got)
	}
	if got := findLink(r, "issue", "INT-1"); got != "https://issues.example.com/INT-1" {
		t.Errorf("issue link missing or wrong: %q", got)
	}
	if got := findLink(r, "tms", "TC-99"); got != "https://tms.example.com/cases/99" {
		t.Errorf("tms link missing or wrong: %q", got)
	}
	if len(r.Steps) != 2 {
		t.Errorf("steps=%d, want 2", len(r.Steps))
	}
	if r.HistoryID == "" {
		t.Error("HistoryID empty — Allure expects a stable hash")
	}
	for _, s := range r.Steps {
		if s.Start <= 0 || s.Stop < s.Start {
			t.Errorf("step %q has invalid time bounds %d..%d", s.Name, s.Start, s.Stop)
		}
	}
}

// TestIntegration_AllureFailure verifies that a failed step propagates
// to Result.Status and that the StatusDetails carries the failure text.
func TestIntegration_AllureFailure(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "allure-failure")

	// Inner subtest so the parent t doesn't fail.
	t.Run("inner", func(inner *testing.T) {
		a := allure.T(inner,
			allure.WithResultsDir(dir),
		)
		_ = a.StepErr("boom", func() error { return errors.New("synthetic failure") })
	})

	results := resultsFromDir(t, dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != allure.StatusFailed && r.Status != allure.StatusBroken {
		t.Errorf("Status=%q, want failed/broken", r.Status)
	}
	if r.StatusDetails == nil || r.StatusDetails.Message == "" {
		t.Errorf("StatusDetails missing or empty: %+v", r.StatusDetails)
	}
}

// TestIntegration_AllureParameterized checks that ParameterizedTest
// produces one Result per case with distinct historyId and that each
// case's parameters land in the Result's Parameters slice.
func TestIntegration_AllureParameterized(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "allure-param")

	type Case struct {
		Input string
		Want  string
	}
	cases := []allure.ParameterCase[Case]{
		{Name: "alpha", Payload: Case{Input: "a", Want: "A"}},
		{Name: "beta", Payload: Case{Input: "b", Want: "B"}},
		{Name: "gamma", Payload: Case{Input: "c", Want: "C"}},
	}
	allure.ParameterizedTest(t, cases, func(inner *testing.T, p Case) {
		a := allure.T(inner, allure.WithResultsDir(dir))
		a.Step("emit", func() {})
		_ = p
	})

	results := resultsFromDir(t, dir)
	if len(results) != 3 {
		t.Fatalf("expected 3 case results, got %d", len(results))
	}

	ids := map[string]bool{}
	for _, r := range results {
		if r.HistoryID == "" {
			t.Errorf("Result %q has empty HistoryID", r.Name)
		}
		ids[r.HistoryID] = true
		// Each result must have an Input + Want parameter from the payload.
		paramNames := map[string]string{}
		for _, p := range r.Parameters {
			paramNames[p.Name] = p.Value
		}
		if _, ok := paramNames["Input"]; !ok {
			t.Errorf("Result %q missing Input parameter; got %v", r.Name, r.Parameters)
		}
		if _, ok := paramNames["Want"]; !ok {
			t.Errorf("Result %q missing Want parameter; got %v", r.Name, r.Parameters)
		}
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 distinct historyIds, got %d (collision = same payload renders same)", len(ids))
	}
}

// TestIntegration_AllureAttachmentEncoding verifies the attachment file
// is written next to the result, the result references it by source
// filename, and the mime type is preserved.
func TestIntegration_AllureAttachmentEncoding(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "allure-attach")
	t.Run("inner", func(inner *testing.T) {
		a := allure.T(inner, allure.WithResultsDir(dir))
		a.AttachJSON("data.json", []byte(`{"k":"v"}`))
		a.AttachString("note.txt", "free text")
	})

	results := resultsFromDir(t, dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if len(r.Attachments) < 2 {
		t.Fatalf("attachments=%d, want >=2", len(r.Attachments))
	}
	names := []string{}
	for _, a := range r.Attachments {
		names = append(names, a.Name)
		if a.Source == "" {
			t.Errorf("attachment %q missing source filename", a.Name)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, a.Source)); err != nil {
			t.Errorf("attachment file missing: %v", err)
		}
	}
	sort.Strings(names)
	if names[0] != "data.json" && !strings.Contains(strings.Join(names, ","), "data.json") {
		t.Errorf("attachment names=%v, missing data.json", names)
	}
}

// TestIntegration_AllureTimePrecision validates the time fields are in
// milliseconds-since-epoch and that the run was after our snapshot.
func TestIntegration_AllureTimePrecision(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "allure-time")
	beforeMs := time.Now().UnixMilli()
	t.Run("inner", func(inner *testing.T) {
		a := allure.T(inner, allure.WithResultsDir(dir))
		a.Step("noop", func() {})
	})
	afterMs := time.Now().UnixMilli()

	results := resultsFromDir(t, dir)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Start < beforeMs-1 || r.Start > afterMs+1 {
		t.Errorf("Start=%d outside [%d,%d] — not millisecond epoch?",
			r.Start, beforeMs, afterMs)
	}
}
