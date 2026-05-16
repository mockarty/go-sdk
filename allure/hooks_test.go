// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// TestLifecycleHooks_OrderingBeforeAllBeforeEachAfterEachAfterAll verifies
// the contract: BeforeAll runs once, then BeforeEach runs before EACH
// subtest, then the test body, then AfterEach (LIFO), and finally
// AfterAll once (LIFO) when the parent test completes.
func TestLifecycleHooks_OrderingBeforeAllBeforeEachAfterEachAfterAll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(ResultsDirEnv, dir)

	var order []string
	var mu sync.Mutex
	record := func(s string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, s)
	}

	t.Run("scoped", func(parent *testing.T) {
		BeforeAll(parent, "ba", func(_ *testing.T) { record("BeforeAll") })
		AfterAll(parent, "aa", func(_ *testing.T) { record("AfterAll") })
		BeforeEach(parent, "be", func(_ *testing.T) { record("BeforeEach") })
		AfterEach(parent, "ae", func(_ *testing.T) { record("AfterEach") })

		for _, n := range []string{"one", "two"} {
			n := n
			parent.Run(n, func(inner *testing.T) {
				RunWithHooks(inner, func(tt *testing.T) {
					record("Test:" + n)
					a := T(tt, WithResultsDir(dir))
					a.Step("body", func() {})
				})
			})
		}
	})

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"BeforeAll",
		"BeforeEach", "Test:one", "AfterEach",
		"BeforeEach", "Test:two", "AfterEach",
		"AfterAll",
	}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order mismatch:\n got = %v\nwant = %v", order, want)
	}

	// Verify a container json exists referencing the test UUIDs.
	entries, _ := os.ReadDir(dir)
	foundContainer := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-container.json") {
			foundContainer = true
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatalf("read container: %v", err)
			}
			var c Container
			if err := json.Unmarshal(data, &c); err != nil {
				t.Fatalf("decode container: %v", err)
			}
			if len(c.Children) != 2 {
				t.Errorf("container children = %d, want 2", len(c.Children))
			}
			if len(c.Befores) != 1 || c.Befores[0].Status != StatusPassed {
				t.Errorf("befores = %+v", c.Befores)
			}
			if len(c.Afters) != 1 || c.Afters[0].Status != StatusPassed {
				t.Errorf("afters = %+v", c.Afters)
			}
		}
	}
	if !foundContainer {
		t.Errorf("no container.json found")
	}
}

// TestLifecycleHooks_PanicMarksBroken verifies that a panic inside a hook
// is captured as a broken hook step rather than crashing the test.
func TestLifecycleHooks_PanicMarksBroken(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(ResultsDirEnv, dir)

	t.Run("scoped", func(parent *testing.T) {
		BeforeAll(parent, "boom", func(_ *testing.T) { panic("kaboom") })
		AfterAll(parent, "noop", func(_ *testing.T) {})
		parent.Run("body", func(inner *testing.T) {
			RunWithHooks(inner, func(tt *testing.T) {
				a := T(tt, WithResultsDir(dir))
				a.Step("body", func() {})
			})
		})
	})

	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "-container.json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var c Container
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(c.Befores) == 1 && c.Befores[0].Status == StatusBroken {
			found = true
		}
	}
	if !found {
		t.Errorf("expected broken befores hook in container")
	}
}

// TestParameterizedTest_DistinctHistoryIDs ensures each parameterised case
// gets a distinct history-id derived from its parameter values.
func TestParameterizedTest_DistinctHistoryIDs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(ResultsDirEnv, dir)
	type Payload struct {
		In, Want string
	}

	cases := []ParameterCase[Payload]{
		{"happy", Payload{"a", "b"}},
		{"empty", Payload{"", ""}},
		{"unicode-кир", Payload{"и", "к"}},
	}
	t.Run("scoped", func(parent *testing.T) {
		ParameterizedTest(parent, cases, func(tt *testing.T, p Payload) {
			a := T(tt, WithResultsDir(dir))
			a.Step("act", func() {})
			_ = p
		})
	})

	// Find all *-result.json. We expect 3 with distinct historyId values.
	files, _ := os.ReadDir(dir)
	historyIDs := map[string]bool{}
	var resultCount int
	for _, e := range files {
		if !strings.HasSuffix(e.Name(), "-result.json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var r Result
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		resultCount++
		historyIDs[r.HistoryID] = true
		// Each result must carry the case's parameters.
		if len(r.Parameters) == 0 {
			t.Errorf("result %s has no parameters", r.Name)
		}
	}
	if resultCount != 3 {
		t.Errorf("results = %d, want 3", resultCount)
	}
	if len(historyIDs) != 3 {
		t.Errorf("distinct historyIDs = %d, want 3", len(historyIDs))
	}
}

