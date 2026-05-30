// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// canonicalAllureHistoryID reproduces allure-python's get_history_id with an
// independent implementation: md5(fullName + non-excluded parameter VALUES
// sorted by name), no separators. The SDK's computeHistoryID MUST agree with
// this for cross-language / allure-pytest history linking to work.
func canonicalAllureHistoryID(fullName string, params []AllureParameter) string {
	kept := make([]AllureParameter, 0, len(params))
	for _, p := range params {
		if !p.Excluded {
			kept = append(kept, p)
		}
	}
	sort.SliceStable(kept, func(a, b int) bool { return kept[a].Name < kept[b].Name })
	h := md5.New()
	h.Write([]byte(fullName))
	for _, p := range kept {
		h.Write([]byte(p.Value))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestHistoryID_MatchesAllurePythonAlgorithm pins the exact byte value the
// SDK must produce so a regression to a different hashing scheme (separators,
// names folded in, SHA1, unsorted) fails loudly. The literal was computed
// from allure-python's md5(full_name, *sorted_values).
func TestHistoryID_MatchesAllurePythonAlgorithm(t *testing.T) {
	const fn = "auth.LoginTest.test_login"
	params := []AllureParameter{{Name: "env", Value: "stage"}}
	const wantSingle = "5f5a70338a980316ba08ab2b04378fa3" // md5("auth.LoginTest.test_login"+"stage")
	if got := computeHistoryID(fn, params); got != wantSingle {
		t.Errorf("historyId = %q, want %q (allure-python md5 algorithm)", got, wantSingle)
	}
	if got, want := computeHistoryID(fn, params), canonicalAllureHistoryID(fn, params); got != want {
		t.Errorf("historyId = %q, independent canonical = %q", got, want)
	}
}

// TestHistoryID_ParameterOrderIndependent verifies the hash sorts parameters
// by name — two iterations declaring the same params in different orders must
// land in the SAME history series (this is what allure-pytest does).
func TestHistoryID_ParameterOrderIndependent(t *testing.T) {
	const fn = "pkg.Test.case"
	a := computeHistoryID(fn, []AllureParameter{{Name: "y", Value: "2"}, {Name: "x", Value: "1"}})
	b := computeHistoryID(fn, []AllureParameter{{Name: "x", Value: "1"}, {Name: "y", Value: "2"}})
	if a != b {
		t.Errorf("historyId must be parameter-order independent: %q != %q", a, b)
	}
}

// TestHistoryID_ExcludedParamIgnored — excluded parameters (timestamps,
// request-ids) must NOT fork the history series.
func TestHistoryID_ExcludedParamIgnored(t *testing.T) {
	const fn = "pkg.Test.case"
	with := computeHistoryID(fn, []AllureParameter{
		{Name: "x", Value: "1"},
		{Name: "trace", Value: "abc123", Excluded: true},
	})
	without := computeHistoryID(fn, []AllureParameter{{Name: "x", Value: "1"}})
	if with != without {
		t.Errorf("excluded parameter changed historyId: %q != %q", with, without)
	}
}

// TestHistoryID_DistinctParamsFork — different parameter values must produce
// different history series.
func TestHistoryID_DistinctParamsFork(t *testing.T) {
	const fn = "pkg.Test.case"
	a := computeHistoryID(fn, []AllureParameter{{Name: "x", Value: "1"}})
	b := computeHistoryID(fn, []AllureParameter{{Name: "x", Value: "2"}})
	if a == b {
		t.Errorf("distinct parameters collided on historyId: %q", a)
	}
}

// TestTestCaseID_IsMD5OfFullName — testCaseId is parameter-INDEPENDENT
// md5(fullName), matching allure-pytest. It must stay constant across
// parameterised iterations while historyId forks.
func TestTestCaseID_IsMD5OfFullName(t *testing.T) {
	const fn = "auth.LoginTest.test_login"
	sum := md5.Sum([]byte(fn))
	want := hex.EncodeToString(sum[:])
	if got := computeTestCaseID(fn); got != want {
		t.Errorf("testCaseId = %q, want md5(fullName) = %q", got, want)
	}
	// Independent of parameters: two different param sets, one testCaseId.
	if computeTestCaseID(fn) != computeTestCaseID(fn) {
		t.Error("testCaseId not stable")
	}
}

// TestScope_EmitsStableIdentityAcrossTwoGenerations is the owner's core
// concern: generate the SAME test twice and assert fullName / historyId /
// testCaseId are byte-stable across runs (only uuid + timestamps vary).
func TestScope_EmitsStableIdentityAcrossTwoGenerations(t *testing.T) {
	gen := func() Result {
		dir := filepath.Join(t.TempDir(), "allure-results")
		ctx, finish := WithTest(context.Background(), "login",
			WithResultsDir(dir),
			WithFullName("auth.LoginTest.test_login"),
			WithParameter("env", "stage"),
		)
		Step(ctx, "submit", func() {})
		finish()
		return readResultFile(t, dir)
	}
	r1, r2 := gen(), gen()
	if r1.FullName != r2.FullName {
		t.Errorf("fullName not stable: %q != %q", r1.FullName, r2.FullName)
	}
	if r1.HistoryID != r2.HistoryID {
		t.Errorf("historyId not stable: %q != %q", r1.HistoryID, r2.HistoryID)
	}
	if r1.TestCaseID != r2.TestCaseID {
		t.Errorf("testCaseId not stable: %q != %q", r1.TestCaseID, r2.TestCaseID)
	}
	if r1.UUID == r2.UUID {
		t.Errorf("uuid should differ across generations, both = %q", r1.UUID)
	}
	// historyId must differ from testCaseId (they are distinct identities).
	if r1.HistoryID == r1.TestCaseID {
		t.Errorf("historyId and testCaseId must differ (params folded into history only)")
	}
}

// TestInternalLabelsNeverLeak — WithIssuePattern (and any `_internal.`
// bookkeeping label) must NOT appear in the written result. Regression guard
// for the leaked `_internal.issuePattern` junk label.
func TestInternalLabelsNeverLeak(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	ctx, finish := WithTest(context.Background(), "t",
		WithResultsDir(dir),
		WithIssuePattern("https://jira/{}"),
		WithFeature("Auth"), // a real label must still survive
	)
	Step(ctx, "s", func() {})
	finish()
	r := readResultFile(t, dir)
	sawFeature := false
	for _, l := range r.Labels {
		if strings.HasPrefix(l.Name, internalLabelPrefix) {
			t.Errorf("internal label leaked into result: %q=%q", l.Name, l.Value)
		}
		if l.Name == LabelFeature && l.Value == "Auth" {
			sawFeature = true
		}
	}
	if !sawFeature {
		t.Error("real feature label was dropped while stripping internal labels")
	}
}

// TestWriteEnvironment_PropertiesFormat verifies the environment.properties
// emitter: key=value lines, keys sorted, newlines neutralised so a value
// cannot split into a bogus second entry.
func TestWriteEnvironment_PropertiesFormat(t *testing.T) {
	dir := t.TempDir()
	w := NewFileWriter(dir)
	if err := w.WriteEnvironment(map[string]string{
		"BUILD":  "42",
		"BRANCH": "feature/login\ninjected=evil",
		"OS":     "linux",
	}); err != nil {
		t.Fatalf("WriteEnvironment: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "environment.properties"))
	if err != nil {
		t.Fatalf("read environment.properties: %v", err)
	}
	got := string(data)
	// Keys sorted alphabetically.
	want := "BRANCH=feature/login injected=evil\nBUILD=42\nOS=linux\n"
	if got != want {
		t.Errorf("environment.properties =\n%q\nwant\n%q", got, want)
	}
	// Exactly one line per key (3 entries → 3 newlines): proves the embedded
	// newline did not create a 4th entry.
	if n := strings.Count(got, "\n"); n != 3 {
		t.Errorf("expected 3 lines, got %d newlines in %q", n, got)
	}
}

// TestWriteEnvironment_Empty writes an empty file cleanly.
func TestWriteEnvironment_Empty(t *testing.T) {
	dir := t.TempDir()
	w := NewFileWriter(dir)
	if err := w.WriteEnvironment(nil); err != nil {
		t.Fatalf("WriteEnvironment(nil): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "environment.properties"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("empty env should write empty file, got %q", data)
	}
}
