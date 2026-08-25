package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeliveryPolicyAPIForwardsStrongMutationHeaders(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("namespace"); got != "team-a" {
			t.Fatalf("request %d namespace=%q, want client namespace", requests, got)
		}
		switch requests {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/admin/delivery-policy/environments" || r.Header.Get("Idempotency-Key") != "create-a" {
				t.Fatalf("create request=%s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
		case 2:
			if r.Method != http.MethodPut || r.URL.Path != "/api/v1/admin/delivery-policy/environments/staging" || r.Header.Get("If-Match") != `"dp-env:staging:1:a"` || r.Header.Get("Idempotency-Key") != "advance-a" {
				t.Fatalf("advance request=%s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
		case 3:
			if r.Method != http.MethodDelete || r.Header.Get("If-Match") != `"dp-env:staging:2:b"` {
				t.Fatalf("revoke request=%s %s headers=%v", r.Method, r.URL.Path, r.Header)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(DeliveryPolicyEnvironment{ID: "staging", ETag: `"dp-env:staging:1:a"`, Revision: 1})
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.URL, WithAPIKey("mk_test"), WithNamespace("team-a"))
	request := DeliveryPolicyEnvironmentRequest{ID: "staging", ProjectID: "project-a", Class: "staging", Profile: "standard", AuditID: "audit-a", EvidenceID: "evidence-a"}
	if _, err := client.DeliveryPolicy().Create(context.Background(), request, "create-a"); err != nil {
		t.Fatal(err)
	}
	request.ID = ""
	if _, err := client.DeliveryPolicy().Advance(context.Background(), "staging", request, `"dp-env:staging:1:a"`, "advance-a"); err != nil {
		t.Fatal(err)
	}
	if err := client.DeliveryPolicy().Revoke(context.Background(), "staging", `"dp-env:staging:2:b"`); err != nil {
		t.Fatal(err)
	}
}
