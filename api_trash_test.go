// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestTrash_ListTrash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if got, want := r.URL.Path, "/api/v1/namespaces/production/trash"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		q := r.URL.Query()
		if got := q.Get("type"); got != "mock,store" {
			t.Errorf("type=%q, want mock,store", got)
		}
		if got := q.Get("q"); got != "users" {
			t.Errorf("q=%q", got)
		}
		if got := q.Get("limit"); got != "25" {
			t.Errorf("limit=%q", got)
		}
		if got := q.Get("offset"); got != "10" {
			t.Errorf("offset=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashListResult{
			Items: []TrashItem{
				{
					ID:               "11111111-2222-3333-4444-555555555555",
					Name:             "users",
					Namespace:        "production",
					EntityType:       "mock",
					ClosedAt:         time.Now().UTC(),
					ClosedBy:         "alice@example.com",
					CascadeGroupID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					RestoreAvailable: true,
				},
			},
			Total:  1,
			Limit:  25,
			Offset: 10,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	out, err := c.Trash().ListTrash(context.Background(), "production", TrashListOptions{
		EntityTypes: []string{"mock", "store"},
		SearchQuery: "users",
		Limit:       25,
		Offset:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || len(out.Items) != 1 {
		t.Fatalf("unexpected result %#v", out)
	}
	if !out.Items[0].RestoreAvailable {
		t.Error("expected restore_available=true")
	}
}

func TestTrash_ListTrash_FromToCascade(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 19, 23, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("from"); got != "2026-04-01T00:00:00Z" {
			t.Errorf("from=%q", got)
		}
		if got := q.Get("to"); got != "2026-04-19T23:00:00Z" {
			t.Errorf("to=%q", got)
		}
		if got := q.Get("cascade"); got != "cc-1" {
			t.Errorf("cascade=%q", got)
		}
		if got := q.Get("closed_by"); got != "bob@example.com" {
			t.Errorf("closed_by=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashListResult{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Trash().ListTrash(context.Background(), "ns", TrashListOptions{
		FromTime:       from,
		ToTime:         to,
		ClosedBy:       "bob@example.com",
		CascadeGroupID: "cc-1",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrash_ListTrash_NamespaceRequired(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().ListTrash(context.Background(), "  ", TrashListOptions{})
	if err == nil || !strings.Contains(err.Error(), "namespace is required") {
		t.Fatalf("expected namespace error, got %v", err)
	}
}

func TestTrash_AdminListTrash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashListResult{Total: 3})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	out, err := c.Trash().AdminListTrash(context.Background(), TrashListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 3 {
		t.Fatalf("total=%d", out.Total)
	}
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func TestTrash_Summary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/namespaces/team-a/trash/summary"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashSummary{
			Counts: []TrashSummaryCount{{EntityType: "mock", Count: 4}},
			Total:  4,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().TrashSummary(context.Background(), "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 4 || len(got.Counts) != 1 {
		t.Fatalf("summary=%#v", got)
	}
}

func TestTrash_AdminSummary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash/summary"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashSummary{Total: 42})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().AdminTrashSummary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 42 {
		t.Fatalf("total=%d", got.Total)
	}
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func TestTrash_GetSettings_Inherited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/namespaces/finance/trash/settings"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashSettings{
			Scope:         "namespace",
			Namespace:     "finance",
			RetentionDays: 7,
			Enabled:       true,
			Inherited:     true,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().GetTrashSettings(context.Background(), "finance")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Inherited || got.RetentionDays != 7 {
		t.Fatalf("settings=%#v", got)
	}
}

func TestTrash_UpdateSettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method=%s", r.Method)
		}
		var got TrashSettingsUpdate
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.RetentionDays != 14 || !got.Enabled {
			t.Fatalf("body=%#v", got)
		}
		_ = json.NewEncoder(w).Encode(TrashSettings{
			Scope:         "namespace",
			Namespace:     "ns1",
			RetentionDays: 14,
			Enabled:       true,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Trash().UpdateTrashSettings(context.Background(), "ns1", TrashSettingsUpdate{
		RetentionDays: 14,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTrash_GetGlobalSettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash/settings/global"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(TrashSettings{Scope: "global", RetentionDays: 30, Enabled: true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().GetGlobalTrashSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "global" || got.RetentionDays != 30 {
		t.Fatalf("%#v", got)
	}
}

func TestTrash_UpdateGlobalSettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/admin/trash/settings/global" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(TrashSettings{Scope: "global", RetentionDays: 60})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().UpdateGlobalTrashSettings(context.Background(), TrashSettingsUpdate{
		RetentionDays: 60, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RetentionDays != 60 {
		t.Fatalf("retention=%d", got.RetentionDays)
	}
}

// ---------------------------------------------------------------------------
// Restore — single cascade
// ---------------------------------------------------------------------------

func TestTrash_RestoreCascade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%s", r.Method)
		}
		if got, want := r.URL.Path, "/api/v1/namespaces/prod/trash/restore-cascade/grp-1"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(RestoreResult{
			CascadeGroupID: "grp-1",
			RestoredCount:  5,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().RestoreCascade(context.Background(), "prod", "grp-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RestoredCount != 5 {
		t.Fatalf("restored=%d", got.RestoredCount)
	}
}

func TestTrash_RestoreCascade_GroupRequired(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().RestoreCascade(context.Background(), "ns", "   ")
	if err == nil || !strings.Contains(err.Error(), "cascade_group_id") {
		t.Fatalf("expected cascade_group_id error, got %v", err)
	}
}

func TestTrash_AdminRestoreCascade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash/restore-cascade/grp-admin"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(RestoreResult{CascadeGroupID: "grp-admin", RestoredCount: 1})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().AdminRestoreCascade(context.Background(), "grp-admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.RestoredCount != 1 {
		t.Fatalf("restored=%d", got.RestoredCount)
	}
}

// ---------------------------------------------------------------------------
// Bulk restore
// ---------------------------------------------------------------------------

func TestTrash_BulkRestore_PartialSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/namespaces/ns/trash/restore" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		var body BulkRestoreRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.CascadeGroupIDs) != 3 {
			t.Fatalf("ids=%v", body.CascadeGroupIDs)
		}
		_ = json.NewEncoder(w).Encode(BulkRestoreResult{
			Restored: []BulkRestoreOutcome{{CascadeGroupID: "g1", RestoredCount: 2}},
			Failed:   []BulkRestoreOutcome{{CascadeGroupID: "g2", Error: "denied"}},
			NotFound: []string{"g3"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().BulkRestore(context.Background(), "ns", BulkRestoreRequest{
		CascadeGroupIDs: []string{"g1", "g2", "g3"},
		Reason:          "cleanup fix",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Restored) != 1 || len(got.Failed) != 1 || len(got.NotFound) != 1 {
		t.Fatalf("result=%#v", got)
	}
}

func TestTrash_BulkRestore_EmptyIDs(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().BulkRestore(context.Background(), "ns", BulkRestoreRequest{})
	if err == nil || !strings.Contains(err.Error(), "cascade_group_ids") {
		t.Fatalf("expected cascade_group_ids error, got %v", err)
	}
}

func TestTrash_AdminBulkRestore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash/restore"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(BulkRestoreResult{
			Restored: []BulkRestoreOutcome{{CascadeGroupID: "gA", RestoredCount: 1}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().AdminBulkRestore(context.Background(), BulkRestoreRequest{
		CascadeGroupIDs: []string{"gA"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Restored) != 1 {
		t.Fatalf("restored=%d", len(got.Restored))
	}
}

// ---------------------------------------------------------------------------
// Bulk purge
// ---------------------------------------------------------------------------

func TestTrash_BulkPurge_ConfirmationMissing(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().BulkPurge(context.Background(), "ns", BulkPurgeRequest{
		CascadeGroupIDs: []string{"g1"},
		// Confirmation left blank
	})
	if !errors.Is(err, ErrTrashConfirmationMissing) {
		t.Fatalf("expected ErrTrashConfirmationMissing, got %v", err)
	}
}

func TestTrash_BulkPurge_WrongConfirmation(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().BulkPurge(context.Background(), "ns", BulkPurgeRequest{
		CascadeGroupIDs: []string{"g1"},
		Confirmation:    "YES DELETE",
	})
	if !errors.Is(err, ErrTrashConfirmationMissing) {
		t.Fatalf("expected ErrTrashConfirmationMissing, got %v", err)
	}
}

func TestTrash_BulkPurge_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/namespaces/ns/trash/purge" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		var body BulkPurgeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Confirmation != TrashPurgeConfirmationPhrase {
			t.Fatalf("confirmation=%q", body.Confirmation)
		}
		_ = json.NewEncoder(w).Encode(BulkPurgeResult{
			Purged: []BulkPurgeOutcome{{CascadeGroupID: "g1", RowsDeleted: 7}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().BulkPurge(context.Background(), "ns", BulkPurgeRequest{
		CascadeGroupIDs: []string{"g1"},
		Confirmation:    TrashPurgeConfirmationPhrase,
		Reason:          "GDPR request",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Purged) != 1 || got.Purged[0].RowsDeleted != 7 {
		t.Fatalf("purged=%#v", got)
	}
}

func TestTrash_BulkPurge_EmptyIDs(t *testing.T) {
	c := NewClient("http://unused")
	_, err := c.Trash().BulkPurge(context.Background(), "ns", BulkPurgeRequest{
		Confirmation: TrashPurgeConfirmationPhrase,
	})
	if err == nil || !strings.Contains(err.Error(), "cascade_group_ids") {
		t.Fatalf("expected cascade_group_ids error, got %v", err)
	}
}

func TestTrash_AdminBulkPurge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/api/v1/admin/trash/purge"; got != want {
			t.Fatalf("path=%q", got)
		}
		_ = json.NewEncoder(w).Encode(BulkPurgeResult{
			Purged: []BulkPurgeOutcome{{CascadeGroupID: "gX", RowsDeleted: 1}},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().AdminBulkPurge(context.Background(), BulkPurgeRequest{
		CascadeGroupIDs: []string{"gX"},
		Confirmation:    TrashPurgeConfirmationPhrase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Purged) != 1 {
		t.Fatalf("purged=%d", len(got.Purged))
	}
}

func TestTrash_AdminBulkPurge_SupportForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"support role cannot purge","code":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Trash().AdminBulkPurge(context.Background(), BulkPurgeRequest{
		CascadeGroupIDs: []string{"g"},
		Confirmation:    TrashPurgeConfirmationPhrase,
	})
	if err == nil {
		t.Fatal("expected 403")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 {
		t.Fatalf("expected APIError 403, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Purge now
// ---------------------------------------------------------------------------

func TestTrash_AdminPurgeNow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/admin/trash/purge-now" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(PurgeNowResult{
			Status:      "ok",
			PurgedTotal: 123,
			Namespaces:  4,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.Trash().AdminPurgeNow(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.PurgedTotal != 123 || got.Namespaces != 4 {
		t.Fatalf("purge-now=%#v", got)
	}
}

func TestTrash_AdminPurgeNow_FollowerUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"not leader","code":"not_leader"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Trash().AdminPurgeNow(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Accessor
// ---------------------------------------------------------------------------

func TestTrash_AccessorCached(t *testing.T) {
	c := NewClient("http://unused")
	if c.Trash() != c.Trash() {
		t.Error("expected cached TrashAPI singleton")
	}
}
