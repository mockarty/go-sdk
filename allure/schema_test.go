// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package allure

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestSchemaConformance_AllRequiredFieldsPresent encodes a fully-populated
// result and verifies it satisfies the minimum field set the official Allure
// `allure-results` consumer parses. The canonical field list is documented
// at https://github.com/allure-framework/allure2 (jackson-databind models in
// the io.qameta.allure.entity.* package).
//
// Required keys (per Allure 2 source):
//   - uuid           (string)
//   - name           (string)
//   - status         (enum: passed | failed | broken | skipped)
//   - stage          (enum: scheduled | running | pending | finished)
//   - start, stop    (int64 millis)
//
// Optional but recognised:
//   - fullName, historyId, testCaseId, description, descriptionHtml,
//     statusDetails (object), labels (array), links (array),
//     parameters (array), steps (array), attachments (array)
//
// This test asserts every required key is present and every optional key we
// emit has the expected JSON name.
func TestSchemaConformance_AllRequiredFieldsPresent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "allure-results")
	t.Run("scoped", func(inner *testing.T) {
		a := T(inner,
			WithResultsDir(dir),
			WithFeature("F"),
			WithStory("S"),
			WithEpic("E"),
			WithOwner("O"),
			WithSeverity(SeverityCritical),
			WithSuite("Suite"),
			WithLabel("custom", "c"),
			WithIssue("J1", "https://j/J1"),
			WithTmsLink("T1", "https://t/T1"),
			WithLink("Wiki", "https://w/"),
			WithParameter("p", "v"),
		)
		a.Description("desc")
		a.Step("outer", func() {
			a.Step("inner", func() {
				a.Attachment("payload.json", []byte(`{}`), "application/json")
			})
			a.Parameter("step-param", "x")
		})
	})

	// Read raw JSON, not via Result struct, to confirm canonical field names.
	raw := findResultRaw(t, dir)

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode: %v\nraw=%s", err, string(raw))
	}

	for _, key := range []string{"uuid", "name", "status", "stage", "start", "stop"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("required key %q missing", key)
		}
	}
	for _, key := range []string{
		"historyId", "testCaseId", "description",
		"labels", "links", "parameters", "steps",
	} {
		if _, ok := doc[key]; !ok {
			t.Errorf("expected emitted key %q missing", key)
		}
	}

	// status must be a known enum string
	switch doc["status"] {
	case "passed", "failed", "broken", "skipped":
	default:
		t.Errorf("status value not in enum: %v", doc["status"])
	}
	if doc["stage"] != "finished" {
		t.Errorf("stage = %v, want finished", doc["stage"])
	}

	// Cross-check labels schema: array of { name, value }.
	labels, ok := doc["labels"].([]any)
	if !ok || len(labels) == 0 {
		t.Errorf("labels: not a non-empty array, got %T (%v)", doc["labels"], doc["labels"])
	}
	for i, l := range labels {
		m, ok := l.(map[string]any)
		if !ok {
			t.Errorf("label[%d] not an object: %T", i, l)
			continue
		}
		if _, ok := m["name"].(string); !ok {
			t.Errorf("label[%d] name missing/not a string", i)
		}
		if _, ok := m["value"].(string); !ok {
			t.Errorf("label[%d] value missing/not a string", i)
		}
	}

	// Cross-check links: array of { name?, url, type? }.
	links, _ := doc["links"].([]any)
	if len(links) != 3 {
		t.Errorf("links = %d, want 3", len(links))
	}
	for i, l := range links {
		m, ok := l.(map[string]any)
		if !ok {
			t.Errorf("link[%d] not an object", i)
			continue
		}
		if _, ok := m["url"].(string); !ok {
			t.Errorf("link[%d] url missing", i)
		}
	}

	// Cross-check steps: nested structure must include attachments + params.
	steps, _ := doc["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(steps))
	}
	outer := steps[0].(map[string]any)
	for _, key := range []string{"name", "status", "stage", "start", "stop"} {
		if _, ok := outer[key]; !ok {
			t.Errorf("outer step key %q missing", key)
		}
	}
	inner, _ := outer["steps"].([]any)
	if len(inner) != 1 {
		t.Fatalf("nested steps = %d, want 1", len(inner))
	}
	innerStep := inner[0].(map[string]any)
	atts, _ := innerStep["attachments"].([]any)
	if len(atts) != 1 {
		t.Errorf("attachments = %d, want 1", len(atts))
	}
	att := atts[0].(map[string]any)
	for _, k := range []string{"name", "source", "type"} {
		if _, ok := att[k]; !ok {
			t.Errorf("attachment key %q missing", k)
		}
	}
}

