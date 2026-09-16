// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// delegatedServer serves the five delegated-credential endpoints and records
// what the client sent, so a test can assert on the wire rather than on the
// Go struct that produced it.
type delegatedServer struct {
	t          *testing.T
	calls      int
	createBody map[string]interface{}
}

func (s *delegatedServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls++
		if got := r.Header.Get("X-API-Key"); got != "key" {
			s.t.Fatalf("X-API-Key=%q, want key", got)
		}
		path := r.URL.EscapedPath()
		switch {
		case r.Method == http.MethodPost && path == "/api/v1/auth/delegated-credentials":
			if err := json.NewDecoder(r.Body).Decode(&s.createBody); err != nil {
				s.t.Fatalf("create body decode: %v", err)
			}
			expires, _ := s.createBody["expires_at"].(string)
			if expires == "" {
				s.t.Fatalf("create body must carry expires_at in snake_case: %v", s.createBody)
			}
			if _, err := time.Parse(time.RFC3339, expires); err != nil {
				s.t.Fatalf("expires_at %q is not RFC3339: %v", expires, err)
			}
			if s.createBody["expiresAt"] != nil {
				s.t.Fatalf("create body leaked a camelCase expiresAt: %v", s.createBody)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"credential":` + delegatedCredentialJSON + `,"token":"mkd_plainbearer","message":"created"}`))

		case r.Method == http.MethodGet && path == "/api/v1/auth/delegated-credentials":
			_, _ = w.Write([]byte(`{"credentials":[` + delegatedCredentialJSON + `],"count":1}`))

		case r.Method == http.MethodGet && path == "/api/v1/auth/delegated-credentials/cred-1":
			_, _ = w.Write([]byte(delegatedCredentialJSON))

		case r.Method == http.MethodPost && path == "/api/v1/auth/delegated-credentials/cred-1/rotate":
			_, _ = w.Write([]byte(`{"credential":` + delegatedCredentialJSON + `,"token":"mkd_rotated","message":"rotated"}`))

		case r.Method == http.MethodDelete && path == "/api/v1/auth/delegated-credentials/cred-1":
			_, _ = w.Write([]byte(`{"message":"Delegated credential revoked"}`))

		case r.Method == http.MethodDelete && path == "/api/v1/auth/delegated-credentials/cred-forbidden":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"requested scope exceeds this delegated credential's own scope","code":"forbidden"}`))

		default:
			s.t.Fatalf("unexpected request %s %s", r.Method, path)
		}
	})
}

// delegatedCredentialJSON is the response body the SERVER actually emits, key
// for key, after the json tags were restored. Written as a literal rather than
// by marshalling the Go struct on purpose: marshalling the same struct the
// client decodes into would make this test agree with itself no matter what
// the server sends.
const delegatedCredentialJSON = `{
	"created_at":"2026-09-16T12:00:00Z",
	"updated_at":"2026-09-16T12:00:00Z",
	"expires_at":"2026-09-17T12:00:00Z",
	"revoked_at":"2026-09-16T13:00:00Z",
	"last_used_at":"2026-09-16T12:30:00Z",
	"metadata":{"ticket":"C-D-05"},
	"id":"cred-1",
	"name":"ci-reader",
	"description":"read-only CI credential",
	"issuer_user_id":"user-1",
	"issuer_credential_id":"cred-parent",
	"namespaces":["alpha","beta"],
	"allowed_actions":["read"],
	"token_prefix":"mkd_abcd1234…",
	"issuer_user_role":"admin",
	"issuer_user_email":"owner@example.com",
	"issuer_user_login":"owner"
}`

func newDelegatedTestClient(t *testing.T, server *delegatedServer) *Client {
	t.Helper()
	httpServer := httptest.NewServer(server.handler())
	t.Cleanup(httpServer.Close)
	return NewClient(httpServer.URL, WithAPIKey("key"))
}

// The whole point of the cascade: a client generated from the published
// contract must read the frozen scope off the response. Before the server's
// json tags were restored the keys were Go field names, so every field below
// except namespaces/allowed_actions/token_prefix decoded as its zero value and
// this test found a credential with no expiry and no issuer.
func TestDelegatedCredentialsResponseDecodesTheFrozenScope(t *testing.T) {
	server := &delegatedServer{t: t}
	client := newDelegatedTestClient(t, server)

	cred, err := client.DelegatedCredentials().Get(context.Background(), "cred-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if cred.ID != "cred-1" || cred.Name != "ci-reader" || cred.Description != "read-only CI credential" {
		t.Fatalf("identity did not decode: %+v", cred)
	}
	if cred.IssuerUserID != "user-1" || cred.IssuerUserRole != "admin" ||
		cred.IssuerUserEmail != "owner@example.com" || cred.IssuerUserLogin != "owner" {
		t.Fatalf("issuer identity did not decode: %+v", cred)
	}
	if cred.IssuerCredentialID != "cred-parent" {
		t.Fatalf("issuer_credential_id=%q", cred.IssuerCredentialID)
	}
	if !cred.ExpiresAt.Equal(time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expires_at did not decode: %v", cred.ExpiresAt)
	}
	if cred.CreatedAt.IsZero() || cred.UpdatedAt.IsZero() {
		t.Fatalf("timestamps did not decode: %+v", cred)
	}
	if cred.RevokedAt == nil || cred.LastUsedAt == nil {
		t.Fatalf("optional timestamps did not decode: %+v", cred)
	}
	if cred.TokenPrefix != "mkd_abcd1234…" {
		t.Fatalf("token_prefix=%q", cred.TokenPrefix)
	}
	if len(cred.Namespaces) != 2 || cred.Namespaces[0] != "alpha" {
		t.Fatalf("namespaces=%v", cred.Namespaces)
	}
	if len(cred.AllowedActions) != 1 || cred.AllowedActions[0] != "read" {
		t.Fatalf("allowed_actions=%v", cred.AllowedActions)
	}

	// The client-side scope helpers must answer from the decoded row.
	if !cred.HasNamespace("beta") || cred.HasNamespace("gamma") {
		t.Fatalf("HasNamespace disagreed with the decoded set: %v", cred.Namespaces)
	}
	if !cred.IsRevoked() {
		t.Fatalf("a credential carrying revoked_at must report revoked")
	}
	if cred.IsExpired(time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("a credential whose expiry is a day away must not read as expired")
	}
	if !cred.IsExpired(time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("a credential past its expiry must read as expired")
	}
}

