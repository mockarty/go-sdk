// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

func TestTestPlans_Create(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/test-plans" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var got TestPlan
		_ = json.NewDecoder(r.Body).Decode(&got)
		if got.Namespace != "production" {
			t.Fatalf("expected namespace=production, got %q", got.Namespace)
		}
		if len(got.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(got.Items))
		}
		w.WriteHeader(http.StatusCreated)
		got.ID = "11111111-2222-3333-4444-555555555555"
		got.NumericID = 42
		_ = json.NewEncoder(w).Encode(got)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"), WithNamespace("production"))
	plan, err := c.TestPlans().Create(context.Background(), TestPlan{
		Name: "Nightly",
		Items: []TestPlanItem{
			{Order: 1, Type: PlanItemTypeFunctional, ResourceID: "11111111-1111-1111-1111-111111111111"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.NumericID != 42 {
		t.Fatalf("expected numericId=42, got %d", plan.NumericID)
	}
}

func TestTestPlans_Get_ByUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/test-plans/my-id" {
			t.Fatalf("unexpected path %q", got)
		}
		_ = json.NewEncoder(w).Encode(TestPlan{ID: "my-id", Name: "Found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	p, err := c.TestPlans().Get(context.Background(), "my-id")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Found" {
		t.Errorf("expected name Found, got %q", p.Name)
	}
}

func TestTestPlans_Get_ByNumericID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/test-plans/1042" {
			t.Fatalf("unexpected path %q", got)
		}
		_ = json.NewEncoder(w).Encode(TestPlan{ID: "uuid-here", NumericID: 1042})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	// Accept both "#1042" and "1042".
	p, err := c.TestPlans().Get(context.Background(), "#1042")
	if err != nil {
		t.Fatal(err)
	}
	if p.NumericID != 1042 {
		t.Errorf("expected numericId 1042, got %d", p.NumericID)
	}
}

func TestTestPlans_Get_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"plan missing","code":"not_found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.TestPlans().Get(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrPlanNotFound) {
		t.Errorf("expected ErrPlanNotFound, got %v", err)
	}
}

func TestTestPlans_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespace") != "ops" {
			t.Fatalf("expected namespace=ops, got %q", r.URL.Query().Get("namespace"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []TestPlan{
				{ID: "a", Name: "One"},
				{ID: "b", Name: "Two"},
			},
			"count": 2,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	plans, err := c.TestPlans().List(context.Background(), ListPlansOptions{Namespace: "ops"})
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2, got %d", len(plans))
	}
}

func TestTestPlans_Delete(t *testing.T) {
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		hit.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deleted":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.TestPlans().Delete(context.Background(), "plan-1"); err != nil {
		t.Fatal(err)
	}
	if hit.Load() != 1 {
		t.Error("handler not called")
	}
}