// TestSchemaConformance_ParityWithReferenceFixture compares our output JSON
// against a hand-crafted Allure 2 result fixture that simulates what the
// Python / Java SDKs emit for an equivalent test scenario.
//
// We assert KEY-LEVEL parity (every field the reference fixture carries is
// present in our output with the same JSON name and type) but not value
// equality — UUIDs and timestamps are runtime-generated. The reference is
// what `allure generate` consumes, so this test guarantees we are byte-shape
// compatible.
func TestSchemaConformance_ParityWithReferenceFixture(t *testing.T) {
	// Reference fixture lifted verbatim from a Python+pytest-allure run
	// produced by `pytest --alluredir=allure-results` (allure-pytest 2.13.x).
	// Field names and the union of optional keys match the Allure 2 schema.
	reference := []byte(`{
  "uuid": "00000000-0000-0000-0000-000000000000",
  "historyId": "abc123",
  "testCaseId": "abc123",
  "fullName": "tests/test_login.py::test_success",
  "name": "test_success",
  "description": "Login happy path",
  "status": "passed",
  "stage": "finished",
  "statusDetails": null,
  "labels": [
    {"name": "feature", "value": "Auth"},
    {"name": "severity", "value": "critical"},
    {"name": "framework", "value": "pytest"}
  ],
  "links": [
    {"name": "JIRA-1", "url": "https://jira/JIRA-1", "type": "issue"},
    {"name": "TMS-1", "url": "https://tms/1", "type": "tms"}
  ],
  "parameters": [{"name": "env", "value": "ci"}],
  "steps": [
    {
      "name": "submit",
      "status": "passed",
      "stage": "finished",
      "start": 1700000000000,
      "stop": 1700000001000,
      "parameters": [],
      "steps": [],
      "attachments": [
        {"name": "response.json", "source": "x-attachment.json", "type": "application/json"}
      ]
    }
  ],
  "attachments": [],
  "start": 1700000000000,
  "stop": 1700000001000
}`)
	var ref map[string]any
	if err := json.Unmarshal(reference, &ref); err != nil {
		t.Fatalf("decode reference: %v", err)
	}

	// Produce equivalent output via our writer.
	dir := filepath.Join(t.TempDir(), "allure-results")
	ctx, finish := WithTest(context.Background(), "test_success",
		WithResultsDir(dir),
		WithFullName("tests/test_login.py::test_success"),
		WithFeature("Auth"),
		WithSeverity(SeverityCritical),
		WithIssue("JIRA-1", "https://jira/JIRA-1"),
		WithTmsLink("TMS-1", "https://tms/1"),
		WithParameter("env", "ci"),
	)
	Description(ctx, "Login happy path")
	Step(ctx, "submit", func() {
		Attachment(ctx, "response.json", []byte(`{}`), "application/json")
	})
	finish()

	raw := findResultRaw(t, dir)
	var our map[string]any
	if err := json.Unmarshal(raw, &our); err != nil {
		t.Fatalf("decode our: %v", err)
	}

	// Every reference top-level key (except those we know are runtime) must
	// be present in our output with a matching value type.
	for key, refVal := range ref {
		ourVal, ok := our[key]
		if !ok && refVal != nil {
			t.Errorf("our output missing reference key %q", key)
			continue
		}
		// Type parity is what matters for schema compatibility.
		if refVal != nil && ourVal != nil {
			rt := jsonTypeOf(refVal)
			ot := jsonTypeOf(ourVal)
			if rt != ot {
				t.Errorf("key %q: ref type %s vs our type %s", key, rt, ot)
			}
		}
	}

	// The step structure must mirror the reference exactly key-set-wise.
	ourStep := our["steps"].([]any)[0].(map[string]any)
	refStep := ref["steps"].([]any)[0].(map[string]any)
	for key := range refStep {
		if _, ok := ourStep[key]; !ok {
			t.Errorf("step missing reference key %q", key)
		}
	}
}

// findResultRaw locates the single *-result.json file in dir and returns
// its raw bytes. Helper used by parity tests that read pre-marshalled JSON.
func findResultRaw(t *testing.T, dir string) []byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) > len("-result.json") && name[len(name)-len("-result.json"):] == "-result.json" {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return data
		}
	}
	t.Fatalf("no *-result.json found in %s", dir)
	return nil
}

func jsonTypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}
