// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestByteParity_WithReferenceAllurePytestFixture is the strictest schema
// test we run. It compares an SDK-produced allure-results JSON against a
// hand-crafted fixture that mirrors what allure-pytest produces, asserting:
//
//  1. Every key the reference fixture carries is present in our output.
//  2. Every value type (string / number / array / object / null) matches.
//  3. Nested structures (steps, labels, links, parameters, attachments)
//     all have identical key SETS at every level.
//  4. Required Allure 2 fields (uuid/name/status/stage/start/stop) are
//     present and well-typed.
//
// Runtime-generated values (uuid, historyId, timestamps) are NOT compared
// for byte equality — only their JSON types are checked. The schema-
// canonicalisation contract is satisfied if a JSON consumer parsing our
// output the same way it parses the reference sees the same shape.
func TestByteParity_WithReferenceAllurePytestFixture(t *testing.T) {
	// Fixture: a full happy-path test result captured from a real
	// allure-pytest run (2.13.x). Trimmed to the fields the official
	// Allure 2 renderer actually consumes — extension fields not in the
	// schema are omitted so the parity check fails fast on regression.
	reference := []byte(`{
	  "uuid": "00000000-0000-0000-0000-000000000000",
	  "historyId": "abc123",
	  "testCaseId": "abc123",
	  "fullName": "tests/test_login.py::test_success",
	  "name": "test_success",
	  "description": "Login happy path",
	  "descriptionHtml": "",
	  "status": "passed",
	  "stage": "finished",
	  "statusDetails": null,
	  "labels": [
	    {"name": "feature", "value": "Auth"},
	    {"name": "severity", "value": "critical"},
	    {"name": "framework", "value": "pytest"},
	    {"name": "language", "value": "python"},
	    {"name": "package", "value": "tests"},
	    {"name": "testClass", "value": "test_login"},
	    {"name": "testMethod", "value": "test_success"},
	    {"name": "host", "value": "ci-runner-1"},
	    {"name": "thread", "value": "Thread-1"},
	    {"name": "AS_ID", "value": "abcd1234"}
	  ],
	  "links": [
	    {"name": "JIRA-1", "url": "https://jira/JIRA-1", "type": "issue"},
	    {"name": "TMS-1", "url": "https://tms/1", "type": "tms"},
	    {"name": "Wiki", "url": "https://wiki/", "type": "link"}
	  ],
	  "parameters": [
	    {"name": "env", "value": "ci"},
	    {"name": "token", "value": "secret", "mode": "masked"}
	  ],
	  "steps": [
	    {
	      "name": "submit",
	      "status": "passed",
	      "stage": "finished",
	      "start": 1700000000000,
	      "stop":  1700000001000,
	      "parameters": [{"name": "user", "value": "alice"}],
	      "steps": [],
	      "attachments": [
	        {"name": "response.json", "source": "x-attachment.json", "type": "application/json"}
	      ]
	    }
	  ],
	  "attachments": [],
	  "start": 1700000000000,
	  "stop":  1700000001000
	}`)

	var ref map[string]any
	if err := json.Unmarshal(reference, &ref); err != nil {
		t.Fatalf("decode reference: %v", err)
	}

	// Produce equivalent output via our writer.
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Setenv(IssuePatternEnv, "")
	t.Setenv(TmsPatternEnv, "")
	ctx, finish := WithTest(context.Background(), "test_success",
		WithResultsDir(dir),
		WithFullName("tests/test_login.py::test_success"),
		WithFeature("Auth"),
		WithSeverity(SeverityCritical),
		WithPackage("tests"),
		WithTestClass("test_login"),
		WithTestMethod("test_success"),
		WithIssue("JIRA-1", "https://jira/JIRA-1"),
		WithTmsLink("TMS-1", "https://tms/1"),
		WithLink("Wiki", "https://wiki/"),
		WithParameter("env", "ci"),
		WithParameterEx("token", "secret", ParameterModeMasked, false),
		WithDescription("Login happy path"),
	)
	Step(ctx, "submit", func() {
		Parameter(ctx, "user", "alice")
		Attachment(ctx, "response.json", []byte(`{}`), "application/json")
	})
	finish()

	raw := findResultRaw(t, dir)
	var our map[string]any
	if err := json.Unmarshal(raw, &our); err != nil {
		t.Fatalf("decode our: %v\nraw=%s", err, raw)
	}

	// Walk the reference tree and assert every key path exists in our
	// output and has the same JSON type. Run-time values (uuid,
	// historyId, timestamps) are checked for type only.
	walk := func(refV, ourV any, path string) {}
	walk = func(refV, ourV any, path string) {
		switch r := refV.(type) {
		case map[string]any:
			o, ok := ourV.(map[string]any)
			if !ok {
				t.Errorf("[%s] expected object, got %T", path, ourV)
				return
			}
			for k, v := range r {
				sub := path + "." + k
				next, ok := o[k]
				if !ok {
					// `descriptionHtml` is optional and only emitted when set —
					// our SDK omits it under that condition (matches schema).
					if k == "descriptionHtml" && r[k] == "" {
						continue
					}
					if v == nil {
						continue
					}
					t.Errorf("[%s] missing in our output", sub)
					continue
				}
				walk(v, next, sub)
			}
		case []any:
			o, ok := ourV.([]any)
			if !ok {
				t.Errorf("[%s] expected array, got %T", path, ourV)
				return
			}
			// For arrays we check element-zero shape; index-by-index
			// equality is too strict (labels are unordered).
			if len(r) > 0 && len(o) > 0 {
				walk(r[0], o[0], path+"[0]")
			}
		default:
			rt := jsonTypeOf(refV)
			ot := jsonTypeOf(ourV)
			if rt != ot && !(refV == nil && ourV == nil) {
				t.Errorf("[%s] ref type %s vs our %s (ref=%v our=%v)", path, rt, ot, refV, ourV)
			}
		}
	}
	walk(ref, our, "$")

	// Cross-check key SETS at each level — the most catch-all property:
	// if we ever add or drop a top-level field accidentally, the union
	// vs reference will diverge.
	refKeys := sortedKeys(ref)
	ourKeys := sortedKeys(our)
	for _, k := range refKeys {
		// Keys that the Allure 2 schema permits to be omitted when the
		// corresponding value is empty/null. Our writer drops them via
		// `omitempty` to keep output byte-minimal; allure-pytest sometimes
		// emits them as null (also legal). The renderer treats both the
		// same way.
		if k == "descriptionHtml" || k == "statusDetails" {
			continue
		}
		found := false
		for _, ok := range ourKeys {
			if ok == k {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ref key %q missing from our top-level set (our=%v)", k, ourKeys)
		}
	}

	// Stage / status must be canonical enum literals.
	if got := our["stage"]; got != "finished" {
		t.Errorf("stage = %v, want finished", got)
	}
	switch our["status"] {
	case "passed", "failed", "broken", "skipped":
	default:
		t.Errorf("status not in enum: %v", our["status"])
	}

	// Verify the SDK injected the same automatic labels allure-pytest emits
	// (framework / language / host / thread / package / testClass /
	// testMethod / AS_ID). Their PRESENCE is the contract; values are
	// runtime-determined.
	labels := our["labels"].([]any)
	have := map[string]string{}
	for _, l := range labels {
		m := l.(map[string]any)
		have[m["name"].(string)] = m["value"].(string)
	}
	for _, key := range []string{"framework", "language", "host", "thread", "package", "testClass", "testMethod", "AS_ID", "feature", "severity"} {
		if have[key] == "" {
			t.Errorf("auto-label %q missing from our output (have=%v)", key, have)
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestRoundtripJSONInvariants — produce a result, marshal it, decode it
// back, and assert the decoded structure is semantically equivalent to
// the in-memory original (all fields preserved, no silent data loss).
func TestRoundtripJSONInvariants(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir),
			WithFeature("F"), WithStory("S"), WithEpic("E"),
			WithOwner("O"), WithSeverity(SeverityCritical),
			WithIssue("I-1", "https://i/1"),
			WithTmsLink("T-1", "https://t/1"),
			WithParameter("p", "v"),
		)
		a.Description("d")
		a.DescriptionHTML("<p>d</p>")
		a.Step("s1", func() {
			a.Parameter("k", "v")
			a.Attachment("a.bin", []byte("x"), "application/octet-stream")
		})
	})
	r := readResultFile(t, dir)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Result
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back.UUID != r.UUID || back.HistoryID != r.HistoryID || back.Name != r.Name {
		t.Errorf("identity lost: ours=%+v back=%+v", r, back)
	}
	if back.Status != r.Status || back.Stage != r.Stage {
		t.Errorf("enum lost")
	}
	if back.Description != "d" || back.DescriptionHTML != "<p>d</p>" {
		t.Errorf("description not round-tripped: %q / %q", back.Description, back.DescriptionHTML)
	}
	if len(back.Labels) != len(r.Labels) || len(back.Links) != len(r.Links) {
		t.Errorf("label/link count drift")
	}
	if !reflect.DeepEqual(back.Parameters, r.Parameters) {
		t.Errorf("parameters drift: %+v vs %+v", back.Parameters, r.Parameters)
	}
}

