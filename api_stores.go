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

// nsParam returns "?namespace=<X>" using the client's default NS, or
// "?namespace=sandbox" as a last-resort fallback. Centralised here so
// every store endpoint threads the NS uniformly — the global/chain/mock
// store handlers all read the namespace from the query string and
// silently fall back to 'sandbox' when it's missing.
func (a *StoreAPI) nsParam() string {
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	return "?namespace=" + url.QueryEscape(ns)
}

// GlobalGet retrieves the entire global store for the client's namespace.
//
// Namespace plumbing: the server reads ?namespace= from the query
// string and defaults to 'sandbox' when missing. Without this thread
// callers in any non-default namespace would silently see the wrong
// store contents.
func (a *StoreAPI) GlobalGet(ctx context.Context) (map[string]any, error) {
	var store map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/stores/global"+a.nsParam(), nil, &store); err != nil {
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
	return a.client.do(ctx, "DELETE", "/api/v1/stores/global/"+url.PathEscape(key)+a.nsParam(), nil, nil)
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
//
// Namespace plumbing mirrors GlobalGet — the server reads ?namespace=
// from the query and silently falls back to 'sandbox' without it,
// so callers in any non-default workspace see the wrong store.
func (a *StoreAPI) ChainGet(ctx context.Context, chainID string) (map[string]any, error) {
	var store map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/stores/chain/"+url.PathEscape(chainID)+a.nsParam(), nil, &store); err != nil {
		return nil, err
	}
	return store, nil
}

// ChainSet sets a key-value pair in a chain store.
//
// Namespace is read from ?namespace= server-side (body field ignored).
// SDK now threads it via the URL so callers in any workspace land in
// the right store.
func (a *StoreAPI) ChainSet(ctx context.Context, chainID, key string, value any) error {
	body := map[string]any{
		"key":   key,
		"value": value,
	}
	return a.client.do(ctx, "POST", "/api/v1/stores/chain/"+url.PathEscape(chainID)+a.nsParam(), body, nil)
}

// ChainDelete deletes a key from a chain store.
func (a *StoreAPI) ChainDelete(ctx context.Context, chainID string, key string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/stores/chain/"+url.PathEscape(chainID)+"/"+url.PathEscape(key)+a.nsParam(), nil, nil)
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