func TestTestPlans_EmptyID(t *testing.T) {
	c := NewClient("http://x")
	if _, err := c.TestPlans().Get(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
	if err := c.TestPlans().Delete(context.Background(), ""); err == nil {
		t.Error("expected error for empty id")
	}
}

// TestTestPlans_Update exercises the PUT-full-replace path: namespace fallback
// from the client, ID propagation into the body, and round-trip of the server
// response.
func TestTestPlans_Update(t *testing.T) {
	var hit atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/test-plans/plan-upd" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		var got TestPlan
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != "plan-upd" {
			t.Fatalf("expected id=plan-upd, got %q", got.ID)
		}
		if got.Namespace != "production" {
			t.Fatalf("expected namespace=production (client fallback), got %q", got.Namespace)
		}
		hit.Add(1)
		got.Name = got.Name + "-saved"
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(got)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("production"))
	out, err := c.TestPlans().Update(context.Background(), "plan-upd", TestPlan{
		Name: "Nightly",
		Items: []TestPlanItem{
			{Order: 1, Type: PlanItemTypeFunctional, ResourceID: "11111111-1111-1111-1111-111111111111"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Nightly-saved" {
		t.Fatalf("expected server round-trip name=Nightly-saved, got %q", out.Name)
	}
	if hit.Load() != 1 {
		t.Error("handler not called")
	}

	if _, err := c.TestPlans().Update(context.Background(), "", TestPlan{}); err == nil {
		t.Error("expected error for empty id")
	}
}

// ---------------------------------------------------------------------------
// Runs
// ---------------------------------------------------------------------------

func TestTestPlans_Run(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var opts RunOptions
		_ = json.NewDecoder(r.Body).Decode(&opts)
		if len(opts.Items) != 2 || opts.Items[0] != 1 {
			t.Fatalf("unexpected items: %v", opts.Items)
		}
		if opts.Mode != "parallel" {
			t.Fatalf("expected mode=parallel, got %q", opts.Mode)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"runId":  "run-1",
			"planId": "plan-1",
			"status": "pending",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	run, err := c.TestPlans().Run(context.Background(), "plan-1", RunOptions{
		Items: []int{1, 3},
		Mode:  "parallel",
	})
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "run-1" || run.Status != "pending" {
		t.Fatalf("unexpected run: %+v", run)
	}
}

func TestTestPlans_WaitForRun_Success(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		status := "running"
		if n >= 3 {
			status = "completed"
		}
		_ = json.NewEncoder(w).Encode(TestPlanRun{ID: "r1", Status: status})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	run, err := c.TestPlans().WaitForRun(ctx, "r1", 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" {
		t.Errorf("expected completed, got %s", run.Status)
	}
	if calls.Load() < 3 {
		t.Errorf("expected ≥3 polls, got %d", calls.Load())
	}
}

func TestTestPlans_WaitForRun_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TestPlanRun{ID: "r1", Status: "failed"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	run, err := c.TestPlans().WaitForRun(context.Background(), "r1", 1*time.Millisecond)
	if !errors.Is(err, ErrRunFailed) {
		t.Fatalf("expected ErrRunFailed, got %v", err)
	}
	if run == nil || run.Status != "failed" {
		t.Errorf("expected run carried through")
	}
}

func TestTestPlans_WaitForRun_Cancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TestPlanRun{ID: "r1", Status: "cancelled"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.TestPlans().WaitForRun(context.Background(), "r1", 1*time.Millisecond)
	if !errors.Is(err, ErrRunCancelled) {
		t.Fatalf("expected ErrRunCancelled, got %v", err)
	}
}

func TestTestPlans_CancelRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/runs/r1/cancel") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.TestPlans().CancelRun(context.Background(), "r1"); err != nil {
		t.Fatal(err)
	}
}

func TestTestPlans_GetRunStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(PlanRunStatus{
			Status:         "running",
			TotalItems:     4,
			CompletedItems: 2,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	s, err := c.TestPlans().GetRunStatus(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if s.CompletedItems != 2 {
		t.Errorf("expected 2, got %d", s.CompletedItems)
	}
}

func TestTestPlans_ListRuns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" {
			t.Fatalf("expected limit=10, got %q", r.URL.Query().Get("limit"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []TestPlanRun{
				{ID: "r1", Status: "completed"},
				{ID: "r2", Status: "running"},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	runs, err := c.TestPlans().ListRuns(context.Background(), "plan-1", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2, got %d", len(runs))
	}
}

// ---------------------------------------------------------------------------
// Report + SSE
// ---------------------------------------------------------------------------

func TestTestPlans_GetReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "allure" {
			t.Fatalf("expected format=allure, got %q", r.URL.Query().Get("format"))
		}
		_, _ = w.Write([]byte(`{"hello":"world"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	data, err := c.TestPlans().GetReport(context.Background(), "r1", ReportFormatAllure)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "world") {
		t.Errorf("unexpected body: %s", data)
	}
}

func TestTestPlans_DownloadReportZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("PK\x03\x04"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var buf bytes.Buffer
	if err := c.TestPlans().DownloadReportZip(context.Background(), "r1", &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 4 || !bytes.HasPrefix(buf.Bytes(), []byte("PK")) {
		t.Errorf("unexpected bytes: %q", buf.Bytes())
	}
}

func TestTestPlans_StreamRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: run.started\ndata: {\"runId\":\"r1\"}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "event: item.finished\ndata: {\"order\":1,\"status\":\"passed\"}\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, ": heartbeat\n\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "event: run.completed\ndata: {\"status\":\"completed\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, err := c.TestPlans().StreamRun(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for ev := range ch {
		got = append(got, ev.Kind)
	}
	wantPrefix := []string{"run.started", "item.finished", "run.completed"}
	if len(got) < 3 {
		t.Fatalf("expected ≥3 events, got %v", got)
	}
	for i, want := range wantPrefix {
		if got[i] != want {
			t.Errorf("event[%d] = %q, want %q", i, got[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Schedules + Webhooks
// ---------------------------------------------------------------------------

func TestTestPlans_Schedules(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/test-plans/plan-1/schedules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"s1","kind":"cron","cronExpr":"0 9 * * *","enabled":true}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"items":[{"id":"s1","kind":"cron","enabled":true}]}`))
		}
	})
	mux.HandleFunc("/api/v1/test-plans/plan-1/schedules/s1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPatch:
			_, _ = w.Write([]byte(`{"id":"s1","enabled":false}`))
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	created, err := c.TestPlans().AddSchedule(ctx, "plan-1", Schedule{Kind: ScheduleKindCron, CronExpr: "0 9 * * *", Enabled: true})
	if err != nil || created.ID != "s1" {
		t.Fatalf("create: %v, %+v", err, created)
	}

	list, err := c.TestPlans().ListSchedules(ctx, "plan-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v, %+v", err, list)
	}

	upd, err := c.TestPlans().UpdateSchedule(ctx, "plan-1", "s1", Schedule{Enabled: false})
	if err != nil || upd.Enabled {
		t.Fatalf("update: %v, %+v", err, upd)
	}

	if err := c.TestPlans().DeleteSchedule(ctx, "plan-1", "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestTestPlans_Webhooks_CRUD_and_Test(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/test-plans/p/webhooks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body Webhook
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Secret != "s3cret" {
				t.Errorf("expected secret to be sent, got %q", body.Secret)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"w1","url":"https://ci","events":["run.completed"],"enabled":true}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"items":[{"id":"w1","url":"https://ci","enabled":true}]}`))
		}
	})
	mux.HandleFunc("/api/v1/test-plans/p/webhooks/w1/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"status":200}`))
	})
	mux.HandleFunc("/api/v1/test-plans/p/webhooks/w2/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"status":503,"error":"timeout"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx := context.Background()

	got, err := c.TestPlans().AddWebhook(ctx, "p", Webhook{URL: "https://ci", Events: []string{"run.completed"}, Secret: "s3cret"})
	if err != nil || got.ID != "w1" {
		t.Fatalf("add: %v, %+v", err, got)
	}

	list, err := c.TestPlans().ListWebhooks(ctx, "p")
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v, %+v", err, list)
	}

	if err := c.TestPlans().TestWebhook(ctx, "p", "w1"); err != nil {
		t.Fatalf("test success unexpectedly failed: %v", err)
	}

	if err := c.TestPlans().TestWebhook(ctx, "p", "w2"); !errors.Is(err, ErrWebhookDeliveryFailed) {
		t.Fatalf("expected ErrWebhookDeliveryFailed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Retry integration with the generic client retry loop
// ---------------------------------------------------------------------------

func TestTestPlans_Retry_OnTransientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy","code":"unavailable"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(TestPlanRun{ID: "r1", Status: "completed"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(3, 1*time.Millisecond))
	run, err := c.TestPlans().GetRun(context.Background(), "r1")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" {
		t.Errorf("expected completed, got %s", run.Status)
	}
}

