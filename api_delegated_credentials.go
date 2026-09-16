// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DelegatedCredentialsAPI manages project-scoped delegated credentials.
//
// A delegated credential is the primitive for SHARED work: an issuer (a Space
// owner, or one of their automation principals) freezes
//
//   - the set of namespaces the credential may act in,
//   - the set of actions it may perform (read / write / delete),
//   - the instant it stops working,
//
// and the holder has no path to widen any of the three. That is what makes it
// different from an API key, whose namespace is a DEFAULT its holder can
// change and which carries no expiry of its own.
//
// The three properties are enforced by the server, not by this client: the
// scope is read from the credential's stored row on every request, a namespace
// header or query parameter cannot widen it, and the row itself rejects any
// write to the scope columns. This client therefore validates only what a
// round-trip cannot fix — a missing namespace set, a missing expiry, an
// over-long TTL — and surfaces every server refusal verbatim.
type DelegatedCredentialsAPI struct{ client *Client }

// DelegatedActionVocabulary is the closed set of actions a delegated scope may
// name. The server rejects anything outside it, so a caller can validate
// locally before spending a round-trip.
var DelegatedActionVocabulary = []string{"read", "write", "delete"}

// DelegatedCredentialMaxTTL mirrors the server's cap on how far ahead an
// expiry may be pinned. A credential that outlives a review cycle is the
// property this primitive exists to prevent, so the cap is enforced on both
// sides.
const DelegatedCredentialMaxTTL = 365 * 24 * time.Hour

// DelegatedCredential is one issued delegated credential. The stored bcrypt
// hash and sha256 index are never part of this payload.
type DelegatedCredential struct {
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	ExpiresAt          time.Time              `json:"expires_at"`
	RevokedAt          *time.Time             `json:"revoked_at,omitempty"`
	LastUsedAt         *time.Time             `json:"last_used_at,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	ID                 string                 `json:"id"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description,omitempty"`
	IssuerUserID       string                 `json:"issuer_user_id"`
	IssuerCredentialID string                 `json:"issuer_credential_id,omitempty"`
	Namespaces         []string               `json:"namespaces"`
	AllowedActions     []string               `json:"allowed_actions"`
	TokenPrefix        string                 `json:"token_prefix"`
	IssuerUserRole     string                 `json:"issuer_user_role,omitempty"`
	IssuerUserEmail    string                 `json:"issuer_user_email,omitempty"`
	IssuerUserLogin    string                 `json:"issuer_user_login,omitempty"`
}

// IsRevoked reports whether the credential has been revoked. Revocation is
// terminal — there is no un-revoke.
func (d *DelegatedCredential) IsRevoked() bool { return d != nil && d.RevokedAt != nil }

// IsExpired reports whether the credential has lapsed at now.
func (d *DelegatedCredential) IsExpired(now time.Time) bool {
	return d != nil && !now.Before(d.ExpiresAt)
}

// HasNamespace reports whether ns is inside the frozen namespace set.
func (d *DelegatedCredential) HasNamespace(ns string) bool {
	if d == nil {
		return false
	}
	for _, candidate := range d.Namespaces {
		if candidate == ns {
			return true
		}
	}
	return false
}

// DelegatedCredentialCreateRequest is the create payload. Note what is NOT
// here: no issuer field and no scope that can later be extended. The issuer is
// taken from the authenticated principal; the scope is frozen at creation.
type DelegatedCredentialCreateRequest struct {
	// ExpiresAt is required. An unbounded delegated credential is an API key,
	// which is the thing this primitive exists to replace.
	ExpiresAt *time.Time
	Metadata  map[string]interface{}
	// Namespaces is required and must contain at least one namespace.
	Namespaces []string
	// AllowedActions may be omitted, which the server reads as the full
	// vocabulary ("read", "write", "delete").
	AllowedActions []string
	Name           string
	Description    string
}

// DelegatedCredentialWithToken is the create/rotate response. Token is the
// ONLY time the plain bearer is ever returned: the row stores bcrypt + sha256,
// so it cannot be read back. A caller that loses it must rotate.
type DelegatedCredentialWithToken struct {
	Credential DelegatedCredential `json:"credential"`
	Token      string              `json:"token"`
	Message    string              `json:"message"`
}

// DelegatedCredentialList is the list response.
type DelegatedCredentialList struct {
	Credentials []DelegatedCredential `json:"credentials"`
	Count       int                   `json:"count"`
}

const delegatedCredentialsPath = "/api/v1/auth/delegated-credentials"

