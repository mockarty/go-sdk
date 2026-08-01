// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// testplan.go — consumption of the Allure test plan (ALLURE_TESTPLAN_PATH).
//
// Allure TestOps — and Mockarty's own `mockarty-cli allure rerun-failed` /
// `allure selection-plan` — drive SELECTIVE execution by writing a
// testplan.json and exporting ALLURE_TESTPLAN_PATH. The adapter is expected
// to read that file and run ONLY the listed tests.
//
// File format (schema version "1.0"):
//
//	{
//	  "version": "1.0",
//	  "tests": [
//	    {"id": 11111, "selector": "my.company.SimpleTest.simpleTestOne"}
//	  ]
//	}
//
// `id` is the Allure id (the value carried on results as the AS_ID /
// ALLURE_ID label); `selector` is a full-name style unique identifier.
// At least one of the two must be present on an entry, and a test is
// selected when EITHER matches.
//
// Behaviour — deliberately stricter than the reference adapters, which
// silently fall back to a full run:
//
//	env unset / empty ........... no filtering, normal run
//	MOCKARTY_TESTPLAN_MODE=off .. no filtering, even with the path set
//	plan with N entries ......... only matching tests run, the rest skip
//	plan with "tests": [] ....... NOTHING runs, reported explicitly, and
//	                              TestMain exits non-zero
//	missing/unreadable/broken ... hard error — never a silent full run
//
// A user who asked to re-run 3 failed tests must never get 3000 executed
// tests and a green tick — nor a green tick for zero executed tests.

package allure

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// Environment contract. Identical spelling in the Python and Java SDKs.
const (
	// EnvTestPlanPath names the testplan.json to honour.
	EnvTestPlanPath = "ALLURE_TESTPLAN_PATH"
	// EnvTestPlanMode is the escape hatch: "off" ignores the plan.
	EnvTestPlanMode = "MOCKARTY_TESTPLAN_MODE"
)

// Test-plan modes.
const (
	TestPlanModeEnforce = "enforce"
	TestPlanModeOff     = "off"
)

// SupportedTestPlanVersion is the only schema version Allure TestOps emits
// today. A plan declaring another version is still honoured as long as it
// carries a usable "tests" array — refusing it outright would turn a
// forward-compatible plan into a hard stop, while accepting it can never
// widen the selection.
const SupportedTestPlanVersion = "1.0"

// Exit codes returned by [TestMain] for the two "the plan could not be
// honoured" outcomes. They mirror pytest's usage-error (4) and
// no-tests-collected (5) codes so a polyglot CI pipeline can branch on the
// same numbers regardless of which SDK produced them.
const (
	TestPlanUsageExitCode   = 4
	TestPlanNoTestsExitCode = 5
)

// TestPlanEntry is one "tests[]" element. At least one field is non-empty.
type TestPlanEntry struct {
	ID       string `json:"id,omitempty"`
	Selector string `json:"selector,omitempty"`
}

// TestPlan is a parsed, validated testplan.json.
type TestPlan struct {
	Path    string
	Version string
	Entries []TestPlanEntry
}

// IsEmpty reports whether the plan selects nothing ("tests": []).
func (p *TestPlan) IsEmpty() bool { return p == nil || len(p.Entries) == 0 }

// Matches reports whether any entry matches one of the test's ids or
// selectors. Empty candidate strings never match.
func (p *TestPlan) Matches(ids, selectors []string) bool {
	if p == nil {
		return false
	}
	for _, e := range p.Entries {
		if e.ID != "" && containsString(ids, e.ID) {
			return true
		}
		if e.Selector != "" && containsString(selectors, e.Selector) {
			return true
		}
	}
	return false
}

func containsString(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// rawTestPlan mirrors the on-disk document. `id` may be a JSON number, so
// it is decoded through json.RawMessage and normalised to a string.
type rawTestPlan struct {
	Version *json.RawMessage   `json:"version"`
	Tests   *[]rawTestPlanCase `json:"tests"`
}

type rawTestPlanCase struct {
	ID       *json.RawMessage `json:"id"`
	Selector *json.RawMessage `json:"selector"`
}

// ParseTestPlan parses and validates plan bytes. Every rejection is an
// error rather than a fallback, because the fallback would be "run the
// whole suite", which is exactly what the caller did not ask for.
func ParseTestPlan(raw []byte, path string) (*TestPlan, error) {
	var doc rawTestPlan
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s=%s: not a valid Allure test plan (%w); refusing to run the full suite — fix the plan or unset %s",
			EnvTestPlanPath, path, err, EnvTestPlanPath)
	}
	if doc.Tests == nil {
		return nil, fmt.Errorf("%s=%s: the plan has no 'tests' array; a plan that selects nothing cannot be honoured, and silently running the whole suite is worse",
			EnvTestPlanPath, path)
	}
	plan := &TestPlan{Path: path, Version: rawToString(doc.Version), Entries: make([]TestPlanEntry, 0, len(*doc.Tests))}
	for i, tc := range *doc.Tests {
		id := rawToString(tc.ID)
		selector := rawToString(tc.Selector)
		if id == "" && selector == "" {
			return nil, fmt.Errorf("%s=%s: tests[%d] has neither 'id' nor 'selector' — it can never match a test",
				EnvTestPlanPath, path, i)
		}
		plan.Entries = append(plan.Entries, TestPlanEntry{ID: id, Selector: selector})
	}
	return plan, nil
}