// ---------------------------------------------------------------------------
// SSE parser edge cases
// ---------------------------------------------------------------------------

func TestParseSSE_MultiLineData(t *testing.T) {
	src := strings.NewReader("event: run.completed\ndata: {\"a\":1}\ndata: {\"b\":2}\n\n")
	ch := make(chan RunEvent, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		defer close(ch)
		parseSSE(ctx, src, ch)
	}()

	got := <-ch
	if got.Kind != "run.completed" {
		t.Fatalf("kind=%q", got.Kind)
	}
	// data lines are newline-joined
	if !strings.Contains(string(got.Raw), "\n") {
		t.Fatalf("expected joined data, got %q", got.Raw)
	}
	// Drain + expect closed.
	for range ch {
	}
}

// ---------------------------------------------------------------------------
// TP-6b: Patch (namespace-scoped, If-Match)
// ---------------------------------------------------------------------------

func TestTestPlans_Patch_WithExplicitIfMatch(t *testing.T) {
	var (
		gotIfMatch string
		gotPath    string
		gotBody    PatchPlanRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		gotIfMatch = r.Header.Get("If-Match")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(TestPlan{ID: "p1", Name: "Renamed", UpdatedAt: time.Unix(0, 1_700_000_001_000_000).UTC()})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("qa"))
	name := "Renamed"
	updated, err := c.TestPlans().Patch(context.Background(), "p1", PatchPlanRequest{Name: &name}, PatchOptions{IfMatch: `"1700000000000"`})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("expected Renamed, got %q", updated.Name)
	}
	if gotIfMatch != `"1700000000000"` {
		t.Errorf("If-Match not forwarded: %q", gotIfMatch)
	}
	if gotPath != "/api/v1/namespaces/qa/test-plans/p1" {
		t.Errorf("wrong path: %q", gotPath)
	}
	if gotBody.Name == nil || *gotBody.Name != "Renamed" {
		t.Errorf("body name not forwarded: %+v", gotBody)
	}
}

