// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// SecretsAPI provides methods for the centralised Secrets Storage feature.
//
// A secret store is a namespace-scoped container of encrypted key/value
// entries. Entry values are only exposed to callers with the `secret:read`
// permission — list/get on the store itself never returns decrypted values.
//
// Stores may optionally be backed by HashiCorp Vault (or any compatible
// KV v2 engine) via PUT /api/v1/namespaces/:ns/integrations/vault; in that
// case entries are read-through proxies and writes are forwarded to Vault.
type SecretsAPI struct {
	client *Client
}

// SecretStore describes a container of secret entries.
type SecretStore struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Description string    `json:"description,omitempty"`
	Backend     string    `json:"backend,omitempty"` // "inline" | "vault" | "aws_sm" | "gcp_sm" | "azure_kv" | "custom_api"
	EntryCount  int       `json:"entryCount"`
}

// SecretEntry is a single key/value pair. Value is only populated on
// single-entry GET requests when the caller has `secret:read`.
type SecretEntry struct {
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	RotatedAt   time.Time `json:"rotatedAt,omitempty"`
	Key         string    `json:"key"`
	Value       string    `json:"value,omitempty"`
	Description string    `json:"description,omitempty"`
	Version     int       `json:"version"`
}

// VaultIntegration configures a namespace's optional HashiCorp Vault
// backend. AuthMethod is one of "token", "approle", "kubernetes".
type VaultIntegration struct {
	URL        string `json:"url"`
	AuthMethod string `json:"authMethod"`
	Token      string `json:"token,omitempty"`
	RoleID     string `json:"roleId,omitempty"`
	SecretID   string `json:"secretId,omitempty"`
	MountPath  string `json:"mountPath,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

// ListStores returns every secret store visible to the caller in the
// client's default namespace.
//
// Wire shape: `{"stores":[...], "namespace":"..."}` — we unmarshal into
// the envelope and return the stores slice.
func (a *SecretsAPI) ListStores(ctx context.Context) ([]SecretStore, error) {
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	var env struct {
		Stores []SecretStore `json:"stores"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets?namespace="+url.QueryEscape(ns), nil, &env); err != nil {
		return nil, err
	}
	return env.Stores, nil
}

// CreateStore creates a new secret store. Backend defaults to "inline"
// (local AES-GCM via the KeyStore) when empty.
//
// Wire shape: server replies with `{"store":<SecretStore>}` — we
// unwrap before returning so the caller never has to know.
//
// Namespace plumbing: the server reads the namespace from the
// `?namespace=` query param, NOT from the request body — passing
// store.Namespace in the body is silently ignored. We pull it out
// of the struct and forward it on the URL.
func (a *SecretsAPI) CreateStore(ctx context.Context, store SecretStore) (*SecretStore, error) {
	ns := store.Namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		ns = "sandbox"
	}
	var env struct {
		Store SecretStore `json:"store"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets?namespace="+url.QueryEscape(ns), store, &env); err != nil {
		return nil, err
	}
	return &env.Store, nil
}

// GetStore fetches a single store by ID.
//
// Wire shape: `{"store":<SecretStore>}` — unwrapped before return.
// Namespace is read from the query param server-side.
func (a *SecretsAPI) GetStore(ctx context.Context, id string) (*SecretStore, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	var env struct {
		Store SecretStore `json:"store"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(id)+"?namespace="+url.QueryEscape(ns), nil, &env); err != nil {
		return nil, err
	}
	return &env.Store, nil
}

// UpdateStore updates the name/description/backend of an existing store.
//
// Wire shape: `{"store":<SecretStore>}` — unwrapped before return.
func (a *SecretsAPI) UpdateStore(ctx context.Context, id string, store SecretStore) (*SecretStore, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	ns := store.Namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		ns = "sandbox"
	}
	var env struct {
		Store SecretStore `json:"store"`
	}
	if err := a.client.do(ctx, "PUT", "/api/v1/stores/secrets/"+url.PathEscape(id)+"?namespace="+url.QueryEscape(ns), store, &env); err != nil {
		return nil, err
	}
	return &env.Store, nil
}