// rawToString normalises a plan scalar: JSON strings keep their value,
// numbers are rendered verbatim, null/absent become "". Booleans and
// composites are rejected by returning "" (the caller then reports the
// entry as unusable rather than matching "true" against a test id).
func rawToString(raw *json.RawMessage) string {
	if raw == nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(*raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(*raw, &s); err != nil {
			return ""
		}
		return strings.TrimSpace(s)
	}
	if trimmed[0] == '-' || (trimmed[0] >= '0' && trimmed[0] <= '9') {
		return trimmed
	}
	return ""
}

// TestPlanMode resolves EnvTestPlanMode through getenv. Unknown values fall
// back to "enforce" on purpose: a typo in the opt-out must not silently
// re-enable the full run.
func TestPlanMode(getenv func(string) string) string {
	switch strings.ToLower(strings.TrimSpace(getenv(EnvTestPlanMode))) {
	case "off", "false", "0", "no", "disabled":
		return TestPlanModeOff
	default:
		return TestPlanModeEnforce
	}
}

// LoadTestPlanFrom reads the plan named by EnvTestPlanPath using the given
// environment accessor. Returns (nil, nil) when no plan is configured — the
// caller then runs unfiltered. Returns an error when a plan IS configured
// but cannot be read or parsed.
func LoadTestPlanFrom(getenv func(string) string) (*TestPlan, error) {
	path := strings.TrimSpace(getenv(EnvTestPlanPath))
	if path == "" {
		return nil, nil
	}
	if TestPlanMode(getenv) == TestPlanModeOff {
		return nil, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s=%s: cannot read the test plan (%w); refusing to run the full suite — a selective run that silently becomes a full run is worse than a failed one",
			EnvTestPlanPath, path, err)
	}
	return ParseTestPlan(raw, path)
}

// LoadTestPlan is LoadTestPlanFrom(os.Getenv).
func LoadTestPlan() (*TestPlan, error) { return LoadTestPlanFrom(os.Getenv) }

// ── process-wide plan state ─────────────────────────────────────────────

var (
	testPlanOnce sync.Once
	testPlanVal  *TestPlan
	testPlanErr  error

	// Counters so TestMain can turn "nothing ran" into a non-zero exit.
	testPlanSelected atomic.Int64
	testPlanSkipped  atomic.Int64
	testPlanBanner   sync.Once
)

// activeTestPlan loads (once per process) the plan named by the
// environment. Both return values are nil when no plan is configured.
func activeTestPlan() (*TestPlan, error) {
	testPlanOnce.Do(func() { testPlanVal, testPlanErr = LoadTestPlan() })
	return testPlanVal, testPlanErr
}

// resetTestPlanState clears the memoised plan. Test-only.
func resetTestPlanState() {
	testPlanOnce = sync.Once{}
	testPlanBanner = sync.Once{}
	testPlanVal, testPlanErr = nil, nil
	testPlanSelected.Store(0)
	testPlanSkipped.Store(0)
}

// TestPlanStats reports how the active plan partitioned this process's
// tests. Both counters stay 0 when no plan is configured.
func TestPlanStats() (selected, skipped int64) {
	return testPlanSelected.Load(), testPlanSkipped.Load()
}