func TestTestPlans_Patch_AutoFetchesEtagWhenMissing(t *testing.T) {
	var (
		getCalls   int32
		patchCalls int32
		etagSeen   string
	)
	updatedAt := time.Unix(0, 1_700_000_000_000_000).UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			atomic.AddInt32(&getCalls, 1)
			_ = json.NewEncoder(w).Encode(TestPlan{ID: "p1", UpdatedAt: updatedAt})
		case http.MethodPatch:
			atomic.AddInt32(&patchCalls, 1)
			etagSeen = r.Header.Get("If-Match")
			_ = json.NewEncoder(w).Encode(TestPlan{ID: "p1", Description: "new"})
		default:
			t.Fatalf("unexpected %s", r.Method)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("prod"))
	desc := "new"
	_, err := c.TestPlans().Patch(context.Background(), "p1", PatchPlanRequest{Description: &desc}, PatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&getCalls) != 1 {
		t.Errorf("expected 1 GET (pre-fetch), got %d", atomic.LoadInt32(&getCalls))
	}
	if atomic.LoadInt32(&patchCalls) != 1 {
		t.Errorf("expected 1 PATCH, got %d", atomic.LoadInt32(&patchCalls))
	}
	// The etag is quoted UnixMilli of updatedAt.
	wantEtag := `"` + intToString(int(updatedAt.UnixMilli())) + `"`
	if etagSeen != wantEtag {
		t.Errorf("auto etag = %q, want %q", etagSeen, wantEtag)
	}
}