// TestParameterizedRows_PytestStyle covers the simpler rows API.
func TestParameterizedRows_PytestStyle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(ResultsDirEnv, dir)
	var ran int32
	t.Run("scoped", func(parent *testing.T) {
		ParameterizedRows(parent,
			[]string{"name", "n"},
			[][]string{
				{"a", "1"},
				{"b", "2"},
			},
			func(tt *testing.T, params map[string]string) {
				atomic.AddInt32(&ran, 1)
				a := T(tt, WithResultsDir(dir))
				a.Step("body", func() {})
				if params["name"] == "" {
					t.Errorf("missing name param: %v", params)
				}
			},
		)
	})
	if got := atomic.LoadInt32(&ran); got != 2 {
		t.Errorf("ran = %d, want 2", got)
	}
}

// TestLinkPattern_Expansion verifies env-driven link URL expansion for
// issue / tms / custom link types.
func TestLinkPattern_Expansion(t *testing.T) {
	t.Setenv(IssuePatternEnv, "https://jira/{0}")
	t.Setenv(TmsPatternEnv, "https://tms/{}")
	t.Setenv(LinkPatternPfx+"DOCS", "https://docs/{0}")

	cases := []struct {
		pattern, id, want string
	}{
		{"https://jira/{0}", "JIRA-1", "https://jira/JIRA-1"},
		{"https://jira/{}", "JIRA-2", "https://jira/JIRA-2"},
		{"", "raw", "raw"},
		{"https://prefix/", "tail", "https://prefix/tail"},
	}
	for _, c := range cases {
		if got := expandLinkPattern(c.pattern, c.id); got != c.want {
			t.Errorf("expand(%q,%q) = %q, want %q", c.pattern, c.id, got, c.want)
		}
	}

	if got := resolveLinkURL(LinkTypeIssue, "JIRA-9", ""); got != "https://jira/JIRA-9" {
		t.Errorf("issue resolve = %q", got)
	}
	if got := resolveLinkURL(LinkTypeTMS, "T-9", ""); got != "https://tms/T-9" {
		t.Errorf("tms resolve = %q", got)
	}
	if got := resolveLinkURL("docs", "page-1", ""); got != "https://docs/page-1" {
		t.Errorf("docs resolve = %q", got)
	}
}

// TestStageLifecycle_TransitionsBetweenStates verifies scheduled →
// running → finished progression.
func TestStageLifecycle_TransitionsBetweenStates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		// Before any step push, scope stage should be scheduled.
		a.scope.mu.Lock()
		got := a.scope.result.Stage
		a.scope.mu.Unlock()
		if got != StageScheduled {
			t.Errorf("initial stage = %s, want scheduled", got)
		}
		a.Step("inner-step", func() {
			a.scope.mu.Lock()
			st := a.scope.result.Stage
			a.scope.mu.Unlock()
			if st != StageRunning {
				t.Errorf("during step stage = %s, want running", st)
			}
		})
	})
	r := readResultFile(t, dir)
	if r.Stage != StageFinished {
		t.Errorf("final stage = %s, want finished", r.Stage)
	}
	if len(r.Steps) != 1 || r.Steps[0].Stage != StageFinished {
		t.Errorf("step stage = %s, want finished", r.Steps[0].Stage)
	}
}

// TestAutoInjectedLabels covers host / thread / package / testClass /
// testMethod / AS_ID populated automatically on T().
func TestAutoInjectedLabels(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("subtest_name", func(inner *testing.T) {
		_ = T(inner, WithResultsDir(dir))
	})
	r := readResultFile(t, dir)
	labels := map[string]string{}
	for _, l := range r.Labels {
		labels[l.Name] = l.Value
	}
	for _, key := range []string{
		LabelHost, LabelThread, LabelPackage,
		LabelTestClass, LabelTestMethod, LabelAllureID,
		LabelFramework, LabelLanguage,
	} {
		if labels[key] == "" {
			t.Errorf("auto-label %s missing (labels=%v)", key, labels)
		}
	}
	if labels[LabelFramework] != FrameworkName {
		t.Errorf("framework label = %q, want %q", labels[LabelFramework], FrameworkName)
	}
	if labels[LabelTestClass] != "TestAutoInjectedLabels" {
		t.Errorf("testClass = %q, want TestAutoInjectedLabels", labels[LabelTestClass])
	}
	if labels[LabelTestMethod] != "subtest_name" {
		t.Errorf("testMethod = %q, want subtest_name", labels[LabelTestMethod])
	}
}

