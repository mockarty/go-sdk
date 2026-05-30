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

func TestSecurity_StartScan_RequiresNamespaceAndTitle(t *testing.T) {
	c := NewClient("http://example.invalid")
	// Override the default "sandbox" namespace so the empty-namespace
	// guard fires.
	c.namespace = ""

	if _, err := c.Security().StartScan(context.Background(), StartScanRequest{Title: "t"}); err == nil {
		t.Fatalf("expected error on empty namespace")
	}
	if _, err := c.Security().StartScan(context.Background(), StartScanRequest{Namespace: "ns"}); err == nil {
		t.Fatalf("expected error on empty title")
	}
}

func TestSecurity_StartScan_PostsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/security/scans" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var got StartScanRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got.Namespace != "prod" || got.Title != "nightly" {
			t.Fatalf("unexpected body: %+v", got)
		}
		if got.Profile.Intensity != "passive" {
			t.Errorf("intensity = %q", got.Profile.Intensity)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"report": SecurityReport{
				ID:        "rep-123",
				Namespace: "prod",
				Title:     "nightly",
				Status:    "running",
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	rep, err := c.Security().StartScan(context.Background(), StartScanRequest{
		Title:     "nightly",
		Namespace: "prod",
		Profile: SecurityScanProfile{
			Intensity:        "passive",
			ScopeDescription: "https://api.example.com",
			Targets: []SecurityTarget{
				{URL: "https://api.example.com/v1", Method: "GET"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.ID != "rep-123" || rep.Status != "running" {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestSecurity_GetReport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/security/reports/rep-9" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"report": SecurityReport{ID: "rep-9", Status: "done"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	rep, err := c.Security().GetReport(context.Background(), "rep-9")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "done" {
		t.Fatalf("status = %q", rep.Status)
	}
	if _, err := c.Security().GetReport(context.Background(), ""); err == nil {
		t.Fatalf("expected error on empty reportID")
	}
}

func TestSecurity_ListFindings_SeverityFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/security/reports/rep-1/findings" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"findings": []SecurityFinding{
				{ID: "f1", Severity: "high"},
				{ID: "f2", Severity: "low"},
				{ID: "f3", Severity: "High"}, // case-insensitive match
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))

	all, err := c.Security().ListFindings(context.Background(), "rep-1", ListFindingsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("len(all) = %d", len(all))
	}

	high, err := c.Security().ListFindings(context.Background(), "rep-1", ListFindingsOptions{Severity: "HIGH"})
	if err != nil {
		t.Fatal(err)
	}
	if len(high) != 2 {
		t.Fatalf("expected 2 high findings, got %d", len(high))
	}
}

func TestSecurity_ExportReport_FormatValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/security/reports/rep-1/export" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "sarif" {
			t.Fatalf("format = %q", r.URL.Query().Get("format"))
		}
		w.Header().Set("Content-Type", "application/sarif+json")
		_, _ = w.Write([]byte(`{"runs":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	data, err := c.Security().ExportReport(context.Background(), "rep-1", "sarif")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "runs") {
		t.Fatalf("body = %s", data)
	}

	if _, err := c.Security().ExportReport(context.Background(), "rep-1", "csv"); err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if _, err := c.Security().ExportReport(context.Background(), "", "sarif"); err == nil {
		t.Fatalf("expected error for empty reportID")
	}
}

func TestSecurity_ListScanners_AndCancel(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/scanners":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"scanners": []SecurityScanner{
					{Key: "scan_headers", Persona: "web_pentester", Intensity: "passive"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/reports/rep-x/cancel":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":    "cancelled",
				"signalled": true,
			})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("k"))
	scanners, err := c.Security().ListScanners(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(scanners) != 1 || scanners[0].Key != "scan_headers" {
		t.Fatalf("scanners = %+v", scanners)
	}
	if err := c.Security().CancelScan(context.Background(), "rep-x"); err != nil {
		t.Fatal(err)
	}
	if err := c.Security().CancelScan(context.Background(), ""); err == nil {
		t.Fatalf("expected error on empty reportID")
	}
	if hits != 2 {
		t.Fatalf("expected 2 server hits, got %d", hits)
	}
}
