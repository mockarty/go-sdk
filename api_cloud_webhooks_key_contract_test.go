// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
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
	for _, key := range []string{"", " ", "\t"} {
		t.Run(fmt.Sprintf("key=%q", key), func(t *testing.T) {
			before := calls.Load()
			_, err := api.CreateWithIdempotencyKey(context.Background(), "owned-space", "owned-hook", "https://example.invalid/hook", []string{"instance.created"}, key)
			if err == nil || calls.Load() != before {
				t.Errorf("blank named key sent an unkeyed creation: error=%v calls=%d", err, calls.Load()-before)
			}
		})
	}
}