func delegatedCredentialPath(id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("mockarty: delegated credential id is required")
	}
	return delegatedCredentialsPath + "/" + url.PathEscape(id), nil
}

// validateDelegatedCreate mirrors the server's scope rules so a caller learns
// about a malformed request without a round-trip. It deliberately does NOT
// narrow anything: a scope that would exceed the caller's own authority is the
// server's refusal to make, and silently shrinking the request here would hand
// the caller a credential that does not do what it asked for.
func validateDelegatedCreate(req DelegatedCredentialCreateRequest, now time.Time) error {
	if len(req.Namespaces) == 0 {
		return fmt.Errorf("mockarty: at least one namespace is required — an unscoped delegated credential is an API key")
	}
	for _, ns := range req.Namespaces {
		if strings.TrimSpace(ns) == "" {
			return fmt.Errorf("mockarty: namespace entries must not be blank")
		}
	}
	if req.ExpiresAt == nil || req.ExpiresAt.IsZero() {
		return fmt.Errorf("mockarty: ExpiresAt is required — an unbounded delegated credential is an API key")
	}
	if !req.ExpiresAt.After(now) {
		return fmt.Errorf("mockarty: ExpiresAt must be in the future")
	}
	if req.ExpiresAt.Sub(now) > DelegatedCredentialMaxTTL {
		return fmt.Errorf("mockarty: ExpiresAt may be at most %d days ahead", int(DelegatedCredentialMaxTTL.Hours()/24))
	}
	allowed := make(map[string]bool, len(DelegatedActionVocabulary))
	for _, action := range DelegatedActionVocabulary {
		allowed[action] = true
	}
	for _, action := range req.AllowedActions {
		if !allowed[strings.TrimSpace(action)] {
			return fmt.Errorf("mockarty: %q is not a known action (want one of %s)",
				action, strings.Join(DelegatedActionVocabulary, ", "))
		}
	}
	return nil
}

// Create issues a delegated credential and returns its plain bearer exactly
// once.
func (a *DelegatedCredentialsAPI) Create(ctx context.Context, req DelegatedCredentialCreateRequest) (*DelegatedCredentialWithToken, error) {
	now := time.Now().UTC()
	if err := validateDelegatedCreate(req, now); err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"name":       req.Name,
		"namespaces": req.Namespaces,
		"expires_at": req.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if len(req.AllowedActions) > 0 {
		body["allowed_actions"] = req.AllowedActions
	}
	if req.Metadata != nil {
		body["metadata"] = req.Metadata
	}

	var out DelegatedCredentialWithToken
	if err := a.client.do(ctx, http.MethodPost, delegatedCredentialsPath, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the credentials THIS caller may manage: a user session sees the
// credentials it issued, and a delegated credential sees only the ones it
// minted itself — never its issuer's.
func (a *DelegatedCredentialsAPI) List(ctx context.Context) (*DelegatedCredentialList, error) {
	var out DelegatedCredentialList
	if err := a.client.do(ctx, http.MethodGet, delegatedCredentialsPath, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get reads one credential. A credential outside the caller's authority is
// reported as not-found rather than forbidden, so a delegated credential
// cannot enumerate its issuer's credentials by probing ids.
func (a *DelegatedCredentialsAPI) Get(ctx context.Context, id string) (*DelegatedCredential, error) {
	path, err := delegatedCredentialPath(id)
	if err != nil {
		return nil, err
	}
	var out DelegatedCredential
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke permanently disables a credential. Idempotent: revoking an already
// revoked credential is a no-op rather than an error, because the terminal
// state the caller wanted is the state that holds.
func (a *DelegatedCredentialsAPI) Revoke(ctx context.Context, id string) error {
	path, err := delegatedCredentialPath(id)
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, path, nil, nil)
}

// Rotate issues a fresh bearer for an existing credential and leaves its scope
// untouched — the operation to reach for when a secret leaked but the work must
// continue. The previous bearer stops working as soon as this returns.
//
// Rotation is deliberately not a scope change: widening or narrowing a live
// credential's authority is what re-issuing is for, so that the change is a new
// credential with its own audit trail rather than a silent edit.
func (a *DelegatedCredentialsAPI) Rotate(ctx context.Context, id string) (*DelegatedCredentialWithToken, error) {
	path, err := delegatedCredentialPath(id)
	if err != nil {
		return nil, err
	}
	var out DelegatedCredentialWithToken
	if err := a.client.do(ctx, http.MethodPost, path+"/rotate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