// DeleteStore removes a secret store and all of its entries.
func (a *SecretsAPI) DeleteStore(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("mockarty: secret store id is required")
	}
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	return a.client.do(ctx, "DELETE", "/api/v1/stores/secrets/"+url.PathEscape(id)+"?namespace="+url.QueryEscape(ns), nil, nil)
}

// nsQuery returns "?namespace=<X>" using the client's default NS, or
// "?namespace=sandbox" as a last-resort fallback. Centralised so every
// entry-level call adds it uniformly — the server reads NS from the
// query string and silently rejects requests that omit it.
func (a *SecretsAPI) nsQuery() string {
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	return "?namespace=" + url.QueryEscape(ns)
}

// ListEntries returns metadata (keys, versions, timestamps) for every
// entry in the store. Decrypted values are NOT returned.
//
// Wire shape: `{"entries":[...], "store_id":"...", "namespace":"..."}`.
func (a *SecretsAPI) ListEntries(ctx context.Context, storeID string) ([]SecretEntry, error) {
	if storeID == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var env struct {
		Entries []SecretEntry `json:"entries"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries"+a.nsQuery(), nil, &env); err != nil {
		return nil, err
	}
	return env.Entries, nil
}

// CreateEntry writes a new key/value pair to the store.
//
// Wire shape: `{"entry": {id, key, version, sensitive, created_at}}`.
// The decrypted value is NOT echoed back on create — fetch via
// GetEntry when needed.
func (a *SecretsAPI) CreateEntry(ctx context.Context, storeID string, entry SecretEntry) (*SecretEntry, error) {
	if storeID == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var env struct {
		Entry SecretEntry `json:"entry"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries"+a.nsQuery(), entry, &env); err != nil {
		return nil, err
	}
	return &env.Entry, nil
}

// GetEntry fetches a single entry including its decrypted value. Requires
// the `secret:read` permission on the caller's API key.
//
// Wire shape: flat `{id, key, value, version, sensitive, ...}` (NOT
// wrapped in an envelope on this endpoint) — unmarshals directly.
func (a *SecretsAPI) GetEntry(ctx context.Context, storeID, key string) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	var out SecretEntry
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key)+a.nsQuery(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEntry overwrites the value/description of an existing entry.
//
// Wire shape: `{"message":"entry updated", "id":"..."}` — no full entry
// echoed back, so we return only Key + ID after a successful update.
func (a *SecretsAPI) UpdateEntry(ctx context.Context, storeID, key string, entry SecretEntry) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	var env struct {
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	if err := a.client.do(ctx, "PUT", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key)+a.nsQuery(), entry, &env); err != nil {
		return nil, err
	}
	return &SecretEntry{Key: key}, nil
}

// RotateEntry replaces the entry's value with `newValue`, bumping its
// version and stamping RotatedAt. The old value is not retained —
// pair with store-level backups if a fallback is needed.
//
// Wire shape: server requires `{value: "..."}` in the request body;
// callers must supply the new secret. An empty newValue returns an
// explicit local error (the server would 400 with the same message).
func (a *SecretsAPI) RotateEntry(ctx context.Context, storeID, key, newValue string) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	if newValue == "" {
		return nil, fmt.Errorf("mockarty: rotate: newValue is required")
	}
	body := map[string]string{"value": newValue}
	var env struct {
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key)+"/rotate"+a.nsQuery(), body, &env); err != nil {
		return nil, err
	}
	return &SecretEntry{Key: key}, nil
}

// DeleteEntry removes a single key from the store.
func (a *SecretsAPI) DeleteEntry(ctx context.Context, storeID, key string) error {
	if storeID == "" || key == "" {
		return fmt.Errorf("mockarty: storeID and key are required")
	}
	return a.client.do(ctx, "DELETE", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key)+a.nsQuery(), nil, nil)
}

// ConfigureVault registers or updates a Vault backend for the namespace.
// Pass an empty VaultIntegration (or call with cfg.URL == "") to clear it.
func (a *SecretsAPI) ConfigureVault(ctx context.Context, namespace string, cfg VaultIntegration) error {
	if namespace == "" {
		namespace = a.client.namespace
	}
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(namespace)+"/integrations/vault", cfg, nil)
}