// testPlanIdentity derives every id and selector a test can be addressed
// by, from an assembled config. Kept pure so it is unit-testable.
//
// Selector shapes cover what a plan realistically carries: the Go test name
// (`TestLogin/subtest`), the Allure fullName this SDK reports
// (`package::TestLogin`), the dotted form the Allure TestOps docs use for
// `selector` (`package.TestLogin`), and the class/method pair
// (`TestLogin#subtest`) that mirrors the Java adapter.
func testPlanIdentity(cfg config, testName string) (ids, selectors []string) {
	add := func(dst *[]string, v string) {
		v = strings.TrimSpace(v)
		if v == "" || containsString(*dst, v) {
			return
		}
		*dst = append(*dst, v)
	}

	// Ids: explicit pin first, then any user label carrying an Allure id,
	// then the auto-computed stable id.
	add(&ids, cfg.allureID)
	for _, l := range cfg.labels {
		switch strings.ToLower(l.Name) {
		case "as_id", "allure_id", "allureid":
			add(&ids, l.Value)
		}
	}
	s := scope{cfg: cfg}
	add(&ids, s.computeAllureStableID())

	add(&selectors, testName)
	add(&selectors, cfg.fullName)
	if cfg.pkg != "" && testName != "" {
		add(&selectors, cfg.pkg+"::"+testName)
		add(&selectors, cfg.pkg+"."+testName)
		add(&selectors, cfg.pkg+"/"+testName)
	}
	if cfg.testClass != "" && cfg.testMethod != "" {
		add(&selectors, cfg.testClass+"#"+cfg.testMethod)
		add(&selectors, cfg.testClass+"."+cfg.testMethod)
		if cfg.pkg != "" {
			add(&selectors, cfg.pkg+"."+cfg.testClass+"#"+cfg.testMethod)
			add(&selectors, cfg.pkg+"."+cfg.testClass+"."+cfg.testMethod)
		}
	}
	return ids, selectors
}

// applyTestPlan enforces the active plan for one test.
//
//   - no plan            → returns, the test runs normally;
//   - unreadable plan    → t.Fatalf, so the suite goes RED instead of
//     quietly running everything;
//   - empty plan         → t.Skip for every test + a banner, and TestMain
//     turns the run into a non-zero exit;
//   - test not selected  → t.Skip.
//
// Returns true when the test may proceed. When it returns false the calling
// goroutine has already been unwound by t.Skip/t.Fatal.
func applyTestPlan(t *testing.T, cfg config, testName string) bool {
	t.Helper()
	plan, err := activeTestPlan()
	if err != nil {
		t.Fatalf("mockarty/allure: %v", err)
		return false
	}
	if plan == nil {
		return true
	}
	if plan.IsEmpty() {
		announceEmptyTestPlan(plan)
		testPlanSkipped.Add(1)
		t.Skipf("mockarty/allure: %s=%s selects no tests (\"tests\": []) — nothing to run",
			EnvTestPlanPath, plan.Path)
		return false
	}
	ids, selectors := testPlanIdentity(cfg, testName)
	if !plan.Matches(ids, selectors) {
		testPlanSkipped.Add(1)
		t.Skipf("mockarty/allure: not selected by %s=%s", EnvTestPlanPath, plan.Path)
		return false
	}
	testPlanSelected.Add(1)
	return true
}

// announceEmptyTestPlan prints the "this run proves nothing" banner once.
func announceEmptyTestPlan(plan *TestPlan) {
	testPlanBanner.Do(func() {
		fmt.Fprintf(os.Stderr,
			"\nmockarty/allure: the Allure test plan %s is EMPTY (\"tests\": []) — every test is skipped.\n"+
				"This run proves nothing; it is NOT a pass. Use allure.TestMain to make it a non-zero exit.\n\n",
			plan.Path)
	})
}

// SkipIfNotSelected enforces the active Allure test plan for a test that
// does not go through [T].
//
//	func TestLogin(t *testing.T) {
//	    allure.SkipIfNotSelected(t)
//	    ...
//	}
//
// Extra ids/selectors (e.g. an @AllureId equivalent the suite tracks itself)
// can be supplied via [WithAllureID] / [WithLabel] / [WithFullName] options,
// mirroring what [T] would have derived.
func SkipIfNotSelected(t *testing.T, opts ...Option) {
	t.Helper()
	cfg := config{name: t.Name(), fullName: t.Name()}
	if pkg, _ := detectCallerPackage(2); pkg != "" {
		cfg.pkg = pkg
	}
	cls, mtd := splitTestPath(t.Name())
	cfg.testClass, cfg.testMethod = cls, mtd
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.fullName == "" || cfg.fullName == t.Name() {
		if cfg.pkg != "" {
			cfg.fullName = cfg.pkg + "::" + t.Name()
		}
	}
	applyTestPlan(t, cfg, t.Name())
}

// testPlanExitCode maps the end-of-run plan state onto a process exit code.
// `code` is the code testing.M produced. Exported behaviour lives in
// [TestMain]; kept separate so it is unit-testable without a real m.Run.
func testPlanExitCode(code int, plan *TestPlan, planErr error, selected int64) int {
	if planErr != nil {
		return TestPlanUsageExitCode
	}
	if plan == nil || code != 0 {
		return code
	}
	if plan.IsEmpty() || selected == 0 {
		return TestPlanNoTestsExitCode
	}
	return code
}
