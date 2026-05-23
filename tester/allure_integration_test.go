// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/allure"
	"github.com/mockarty/mockarty-go/tester"
)

// TestAllureIntegrationEndToEnd proves Tester chains land as Allure step
// records on disk when wrapped in allure.WithTest. Reads the result
// JSON back and asserts each step name appears under the test's steps[].
func TestAllureIntegrationEndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(allure.ResultsDirEnv, dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Alice"}`))
	}))
	t.Cleanup(srv.Close)

	ctx, finish := allure.WithTest(context.Background(), "Allure x Tester integration")
	tt := tester.New(
		tester.WithBaseURL(srv.URL),
		tester.WithContext(ctx),
	)
	tt.Wrap("login chain", func() {
		tt.HTTP().GET("/users/42").
			ExpectStatus(200).
			ExpectJSONPath("$.name", "Alice").
			Extract("$.name", "user")
	})
	tt.HTTP().GET("/users/42").ExpectStatus(200)
	tt.Finish()
	finish()

	// Find the *-result.json file Allure wrote and assert structure.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read result dir: %v", err)
	}
	var resultPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-result.json") {
			resultPath = filepath.Join(dir, e.Name())
			break
		}
	}
	if resultPath == "" {
		t.Fatalf("no allure result file in %s: %+v", dir, entries)
	}
	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var doc struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Steps  []struct {
			Name  string `json:"name"`
			Steps []struct {
				Name string `json:"name"`
			} `json:"steps"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v\n%s", resultPath, err, data)
	}
	if doc.Name != "Allure x Tester integration" {
		t.Fatalf("test name: %q", doc.Name)
	}
	if doc.Status != "passed" {
		t.Fatalf("status: %q (raw=%s)", doc.Status, data)
	}
	// Expect 2 top-level steps: the Wrap("login chain") group + the
	// trailing HTTP GET. The wrap group nests one HTTP step inside.
	if len(doc.Steps) != 2 {
		t.Fatalf("want 2 top-level steps, got %d: %s", len(doc.Steps), data)
	}
	if doc.Steps[0].Name != "login chain" {
		t.Fatalf("first step name: %q", doc.Steps[0].Name)
	}
	if len(doc.Steps[0].Steps) != 1 {
		t.Fatalf("wrap should contain 1 nested HTTP step, got %d", len(doc.Steps[0].Steps))
	}
}

// TestAllureFailedStepReflectsInStatus verifies that an assertion
// failure inside a Tester chain causes the surrounding Allure step to
// be marked failed and the test result status follows.
func TestAllureFailedStepReflectsInStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(allure.ResultsDirEnv, dir)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(srv.Close)

	ctx, finish := allure.WithTest(context.Background(), "Failing chain")
	tt := tester.New(tester.WithBaseURL(srv.URL), tester.WithContext(ctx))
	tt.HTTP().GET("/").ExpectStatus(200)
	tt.Finish()
	finish()

	entries, _ := os.ReadDir(dir)
	var path string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-result.json") {
			path = filepath.Join(dir, e.Name())
			break
		}
	}
	if path == "" {
		t.Fatalf("no result file")
	}
	data, _ := os.ReadFile(path)
	var doc struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(data, &doc)
	if doc.Status != "failed" {
		t.Fatalf("status: %q (raw=%s)", doc.Status, data)
	}
}
