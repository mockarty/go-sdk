// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIssueTracker_Flow(t *testing.T) {
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/api/v1/namespaces/mockarty-dev/issuetracker"
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			lastBody = nil
			_ = json.Unmarshal(b, &lastBody)
		}
		switch {
		case r.Method == "POST" && r.URL.Path == base+"/issues":
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "issueKey": "MK-1", "title": lastBody["title"]})
		case r.Method == "GET" && r.URL.Path == base+"/issues/u1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "issueKey": "MK-1", "status": "open"})
		case r.Method == "GET" && r.URL.Path == base+"/issues/by-key/MK-1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "issueKey": "MK-1"})
		case r.Method == "GET" && r.URL.Path == base+"/issues":
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []any{map[string]any{"id": "u1"}}})
		case r.Method == "GET" && r.URL.Path == base+"/issues/next":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "issueKey": "MK-1"})
		case r.Method == "PUT" && r.URL.Path == base+"/issues/u1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "title": lastBody["title"]})
		case r.Method == "POST" && r.URL.Path == base+"/issues/u1/move":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "status": lastBody["status"]})
		case r.Method == "POST" && r.URL.Path == base+"/issues/u1/comments":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "c1", "body": lastBody["body"]})
		case r.Method == "GET" && r.URL.Path == base+"/issues/u1/comments":
			_ = json.NewEncoder(w).Encode(map[string]any{"comments": []any{map[string]any{"id": "c1"}}})
		case r.Method == "POST" && r.URL.Path == base+"/issues/bulk/assign":
			w.WriteHeader(200)
		case r.Method == "DELETE" && r.URL.Path == base+"/issues/u1":
			w.WriteHeader(200)
		case r.Method == "GET" && r.URL.Path == base+"/projects":
			_ = json.NewEncoder(w).Encode(map[string]any{"projects": []any{map[string]any{"id": "p1"}}})
		case r.Method == "GET" && r.URL.Path == base+"/sprints":
			_ = json.NewEncoder(w).Encode(map[string]any{"sprints": []any{map[string]any{"id": "s1"}}})
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, 404)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("mockarty-dev"))
	ctx := context.Background()
	it := c.IssueTracker()

	iss, err := it.CreateIssue(ctx, "", Issue{"projectId": "p1", "title": "Bug", "type": "bug"})
	if err != nil || iss["issueKey"] != "MK-1" || iss["title"] != "Bug" {
		t.Fatalf("CreateIssue: %v %+v", err, iss)
	}
	if got, err := it.GetIssue(ctx, "", "u1"); err != nil || got["status"] != "open" {
		t.Fatalf("GetIssue: %v %+v", err, got)
	}
	if got, err := it.GetIssueByKey(ctx, "", "MK-1"); err != nil || got["id"] != "u1" {
		t.Fatalf("GetIssueByKey: %v %+v", err, got)
	}
	if list, err := it.ListIssues(ctx, "", map[string]string{"status": "open"}); err != nil || len(list) != 1 {
		t.Fatalf("ListIssues: %v %+v", err, list)
	}
	if nx, err := it.NextIssue(ctx, "", map[string]string{"assigneeId": "me"}); err != nil || nx["issueKey"] != "MK-1" {
		t.Fatalf("NextIssue: %v %+v", err, nx)
	}
	if up, err := it.UpdateIssue(ctx, "", "u1", Issue{"title": "Bug2"}); err != nil || up["title"] != "Bug2" {
		t.Fatalf("UpdateIssue: %v %+v", err, up)
	}
	if mv, err := it.MoveIssue(ctx, "", "u1", "done", "fixed"); err != nil || mv["status"] != "done" {
		t.Fatalf("MoveIssue: %v %+v", err, mv)
	}
	if cm, err := it.AddComment(ctx, "", "u1", "looks good"); err != nil || cm["body"] != "looks good" {
		t.Fatalf("AddComment: %v %+v", err, cm)
	}
	if cs, err := it.ListComments(ctx, "", "u1"); err != nil || len(cs) != 1 {
		t.Fatalf("ListComments: %v %+v", err, cs)
	}
	if err := it.BulkAssign(ctx, "", []string{"u1"}, "me"); err != nil {
		t.Fatalf("BulkAssign: %v", err)
	}
	if ps, err := it.ListProjects(ctx, ""); err != nil || len(ps) != 1 {
		t.Fatalf("ListProjects: %v %+v", err, ps)
	}
	if sp, err := it.ListSprints(ctx, "", nil); err != nil || len(sp) != 1 {
		t.Fatalf("ListSprints: %v %+v", err, sp)
	}
	if err := it.DeleteIssue(ctx, "", "u1"); err != nil {
		t.Fatalf("DeleteIssue: %v", err)
	}
}

func TestIssueTracker_NamespaceRequired(t *testing.T) {
	c := NewClient("http://x")
	c.namespace = ""
	_, err := c.IssueTracker().ListIssues(context.Background(), "", nil)
	if err == nil || !strings.Contains(err.Error(), "namespace required") {
		t.Fatalf("expected namespace-required, got %v", err)
	}
}