func TestDelegatedCredentialsLifecycleOverTheWire(t *testing.T) {
	server := &delegatedServer{t: t}
	client := newDelegatedTestClient(t, server)
	api := client.DelegatedCredentials()
	expires := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	created, err := api.Create(context.Background(), DelegatedCredentialCreateRequest{
		Name:           "ci-reader",
		Namespaces:     []string{"alpha"},
		AllowedActions: []string{"read"},
		ExpiresAt:      &expires,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Token != "mkd_plainbearer" {
		t.Fatalf("create token=%q — the plain bearer is returned exactly once and must be readable", created.Token)
	}
	if created.Credential.ID != "cred-1" {
		t.Fatalf("create credential=%+v", created.Credential)
	}
	if got := server.createBody["namespaces"]; got == nil {
		t.Fatalf("create body dropped namespaces: %v", server.createBody)
	}
	if got, _ := server.createBody["allowed_actions"].([]interface{}); len(got) != 1 || got[0] != "read" {
		t.Fatalf("create body allowed_actions=%v", server.createBody["allowed_actions"])
	}

	listed, err := api.List(context.Background())
	if err != nil || listed.Count != 1 || len(listed.Credentials) != 1 {
		t.Fatalf("list=%+v err=%v", listed, err)
	}

	rotated, err := api.Rotate(context.Background(), "cred-1")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rotated.Token != "mkd_rotated" {
		t.Fatalf("rotate token=%q", rotated.Token)
	}

	if err := api.Revoke(context.Background(), "cred-1"); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// create + list + rotate + revoke; nothing retried, nothing extra.
	if server.calls != 4 {
		t.Fatalf("expected 4 lifecycle calls, saw %d", server.calls)
	}
}

// A malformed scope must be refused by the client, without a round-trip: the
// server's own refusals are 400/403 incidents that an operator has to read,
// and spending one on a request the client could have rejected locally hides
// the incidents that matter.
func TestDelegatedCredentialsCreateRefusesAMalformedScopeLocally(t *testing.T) {
	server := &delegatedServer{t: t}
	client := newDelegatedTestClient(t, server)
	api := client.DelegatedCredentials()

	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)
	tooFar := time.Now().UTC().Add(DelegatedCredentialMaxTTL + 24*time.Hour)

	cases := []struct {
		name string
		req  DelegatedCredentialCreateRequest
	}{
		{"no namespaces", DelegatedCredentialCreateRequest{ExpiresAt: &future}},
		{"blank namespace", DelegatedCredentialCreateRequest{Namespaces: []string{"  "}, ExpiresAt: &future}},
		{"no expiry", DelegatedCredentialCreateRequest{Namespaces: []string{"alpha"}}},
		{"expiry in the past", DelegatedCredentialCreateRequest{Namespaces: []string{"alpha"}, ExpiresAt: &past}},
		{"expiry beyond the cap", DelegatedCredentialCreateRequest{Namespaces: []string{"alpha"}, ExpiresAt: &tooFar}},
		{"unknown action", DelegatedCredentialCreateRequest{
			Namespaces: []string{"alpha"}, ExpiresAt: &future, AllowedActions: []string{"admin"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := api.Create(context.Background(), tc.req); err == nil {
				t.Fatalf("a malformed scope must be refused before the request is sent")
			}
		})
	}

	if server.calls != 0 {
		t.Fatalf("a locally-refused create must not reach the server; saw %d calls", server.calls)
	}
}

// The two refusals a holder actually hits must stay distinguishable: "no such
// credential" is what a delegated credential sees when it probes its issuer's
// ids, and a scope-escalation refusal is what it sees when it asks for more
// than it holds. Collapsing them would hide the escalation attempt.
func TestDelegatedCredentialsSurfacesServerRefusals(t *testing.T) {
	server := &delegatedServer{t: t}
	client := newDelegatedTestClient(t, server)

	err := client.DelegatedCredentials().Revoke(context.Background(), "cred-forbidden")
	if err == nil {
		t.Fatalf("a 403 must surface as an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", apiErr.StatusCode)
	}
}

func TestDelegatedCredentialsRefusesAnEmptyIDLocally(t *testing.T) {
	server := &delegatedServer{t: t}
	client := newDelegatedTestClient(t, server)
	api := client.DelegatedCredentials()

	for name, call := range map[string]func() error{
		"get":    func() error { _, err := api.Get(context.Background(), "  "); return err },
		"rotate": func() error { _, err := api.Rotate(context.Background(), ""); return err },
		"revoke": func() error { return api.Revoke(context.Background(), "") },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatalf("an empty credential id must be refused before the request is sent")
			}
		})
	}
	if server.calls != 0 {
		t.Fatalf("an empty id must not reach the server; saw %d calls", server.calls)
	}
}
