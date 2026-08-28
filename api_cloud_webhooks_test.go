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

func TestCloudWebhooksLifecycleAndOneTimeRotation(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer session-token" || r.Header.Get("X-API-Key") != "" {
			t.Fatalf("credential headers: %#v", r.Header)
		}
		if got := r.URL.Query().Get("workspace_id"); got != "space-a" {
			t.Fatalf("workspace_id = %q", got)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloud/webhooks":
			_ = json.NewEncoder(w).Encode(map[string]any{"webhooks": []map[string]any{{"id": "hook-1"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloud/webhooks":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["name"] != "Build events" || body["url"] != "https://hooks.example/events" {
				t.Fatalf("create body = %#v", body)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"webhook": map[string]any{"id": "hook-1"}, "secret": "whsec_once"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloud/webhooks/hook-1/rotate-secret":
			if got := r.Header.Get("Idempotency-Key"); got != "retry-1" {
				t.Fatalf("Idempotency-Key = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"webhook": map[string]any{"id": "hook-1"}, "secret": "whsec_rotated"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloud/webhooks/hook-1/test":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "test_dispatched"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloud/webhooks/hook-1/deliveries":
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Fatalf("limit = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"deliveries": []map[string]any{{"id": "delivery-1"}}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/cloud/webhooks/hook-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	api := NewClient(srv.URL, WithAPIKey("session-token")).CloudWebhooks()
	ctx := context.Background()
	if hooks, err := api.List(ctx, "space-a"); err != nil || len(hooks) != 1 {
		t.Fatalf("list = %+v err=%v", hooks, err)
	}
	if created, err := api.Create(ctx, "space-a", "Build events", "https://hooks.example/events", []string{"instance.created"}); err != nil || created.Secret != "whsec_once" {
		t.Fatalf("create = %+v err=%v", created, err)
	}
	if rotated, err := api.RotateSecret(ctx, "space-a", "hook-1", "retry-1"); err != nil || rotated.Secret != "whsec_rotated" {
		t.Fatalf("rotate = %+v err=%v", rotated, err)
	}
	if err := api.Test(ctx, "space-a", "hook-1"); err != nil {
		t.Fatal(err)
	}
	if deliveries, err := api.ListDeliveries(ctx, "space-a", "hook-1", 999); err != nil || len(deliveries) != 1 {
		t.Fatalf("deliveries = %+v err=%v", deliveries, err)
	}
	if err := api.Deactivate(ctx, "space-a", "hook-1"); err != nil {
		t.Fatal(err)
	}
	if calls != 6 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestCloudWebhooksRejectsMissingRotationIdentity(t *testing.T) {
	api := NewClient("http://example.invalid").CloudWebhooks()
	if _, err := api.RotateSecret(context.Background(), "space-a", "", "retry-1"); err == nil {
		t.Fatal("empty webhook id accepted")
	}
	if _, err := api.RotateSecret(context.Background(), "space-a", "hook-1", ""); err == nil {
		t.Fatal("empty idempotency key accepted")
	}
}
