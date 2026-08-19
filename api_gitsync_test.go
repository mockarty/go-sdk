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

// TestGitSyncAPI_Lifecycle exercises the git-sync resource against a fake server:
// create → list → get → pull → push → delete, verifying paths + payload.
func TestGitSyncAPI_Lifecycle(t *testing.T) {
	var createBody string
	var pushQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/git-sync/bindings":
			b := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(b)
			createBody = string(b)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"b-1","repoUrl":"https://x/r.git","branch":"main","kind":"ui","hasToken":true,"enabled":true,"autoSync":true}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/git-sync/bindings":
			_, _ = w.Write([]byte(`{"bindings":[{"id":"b-1","repoUrl":"https://x/r.git","kind":"ui"}]}`))
		case r.Method == "GET" && r.URL.Path == "/api/v1/git-sync/bindings/b-1":
			_, _ = w.Write([]byte(`{"id":"b-1","repoUrl":"https://x/r.git","lastCommit":"abc123","lastSyncAt":"2026-07-02T00:00:00Z"}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/git-sync/bindings/b-1/pull":
			_, _ = w.Write([]byte(`{"commit":"abc123","uiTestsFound":3}`))
		case r.Method == "POST" && r.URL.Path == "/api/v1/git-sync/bindings/b-1/push":
			pushQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"commit":"def456"}`))
		case r.Method == "DELETE" && r.URL.Path == "/api/v1/git-sync/bindings/b-1":
			_, _ = w.Write([]byte(`{"deleted":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"), WithNamespace("sandbox"))
	ctx := context.Background()

	b, err := c.GitSync().CreateBinding(ctx, &GitSyncBindingInput{RepoURL: "https://x/r.git", Kind: "ui", AuthToken: "ghp_x"})
	if err != nil {
		t.Fatalf("CreateBinding: %v", err)
	}
	if b.ID != "b-1" || !b.HasToken {
		t.Fatalf("unexpected binding: %+v", b)
	}
	if !strings.Contains(createBody, "ghp_x") {
		t.Errorf("token not sent in create body: %s", createBody)
	}

	list, err := c.GitSync().ListBindings(ctx)
	if err != nil || len(list) != 1 || list[0].ID != "b-1" {
		t.Fatalf("ListBindings: %v %+v", err, list)
	}

	got, err := c.GitSync().GetBinding(ctx, "b-1")
	if err != nil || got.LastCommit != "abc123" {
		t.Fatalf("GetBinding: %v %+v", err, got)
	}

	pr, err := c.GitSync().Pull(ctx, "b-1")
	if err != nil || pr.UITestsFound != 3 {
		t.Fatalf("Pull: %v %+v", err, pr)
	}

	push, err := c.GitSync().Push(ctx, "b-1", "my message")
	if err != nil || push.Commit != "def456" {
		t.Fatalf("Push: %v %+v", err, push)
	}
	if !strings.Contains(pushQuery, "message=my+message") {
		t.Errorf("push message not in query: %s", pushQuery)
	}

	if err := c.GitSync().DeleteBinding(ctx, "b-1"); err != nil {
		t.Fatalf("DeleteBinding: %v", err)
	}
}
