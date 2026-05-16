// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// WritePactFile serialises consumer to disk under outputDir.
//
// Filename follows the pact-broker convention:
//
//	<consumer>-<provider>.json
//
// with non-filesystem-safe characters replaced by underscores.
//
// The function is safe to call multiple times — the file is rewritten
// atomically (write to tmp + rename) so a concurrent reader either
// sees the previous version or the new one, never a partial blob.
func WritePactFile(c *Consumer) (string, error) {
	if c == nil {
		return "", fmt.Errorf("pact: WritePactFile called with nil consumer")
	}
	pf := c.snapshotForWriter()
	rendered, err := RenderPactFile(pf, c.specVersion)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(c.outputDir, 0o755); err != nil {
		return "", fmt.Errorf("pact: mkdir %s: %w", c.outputDir, err)
	}
	name := safeFilename(pf.Consumer.Name) + "-" + safeFilename(pf.Provider.Name) + ".json"
	path := filepath.Join(c.outputDir, name)
	tmp, err := os.CreateTemp(c.outputDir, "."+name+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("pact: create tmp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		// Best-effort cleanup if rename never happened.
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(rendered); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("pact: write tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("pact: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("pact: rename %s -> %s: %w", tmpName, path, err)
	}
	return path, nil
}

// RenderPactFile turns a PactFile into its on-the-wire JSON
// representation, applying spec-version-specific quirks.
//
// V3 differences vs the in-memory PactFile:
//
//   - Single `providerState` (the first state's name).
//   - `matchingRules` keyed by full JSON path (`$.body.x`).
//   - No `type` field on interactions.
//
// V4 differences:
//
//   - `providerStates` plural, retained as-is.
//   - `matchingRules` keyed by category (`body`/`header`/`query`/
//     `path`) with `{combine, matchers}` entries.
//   - `type` discriminator (Synchronous/HTTP).
//   - Optional `plugins` and `generators` metadata blocks.
func RenderPactFile(pf PactFile, sv SpecVersion) ([]byte, error) {
	if !sv.Valid() {
		return nil, fmt.Errorf("pact: invalid spec version %q", sv)
	}

	// Walk each interaction, extract matchers into matchingRules, and
	// strip the Matcher placeholders out of Body so the on-the-wire
	// body is a plain JSON value.
	out := pf
	out.Metadata.PactSpecification.Version = string(sv)
	out.Interactions = make([]Interaction, len(pf.Interactions))
	for i, ix := range pf.Interactions {
		processed := processInteraction(ix, sv)
		out.Interactions[i] = processed
	}

	// Convert to a generic map so we can elide spec-incompatible fields
	// cleanly without juggling struct tags.
	doc := map[string]any{
		"consumer":     out.Consumer,
		"provider":     out.Provider,
		"interactions": serialiseInteractions(out.Interactions, sv),
		"metadata":     serialiseMetadata(out.Metadata, sv),
	}

	return json.MarshalIndent(doc, "", "  ")
}

// processInteraction extracts matchers from request+response bodies
// and rewrites the placeholders, returning a clone with matchers
// promoted to matchingRules and Body holding plain (no Matcher) JSON
// values.
func processInteraction(ix Interaction, sv SpecVersion) Interaction {
	out := ix
	reqRules := map[string]MatchCategory{}
	resRules := map[string]MatchCategory{}

	// Request body.
	out.Request.Body = walkAndExtract(ix.Request.Body, "$.body", reqRules, sv)
	// Request headers: matchers in headers/queries are uncommon but
	// supported through the same mechanism.
	for name, vals := range ix.Request.Headers {
		newVals := make([]string, 0, len(vals))
		for j, v := range vals {
			newVals = append(newVals, stringifyMaybeMatcher(v, fmt.Sprintf("$.header.%s[%d]", name, j), reqRules, sv))
		}
		out.Request.Headers[name] = newVals
	}

	// Response body.
	out.Response.Body = walkAndExtract(ix.Response.Body, "$.body", resRules, sv)
	for name, vals := range ix.Response.Headers {
		newVals := make([]string, 0, len(vals))
		for j, v := range vals {
			newVals = append(newVals, stringifyMaybeMatcher(v, fmt.Sprintf("$.header.%s[%d]", name, j), resRules, sv))
		}
		out.Response.Headers[name] = newVals
	}

	if len(reqRules) > 0 {
		out.Request.MatchingRules = reqRules
	}
	if len(resRules) > 0 {
		out.Response.MatchingRules = resRules
	}
	return out
}

// stringifyMaybeMatcher returns the example value of a Matcher
// embedded in a header value or the string itself. Header values
// only ever serialise as strings — we ignore matcher example types
// that don't stringify cleanly.
func stringifyMaybeMatcher(v string, _ string, _ map[string]MatchCategory, _ SpecVersion) string {
	// Header values are strings already; the path argument is
	// reserved for a future matcher-aware header DSL.
	return v
}

// walkAndExtract is the body walker. For every Matcher placeholder it
// finds, it records the matcher under the appropriate path key and
// returns the matcher's Example in the placeholder's place. Recurses
// into maps and slices.
//
// The returned value is JSON-clean (no Matcher values left).
func walkAndExtract(body any, path string, sink map[string]MatchCategory, sv SpecVersion) any {
	if body == nil {
		return nil
	}
	switch v := body.(type) {
	case Matcher:
		// Record the matcher itself.
		recordMatcher(v, path, sink, sv)
		// Recurse into the matcher's example so nested matchers under
		// Like(map{...}) still get extracted.
		// We also descend into children for compound matchers (EachLike
		// etc.); the path segment for the child is glued onto path.
		for _, ch := range v.Children {
			childPath := path + ch.PathSegment
			recordMatcher(ch.Matcher, childPath, sink, sv)
			// Continue recursing into the child's example value so a
			// nested {Like, EachLike, ...} placeholder inside a child
			// still finds its way into sink.
			walkAndExtract(ch.Matcher.Example, childPath, sink, sv)
		}
		// Strip the placeholder, leaving the concrete example with its
		// own children resolved recursively.
		return walkAndExtract(v.Example, path, sink, sv)
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, child := range v {
			out[k] = walkAndExtract(child, path+"."+k, sink, sv)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = walkAndExtract(child, fmt.Sprintf("%s[%d]", path, i), sink, sv)
		}
		return out
	case []Matcher:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = walkAndExtract(child, fmt.Sprintf("%s[%d]", path, i), sink, sv)
		}
		return out
	}
	return body
}

// recordMatcher appends a MatcherRule for the given JSON path, keyed
// according to the active spec.
func recordMatcher(m Matcher, path string, sink map[string]MatchCategory, sv SpecVersion) {
	// Avoid duplicating the same matcher when the body walker re-visits
	// a node (e.g. EachLike root and its child both target [*]).
	cat, ok := sink[path]
	if !ok {
		cat = MatchCategory{}
	}
	for _, r := range cat.Matchers {
		if r == m.Rule {
			return
		}
	}
	_ = sv // currently the rule shape is identical across spec versions
	cat.Matchers = append(cat.Matchers, m.Rule)
	sink[path] = cat
}

// serialiseInteractions emits the per-spec interaction list.
func serialiseInteractions(in []Interaction, sv SpecVersion) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, ix := range in {
		entry := map[string]any{
			"description": ix.Description,
			"request":     serialiseRequest(ix.Request, sv),
			"response":    serialiseResponse(ix.Response, sv),
		}
		if sv == SpecV4 {
			t := ix.Type
			if t == "" {
				t = HTTPInteractionType
			}
			entry["type"] = t
			if len(ix.ProviderStates) > 0 {
				entry["providerStates"] = ix.ProviderStates
			}
		} else { // V3
			if len(ix.ProviderStates) > 0 {
				// V3 carries the single state under `providerState`;
				// extras are joined for round-trip fidelity.
				names := make([]string, 0, len(ix.ProviderStates))
				for _, ps := range ix.ProviderStates {
					names = append(names, ps.Name)
				}
				entry["providerState"] = strings.Join(names, "; ")
				// V3 also accepts plural for tolerant readers — emit
				// both so a downstream parser preferring one form
				// finds it.
				entry["providerStates"] = ix.ProviderStates
			}
		}
		out = append(out, entry)
	}
	return out
}

