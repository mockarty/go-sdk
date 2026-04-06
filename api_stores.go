// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
)

// StoreAPI provides methods for managing Mockarty stores.
type StoreAPI struct {
	client *Client
}

// ---------------------------------------------------------------------------
// Global Store
// ---------------------------------------------------------------------------

// GlobalGet retrieves the entire global store.
func (a *StoreAPI) GlobalGet(ctx context.Context) (map[string]any, error) {
	var store map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/stores/global", nil, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// GlobalSet sets a key-value pair in the global store.
func (a *StoreAPI) GlobalSet(ctx context.Context, key string, value any) error {
	body := map[string]any{
		"key":       key,
		"value":     value,
		"namespace": a.client.namespace,
	}
	return a.client.do(ctx, "POST", "/api/v1/stores/global", body, nil)
}

// GlobalDelete deletes a key from the global store.
func (a *StoreAPI) GlobalDelete(ctx context.Context, key string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/stores/global/"+url.PathEscape(key), nil, nil)
}

// GlobalDeleteMany deletes multiple keys from the global store.
func (a *StoreAPI) GlobalDeleteMany(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if err := a.GlobalDelete(ctx, key); err != nil {
			return fmt.Errorf("mockarty: delete global store key %q: %w", key, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Chain Store
// ---------------------------------------------------------------------------

// ChainGet retrieves the chain store for a specific chain ID.
func (a *StoreAPI) ChainGet(ctx context.Context, chainID string) (map[string]any, error) {
	var store map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/stores/chain/"+url.PathEscape(chainID), nil, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// ChainSet sets a key-value pair in a chain store.
func (a *StoreAPI) ChainSet(ctx context.Context, chainID, key string, value any) error {
	body := map[string]any{
		"key":       key,
		"value":     value,
		"namespace": a.client.namespace,
	}
	return a.client.do(ctx, "POST", "/api/v1/stores/chain/"+url.PathEscape(chainID), body, nil)
}

// ChainDelete deletes a key from a chain store.
func (a *StoreAPI) ChainDelete(ctx context.Context, chainID string, key string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/stores/chain/"+url.PathEscape(chainID)+"/"+url.PathEscape(key), nil, nil)
}

// ChainDeleteMany deletes multiple keys from a chain store.
func (a *StoreAPI) ChainDeleteMany(ctx context.Context, chainID string, keys ...string) error {
	for _, key := range keys {
		if err := a.ChainDelete(ctx, chainID, key); err != nil {
			return fmt.Errorf("mockarty: delete chain store key %q: %w", key, err)
		}
	}
	return nil
}
