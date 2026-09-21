// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudTeamReadReportsUseExplicitSpaceAndBearerToken(t *testing.T) {
	paths := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer cloud-token" || r.Header.Get("X-API-Key") != "" {
			t.Errorf("Team read request method/credentials: %s %#v", r.Method, r.Header)
		}
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/cloud/team/packages":
			_, _ = w.Write([]byte(`{"packages":[{"seats":5,"discount_bps":0,"total_minor":50000,"currency":"RUB"}]}`))
		case "/api/v1/cloud/team/billing":
			_, _ = w.Write([]byte(`{"billing":{"team_id":"team-1","payer_kind":"organization","payer_name":"Example Org","package_seats":5,"assigned_seats":3,"reserved_seats":1,"invoices":[],"wallets":[]}}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	api := NewClient(server.URL, WithAPIKey("cloud-token")).CloudSpaces()
	if _, err := api.ListTeamPackages(context.Background(), ""); err == nil {
		t.Fatal("empty Personal Space reached network")
	}
	if _, err := api.GetTeamBilling(context.Background(), " "); err == nil {
		t.Fatal("empty Team Space reached network")
	}
	quotes, err := api.ListTeamPackages(context.Background(), "personal-1")
	if err != nil || len(quotes) != 1 || quotes[0].Seats != 5 || quotes[0].TotalMinor != 50000 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
	billing, err := api.GetTeamBilling(context.Background(), "team-space-1")
	if err != nil || billing.TeamID != "team-1" || billing.PayerKind != "organization" ||
		billing.PayerName != "Example Org" || billing.PackageSeats != 5 || billing.ReservedSeats != 1 {
		t.Fatalf("billing=%+v err=%v", billing, err)
	}
	if len(paths) != 2 || paths[0] != "/api/v1/cloud/team/packages?space_id=personal-1" ||
		paths[1] != "/api/v1/cloud/team/billing?space_id=team-space-1" {
		t.Fatalf("paths=%v", paths)
	}
}
