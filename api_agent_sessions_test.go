// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentTaskAPI_LegacySessionRecovery(t *testing.T) {
	const sessionID = "00000000-0000-4000-8000-000000000401"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agent/sessions/legacy":
			if r.URL.Query().Get("limit") != "25" || r.URL.Query().Get("cursor") != "next token" {
				t.Errorf("list query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"sessions":[{"originalId":"` + sessionID + `","eventCount":2}],"nextCursor":"next"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agent/sessions/legacy/"+sessionID+"/export":
			if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("afterEventId") != "7" {
				t.Errorf("export query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"session":{"id":"` + sessionID + `"},"events":[],"truncated":false}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/agent/sessions/legacy/"+sessionID+"/claim":
			var request LegacyAgentSessionClaimRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode claim: %v", err)
			}
			if request.Namespace != "payments" || !request.AcknowledgeUnknownOrigin || request.SessionKey != "tab_1" {
				t.Errorf("claim request = %+v", request)
			}
			_, _ = w.Write([]byte(`{"session":{"id":"scoped","namespace":"payments"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	api := NewClient(server.URL).AgentTasks()
	page, err := api.ListLegacySessions(context.Background(), 25, "next token")
	if err != nil || len(page.Sessions) != 1 || page.NextCursor != "next" {
		t.Fatalf("list page=%+v err=%v", page, err)
	}
	exported, err := api.ExportLegacySession(context.Background(), sessionID, 10, 7)
	if err != nil || exported.Session.ID != sessionID || exported.Events == nil {
		t.Fatalf("export=%+v err=%v", exported, err)
	}
	claimed, err := api.ClaimLegacySession(context.Background(), sessionID, LegacyAgentSessionClaimRequest{
		Namespace: "payments", SessionKey: "tab_1", AcknowledgeUnknownOrigin: true,
	})
	if err != nil || claimed.ID != "scoped" || claimed.Namespace != "payments" {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
}

func TestAgentTaskAPI_LegacySessionRecoveryRejectsInvalidInput(t *testing.T) {
	api := NewClient("http://127.0.0.1:1").AgentTasks()
	ctx := context.Background()
	if _, err := api.ListLegacySessions(ctx, 0, ""); err == nil {
		t.Fatal("expected list limit validation")
	}
	if _, err := api.ExportLegacySession(ctx, "id", 2001, 0); err == nil {
		t.Fatal("expected export limit validation")
	}
	if _, err := api.ExportLegacySession(ctx, "id", 10, -1); err == nil {
		t.Fatal("expected event cursor validation")
	}
	if _, err := api.ClaimLegacySession(ctx, "id", LegacyAgentSessionClaimRequest{Namespace: "payments"}); err == nil {
		t.Fatal("expected acknowledgement validation")
	}
}
