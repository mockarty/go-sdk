// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// testplan_e2e_test.go — proves the Allure test plan actually restricts a
// real `go test` run.
//
// The unit tests above cover parsing and matching; this one compiles the
// fixture suite in allure/internal/testplanprobe into a test binary and runs
// it under different ALLURE_TESTPLAN_PATH values, asserting BOTH which tests
// executed AND the process exit code. Running the compiled binary directly
// (instead of `go test`) is what makes the exit code observable — `go test`
// collapses every non-zero code to 1.

package allure

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildProbe compiles the fixture suite once per test process.
func buildProbe(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles a test binary; skipped in -short")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}
	bin := filepath.Join(t.TempDir(), "probe.test")
	cmd := exec.Command(goBin, "test", "-c", "-o", bin, "./allure/internal/testplanprobe")
	cmd.Dir = ".." // module root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compiling the probe suite failed: %v\n%s", err, out)
	}
	return bin
}

type probeRun struct {
	output string
	code   int
}

func (r probeRun) ran(name string) bool     { return strings.Contains(r.output, "--- PASS: "+name) }
func (r probeRun) skipped(name string) bool { return strings.Contains(r.output, "--- SKIP: "+name) }

// runProbe executes the compiled fixture suite with the given extra env.
func runProbe(t *testing.T, bin string, env map[string]string) probeRun {
	t.Helper()
	cmd := exec.Command(bin, "-test.v")
	cmd.Env = append(os.Environ(),
		ResultsDirEnv+"="+t.TempDir(),
		// Inherited state from the outer run must not leak in.
		EnvTestPlanPath+"=",
		EnvTestPlanMode+"=",
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running the probe suite failed: %v\n%s", err, out)
		}
		code = exitErr.ExitCode()
	}
	return probeRun{output: string(out), code: code}
}

func TestE2ENoPlanRunsEverything(t *testing.T) {
	// Regression guard: without the env var nothing is filtered.
	run := runProbe(t, buildProbe(t), nil)
	if run.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", run.code, run.output)
	}
	for _, name := range []string{"TestAlpha", "TestBeta", "TestGamma", "TestDelta"} {
		if !run.ran(name) {
			t.Errorf("%s did not run without a plan\n%s", name, run.output)
		}
	}
}

func TestE2EPlanSelectorRunsOnlyListedTest(t *testing.T) {
	bin := buildProbe(t)
	plan := writePlan(t, `{"version":"1.0","tests":[
		{"selector":"github.com/mockarty/mockarty-go/allure/internal/testplanprobe::TestAlpha"}
	]}`)
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: plan})
	if run.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", run.code, run.output)
	}
	if !run.ran("TestAlpha") {
		t.Errorf("the selected test did not run\n%s", run.output)
	}
	for _, name := range []string{"TestBeta", "TestGamma", "TestDelta"} {
		if !run.skipped(name) {
			t.Errorf("%s was NOT skipped — the plan was ignored\n%s", name, run.output)
		}
	}
}

func TestE2EPlanSelectsByGoTestName(t *testing.T) {
	bin := buildProbe(t)
	plan := writePlan(t, `{"version":"1.0","tests":[{"selector":"TestGamma"}]}`)
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: plan})
	if !run.ran("TestGamma") || !run.skipped("TestAlpha") {
		t.Fatalf("plain go test name must be a usable selector\n%s", run.output)
	}
}

func TestE2EPlanSelectsByAllureID(t *testing.T) {
	bin := buildProbe(t)
	// TestBeta pins WithAllureID("777"); the plan addresses it numerically,
	// exactly as Allure TestOps writes it.
	plan := writePlan(t, `{"version":"1.0","tests":[{"id":777}]}`)
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: plan})
	if !run.ran("TestBeta") {
		t.Errorf("the test pinned with WithAllureID was not selected by id\n%s", run.output)
	}
	if !run.skipped("TestAlpha") || !run.skipped("TestGamma") {
		t.Errorf("unlisted tests must be skipped\n%s", run.output)
	}
}

func TestE2EEmptyPlanRunsNothingAndExitsNonZero(t *testing.T) {
	// The headline bug: an empty plan must NOT read as "everything passed".
	bin := buildProbe(t)
	plan := writePlan(t, `{"version":"1.0","tests":[]}`)
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: plan})
	if run.code != TestPlanNoTestsExitCode {
		t.Fatalf("exit = %d, want %d — an empty plan must never exit 0\n%s",
			run.code, TestPlanNoTestsExitCode, run.output)
	}
	for _, name := range []string{"TestAlpha", "TestBeta", "TestGamma", "TestDelta"} {
		if !run.skipped(name) {
			t.Errorf("%s ran under an empty plan\n%s", name, run.output)
		}
	}
	if !strings.Contains(run.output, "is EMPTY") || !strings.Contains(run.output, "proves nothing") {
		t.Errorf("the empty plan was not reported explicitly\n%s", run.output)
	}
}

func TestE2EPlanMatchingNothingExitsNonZero(t *testing.T) {
	bin := buildProbe(t)
	plan := writePlan(t, `{"version":"1.0","tests":[{"selector":"does.not.exist"}]}`)
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: plan})
	if run.code != TestPlanNoTestsExitCode {
		t.Fatalf("exit = %d, want %d — zero executed tests must never exit 0\n%s",
			run.code, TestPlanNoTestsExitCode, run.output)
	}
}

func TestE2EBrokenPlanIsAnErrorNotAFullRun(t *testing.T) {
	bin := buildProbe(t)
	broken := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(broken, []byte("{ this is not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := runProbe(t, bin, map[string]string{EnvTestPlanPath: broken})
	if run.code != TestPlanUsageExitCode {
		t.Fatalf("exit = %d, want %d\n%s", run.code, TestPlanUsageExitCode, run.output)
	}
	if strings.Contains(run.output, "--- PASS") {
		t.Errorf("a broken plan must not run a single test\n%s", run.output)
	}
	if !strings.Contains(run.output, "not a valid Allure test plan") {
		t.Errorf("the failure was not explained\n%s", run.output)
	}
}

func TestE2EMissingPlanFileIsAnError(t *testing.T) {
	bin := buildProbe(t)
	run := runProbe(t, bin, map[string]string{
		EnvTestPlanPath: filepath.Join(t.TempDir(), "absent.json"),
	})
	if run.code != TestPlanUsageExitCode {
		t.Fatalf("exit = %d, want %d\n%s", run.code, TestPlanUsageExitCode, run.output)
	}
	if !strings.Contains(run.output, "cannot read the test plan") {
		t.Errorf("the failure was not explained\n%s", run.output)
	}
}

func TestE2EModeOffRestoresTheFullRun(t *testing.T) {
	bin := buildProbe(t)
	plan := writePlan(t, `{"version":"1.0","tests":[{"selector":"TestAlpha"}]}`)
	run := runProbe(t, bin, map[string]string{
		EnvTestPlanPath: plan,
		EnvTestPlanMode: "off",
	})
	if run.code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", run.code, run.output)
	}
	for _, name := range []string{"TestAlpha", "TestBeta", "TestGamma", "TestDelta"} {
		if !run.ran(name) {
			t.Errorf("%s did not run with MOCKARTY_TESTPLAN_MODE=off\n%s", name, run.output)
		}
	}
}
