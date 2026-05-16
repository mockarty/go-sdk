// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package plugins

import (
	"fmt"
	"strings"
	"sync"
)

// Registry is the concurrent-safe lookup table mapping plugin names
// (and content types) to registered Plugin implementations.
//
// A package-level [Default] registry is used by the rest of the SDK;
// tests can build their own Registry to scope plugin registrations
// without leaking into the global state.
//
// Concurrency: every method holds an internal RWMutex; Register is
// O(N) in the number of supported content types; lookups are O(1) for
// exact-MIME and O(#registered patterns) for wildcard fallback.
type Registry struct {
	byName        map[string]Plugin
	byContentType map[string]Plugin // exact MIME match
	wildcardType  map[string]Plugin // "type/*" prefix
	mu            sync.RWMutex
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byName:        make(map[string]Plugin),
		byContentType: make(map[string]Plugin),
		wildcardType:  make(map[string]Plugin),
	}
}

// Register adds p to the registry. Re-registering a plugin name
// overwrites the previous entry (useful for tests that swap stubs);
// callers that need strict-mode should call [Registry.Has] first.
//
// Returns an error if the plugin returns an empty Name.
func (r *Registry) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("pact/plugins: nil plugin")
	}
	name := p.Name()
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("pact/plugins: plugin has empty Name()")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byName[name] = p
	for _, ct := range p.SupportedContentTypes() {
		ct = strings.ToLower(strings.TrimSpace(ct))
		if ct == "" {
			continue
		}
		if strings.HasSuffix(ct, "/*") {
			prefix := strings.TrimSuffix(ct, "/*")
			r.wildcardType[prefix] = p
			continue
		}
		r.byContentType[ct] = p
	}
	return nil
}

// Unregister removes a plugin by name. Idempotent — calling it on a
// missing plugin is a no-op (and returns nil).
//
// Content-type indexes are pruned accordingly so a removed plugin
// stops attracting wire-side traffic.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.byName[name]
	if !ok {
		return
	}
	delete(r.byName, name)
	for _, ct := range p.SupportedContentTypes() {
		ct = strings.ToLower(strings.TrimSpace(ct))
		if ct == "" {
			continue
		}
		if strings.HasSuffix(ct, "/*") {
			prefix := strings.TrimSuffix(ct, "/*")
			if r.wildcardType[prefix] == p {
				delete(r.wildcardType, prefix)
			}
			continue
		}
		if r.byContentType[ct] == p {
			delete(r.byContentType, ct)
		}
	}
}

// Get returns the plugin registered under name. The boolean is false
// when no plugin has been registered.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byName[name]
	return p, ok
}

// Has reports whether a plugin is registered under name.
func (r *Registry) Has(name string) bool {
	_, ok := r.Get(name)
	return ok
}

// ResolveByContentType returns the plugin whose content-type list
// covers the supplied MIME. Resolution order:
//
//  1. Exact match on the full MIME (after stripping `; charset=...`).
//  2. Wildcard `type/*` (e.g. `application/*`).
//  3. Catch-all `*/*` if registered.
//
// The boolean is false when no plugin claims the content type.
func (r *Registry) ResolveByContentType(mime string) (Plugin, bool) {
	mime = stripParams(mime)
	if mime == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.byContentType[mime]; ok {
		return p, true
	}
	// type/* wildcard
	if idx := strings.Index(mime, "/"); idx > 0 {
		major := mime[:idx]
		if p, ok := r.wildcardType[major]; ok {
			return p, true
		}
	}
	if p, ok := r.wildcardType["*"]; ok {
		return p, true
	}
	return nil, false
}

// Names returns the registered plugin names in deterministic order
// (alphabetical) so tests asserting on the catalogue stay stable.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.byName))
	for n := range r.byName {
		out = append(out, n)
	}
	// Tiny manual sort — keeps the file dep-free.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// stripParams returns the MIME type without any `; charset=...` suffix
// and with surrounding whitespace removed.
func stripParams(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if idx := strings.Index(mime, ";"); idx >= 0 {
		mime = strings.TrimSpace(mime[:idx])
	}
	return mime
}

// Default is the package-level Registry consulted by the in-process
// mock server. Tests can override the global by calling Reset() in a
// t.Cleanup() block.
var Default = NewRegistry()

// Register is a shortcut for Default.Register.
func Register(p Plugin) error { return Default.Register(p) }

// Get is a shortcut for Default.Get.
func Get(name string) (Plugin, bool) { return Default.Get(name) }

// ResolveByContentType is a shortcut for Default.ResolveByContentType.
func ResolveByContentType(mime string) (Plugin, bool) {
	return Default.ResolveByContentType(mime)
}

// Reset clears every registration on the package-level [Default]
// registry. Intended for test isolation only.
func Reset() {
	Default.mu.Lock()
	Default.byName = make(map[string]Plugin)
	Default.byContentType = make(map[string]Plugin)
	Default.wildcardType = make(map[string]Plugin)
	Default.mu.Unlock()
}
