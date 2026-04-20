// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

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
	Backend     string    `json:"backend,omitempty"` // "software" | "vault"
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
func (a *SecretsAPI) ListStores(ctx context.Context) ([]SecretStore, error) {
	var stores []SecretStore
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets?namespace="+url.QueryEscape(a.client.namespace), nil, &stores); err != nil {
		return nil, err
	}
	return stores, nil
}

// CreateStore creates a new secret store. Backend defaults to "software"
// (local AES-GCM via the KeyStore) when empty.
func (a *SecretsAPI) CreateStore(ctx context.Context, store SecretStore) (*SecretStore, error) {
	if store.Namespace == "" {
		store.Namespace = a.client.namespace
	}
	var out SecretStore
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets", store, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStore fetches a single store by ID.
func (a *SecretsAPI) GetStore(ctx context.Context, id string) (*SecretStore, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var out SecretStore
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateStore updates the name/description/backend of an existing store.
func (a *SecretsAPI) UpdateStore(ctx context.Context, id string, store SecretStore) (*SecretStore, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var out SecretStore
	if err := a.client.do(ctx, "PUT", "/api/v1/stores/secrets/"+url.PathEscape(id), store, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteStore removes a secret store and all of its entries.
func (a *SecretsAPI) DeleteStore(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("mockarty: secret store id is required")
	}
	return a.client.do(ctx, "DELETE", "/api/v1/stores/secrets/"+url.PathEscape(id), nil, nil)
}

// ListEntries returns metadata (keys, versions, timestamps) for every
// entry in the store. Decrypted values are NOT returned.
func (a *SecretsAPI) ListEntries(ctx context.Context, storeID string) ([]SecretEntry, error) {
	if storeID == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var out []SecretEntry
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateEntry writes a new key/value pair to the store.
func (a *SecretsAPI) CreateEntry(ctx context.Context, storeID string, entry SecretEntry) (*SecretEntry, error) {
	if storeID == "" {
		return nil, fmt.Errorf("mockarty: secret store id is required")
	}
	var out SecretEntry
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries", entry, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEntry fetches a single entry including its decrypted value. Requires
// the `secret:read` permission on the caller's API key.
func (a *SecretsAPI) GetEntry(ctx context.Context, storeID, key string) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	var out SecretEntry
	if err := a.client.do(ctx, "GET", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEntry overwrites the value/description of an existing entry.
func (a *SecretsAPI) UpdateEntry(ctx context.Context, storeID, key string, entry SecretEntry) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	var out SecretEntry
	if err := a.client.do(ctx, "PUT", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key), entry, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RotateEntry generates a new value for the entry, bumping its version.
// The old value is not retained — pair with store-level backups if a
// fallback is needed.
func (a *SecretsAPI) RotateEntry(ctx context.Context, storeID, key string) (*SecretEntry, error) {
	if storeID == "" || key == "" {
		return nil, fmt.Errorf("mockarty: storeID and key are required")
	}
	var out SecretEntry
	if err := a.client.do(ctx, "POST", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key)+"/rotate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEntry removes a single key from the store.
func (a *SecretsAPI) DeleteEntry(ctx context.Context, storeID, key string) error {
	if storeID == "" || key == "" {
		return fmt.Errorf("mockarty: storeID and key are required")
	}
	return a.client.do(ctx, "DELETE", "/api/v1/stores/secrets/"+url.PathEscape(storeID)+"/entries/"+url.PathEscape(key), nil, nil)
}

// ConfigureVault registers or updates a Vault backend for the namespace.
// Pass an empty VaultIntegration (or call with cfg.URL == "") to clear it.
func (a *SecretsAPI) ConfigureVault(ctx context.Context, namespace string, cfg VaultIntegration) error {
	if namespace == "" {
		namespace = a.client.namespace
	}
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(namespace)+"/integrations/vault", cfg, nil)
}
