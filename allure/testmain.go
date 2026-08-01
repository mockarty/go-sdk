// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestMain is the optional convenience entry-point for test packages that
// want global Allure plumbing — writing executor.json on start, flushing
// any open suite containers on exit, and propagating the *_test.go process
// exit code.
//
// Drop it into the package next to your tests:
//
//	func TestMain(m *testing.M) {
//	    os.Exit(allure.TestMain(m))
//	}
//
// Returns the testing.Main exit code so callers can pass it straight to
// os.Exit. Calling it manually is optional — every other allure helper
// works without TestMain, this just adds the executor.json + final flush.
// It also owns the Allure test-plan lifecycle (ALLURE_TESTPLAN_PATH):
//
//   - a plan that cannot be read/parsed aborts BEFORE any test runs and
//     returns [TestPlanUsageExitCode] (4) — never a silent full run;
//   - a plan that selected nothing (empty plan, or no test matched)
//     returns [TestPlanNoTestsExitCode] (5) — never a green exit 0 for a
//     run that proved nothing.
//
// Both codes mirror pytest's, so a polyglot pipeline branches on the same
// numbers whichever SDK produced them.
func TestMain(m *testing.M) int {
	plan, planErr := activeTestPlan()
	if planErr != nil {
		fmt.Fprintf(os.Stderr, "mockarty/allure: %v\n", planErr)
		return TestPlanUsageExitCode
	}
	dir := ResolveResultsDir("")
	if err := os.MkdirAll(dir, 0o755); err == nil {
		w := NewFileWriter(dir)
		_ = w.WriteExecutor(defaultExecutor())
	}
	code := m.Run()
	flushAllSuites()
	selected, _ := TestPlanStats()
	if final := testPlanExitCode(code, plan, planErr, selected); final != code {
		fmt.Fprintf(os.Stderr,
			"mockarty/allure: the Allure test plan %s selected 0 of this package's tests — "+
				"nothing was executed. This run proves nothing; exiting %d instead of 0.\n",
			plan.Path, final)
		return final
	}
	return code
}

// defaultExecutor builds the executor.json with run identity inferred
// from the environment. CI populates ALLURE_EXECUTOR_* variables (mirror
// of the Java SDK's `-Dallure.executor.*` props).
func defaultExecutor() Executor {
	e := Executor{
		Name:       envOr("ALLURE_EXECUTOR_NAME", FrameworkName),
		Type:       envOr("ALLURE_EXECUTOR_TYPE", "go-test"),
		URL:        os.Getenv("ALLURE_EXECUTOR_URL"),
		ReportName: os.Getenv("ALLURE_EXECUTOR_REPORT_NAME"),
		ReportURL:  os.Getenv("ALLURE_EXECUTOR_REPORT_URL"),
		BuildName:  os.Getenv("ALLURE_EXECUTOR_BUILD_NAME"),
		BuildOrder: os.Getenv("ALLURE_EXECUTOR_BUILD_ORDER"),
		BuildURL:   os.Getenv("ALLURE_EXECUTOR_BUILD_URL"),
	}
	if e.Name == "" {
		e.Name = runtime.GOOS + "/" + runtime.GOARCH
	}
	return e
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// flushAllSuites walks the suite registry and writes any leftover
// containers — invoked from TestMain and from package-level cleanup.
func flushAllSuites() {
	suiteRegistryMu.Lock()
	classes := make([]string, 0, len(suiteRegistry))
	for k := range suiteRegistry {
		classes = append(classes, k)
	}
	suiteRegistryMu.Unlock()
	for _, c := range classes {
		flushSuite(c)
	}
}

// CleanResultsDir wipes the allure-results directory before a new run.
// Optional — by default Allure CLI overwrites on regenerate. Useful for
// long-lived CI workspaces that accumulate stale results.
//
// Returns os.ErrNotExist if the directory does not exist (treat as
// non-fatal in callers).
func CleanResultsDir(dir string) error {
	if dir == "" {
		dir = ResolveResultsDir("")
	}
	return os.RemoveAll(filepath.Clean(dir))
}

// processStartOnce ensures the executor.json is written at most once per
// process — useful when tests call helpers (not TestMain) and we want the
// run-level metadata to land deterministically.
var processStartOnce sync.Once

// EnsureExecutor writes executor.json once per process even if TestMain
// is not used. Suites that prefer the `func TestMain` flow can ignore it;
// the convenience helpers (Suite/RunWithHooks) call this implicitly.
func EnsureExecutor() {
	processStartOnce.Do(func() {
		dir := ResolveResultsDir("")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		w := NewFileWriter(dir)
		_ = w.WriteExecutor(defaultExecutor())
	})
}

// Now is an injectable clock so tests can pin timestamps. Defaults to
// time.Now; can be overridden via SetClock for deterministic snapshots.
var nowFn = time.Now

// SetClock injects a deterministic clock — used by test-fixture tests
// that need reproducible Start/Stop values. Production callers must not
// touch this.
func SetClock(fn func() time.Time) {
	if fn != nil {
		nowFn = fn
	}
}
