// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudSpacesCanonicalRoutesAndConcurrencyHeaders(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		if r.Header.Get("Authorization") != "Bearer cloud-token" || r.Header.Get("X-API-Key") != "" {
			t.Fatalf("credential headers: %#v", r.Header)
		}
		if r.Method != http.MethodGet {
			if r.Header.Get("Idempotency-Key") != "retry-1" || r.Header.Get("If-Match") != `"space-s1-r7"` {
				t.Fatalf("mutation headers: %#v", r.Header)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "collection_revision": 7, "space": map[string]any{"id": "s1"}, "organization": map[string]any{"account_id": "a1", "account_kind": "team", "viewer_role": "member", "can_leave_team": true, "seats_used": 2, "seats_reserved": 1, "spaces": []any{map[string]any{"id": "s1", "audience": "team"}}, "members": []any{map[string]any{"user_id": "u1", "can_leave": true}}, "pending_invites": []any{}}, "status": "ok", "revision": 8, "etag": `"space-s1-r7"`, "invite": map[string]any{"id": "i1", "token": "once", "workspace_id": "s1"}})
	}))
	defer srv.Close()

	api := NewClient(srv.URL, WithAPIKey("cloud-token")).CloudSpaces()
	ctx := context.Background()
	if page, err := api.List(ctx, "cursor-a", 25); err != nil || page.CollectionRevision != 7 {
		t.Fatal(err)
	}
	if _, err := api.Get(ctx, "s1"); err != nil {
		t.Fatal(err)
	}
	if organization, err := api.Organization(ctx, "s1"); err != nil || organization.AccountKind != "team" ||
		organization.ViewerRole != "member" || !organization.CanLeaveTeam || organization.CanManageTeam ||
		organization.SeatsReserved != 1 || len(organization.PendingInvites) != 0 || len(organization.Spaces) != 1 ||
		organization.Spaces[0].Audience != "team" || len(organization.Members) != 1 ||
		organization.Members[0].LastTeamActivityAt != nil || !organization.Members[0].CanLeave {
		t.Fatalf("organization=%+v err=%v", organization, err)
	}
	if _, err := api.ListMembers(ctx, "s1", "", 25); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ListInvites(ctx, "s1", "", 25); err != nil {
		t.Fatal(err)
	}
	if preview, err := api.PreviewInvite(ctx, "token/one"); err != nil || preview.ETag != `"space-s1-r7"` || preview.Invite.WorkspaceID != "s1" {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	if mutation, err := api.CreateInvite(ctx, "s1", CloudSpaceInviteRequest{Email: "new@example.test", Role: "viewer"}, `"space-s1-r7"`, "retry-1"); err != nil || mutation.Invite == nil || mutation.Invite.Token != "once" {
		t.Fatal(err)
	}
	if mutation, err := api.RevokeInvite(ctx, "s1", "i1", `"space-s1-r7"`, "retry-1"); err != nil || mutation.Revision != 8 {
		t.Fatal(err)
	}
	if _, err := api.AcceptInvite(ctx, "token/one", `"space-s1-r7"`, "retry-1"); err != nil {
		t.Fatal(err)
	}
	if mutation, err := api.UpdateMemberRole(ctx, "s1", "u1", "editor", `"space-s1-r7"`, "retry-1"); err != nil || mutation.Revision != 8 {
		t.Fatal(err)
	}
	if mutation, err := api.RemoveMember(ctx, "s1", "u1", `"space-s1-r7"`, "retry-1"); err != nil || mutation.Revision != 8 {
		t.Fatal(err)
	}

	wants := []string{
		"GET /api/v1/cloud/spaces?cursor=cursor-a&limit=25",
		"GET /api/v1/cloud/spaces/s1", "GET /api/v1/cloud/organization?space_id=s1",
		"GET /api/v1/cloud/spaces/s1/members?limit=25",
		"GET /api/v1/cloud/spaces/s1/invites?limit=25", "GET /api/v1/cloud/invites/token%2Fone",
		"POST /api/v1/cloud/spaces/s1/invites", "DELETE /api/v1/cloud/spaces/s1/invites/i1", "POST /api/v1/cloud/invites/token%2Fone/accept",
		"PATCH /api/v1/cloud/spaces/s1/members/u1", "DELETE /api/v1/cloud/spaces/s1/members/u1",
	}
	if len(paths) != len(wants) {
		t.Fatalf("paths=%v", paths)
	}
	for i := range wants {
		if paths[i] != wants[i] {
			t.Errorf("path[%d]=%q want %q", i, paths[i], wants[i])
		}
	}
}

func TestCloudSpacesRevokeTeamInviteUsesSelectedAccountAndStableKey(t *testing.T) {
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method=%s", r.Method)
		}
		gotPath, gotKey = r.URL.RequestURI(), r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	api := NewClient(srv.URL, WithAPIKey("cloud-token")).CloudSpaces()
	if err := api.RevokeTeamInvite(context.Background(), "space-id", "invite-id", "same-retry-key"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/cloud/team/invites/invite-id?space_id=space-id" || gotKey != "same-retry-key" {
		t.Fatalf("Team revocation path=%q key=%q", gotPath, gotKey)
	}
	if err := api.RevokeTeamInvite(context.Background(), "space-id", "invite-id", ""); err == nil {
		t.Fatal("Team revocation without idempotency key reached the network")
	}
}

func TestCloudSpacesSetTeamMemberStatusKeepsSeatAndUsesStableKey(t *testing.T) {
	var gotPath, gotKey, gotStatus string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method=%s", r.Method)
		}
		gotPath, gotKey = r.URL.RequestURI(), r.Header.Get("Idempotency-Key")
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotStatus = body["status"]
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	api := NewClient(srv.URL, WithAPIKey("cloud-token")).CloudSpaces()
	if err := api.SetTeamMemberStatus(context.Background(), "space-id", "member-id", " SUSPENDED ", "status-retry-key"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/cloud/team/members/member-id/status?space_id=space-id" ||
		gotKey != "status-retry-key" || gotStatus != "suspended" {
		t.Fatalf("Team status path=%q key=%q status=%q", gotPath, gotKey, gotStatus)
	}
	for _, status := range []string{"", "removed"} {
		if err := api.SetTeamMemberStatus(context.Background(), "space-id", "member-id", status, "status-retry-key"); err == nil {
			t.Fatalf("invalid Team status %q reached the network", status)
		}
	}
	if err := api.SetTeamMemberStatus(context.Background(), "space-id", "member-id", "active", ""); err == nil {
		t.Fatal("Team status without idempotency key reached the network")
	}
}

func TestCloudSpacesNonPositiveLimitUsesServerDefault(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()
	api := NewClient(server.URL, WithAPIKey("cloud-token")).CloudSpaces()
	_, _ = api.List(context.Background(), "", 0)
	_, _ = api.ListMembers(context.Background(), "s1", "next", -1)
	if len(paths) != 2 || paths[0] != "/api/v1/cloud/spaces" || paths[1] != "/api/v1/cloud/spaces/s1/members?cursor=next" {
		t.Fatalf("non-positive limit paths=%v", paths)
	}
}
