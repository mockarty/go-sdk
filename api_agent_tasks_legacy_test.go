package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentTasksLegacySessionRecovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agent/sessions/legacy":
			if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("cursor") != "next/value" {
				t.Fatalf("list query = %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sessions":   []map[string]any{{"originalId": "legacy-1", "eventCount": 3}},
				"nextCursor": "next-2",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/agent/sessions/legacy/legacy-1/claim":
			var request LegacyAgentSessionClaimRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Namespace != "payments" || !request.AcknowledgeUnknownOrigin {
				t.Fatalf("claim request = %+v", request)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"id": "session-1", "namespace": "payments"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	api := NewClient(srv.URL).AgentTasks()
	page, err := api.ListLegacySessions(context.Background(), 10, "next/value")
	if err != nil || page.NextCursor != "next-2" || len(page.Sessions) != 1 || page.Sessions[0].OriginalID != "legacy-1" {
		t.Fatalf("ListLegacySessions page=%+v err=%v", page, err)
	}
	session, err := api.ClaimLegacySession(context.Background(), "legacy-1", LegacyAgentSessionClaimRequest{
		Namespace: "payments", AcknowledgeUnknownOrigin: true,
	})
	if err != nil || session.ID != "session-1" || session.Namespace != "payments" {
		t.Fatalf("ClaimLegacySession session=%+v err=%v", session, err)
	}
}
