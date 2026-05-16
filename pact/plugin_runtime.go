// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

import (
	"context"

	"github.com/mockarty/mockarty-go/pact/plugins"
)

// pluginRuntimeAdapter wraps a plugins.Plugin so it satisfies the
// pact-local pluginRuntime interface — the adapter shape converts the
// strongly-typed *plugins.MatchError into a plain error so the mock
// server can treat plugin and built-in matcher failures uniformly.
//
// We intentionally keep the adapter thin: it carries no state and is
// allocated once per Consumer.WithPlugin call. The underlying plugin
// is goroutine-safe per the SPI contract.
type pluginRuntimeAdapter struct {
	inner plugins.Plugin
}

// Name implements pluginRuntime.
func (a *pluginRuntimeAdapter) Name() string { return a.inner.Name() }

// Version implements pluginRuntime.
func (a *pluginRuntimeAdapter) Version() string { return a.inner.Version() }

// SupportedContentTypes implements pluginRuntime.
func (a *pluginRuntimeAdapter) SupportedContentTypes() []string {
	return a.inner.SupportedContentTypes()
}

// MatchRequest implements pluginRuntime; converts *MatchError -> error.
func (a *pluginRuntimeAdapter) MatchRequest(ctx context.Context, expected map[string]any, actual []byte, contentType string) error {
	me := a.inner.MatchRequest(ctx, expected, actual, contentType)
	if me == nil {
		return nil
	}
	return me
}

// GenerateResponse implements pluginRuntime.
func (a *pluginRuntimeAdapter) GenerateResponse(ctx context.Context, expected map[string]any, contentType string) ([]byte, error) {
	return a.inner.GenerateResponse(ctx, expected, contentType)
}

func init() {
	// Bridge the package-local pluginLookup hook to the plugins.Default
	// registry. This wire-up runs once at process start; tests can
	// shadow it by replacing pluginLookup with a fake.
	pluginLookup = func(name string) (pluginRuntime, bool) {
		p, ok := plugins.Get(name)
		if !ok {
			return nil, false
		}
		return &pluginRuntimeAdapter{inner: p}, true
	}
}
