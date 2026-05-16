// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"sort"
	"sync"
)

// Mutator is the kind-of-mutation the runtime should apply when warping
// seed payloads. The string value is the key the server-side dispatcher
// looks up in its mutator registry.
type Mutator string

// Pre-defined mutator presets. Each maps to one or more payload
// categories the existing fuzz engine knows how to apply.
const (
	// MutatorJSON corrupts JSON structurally (keys, values, type swaps).
	MutatorJSON Mutator = "json"
	// MutatorXML corrupts XML structurally (attributes, CDATA, namespaces).
	MutatorXML Mutator = "xml"
	// MutatorBytes flips bits / inserts / deletes at byte level (raw payloads).
	MutatorBytes Mutator = "bytes"
	// MutatorString applies the "Big List of Naughty Strings" + unicode tricks.
	MutatorString Mutator = "string"
	// MutatorURL mutates the URL path / query parameters.
	MutatorURL Mutator = "url"
	// MutatorHeader mutates header values (CRLF injection, length, case).
	MutatorHeader Mutator = "header"
	// MutatorGRPC produces protobuf-aware mutations (field tags, wire types).
	MutatorGRPC Mutator = "grpc"
	// MutatorGraphQL produces GraphQL-aware mutations (field selection, args).
	MutatorGraphQL Mutator = "graphql"
)

// mutatorRegistry is the lookup the validator uses to confirm a mutator
// is known. Custom mutators registered via WithCustomMutator are added
// to the per-Target list directly and not validated against this map.
//
// Per feedback_dynamic_over_hardcode.md: do NOT switch-case on these
// strings — extend the map.
var mutatorRegistry = struct {
	entries map[Mutator]mutatorMeta
	sync.RWMutex
}{
	entries: map[Mutator]mutatorMeta{
		MutatorJSON:    {Categories: []string{"type_confusion", "boundary_values", "schema_violation", "unicode"}},
		MutatorXML:     {Categories: []string{"xxe", "unicode", "boundary_values"}},
		MutatorBytes:   {Categories: []string{"boundary_values", "unicode", "format_strings"}},
		MutatorString:  {Categories: []string{"naughty_strings", "unicode", "format_strings"}},
		MutatorURL:     {Categories: []string{"path_traversal", "ssrf", "open_redirect"}},
		MutatorHeader:  {Categories: []string{"http_request_smuggling", "auth_bypass", "boundary_values"}},
		MutatorGRPC:    {Categories: []string{"type_confusion", "boundary_values"}},
		MutatorGraphQL: {Categories: []string{"type_confusion", "schema_violation", "boundary_values"}},
	},
}

type mutatorMeta struct {
	JSConfig   string
	Categories []string
}

// Valid reports whether m is a registered preset (built-in or
// previously registered via RegisterMutator).
func (m Mutator) Valid() bool {
	mutatorRegistry.RLock()
	_, ok := mutatorRegistry.entries[m]
	mutatorRegistry.RUnlock()
	return ok
}

// RegisterMutator adds a custom mutator preset to the global registry.
// Useful for organisation-internal mutator names that the server-side
// engine has been extended to recognise.
//
// Concurrent-safe; later registrations overwrite earlier ones for the
// same key (matches Go's stdlib http.HandleFunc semantics).
func RegisterMutator(m Mutator, categories ...string) {
	if m == "" {
		return
	}
	cp := append([]string(nil), categories...)
	mutatorRegistry.Lock()
	mutatorRegistry.entries[m] = mutatorMeta{Categories: cp}
	mutatorRegistry.Unlock()
}

// CategoriesFor returns the payload-category keys the runtime will
// enable when this mutator is selected. Returns nil for unknown.
func (m Mutator) CategoriesFor() []string {
	mutatorRegistry.RLock()
	defer mutatorRegistry.RUnlock()
	meta, ok := mutatorRegistry.entries[m]
	if !ok {
		return nil
	}
	out := append([]string(nil), meta.Categories...)
	return out
}

// ListMutators returns every registered mutator key in deterministic
// (sorted) order. Helpful for catalogue endpoints and debugging.
func ListMutators() []Mutator {
	mutatorRegistry.RLock()
	keys := make([]Mutator, 0, len(mutatorRegistry.entries))
	for k := range mutatorRegistry.entries {
		keys = append(keys, k)
	}
	mutatorRegistry.RUnlock()
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

// WithMutator selects one of the built-in or registered mutator presets.
// Multiple WithMutator calls accumulate; the runtime runs them in order.
func WithMutator(m Mutator) Option {
	return func(t *Target) {
		t.mutators = append(t.mutators, customMutator{Name: m})
	}
}

// WithCustomMutator registers a target-local mutator referenced by name
// plus an opaque JS config blob the server-side engine evaluates.
// The mutator is not validated against the global registry — the server
// is the source of truth for whether the JS payload is acceptable.
func WithCustomMutator(name, jsConfig string) Option {
	return func(t *Target) {
		t.mutators = append(t.mutators, customMutator{Name: Mutator(name), JSConfig: jsConfig})
	}
}

// customMutator is the on-target record for a selected mutator. Built-in
// mutators have an empty JSConfig; custom ones carry the script body.
type customMutator struct {
	Name     Mutator
	JSConfig string
}
