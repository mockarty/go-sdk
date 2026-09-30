// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSavedCILaunchErrorPreservesIdentityWithoutRetry(t *testing.T) {
	for _, tc := range []struct {
		status int
		reason string
		code   string
	}{{409, "ci_dispatch_uncertain", "conflict"}, {503, "ci_dispatch_failed", "unavailable"}} {
		t.Run(tc.reason, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"error":"inspect saved launch","code":%q,"reason":%q,"runId":"saved-run","request_id":"request-1"}`, tc.code, tc.reason)
			}))
			defer srv.Close()
			c := NewClient(srv.URL, WithRetry(2, time.Millisecond))
			_, err := c.TestPlans().Run(context.Background(), "plan", RunOptions{})
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError: %v", err)
			}
			if hits != 1 || apiErr.Reason != tc.reason || apiErr.RunID != "saved-run" || apiErr.RequestID != "request-1" {
				t.Fatalf("saved launch lost or replayed: hits=%d error=%+v", hits, apiErr)
			}
		})
	}
}

func TestOrdinaryUnavailablePOSTStillRetries(t *testing.T) {
	for _, body := range []string{`{"error":"transient"}`, `null`, `[]`, `{"error":"transient","reason":23,"runId":{}}`, `{"error":"transient","reason":"ci_dispatch_failed"}`, `{"error":"transient","runId":"saved-run","reason":"another_reason"}`} {
		hits := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.WriteHeader(503); fmt.Fprint(w, body) }))
		c := NewClient(srv.URL, WithRetry(2, time.Millisecond))
		_ = c.do(context.Background(), "POST", "/test", nil, nil)
		srv.Close()
		if hits != 3 {
			t.Fatalf("ordinary retry changed for %s: %d", body, hits)
		}
	}
}

func TestAPIErrorLegacyDefaults(t *testing.T) {
	e := &APIError{StatusCode: 503, Message: "old error"}
	if e.Reason != "" || e.RunID != "" || !errors.Is(e, ErrUnavailable) {
		t.Fatal("legacy error compatibility changed")
	}
}
