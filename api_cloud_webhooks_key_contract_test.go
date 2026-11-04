// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCloudWebhookNamedCreateRefusesBlankKeyBeforeRequest(t *testing.T) {
	var calls atomic.Int64
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"webhook":{"id":"owned-hook"},"secret":"owned-secret"}`)
	}))
	defer peer.Close()
	api := NewClient(peer.URL).CloudWebhooks()
	for _, key := range []string{"", " ", "\t", "\u00a0", "\u0085"} {
		t.Run(fmt.Sprintf("key=%q", key), func(t *testing.T) {
			before := calls.Load()
			_, err := api.CreateWithIdempotencyKey(context.Background(), "owned-space", "owned-hook", "https://example.invalid/hook", []string{"instance.created"}, key)
			if err == nil || calls.Load() != before {
				t.Errorf("blank named key sent an unkeyed creation: error=%v calls=%d", err, calls.Load()-before)
			}
		})
	}
}

func TestCloudWebhookOrdinaryCreateAndStableRetryRemainCompatible(t *testing.T) {
	type request struct{ key, body string }
	var seen []request
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = append(seen, request{r.Header.Get("Idempotency-Key"), string(raw)})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"webhook":{"id":"owned-hook"},"secret":"owned-secret"}`)
	}))
	defer peer.Close()
	api := NewClient(peer.URL).CloudWebhooks()
	for i := 0; i < 2; i++ {
		created, err := api.CreateWithIdempotencyKey(context.Background(), "owned-space", "owned-hook", "https://example.invalid/hook", []string{"instance.created"}, "caller-owned-key")
		if err != nil || created.Secret != "owned-secret" {
			t.Fatalf("stable keyed create: %v %+v", err, created)
		}
	}
	if len(seen) != 2 || seen[0].key != "caller-owned-key" || seen[1] != seen[0] {
		t.Fatalf("retry identity or body changed: %+v", seen)
	}
	if _, err := api.Create(context.Background(), "owned-space", "owned-hook", "https://example.invalid/hook", []string{"instance.created"}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 || seen[2].key != "" {
		t.Fatal("ordinary Create must remain unkeyed")
	}
	before := len(seen)
	for _, key := range []string{"", " ", "\t", "\u00a0", "\u0085"} {
		if _, err := api.RotateSecret(context.Background(), "owned-space", "owned-hook", key); err == nil {
			t.Errorf("blank rotate key accepted: %q", key)
		}
	}
	if len(seen) != before {
		t.Error("blank rotate key reached network")
	}
}
