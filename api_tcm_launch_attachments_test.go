package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestTCMLaunchAttachmentsReadEveryPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items := []map[string]any{{"id": "a" + strconv.Itoa(offset), "namespace": "qa", "parentKind": "test_plan_run", "parentId": "run"}}
		json.NewEncoder(w).Encode(map[string]any{"items": items, "total": 2, "offset": offset, "limit": 500, "snapshot": strings.Repeat("a", 64), "hasMore": offset == 0})
	}))
	defer srv.Close()
	rows, err := NewClient(srv.URL, WithNamespace("qa")).TCM().ListAttachments(t.Context(), "", "test_plan_run", "run")
	if err != nil || len(rows) != 2 || calls != 2 {
		t.Fatalf("rows=%d calls=%d err=%v", len(rows), calls, err)
	}
}

func TestTCMLaunchAttachmentsRejectPartialAndMalformedPages(t *testing.T) {
	for _, mode := range []string{"duplicate", "empty-more", "changing-total", "bad-offset", "foreign-owner", "oversized-total", "missing-meta", "missing-snapshot", "changed-snapshot", "request-cap"} {
		t.Run(mode, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
				id := "a" + strconv.Itoa(offset)
				if mode == "duplicate" {
					id = "same"
				}
				total := 2
				if mode == "changing-total" && offset > 0 {
					total = 3
				}
				if mode == "oversized-total" {
					total = 10001
				}
				if mode == "request-cap" {
					total = 101
				}
				item := map[string]any{"id": id, "namespace": "qa", "parentKind": "test_plan_run", "parentId": "run"}
				if mode == "foreign-owner" {
					item["parentId"] = "foreign"
				}
				items := []map[string]any{item}
				if mode == "empty-more" {
					items = nil
				}
				resp := map[string]any{"items": items, "total": total, "offset": offset, "limit": 500, "snapshot": strings.Repeat("a", 64), "hasMore": offset+len(items) < total}
				if mode == "missing-snapshot" {
					delete(resp, "snapshot")
				}
				if mode == "changed-snapshot" && offset > 0 {
					resp["snapshot"] = strings.Repeat("b", 64)
				}
				if mode == "bad-offset" {
					resp["offset"] = 1
				}
				if mode == "missing-meta" {
					delete(resp, "total")
				}
				json.NewEncoder(w).Encode(resp)
			}))
			defer srv.Close()
			rows, err := NewClient(srv.URL, WithNamespace("qa")).TCM().ListAttachments(t.Context(), "", "test_plan_run", "run")
			if err == nil || rows != nil {
				t.Fatalf("partial result returned rows=%d err=%v", len(rows), err)
			}
		})
	}
}

func TestTCMLaunchAttachmentsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rows, err := NewClient("http://127.0.0.1:1", WithNamespace("qa")).TCM().ListAttachments(ctx, "", "test_plan_run", "run")
	if rows != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestTCMLaunchAttachmentsTenThousandBoundary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		items := make([]map[string]any, 0, 500)
		for i := offset; i < offset+500; i++ {
			items = append(items, map[string]any{"id": "a" + strconv.Itoa(i), "namespace": "qa", "parentKind": "test_plan_run", "parentId": "run"})
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items, "total": 10000, "offset": offset, "limit": 500, "snapshot": strings.Repeat("a", 64), "hasMore": offset+len(items) < 10000})
	}))
	defer srv.Close()
	rows, err := NewClient(srv.URL, WithNamespace("qa")).TCM().ListAttachments(t.Context(), "", "test_plan_run", "run")
	if err != nil || len(rows) != 10000 {
		t.Fatalf("boundary rows=%d err=%v", len(rows), err)
	}
}
