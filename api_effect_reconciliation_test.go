package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEffectReconciliationAPIListQueueAndReconcileNoEffect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/admin/effects/reconciliation":
			if r.Method != http.MethodGet || r.URL.Query().Get("namespace") != "team a" ||
				r.URL.Query().Get("family") != "llm.chat" || r.URL.Query().Get("minAgeSeconds") != "60" ||
				r.URL.Query().Get("limit") != "25" {
				t.Fatalf("list request = %s %s", r.Method, r.URL.RequestURI())
			}
			_, _ = w.Write([]byte(`{"items":[],"nextCursor":"next"}`))
		case "/api/v1/admin/effects/reconciliation/reconcile":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["namespace"] != "team a" ||
				body["executionId"] != "effect-1" || body["decision"] != "no_effect" || body["autoClaim"] != true {
				t.Fatalf("reconcile body = %#v, error = %v", body, err)
			}
			_, _ = w.Write([]byte(`{"executionId":"effect-1","status":"no_effect","reason":"provider_no_effect","effectFamily":"llm.chat"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, WithNamespace("team a"))
	page, err := client.EffectReconciliation().ListQueue(context.Background(), EffectReconciliationListOptions{
		EffectFamily: "llm.chat", MinAge: time.Minute, Limit: 25,
	})
	if err != nil || page.NextCursor != "next" {
		t.Fatalf("ListQueue() = %#v, %v", page, err)
	}
	result, err := client.EffectReconciliation().ReconcileNoEffect(context.Background(), "effect-1", "invoice-1", "provider_invoice")
	if err != nil || result.Status != "no_effect" {
		t.Fatalf("ReconcileNoEffect() = %#v, %v", result, err)
	}
}
