// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUITestAPI_Lifecycle exercises the UI-test run-API against a fake server:
// create → run → status → export, verifying paths + payload round-trip.
func TestUITestAPI_Lifecycle(t *testing.T) {
	var createBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/ui-tests":
			b := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(b)
			createBody = string(b)
			_, _ = w.Write([]byte(`{"id":"ui-1","name":"demo","platform":"web","actionCount":2}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/ui-tests":
			_, _ = w.Write([]byte(`{"uiTests":[{"id":"ui-1","name":"demo","actionCount":2}]}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/ui-tests/ui-1":
			_, _ = w.Write([]byte(`{"id":"ui-1","name":"demo"}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/ui-tests/ui-1/run":
			_, _ = w.Write([]byte(`{"taskId":"task-9","uiTestId":"ui-1","statusPath":"/api/v1/runner-tasks/task-9"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/runner-tasks/task-9":
			_, _ = w.Write([]byte(`{"id":"task-9","status":"passed"}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/ui-tests/ui-1/export":
			_, _ = w.Write([]byte("package uitests\n"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"), WithNamespace("sandbox"))
	ctx := context.Background()

	ui := NewUITest("demo").Navigate("http://x").Click(".cart")
	saved, err := c.UITests().Create(ctx, ui)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if saved.ID != "ui-1" {
		t.Errorf("Create id = %q", saved.ID)
	}
	if !strings.Contains(createBody, `"actions"`) || !strings.Contains(createBody, "navigate") {
		t.Errorf("create body missing actions: %s", createBody)
	}

	lst, err := c.UITests().List(ctx)
	if err != nil || len(lst) != 1 {
		t.Fatalf("List: %v / %d", err, len(lst))
	}

	got, err := c.UITests().Get(ctx, "ui-1")
	if err != nil || got.Name != "demo" {
		t.Fatalf("Get: %v / %q", err, got.Name)
	}

	h, err := c.UITests().Run(ctx, "ui-1", nil)
	if err != nil || h.TaskID != "task-9" {
		t.Fatalf("Run: %v / %q", err, h.TaskID)
	}

	st, err := c.UITests().RunStatus(ctx, h.TaskID)
	if err != nil || st.Status != "passed" || !st.Terminal() {
		t.Fatalf("RunStatus: %v / %q terminal=%v", err, st.Status, st.Terminal())
	}

	code, err := c.UITests().Export(ctx, "ui-1", "go")
	if err != nil || !strings.HasPrefix(code, "package uitests") {
		t.Fatalf("Export: %v / %q", err, code)
	}
}