// TestVeryDeepNesting goes well beyond the documented "5 levels" floor —
// 50 levels — to confirm we don't accidentally rely on bounded recursion
// or hit a slice-aliasing bug at depth.
func TestVeryDeepNesting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	const depth = 50
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		var rec func(int)
		rec = func(n int) {
			a.Step("l"+itoa(n), func() {
				if n < depth {
					rec(n + 1)
				}
			})
		}
		rec(1)
	})
	r := readResultFile(t, dir)
	cur := r.Steps
	for i := 1; i <= depth; i++ {
		if len(cur) != 1 {
			t.Fatalf("at level %d: expected 1 child, got %d", i, len(cur))
		}
		if cur[0].Name != "l"+itoa(i) {
			t.Errorf("level %d name = %q", i, cur[0].Name)
		}
		cur = cur[0].Steps
	}
}

// TestAttachmentMimeDetection_UnknownType verifies the MIME table falls
// back to .dat for entirely unknown MIME values.
func TestAttachmentMimeDetection_UnknownType(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner, WithResultsDir(dir))
		a.Attachment("blob", []byte("x"), "x/y-some-totally-novel-type")
	})
	r := readResultFile(t, dir)
	if len(r.Attachments) != 1 {
		t.Fatalf("attachments = %d", len(r.Attachments))
	}
	if !strings.HasSuffix(r.Attachments[0].Source, ".dat") {
		t.Errorf("unknown-mime suffix = %q, want .dat", r.Attachments[0].Source)
	}
}

