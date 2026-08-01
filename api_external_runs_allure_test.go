// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// allureUploadServer records every payload the SDK posts.
func allureUploadServer(t *testing.T, bodies *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Errorf("server got non-JSON body: %s", raw)
		}
		*bodies = append(*bodies, doc)
		_, _ = w.Write([]byte(`{"runId":"r-1"}`))
	}))
}

func writeResult(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// The author-pinned identity must land on testCaseId — never on caseId, which
// is Mockarty's internal case UUID. Allure writes it in `testCaseId`; Allure
// TestOps adapters express @AllureId as the AS_ID label instead, and a suite
// migrated from TestOps carries it only there.
func TestUploadAllureDir_ResolvesPinnedIdentity(t *testing.T) {
	cases := []struct {
		name   string
		result string
		want   string
	}{
		{"testCaseId field", `{"uuid":"u1","name":"a","status":"passed","testCaseId":"TC-1"}`, "TC-1"},
		{"AS_ID label", `{"uuid":"u1","name":"a","status":"passed","labels":[{"name":"AS_ID","value":"TC-2"}]}`, "TC-2"},
		{"ALLURE_ID label", `{"uuid":"u1","name":"a","status":"passed","labels":[{"name":"ALLURE_ID","value":"TC-3"}]}`, "TC-3"},
		{"no pin", `{"uuid":"u1","name":"a","status":"passed"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeResult(t, dir, "a-result.json", tc.result)
			var bodies []map[string]any
			srv := allureUploadServer(t, &bodies)
			defer srv.Close()

			c := NewClient(srv.URL, WithNamespace("qa"))
			if _, err := c.ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{}); err != nil {
				t.Fatalf("UploadAllureDir: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("got %d uploads, want 1", len(bodies))
			}
			if got, _ := bodies[0]["testCaseId"].(string); got != tc.want {
				t.Errorf("testCaseId = %q, want %q", got, tc.want)
			}
			if _, present := bodies[0]["caseId"]; present {
				t.Error("caseId must never carry the Allure id (it is the internal case UUID)")
			}
		})
	}
}

// "broken" is part of the server's status vocabulary — an assertion failure and
// a test that blew up are different findings. An ABSENT status is not a pass.
func TestUploadAllureDir_StatusVocabulary(t *testing.T) {
	dir := t.TempDir()
	writeResult(t, dir, "a-result.json", `{"uuid":"u1","name":"a","status":"broken"}`)
	writeResult(t, dir, "b-result.json", `{"uuid":"u2","name":"b"}`)
	writeResult(t, dir, "c-result.json", `{"uuid":"u3","name":"c","status":"passed","steps":[{"name":"s"}]}`)

	var bodies []map[string]any
	srv := allureUploadServer(t, &bodies)
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("qa"))
	if _, err := c.ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{}); err != nil {
		t.Fatalf("UploadAllureDir: %v", err)
	}
	if len(bodies) != 3 {
		t.Fatalf("got %d uploads, want 3", len(bodies))
	}
	// Files are uploaded in sorted order: a, b, c.
	if got := bodies[0]["status"]; got != "broken" {
		t.Errorf("broken flattened to %v", got)
	}
	if got := bodies[1]["status"]; got != "broken" {
		t.Errorf("missing status = %v, want broken (never passed)", got)
	}
	steps, _ := bodies[2]["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("steps = %v", bodies[2]["steps"])
	}
	if got := steps[0].(map[string]any)["status"]; got != "broken" {
		t.Errorf("step without status = %v, want broken", got)
	}
}

// A CI upload that loses results must not look like a clean one.
func TestUploadAllureDir_PartialLossIsReported(t *testing.T) {
	dir := t.TempDir()
	writeResult(t, dir, "a-result.json", `{"uuid":"u1","name":"a","status":"passed"}`)
	writeResult(t, dir, "b-result.json", `{ not json`)

	var bodies []map[string]any
	srv := allureUploadServer(t, &bodies)
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("qa"))
	out, err := c.ExternalRuns().UploadAllureDir(context.Background(), dir, AllureUploadOptions{})
	if err == nil {
		t.Fatal("a skipped result must surface as an error")
	}
	var partial *AllureUploadPartialError
	if !errors.As(err, &partial) {
		t.Fatalf("error = %v (%T), want *AllureUploadPartialError", err, err)
	}
	if partial.Uploaded != 1 || len(partial.Skipped) != 1 {
		t.Fatalf("uploaded=%d skipped=%v", partial.Uploaded, partial.Skipped)
	}
	// The successes are still returned so best-effort callers can use them.
	if len(out) != 1 || len(bodies) != 1 {
		t.Fatalf("returned %d responses, server saw %d uploads", len(out), len(bodies))
	}
}

// "raise" aborts on the first bad file instead of walking the rest.
func TestUploadAllureDir_OnErrorRaise(t *testing.T) {
	dir := t.TempDir()
	writeResult(t, dir, "a-result.json", `{ not json`)
	writeResult(t, dir, "b-result.json", `{"uuid":"u2","name":"b","status":"passed"}`)

	var bodies []map[string]any
	srv := allureUploadServer(t, &bodies)
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("qa"))
	if _, err := c.ExternalRuns().UploadAllureDir(context.Background(), dir,
		AllureUploadOptions{OnError: "raise"}); err == nil {
		t.Fatal("expected an error with OnError=raise")
	}
	if len(bodies) != 0 {
		t.Fatalf("raise must abort before uploading the rest, saw %d", len(bodies))
	}
}

// Nothing to upload means the run produced nothing — not a clean success.
func TestUploadAllureDir_EmptyAndMissingDirAreErrors(t *testing.T) {
	var bodies []map[string]any
	srv := allureUploadServer(t, &bodies)
	defer srv.Close()
	c := NewClient(srv.URL, WithNamespace("qa"))

	if _, err := c.ExternalRuns().UploadAllureDir(context.Background(), t.TempDir(), AllureUploadOptions{}); err == nil {
		t.Error("an empty allure-results directory must be an error")
	}
	missing := filepath.Join(t.TempDir(), "nope")
	if _, err := c.ExternalRuns().UploadAllureDir(context.Background(), missing, AllureUploadOptions{}); err == nil {
		t.Error("a missing allure-results directory must be an error")
	}
}