func TestTestPlans_Patch_PreconditionFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPreconditionFailed)
		_, _ = w.Write([]byte(`{"error":"etag mismatch","code":"precondition_failed"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("prod"))
	name := "x"
	_, err := c.TestPlans().Patch(context.Background(), "p1", PatchPlanRequest{Name: &name}, PatchOptions{IfMatch: `"stale"`})
	if !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("expected ErrPreconditionFailed, got %v", err)
	}
}

func TestTestPlans_Patch_EmptyRequestRejected(t *testing.T) {
	c := NewClient("http://x", WithNamespace("prod"))
	_, err := c.TestPlans().Patch(context.Background(), "p1", PatchPlanRequest{}, PatchOptions{IfMatch: `"1"`})
	if err == nil {
		t.Fatal("expected error for empty request")
	}
}

func TestTestPlans_Patch_HonoursOptionsNamespace(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(TestPlan{ID: "p1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithNamespace("default"))
	name := "x"
	_, err := c.TestPlans().Patch(context.Background(), "p1", PatchPlanRequest{Name: &name},
		PatchOptions{IfMatch: `"1"`, Namespace: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "/namespaces/other/") {
		t.Errorf("namespace override ignored: %q", gotPath)
	}
}

// ---------------------------------------------------------------------------
// TP-6b: CreateAdHocRun
// ---------------------------------------------------------------------------

func TestTestPlans_CreateAdHocRun(t *testing.T) {
	var gotReq CreateAdHocRunRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/namespaces/qa/test-runs/ad-hoc" {
			t.Fatalf("wrong path %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"run_id":  "run-1",
			"plan_id": "plan-1",
			"status":  "pending",
			"adhoc":   true,
			"_links": map[string]string{
				"self":   "/api/v1/test-plans/runs/run-1",
				"status": "/api/v1/test-plans/runs/run-1/status",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	resp, err := c.TestPlans().CreateAdHocRun(context.Background(), CreateAdHocRunRequest{
		Namespace: "qa",
		Name:      "smoke",
		Schedule:  "parallel",
		Items: []AdHocItem{
			{Order: 1, Type: PlanItemTypeFunctional, RefID: "11111111-1111-1111-1111-111111111111"},
			{Order: 2, Type: PlanItemTypeFuzz, RefID: "22222222-2222-2222-2222-222222222222"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RunID != "run-1" || resp.PlanID != "plan-1" || !resp.Adhoc {
		t.Errorf("unexpected response: %+v", resp)
	}
	if resp.Links["self"] == "" {
		t.Error("links not decoded")
	}
	if gotReq.Name != "smoke" || len(gotReq.Items) != 2 {
		t.Errorf("body mismatch: %+v", gotReq)
	}
}

func TestTestPlans_CreateAdHocRun_Validation(t *testing.T) {
	c := NewClient("http://x")
	// No items.
	if _, err := c.TestPlans().CreateAdHocRun(context.Background(), CreateAdHocRunRequest{Namespace: "x"}); err == nil {
		t.Error("expected error for empty items")
	}
	// Missing ref_id.
	if _, err := c.TestPlans().CreateAdHocRun(context.Background(), CreateAdHocRunRequest{
		Namespace: "x",
		Items:     []AdHocItem{{Type: PlanItemTypeFunctional}},
	}); err == nil {
		t.Error("expected error for missing ref_id")
	}
	// Missing type.
	if _, err := c.TestPlans().CreateAdHocRun(context.Background(), CreateAdHocRunRequest{
		Namespace: "x",
		Items:     []AdHocItem{{RefID: "u"}},
	}); err == nil {
		t.Error("expected error for missing type")
	}
}

func TestTestPlans_CreateAdHocRun_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"orchestrator not wired","code":"unavailable"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.TestPlans().CreateAdHocRun(context.Background(), CreateAdHocRunRequest{
		Namespace: "qa",
		Items:     []AdHocItem{{Order: 1, Type: PlanItemTypeFunctional, RefID: "u"}},
	})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// TP-6: namespace-scoped report (JSON + ZIP)
// ---------------------------------------------------------------------------

func TestTestPlans_GetRunReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/api/v1/namespaces/prod/test-plans/plan-1/runs/run-1/report"
		if r.URL.Path != wantPath {
			t.Fatalf("wrong path: %q want %q", r.URL.Path, wantPath)
		}
		_, _ = w.Write([]byte(`{"runId":"run-1","planId":"plan-1","status":"completed","items":[{"status":"passed","durationMs":42,"startedAt":"2026-01-01T00:00:00Z","finishedAt":"2026-01-01T00:00:01Z"}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	rep, err := c.TestPlans().GetRunReport(context.Background(), "prod", "plan-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	if rep.RunID != "run-1" || rep.Status != "completed" {
		t.Errorf("decode failed: %+v", rep)
	}
	if len(rep.Items) != 1 || rep.Items[0].Status != "passed" {
		t.Errorf("items not decoded: %+v", rep.Items)
	}
	if len(rep.Raw) == 0 {
		t.Error("Raw not preserved")
	}
}

func TestTestPlans_GetRunReport_NumericPlanID(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.TestPlans().GetRunReport(context.Background(), "prod", "#42", "run-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "/test-plans/42/") {
		t.Errorf("numeric id not unwrapped: %q", gotPath)
	}
}

func TestTestPlans_GetRunReportZIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/api/v1/namespaces/prod/test-plans/plan-1/runs/run-1/report.zip"
		if r.URL.Path != wantPath {
			t.Fatalf("wrong path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("PK\x03\x04data"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	rc, err := c.TestPlans().GetRunReportZIP(context.Background(), "prod", "plan-1", "run-1")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("PK")) {
		t.Errorf("expected ZIP magic, got %q", data)
	}
}

func TestTestPlans_GetRunReportZIP_ErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no run","code":"not_found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	rc, err := c.TestPlans().GetRunReportZIP(context.Background(), "prod", "plan-1", "run-1")
	if rc != nil {
		rc.Close()
	}
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 APIError, got %v", err)
	}
}

func TestTestPlans_GetRunReport_EmptyIDs(t *testing.T) {
	c := NewClient("http://x")
	if _, err := c.TestPlans().GetRunReport(context.Background(), "ns", "", "run"); err == nil {
		t.Error("expected error for empty plan ref")
	}
	if _, err := c.TestPlans().GetRunReport(context.Background(), "ns", "plan", ""); err == nil {
		t.Error("expected error for empty run id")
	}
	if _, err := c.TestPlans().GetRunReportZIP(context.Background(), "ns", "", "run"); err == nil {
		t.Error("expected error for empty plan ref (zip)")
	}
	if _, err := c.TestPlans().GetRunReportZIP(context.Background(), "ns", "plan", ""); err == nil {
		t.Error("expected error for empty run id (zip)")
	}
}

// ---------------------------------------------------------------------------
// namespaceScopedBase helper
// ---------------------------------------------------------------------------

func TestNamespaceScopedBase_FallsBackToClient(t *testing.T) {
	c := NewClient("http://x", WithNamespace("client-ns"))
	api := c.TestPlans()
	if got := api.namespaceScopedBase(""); got != "/api/v1/namespaces/client-ns" {
		t.Errorf("empty explicit: got %q", got)
	}
	if got := api.namespaceScopedBase("explicit"); got != "/api/v1/namespaces/explicit" {
		t.Errorf("explicit override: got %q", got)
	}
}

