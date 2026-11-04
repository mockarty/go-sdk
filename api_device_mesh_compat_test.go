// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDeviceMeshPublishedNamesPreserveCanonicalWireAndErrors(t *testing.T) {
	for _, deny := range []bool{false, true} {
		t.Run(fmt.Sprintf("deny=%t", deny), func(t *testing.T) {
			type request struct{ path, body string }
			var seen []request
			peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				seen = append(seen, request{r.URL.RequestURI(), string(body)})
				w.Header().Set("Content-Type", "application/json")
				if deny {
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"error":"owned refusal"}`)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/commands") {
					fmt.Fprint(w, `{"command":{"id":"owned-command"},"replayed":true}`)
				} else {
					fmt.Fprint(w, `{"id":"owned-command","state":"failed"}`)
				}
			}))
			defer peer.Close()
			api := NewClient(peer.URL, WithRetry(0, time.Millisecond)).DeviceMesh()
			ctx := context.Background()
			input := MeshSubmitInput{Kind: "ping", IdempotencyKey: "original-retry-key"}
			first, replayed, err := api.SubmitCommand(ctx, "owned-space", input)
			second, replayed2, err2 := api.Submit(ctx, "owned-space", input)
			if deny {
				if err == nil || err2 == nil || err.Error() != err2.Error() {
					t.Fatal("legacy name swallowed refusal")
				}
			} else if err != nil || err2 != nil || first.ID != second.ID || !replayed || !replayed2 {
				t.Fatal("legacy name changed submit response/replay semantics")
			}
			report := MeshCommandReport{State: "failed", FailureReason: "owned refusal"}
			_, err = api.ReportCommand(ctx, "owned-space", "owned-device", "owned-command", report)
			_, err2 = api.Report(ctx, "owned-space", "owned-device", "owned-command", report)
			if deny {
				if err == nil || err2 == nil || err.Error() != err2.Error() {
					t.Fatal("legacy name swallowed report refusal")
				}
			} else if err != nil || err2 != nil {
				t.Fatal("report failed")
			}
			if len(seen) != 4 || seen[0] != seen[1] || seen[2] != seen[3] {
				t.Fatalf("legacy alias changed request or issued extra effects: %+v", seen)
			}
		})
	}
}