// serialiseRequest converts a Request to spec-shaped JSON.
func serialiseRequest(r Request, sv SpecVersion) map[string]any {
	entry := map[string]any{
		"method": r.Method,
		"path":   r.Path,
	}
	if len(r.Headers) > 0 {
		entry["headers"] = flattenHeaders(r.Headers)
	}
	if len(r.Query) > 0 {
		entry["query"] = r.Query
	}
	if r.Body != nil {
		entry["body"] = r.Body
	}
	if len(r.MatchingRules) > 0 {
		entry["matchingRules"] = formatMatchingRules(r.MatchingRules, sv)
	}
	if sv == SpecV4 && len(r.Generators) > 0 {
		entry["generators"] = r.Generators
	}
	return entry
}

// serialiseResponse converts a Response to spec-shaped JSON.
func serialiseResponse(r Response, sv SpecVersion) map[string]any {
	entry := map[string]any{
		"status": r.Status,
	}
	if len(r.Headers) > 0 {
		entry["headers"] = flattenHeaders(r.Headers)
	}
	if r.Body != nil {
		entry["body"] = r.Body
	}
	if len(r.MatchingRules) > 0 {
		entry["matchingRules"] = formatMatchingRules(r.MatchingRules, sv)
	}
	if sv == SpecV4 && len(r.Generators) > 0 {
		entry["generators"] = r.Generators
	}
	return entry
}

