// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

// SpecVersion selects the Pact specification revision emitted by [Writer].
//
// Values are typed strings so they round-trip through JSON metadata
// (`pactSpecification.version`) unchanged.
type SpecVersion string

const (
	// SpecV3 emits the legacy Pact V3 shape:
	//
	//   - Flat `matchingRules` keyed by JSON path (`$.body.X`,
	//     `$.headers.Y`).
	//   - Single `providerState` per interaction.
	//   - No interaction `type` field.
	//
	// V3 is still the lingua franca of pact-broker today (2026); pick this
	// when the provider side runs an older verifier.
	SpecV3 SpecVersion = "3.0.0"

	// SpecV4 emits the modern Pact V4 shape:
	//
	//   - Nested `matchingRules` keyed by category (`body`, `header`,
	//     `query`, `path`) with `{combine, matchers}` entries.
	//   - Plural `providerStates`, each carrying free-form parameters.
	//   - Interaction `type` discriminator (Synchronous/HTTP in Phase 1).
	//   - Optional `generators` and `plugins` metadata blocks.
	//
	// V4 is required for plugin-backed transports (gRPC, async messaging).
	SpecV4 SpecVersion = "4.0"
)

// Valid reports whether v is a supported spec version.
func (v SpecVersion) Valid() bool {
	switch v {
	case SpecV3, SpecV4:
		return true
	}
	return false
}

// Participant names a consumer or provider in a Pact file.
type Participant struct {
	Name string `json:"name"`
}

// PactFile is the top-level serialisation envelope. The same Go struct
// covers V3 and V4 — the encoder elides version-specific fields when
// inappropriate so the JSON output validates against the chosen spec.
type PactFile struct {
	Metadata     Metadata      `json:"metadata"`
	Consumer     Participant   `json:"consumer"`
	Provider     Participant   `json:"provider"`
	Interactions []Interaction `json:"interactions"`
}

// Metadata carries spec-version info plus optional plugin manifests.
//
// `PactSpecification.Version` MUST agree with the [SpecVersion] used to
// produce the file or a strict verifier rejects the contract.
type Metadata struct {
	MockarSDK         *SDKAnnotation `json:"mockarty,omitempty"`
	PactSpecification PactSpec       `json:"pactSpecification"`
	Plugins           []PluginEntry  `json:"plugins,omitempty"`
}

// PactSpec is the version envelope under `metadata.pactSpecification`.
type PactSpec struct {
	Version string `json:"version"`
}

// PluginEntry is a placeholder for V4 plugin metadata. Plugins are
// recorded for round-trip fidelity but the SDK does not load or invoke
// them in Phase 1 — that is a Phase 2 follow-up.
type PluginEntry struct {
	Configuration map[string]any `json:"configuration,omitempty"`
	Name          string         `json:"name"`
	Version       string         `json:"version,omitempty"`
}

// SDKAnnotation tags the producing SDK so the broker UI can show "written
// by mockarty-go vX". Optional — verifiers MUST ignore unknown metadata
// keys per the Pact spec.
type SDKAnnotation struct {
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

// Interaction is one consumer-provider exchange.
//
// V3 leaves Type empty and serialises ProviderStates into a single
// `providerState`; V4 sets Type=Synchronous/HTTP (or another typed value
// in future phases) and uses the plural `providerStates` array as-is.
//
// The `interactionMarshaller` in writer.go owns the per-spec shape
// translation; this struct is the union of both.
type Interaction struct {
	Request        Request         `json:"request"`
	Response       Response        `json:"response"`
	Description    string          `json:"description"`
	Type           string          `json:"type,omitempty"`
	ProviderStates []ProviderState `json:"providerStates,omitempty"`
}

// ProviderState describes a precondition the provider must satisfy
// before replaying the interaction.
type ProviderState struct {
	Params map[string]any `json:"params,omitempty"`
	Name   string         `json:"name"`
}

// Request is the expected request shape.
//
// Body holds the original Go value (with matcher placeholders); the
// writer extracts matchers into MatchingRules during serialisation, so
// pre-Encode the matchers still live in Body as [Matcher] values.
type Request struct {
	Body          any                      `json:"body,omitempty"`
	Query         map[string][]string      `json:"query,omitempty"`
	Headers       map[string][]string      `json:"headers,omitempty"`
	MatchingRules map[string]MatchCategory `json:"matchingRules,omitempty"`
	Generators    map[string]GeneratorCat  `json:"generators,omitempty"`
	Method        string                   `json:"method"`
	Path          string                   `json:"path"`
}

// Response is the expected response shape; same Body+MatchingRules story
// as Request.
type Response struct {
	Body          any                      `json:"body,omitempty"`
	Headers       map[string][]string      `json:"headers,omitempty"`
	MatchingRules map[string]MatchCategory `json:"matchingRules,omitempty"`
	Generators    map[string]GeneratorCat  `json:"generators,omitempty"`
	Status        int                      `json:"status"`
}

// MatchCategory is one `matchingRules` block (per JSON path in V3, per
// category in V4). Combine is "AND" by default; some V4 matchers need
// "OR" (any-of) semantics.
type MatchCategory struct {
	Combine  string        `json:"combine,omitempty"`
	Matchers []MatcherRule `json:"matchers"`
}

// MatcherRule is the on-the-wire representation of a single matcher.
//
// The full union of fields used across V3 and V4 lives on this struct;
// `omitempty` keeps each entry tight in the JSON output. The DSL types
// in matchers.go translate themselves into MatcherRule via
// [Matcher.rule].
type MatcherRule struct {
	Value    any    `json:"value,omitempty"`
	Min      *int   `json:"min,omitempty"`
	Max      *int   `json:"max,omitempty"`
	Match    string `json:"match,omitempty"`
	Regex    string `json:"regex,omitempty"`
	Format   string `json:"format,omitempty"`
	Variants string `json:"variants,omitempty"`
}

// GeneratorCat is V4's `generators` block — provider-side hints for
// generating dynamic values during verification. Phase 1 emits empty
// generator blocks; the type exists so V4 round-trips don't drop the
// field if the parser surfaces it.
type GeneratorCat struct {
	Generators map[string]Generator `json:"generators,omitempty"`
}

// Generator describes one dynamic-value hint.
type Generator struct {
	Extra  map[string]any `json:"-"`
	Type   string         `json:"type"`
	Format string         `json:"format,omitempty"`
}

// HTTPInteractionType is the V4 discriminator value for HTTP/sync
// interactions — the only kind Phase 1 produces.
const HTTPInteractionType = "Synchronous/HTTP"
