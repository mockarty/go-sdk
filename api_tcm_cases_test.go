// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTCMCases_Flow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == "POST" && p == "/api/v1/namespaces/mockarty-dev/test-cases":
			w.WriteHeader(201)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "c1", "title": "Login case"})
		case r.Method == "GET" && p == "/api/v1/namespaces/mockarty-dev/test-cases/c1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "c1", "status": "active"})
		case r.Method == "GET" && p == "/api/v1/namespaces/mockarty-dev/test-cases":
			_ = json.NewEncoder(w).Encode(map[string]any{"test_cases": []any{map[string]any{"id": "c1"}}, "total": 1})
		case r.Method == "PUT" && p == "/api/v1/namespaces/mockarty-dev/test-cases/c1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "c1", "title": "t2"})
		case r.Method == "POST" && p == "/api/v1/namespaces/mockarty-dev/test-cases/c1/run":
			_ = json.NewEncoder(w).Encode(map[string]any{"runId": "r1", "status": "running"})
		case r.Method == "GET" && p == "/api/v1/namespaces/mockarty-dev/test-cases/c1/runs":
			_ = json.NewEncoder(w).Encode(map[string]any{"runs": []any{map[string]any{"id": "r1"}}})
		case r.Method == "GET" && p == "/api/v1/namespaces/mockarty-dev/tcm/case-runs/r1":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "r1", "status": "passed"})
		case r.Method == "POST" && p == "/api/v1/namespaces/mockarty-dev/tcm/case-runs/r1/cancel":
			w.WriteHeader(200)
		case r.Method == "POST" && p == "/api/v1/namespaces/mockarty-dev/tcm/defects":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "d1", "title": "bug"})
		case r.Method == "GET" && p == "/api/v1/namespaces/mockarty-dev/tcm/defects":
			_ = json.NewEncoder(w).Encode(map[string]any{"defects": []any{map[string]any{"id": "d1"}}})
		case r.Method == "DELETE" && (p == "/api/v1/namespaces/mockarty-dev/test-cases/c1" || p == "/api/v1/namespaces/mockarty-dev/tcm/defects/d1"):
			w.WriteHeader(200)
		default:
			http.Error(w, "unexpected "+r.Method+" "+p, 404)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("mockarty-dev"))
	ctx := context.Background()
	tcm := c.TCM()

	cs, err := tcm.CreateCase(ctx, "", TCMObject{"title": "Login case"})
	if err != nil || cs["id"] != "c1" {
		t.Fatalf("CreateCase: %v %+v", err, cs)
	}
	if got, err := tcm.GetCase(ctx, "", "c1"); err != nil || got["status"] != "active" {
		t.Fatalf("GetCase: %v %+v", err, got)
	}
	if list, err := tcm.ListCases(ctx, "", nil); err != nil || len(list) != 1 {
		t.Fatalf("ListCases: %v %+v", err, list)
	}
	if up, err := tcm.UpdateCase(ctx, "", "c1", TCMObject{"title": "t2"}); err != nil || up["title"] != "t2" {
		t.Fatalf("UpdateCase: %v %+v", err, up)
	}
	if run, err := tcm.RunCase(ctx, "", "c1", nil); err != nil || run["runId"] != "r1" {
		t.Fatalf("RunCase: %v %+v", err, run)
	}
	if runs, err := tcm.ListCaseRuns(ctx, "", "c1"); err != nil || len(runs) != 1 {
		t.Fatalf("ListCaseRuns: %v %+v", err, runs)
	}
	if cr, err := tcm.GetCaseRun(ctx, "", "r1"); err != nil || cr["status"] != "passed" {
		t.Fatalf("GetCaseRun: %v %+v", err, cr)
	}
	if err := tcm.CancelCaseRun(ctx, "", "r1"); err != nil {
		t.Fatalf("CancelCaseRun: %v", err)
	}
	if d, err := tcm.CreateDefect(ctx, "", TCMObject{"title": "bug"}); err != nil || d["id"] != "d1" {
		t.Fatalf("CreateDefect: %v %+v", err, d)
	}
	if ds, err := tcm.ListDefects(ctx, "", nil); err != nil || len(ds) != 1 {
		t.Fatalf("ListDefects: %v %+v", err, ds)
	}
	if err := tcm.DeleteDefect(ctx, "", "d1"); err != nil {
		t.Fatalf("DeleteDefect: %v", err)
	}
	if err := tcm.DeleteCase(ctx, "", "c1"); err != nil {
		t.Fatalf("DeleteCase: %v", err)
	}
}

func TestTCMCases_NamespaceRequired(t *testing.T) {
	c := NewClient("http://x")
	c.namespace = ""
	_, err := c.TCM().ListCases(context.Background(), "", nil)
	if err == nil || !strings.Contains(err.Error(), "namespace required") {
		t.Fatalf("expected namespace-required, got %v", err)
	}
}
