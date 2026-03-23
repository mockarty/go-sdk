// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
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
	if err := a.client.do(ctx, "GET", "/mock/store/global", nil, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// GlobalSet sets a key-value pair in the global store.
func (a *StoreAPI) GlobalSet(ctx context.Context, key string, value any) error {
	body := map[string]any{key: value}
	return a.client.do(ctx, "POST", "/mock/store/global", body, nil)
}

// GlobalDelete deletes one or more keys from the global store.
func (a *StoreAPI) GlobalDelete(ctx context.Context, keys ...string) error {
	body := deleteFromStoreRequest{Keys: keys}
	return a.client.do(ctx, "DELETE", "/mock/store/global", body, nil)
}

// ---------------------------------------------------------------------------
// Chain Store
// ---------------------------------------------------------------------------

// ChainGet retrieves the chain store for a specific chain ID.
func (a *StoreAPI) ChainGet(ctx context.Context, chainID string) (map[string]any, error) {
	var store map[string]any
	if err := a.client.do(ctx, "GET", "/mock/store/chain/"+url.PathEscape(chainID), nil, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// ChainSet sets a key-value pair in a chain store.
func (a *StoreAPI) ChainSet(ctx context.Context, chainID, key string, value any) error {
	body := map[string]any{key: value}
	return a.client.do(ctx, "POST", "/mock/store/chain/"+url.PathEscape(chainID), body, nil)
}

// ChainDelete deletes one or more keys from a chain store.
func (a *StoreAPI) ChainDelete(ctx context.Context, chainID string, keys ...string) error {
	body := deleteFromStoreRequest{Keys: keys}
	return a.client.do(ctx, "DELETE", "/mock/store/chain/"+url.PathEscape(chainID), body, nil)
}

// deleteFromStoreRequest mirrors the server's DeleteFromStoreRequest.
type deleteFromStoreRequest struct {
	Keys []string `json:"keys"`
}
