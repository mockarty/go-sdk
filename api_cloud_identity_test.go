// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudIdentityStepUpCookieAndUnlink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cloud-session" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/api/v1/cloud/auth/step-up":
			var request CloudStepUpRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Action != "oauth_identity_unlink" || !request.ForceCredential {
				t.Fatalf("step-up = %#v", request)
			}
			http.SetCookie(w, &http.Cookie{Name: "mockarty_cloud_step_up", Value: "proof", Path: "/api/v1/cloud"})
			_, _ = w.Write([]byte(`{"status":"verified","action":"oauth_identity_unlink","expires_at":"2026-08-29T00:00:00Z"}`))
		case "/api/v1/cloud/auth/oauth/identities/github":
			cookie, err := r.Cookie("mockarty_cloud_step_up")
			if err != nil || cookie.Value != "proof" || r.Header.Get("Idempotency-Key") != "unlink-1" {
				t.Fatalf("unlink cookie=%#v err=%v key=%q", cookie, err, r.Header.Get("Idempotency-Key"))
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, WithAPIKey("cloud-session"))
	if _, err := client.CloudIdentity().StepUp(context.Background(), CloudStepUpRequest{Action: "oauth_identity_unlink", Credential: "current", ForceCredential: true}); err != nil {
		t.Fatal(err)
	}
	if err := client.CloudIdentity().Unlink(context.Background(), "github", "unlink-1"); err != nil {
		t.Fatal(err)
	}
	link, err := client.CloudIdentity().LinkURL("github")
	if err != nil || !strings.HasSuffix(link, "/api/v1/cloud/auth/oauth/github/link") {
		t.Fatalf("link=%q err=%v", link, err)
	}
}

func TestCloudIdentityEmailChangeUsesTwoProofs(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cloud-session" {
			t.Fatalf("missing Cloud session on %s", r.URL.Path)
		}
		paths = append(paths, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPatch {
			var request CloudEmailChangeRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.NewEmail != "new@example.test" {
				t.Fatalf("request=%#v err=%v", request, err)
			}
			_, _ = w.Write([]byte(`{"status":"verification_pending","stage":"awaiting_current_email"}`))
			return
		}
		if r.Method == http.MethodPost {
			var proof map[string]string
			if err := json.NewDecoder(r.Body).Decode(&proof); err != nil || proof["token"] != "proof" {
				t.Fatalf("proof=%#v err=%v", proof, err)
			}
			_, _ = w.Write([]byte(`{"status":"verification_pending","stage":"awaiting_new_email"}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	api := NewClient(server.URL, WithAPIKey("cloud-session")).CloudIdentity()
	stage, err := api.RequestEmailChange(context.Background(), CloudEmailChangeRequest{NewEmail: "new@example.test"})
	if err != nil || stage.Stage != "awaiting_current_email" {
		t.Fatalf("request stage=%#v err=%v", stage, err)
	}
	if _, err := api.AuthorizeCurrentEmail(context.Background(), "proof"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ConfirmNewEmail(context.Background(), "proof"); err != nil {
		t.Fatal(err)
	}
	if err := api.CancelEmailChange(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATCH /api/v1/cloud/email", "POST /api/v1/cloud/auth/email/change/authorize",
		"POST /api/v1/cloud/auth/email/change/confirm", "DELETE /api/v1/cloud/email/pending"}
	if strings.Join(paths, "|") != strings.Join(want, "|") {
		t.Fatalf("paths=%v", paths)
	}
}