// flattenHeaders collapses single-value header slices into a string for
// the legacy pact reader; multi-value headers keep the slice form.
func flattenHeaders(h map[string][]string) map[string]any {
	out := make(map[string]any, len(h))
	for k, v := range h {
		if len(v) == 1 {
			out[k] = v[0]
		} else {
			out[k] = v
		}
	}
	return out
}

// formatMatchingRules emits matchingRules in the form requested by the
// active spec.
//
//   - V3: flat keys (`$.body.id`) -> `{"matchers":[...]}`.
//   - V4: category keys (`body`/`header`/...) -> per-path
//     `{"<path>": {"combine": ..., "matchers": [...]}}`.
func formatMatchingRules(in map[string]MatchCategory, sv SpecVersion) map[string]any {
	out := map[string]any{}
	if sv == SpecV3 {
		for path, cat := range in {
			out[path] = map[string]any{
				"matchers": serialiseMatchers(cat.Matchers, sv),
			}
		}
		return out
	}
	// V4 nests by category.
	categories := map[string]map[string]any{
		"body":   {},
		"header": {},
		"query":  {},
		"path":   {},
	}
	for path, cat := range in {
		category, sub := splitV4Path(path)
		bucket := categories[category]
		entry := map[string]any{
			"matchers": serialiseMatchers(cat.Matchers, sv),
		}
		if cat.Combine != "" {
			entry["combine"] = cat.Combine
		}
		bucket[sub] = entry
	}
	for k, v := range categories {
		if len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

// splitV4Path takes a flat JSON-path key like `$.body.users[0].id` and
// returns (category, "$.users[0].id"). Unknown / unmatched paths fall
// back to the "body" category so legacy callers don't lose data.
//
// V4 spec is consistent: header paths look like `$.header.NAME[0]`,
// body like `$.body.X`, query like `$.query.K`, path like `$.path`.
func splitV4Path(path string) (category, sub string) {
	switch {
	case strings.HasPrefix(path, "$.body"):
		rest := strings.TrimPrefix(path, "$.body")
		if rest == "" {
			rest = "$"
		} else {
			rest = "$" + rest
		}
		return "body", rest
	case strings.HasPrefix(path, "$.header"):
		rest := strings.TrimPrefix(path, "$.header")
		if rest == "" {
			rest = "$"
		} else {
			rest = "$" + rest
		}
		return "header", rest
	case strings.HasPrefix(path, "$.query"):
		rest := strings.TrimPrefix(path, "$.query")
		if rest == "" {
			rest = "$"
		} else {
			rest = "$" + rest
		}
		return "query", rest
	case strings.HasPrefix(path, "$.path"):
		return "path", "$"
	}
	return "body", path
}

// serialiseMatchers renders a slice of MatcherRule for the active
// spec. V3 and V4 use the same JSON shape per entry; the difference
// lives in the surrounding container (see formatMatchingRules).
func serialiseMatchers(rules []MatcherRule, _ SpecVersion) []map[string]any {
	out := make([]map[string]any, 0, len(rules))
	for _, r := range rules {
		entry := map[string]any{}
		if r.Match != "" {
			entry["match"] = r.Match
		}
		if r.Regex != "" {
			entry["regex"] = r.Regex
		}
		if r.Min != nil {
			entry["min"] = *r.Min
		}
		if r.Max != nil {
			entry["max"] = *r.Max
		}
		if r.Value != nil {
			entry["value"] = r.Value
		}
		if r.Format != "" {
			entry["format"] = r.Format
		}
		if r.Variants != "" {
			entry["variants"] = r.Variants
		}
		out = append(out, entry)
	}
	return out
}

// serialiseMetadata produces the metadata block honouring the spec.
func serialiseMetadata(m Metadata, sv SpecVersion) map[string]any {
	out := map[string]any{
		"pactSpecification": map[string]any{
			"version": string(sv),
		},
	}
	if sv == SpecV4 && len(m.Plugins) > 0 {
		out["plugins"] = m.Plugins
	}
	if m.MockarSDK != nil {
		out["mockarty"] = m.MockarSDK
	}
	return out
}

// safeFilenameRe is the set of allowed bytes in a pact filename. The
// goal is portability across Linux/macOS/Windows so we restrict to
// `[A-Za-z0-9_.-]` and replace everything else with `_`.
var safeFilenameRe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// safeFilename normalises a service name into a portable filename.
func safeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unnamed"
	}
	cleaned := safeFilenameRe.ReplaceAllString(s, "_")
	cleaned = strings.Trim(cleaned, "_")
	if cleaned == "" {
		return "unnamed"
	}
	return cleaned
}
