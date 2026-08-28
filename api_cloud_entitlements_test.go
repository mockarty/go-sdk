// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudEntitlementsGetUsesExplicitSpaceAndBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/cloud/entitlements" || r.URL.Query().Get("space_id") != "space-1" {
			t.Fatalf("request=%s %s", r.Method, r.URL.String())
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cloud-token" {
			t.Fatalf("Authorization=%q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "" {
			t.Fatalf("unexpected X-API-Key=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"revision":7,"digest":"abc","snapshot":{"schema_version":"mockarty.entitlement/v1","subject_id":"user-1","billing_account_id":"account-1","benefit_group_id":"group-1","space_id":"space-1","product":"mockarty","plan":"team","policy_version":"cloud-commercial-v1","issuance_id":"issue-1","authority_domain":"cloud-commercial","key_id":"cloud-entitlement-unsigned-projection-v1","key_domain":"mockarty.license.production","modules":[],"capacity":[],"issued_at":"2026-08-24T00:00:00Z","not_before":"2026-08-24T00:00:00Z","effective_at":"2026-08-24T00:00:00Z","expires_at":"2026-09-24T00:00:00Z","grace_until":"2026-09-24T00:00:00Z","revision":7,"human_seats":25,"service_accounts":0}}`))
	}))
	defer server.Close()

	projection, err := NewClient(server.URL, WithAPIKey("cloud-token")).CloudEntitlements().Get(context.Background(), "space-1")
	if err != nil {
		t.Fatal(err)
	}
	if projection.Revision != 7 || projection.Snapshot.Plan != "team" || projection.Snapshot.SpaceID != "space-1" {
		t.Fatalf("projection=%+v", projection)
	}
}

func TestCloudEntitlementsGetRequiresSpace(t *testing.T) {
	_, err := NewClient("http://127.0.0.1", WithAPIKey("cloud-token")).CloudEntitlements().Get(context.Background(), " ")
	if err == nil {
		t.Fatal("empty Space id accepted")
	}
}
