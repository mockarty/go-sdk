// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envFrom(pairs map[string]string) func(string) string {
	return func(k string) string { return pairs[k] }
}

func TestParseTestPlanNormalisesNumericIDs(t *testing.T) {
	raw := []byte(`{"version":"1.0","tests":[{"id":11111,"selector":"my.company.SimpleTest.simpleTestOne"}]}`)
	plan, err := ParseTestPlan(raw, "plan.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Version != "1.0" {
		t.Fatalf("version = %q, want 1.0", plan.Version)
	}
	if len(plan.Entries) != 1 || plan.Entries[0].ID != "11111" {
		t.Fatalf("entries = %+v, want id 11111 as a string", plan.Entries)
	}
	if !plan.Matches([]string{"11111"}, nil) {
		t.Error("numeric id from the plan must match a string id from the test")
	}
	if !plan.Matches(nil, []string{"my.company.SimpleTest.simpleTestOne"}) {
		t.Error("selector must match")
	}
	if plan.Matches([]string{"9"}, []string{"other"}) {
		t.Error("unrelated ids/selectors must not match")
	}
}

func TestParseTestPlanEmptyTestsIsValidButEmpty(t *testing.T) {
	plan, err := ParseTestPlan([]byte(`{"version":"1.0","tests":[]}`), "p")
	if err != nil {
		t.Fatalf("an empty plan is well-formed, got error: %v", err)
	}
	if !plan.IsEmpty() {
		t.Fatal("plan with zero entries must report IsEmpty")
	}
	if plan.Matches([]string{"x"}, []string{"y"}) {
		t.Error("an empty plan must select nothing")
	}
}

func TestParseTestPlanUnknownVersionStillParses(t *testing.T) {
	// Forward-compat: a future schema carrying id/selector must not become
	// a silent full run, so we honour it rather than bail.
	plan, err := ParseTestPlan([]byte(`{"version":"2.0","tests":[{"id":"1"}]}`), "p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Version != "2.0" || !plan.Matches([]string{"1"}, nil) {
		t.Fatalf("plan = %+v, want the entry honoured", plan)
	}
}