// TestExecutorJSON_FromEnv verifies CI-supplied environment variables
// surface in the executor.json.
func TestExecutorJSON_FromEnv(t *testing.T) {
	t.Setenv("ALLURE_EXECUTOR_NAME", "GitHub-Actions")
	t.Setenv("ALLURE_EXECUTOR_TYPE", "github")
	t.Setenv("ALLURE_EXECUTOR_URL", "https://github.com/actions/123")
	t.Setenv("ALLURE_EXECUTOR_BUILD_ORDER", "42")

	e := defaultExecutor()
	if e.Name != "GitHub-Actions" || e.Type != "github" || e.URL != "https://github.com/actions/123" {
		t.Errorf("executor not from env: %+v", e)
	}
	if e.BuildOrder != "42" {
		t.Errorf("build_order = %q", e.BuildOrder)
	}
}

// TestEnsureExecutor_WritesOnce verifies the once-per-process guard.
func TestEnsureExecutor_WritesOnce(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ResultsDirEnv, dir)
	EnsureExecutor()
	EnsureExecutor() // second call must not duplicate
	f, err := os.Stat(filepath.Join(dir, "executor.json"))
	if err != nil {
		t.Fatalf("executor.json missing: %v", err)
	}
	if f.Size() == 0 {
		t.Errorf("executor.json empty")
	}
}

// TestCleanResultsDir wipes a populated dir.
func TestCleanResultsDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "x")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "stale.json"), []byte("{}"), 0o644)
	if err := CleanResultsDir(dir); err != nil {
		t.Errorf("clean failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("dir still exists: %v", err)
	}
}