// TestParameterMode_HiddenMaskedExcluded verifies the per-parameter mode
// flags serialise correctly into the result JSON.
func TestParameterMode_HiddenMaskedExcluded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir),
			WithParameterEx("token", "secret", ParameterModeMasked, false),
			WithParameterEx("timestamp", "now", ParameterModeDefault, true),
			WithParameterEx("internal", "x", ParameterModeHidden, false),
		)
		a.Step("body", func() {})
	})
	r := readResultFile(t, dir)
	modes := map[string]ParameterMode{}
	excluded := map[string]bool{}
	for _, p := range r.Parameters {
		modes[p.Name] = p.Mode
		excluded[p.Name] = p.Excluded
	}
	if modes["token"] != ParameterModeMasked {
		t.Errorf("token mode = %q, want masked", modes["token"])
	}
	if modes["internal"] != ParameterModeHidden {
		t.Errorf("internal mode = %q, want hidden", modes["internal"])
	}
	if !excluded["timestamp"] {
		t.Errorf("timestamp excluded = false, want true")
	}
}

// TestParallelStep_BranchesAreRecorded verifies the parallel step helper
// runs branches concurrently and records each as a child step.
func TestParallelStep_BranchesAreRecorded(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		var ran int32
		ParallelStep(a.ctx, "fan-out", map[string]func(){
			"branch-a": func() { atomic.AddInt32(&ran, 1) },
			"branch-b": func() { atomic.AddInt32(&ran, 1) },
			"branch-c": func() { atomic.AddInt32(&ran, 1) },
		})
		if atomic.LoadInt32(&ran) != 3 {
			t.Errorf("ran = %d, want 3", atomic.LoadInt32(&ran))
		}
	})
	r := readResultFile(t, dir)
	if len(r.Steps) != 1 || r.Steps[0].Name != "fan-out" {
		t.Fatalf("parent step missing: %+v", r.Steps)
	}
	if len(r.Steps[0].Steps) != 3 {
		t.Errorf("branches = %d, want 3", len(r.Steps[0].Steps))
	}
}

// TestAttachJSON_MarshalsArbitraryValue ensures any json-serialisable
// value attaches correctly.
func TestAttachJSON_MarshalsArbitraryValue(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	type Resp struct{ OK bool }
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.AttachJSON("resp", Resp{OK: true})
		a.AttachJSON("raw", []byte(`{"k":1}`))
	})
	r := readResultFile(t, dir)
	if len(r.Attachments) != 2 {
		t.Fatalf("attachments = %d, want 2", len(r.Attachments))
	}
	for _, att := range r.Attachments {
		if att.Type != "application/json" {
			t.Errorf("att %s type = %q, want application/json", att.Name, att.Type)
		}
		body, err := os.ReadFile(filepath.Join(dir, att.Source))
		if err != nil {
			t.Errorf("read attachment: %v", err)
		}
		if len(body) == 0 {
			t.Errorf("empty attachment body for %s", att.Name)
		}
	}
}

// TestStatusPriorityBubbling verifies broken > failed > skipped > passed.
func TestStatusPriorityBubbling(t *testing.T) {
	cases := []struct {
		name     string
		statuses []Status
		want     Status
	}{
		{"all passed", []Status{StatusPassed, StatusPassed}, StatusPassed},
		{"one skipped", []Status{StatusPassed, StatusSkipped}, StatusSkipped},
		{"failed beats skipped", []Status{StatusSkipped, StatusFailed}, StatusFailed},
		{"broken beats failed", []Status{StatusFailed, StatusBroken}, StatusBroken},
		{"broken beats all", []Status{StatusPassed, StatusSkipped, StatusFailed, StatusBroken, StatusPassed}, StatusBroken},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "allure-results")
			t.Run("inner", func(inner *testing.T) {
				a := T(inner, WithResultsDir(dir))
				for i, s := range c.statuses {
					s := s
					i := i
					a.Step(s.String(), func() {
						h := BeginStep(a.ctx, "sub")
						_ = i
						switch s {
						case StatusFailed:
							h.Fail("nope")
						case StatusBroken:
							h.Broken("boom", "stack")
						case StatusSkipped:
							h.Skip("skipped")
						default:
							h.End()
						}
					})
				}
			})
			r := readResultFile(t, dir)
			if r.Status != c.want {
				t.Errorf("aggregate = %s, want %s (statuses=%v)", r.Status, c.want, c.statuses)
			}
		})
	}
}

// String surface for Status (helper for log messages — also documents
// that Status is a string alias).
func (s Status) String() string { return string(s) }