func TestParseTestPlanRejectsBrokenDocuments(t *testing.T) {
	cases := map[string]string{
		"not json":         `{ this is not json`,
		"array root":       `[]`,
		"no tests key":     `{"version":"1.0"}`,
		"tests wrong type": `{"version":"1.0","tests":{}}`,
		"entry not object": `{"version":"1.0","tests":["nope"]}`,
		"entry empty":      `{"version":"1.0","tests":[{"name":"x"}]}`,
		"entry null id":    `{"version":"1.0","tests":[{"id":null,"selector":null}]}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTestPlan([]byte(raw), "plan.json"); err == nil {
				t.Fatal("want an error — a broken plan must never degrade into a full run")
			}
		})
	}
}

func TestLoadTestPlanFromNoEnvIsNoPlan(t *testing.T) {
	plan, err := LoadTestPlanFrom(envFrom(nil))
	if plan != nil || err != nil {
		t.Fatalf("got (%v, %v), want (nil, nil) — no plan configured is a normal full run", plan, err)
	}
	plan, err = LoadTestPlanFrom(envFrom(map[string]string{EnvTestPlanPath: "   "}))
	if plan != nil || err != nil {
		t.Fatalf("blank path: got (%v, %v), want (nil, nil)", plan, err)
	}
}

func TestLoadTestPlanFromModeOff(t *testing.T) {
	path := writePlan(t, `{"version":"1.0","tests":[{"selector":"a"}]}`)
	plan, err := LoadTestPlanFrom(envFrom(map[string]string{
		EnvTestPlanPath: path, EnvTestPlanMode: "off",
	}))
	if plan != nil || err != nil {
		t.Fatalf("mode=off: got (%v, %v), want (nil, nil)", plan, err)
	}
	// A typo in the opt-out must NOT silently re-enable the full run.
	plan, err = LoadTestPlanFrom(envFrom(map[string]string{
		EnvTestPlanPath: path, EnvTestPlanMode: "enfroce",
	}))
	if plan == nil || err != nil {
		t.Fatalf("typo'd mode: got (%v, %v), want the plan enforced", plan, err)
	}
}

func TestLoadTestPlanFromMissingFileIsAnError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.json")
	_, err := LoadTestPlanFrom(envFrom(map[string]string{EnvTestPlanPath: missing}))
	if err == nil {
		t.Fatal("a configured-but-missing plan must be an error, not a full run")
	}
	if !strings.Contains(err.Error(), "cannot read") {
		t.Errorf("error = %q, want it to say the plan could not be read", err)
	}
}

func TestLoadTestPlanFromDirectoryIsAnError(t *testing.T) {
	if _, err := LoadTestPlanFrom(envFrom(map[string]string{EnvTestPlanPath: t.TempDir()})); err == nil {
		t.Fatal("a directory is not a readable plan")
	}
}

func writePlan(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "testplan.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	return path
}

func TestTestPlanIdentityCoversEveryAddressableForm(t *testing.T) {
	cfg := config{
		name:       "TestLogin/happy",
		fullName:   "github.com/acme/auth::TestLogin/happy",
		pkg:        "github.com/acme/auth",
		testClass:  "TestLogin",
		testMethod: "happy",
		allureID:   "777",
		labels:     []AllureLabel{{Name: "ALLURE_ID", Value: "TC-9"}},
	}
	ids, selectors := testPlanIdentity(cfg, "TestLogin/happy")

	for _, want := range []string{"777", "TC-9"} {
		if !containsString(ids, want) {
			t.Errorf("ids %v missing %q", ids, want)
		}
	}
	for _, want := range []string{
		"TestLogin/happy",
		"github.com/acme/auth::TestLogin/happy",
		"github.com/acme/auth.TestLogin/happy",
		"TestLogin#happy",
		"TestLogin.happy",
		"github.com/acme/auth.TestLogin#happy",
	} {
		if !containsString(selectors, want) {
			t.Errorf("selectors %v missing %q", selectors, want)
		}
	}
}

func TestTestPlanIdentityFallsBackToComputedStableID(t *testing.T) {
	cfg := config{name: "TestX", fullName: "pkg::TestX", testClass: "TestX", testMethod: "TestX"}
	ids, _ := testPlanIdentity(cfg, "TestX")
	s := scope{cfg: cfg}
	if want := s.computeAllureStableID(); !containsString(ids, want) {
		t.Fatalf("ids %v must include the auto-computed AS_ID %q", ids, want)
	}
}

func TestTestPlanExitCode(t *testing.T) {
	full := &TestPlan{Path: "p", Entries: []TestPlanEntry{{ID: "1"}}}
	empty := &TestPlan{Path: "p"}
	cases := []struct {
		name     string
		code     int
		plan     *TestPlan
		err      error
		selected int64
		want     int
	}{
		{"no plan keeps the code", 0, nil, nil, 0, 0},
		{"plan error is a usage error", 0, nil, os.ErrNotExist, 0, TestPlanUsageExitCode},
		{"empty plan never exits 0", 0, empty, nil, 0, TestPlanNoTestsExitCode},
		{"plan matched nothing never exits 0", 0, full, nil, 0, TestPlanNoTestsExitCode},
		{"plan matched something keeps 0", 0, full, nil, 3, 0},
		{"a real failure is not masked", 1, full, nil, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := testPlanExitCode(tc.code, tc.plan, tc.err, tc.selected); got != tc.want {
				t.Fatalf("testPlanExitCode = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestActiveTestPlanIsMemoisedAndResettable(t *testing.T) {
	// Not parallel on purpose: it mutates process-wide plan state.
	t.Cleanup(resetTestPlanState)
	resetTestPlanState()
	t.Setenv(EnvTestPlanPath, writePlan(t, `{"version":"1.0","tests":[{"id":"1"}]}`))
	plan, err := activeTestPlan()
	if err != nil || plan == nil || len(plan.Entries) != 1 {
		t.Fatalf("activeTestPlan = (%v, %v)", plan, err)
	}
	// Memoised: a changed environment does not re-read until reset.
	t.Setenv(EnvTestPlanPath, "")
	if again, _ := activeTestPlan(); again != plan {
		t.Fatal("activeTestPlan must memoise the plan for the process")
	}
	resetTestPlanState()
	if after, err := activeTestPlan(); after != nil || err != nil {
		t.Fatalf("after reset = (%v, %v), want (nil, nil)", after, err)
	}
}

func TestWithAllureIDPinsTheASIDLabel(t *testing.T) {
	cfg := config{name: "n", fullName: "f", testClass: "C", testMethod: "m"}
	WithAllureID("PINNED-1")(&cfg)
	s := newScope(cfg, NewFileWriter(t.TempDir()))
	var got string
	for _, l := range s.result.Labels {
		if l.Name == LabelAllureID {
			got = l.Value
		}
	}
	if got != "PINNED-1" {
		t.Fatalf("AS_ID label = %q, want the pinned id (an auto-hash makes the plan's 'id' unmatchable)", got)
	}
}
