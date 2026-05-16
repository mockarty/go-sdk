// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

// InteractionBuilder is the fluent DSL for one consumer-provider
// exchange. Methods return the receiver so the builder chains in the
// usual pact style.
//
// The builder threads through three phases:
//
//  1. Metadata: Given (provider state), UponReceiving (description).
//  2. Request:  WithRequest, WithHeader, WithJSONBody, WithQuery.
//  3. Response: WillRespondWith, WithHeader, WithJSONBody.
//
// After WillRespondWith the builder switches to "response-side" mode —
// further WithHeader / WithJSONBody calls target the response object.
// The transition is automatic and lossless because Request and
// Response carry the same shape on the wire.
type InteractionBuilder struct {
	consumer    *Consumer
	ix          *Interaction
	specVersion SpecVersion
	onResponse  bool
}

// Given seeds (or appends, V4) a provider state for this interaction.
//
// V3 honours only the first call; subsequent calls are silently
// merged into the description for round-trip fidelity. V4 supports
// arbitrary many provider states with parameters; use [GivenWithParams]
// to attach parameters.
func (b *InteractionBuilder) Given(state string) *InteractionBuilder {
	if b == nil {
		return b
	}
	b.ix.ProviderStates = append(b.ix.ProviderStates, ProviderState{Name: state})
	return b
}

// GivenWithParams is the V4-flavour of Given that carries a parameter
// bag (e.g. the user id the state should be set up for).
//
// On V3 the parameters are dropped during serialisation because the
// legacy schema has no place for them; the state name still serialises
// in `providerState`.
func (b *InteractionBuilder) GivenWithParams(state string, params map[string]any) *InteractionBuilder {
	if b == nil {
		return b
	}
	cp := make(map[string]any, len(params))
	for k, v := range params {
		cp[k] = v
	}
	b.ix.ProviderStates = append(b.ix.ProviderStates, ProviderState{Name: state, Params: cp})
	return b
}

// UponReceiving sets the human-readable description shown by the
// broker. Required — verifiers refuse to publish a pact with an empty
// description.
func (b *InteractionBuilder) UponReceiving(desc string) *InteractionBuilder {
	if b == nil {
		return b
	}
	b.ix.Description = desc
	return b
}

// WithRequest pins the request's HTTP method and path and switches the
// builder into request-side mode (the default; calling it on the
// response side resets the cursor back to request — useful when a
// caller wants to amend the request after starting the response).
func (b *InteractionBuilder) WithRequest(method, path string) *InteractionBuilder {
	if b == nil {
		return b
	}
	b.ix.Request.Method = method
	b.ix.Request.Path = path
	b.onResponse = false
	return b
}

// WithQuery adds (or replaces) a query parameter on the request side.
//
// The same key may carry multiple values per HTTP spec; pass them all
// at once.
func (b *InteractionBuilder) WithQuery(key string, values ...string) *InteractionBuilder {
	if b == nil {
		return b
	}
	if b.ix.Request.Query == nil {
		b.ix.Request.Query = make(map[string][]string)
	}
	b.ix.Request.Query[key] = append([]string(nil), values...)
	return b
}

// WithHeader sets a header on the current side (request before
// WillRespondWith, response after).
func (b *InteractionBuilder) WithHeader(name, value string) *InteractionBuilder {
	if b == nil {
		return b
	}
	if b.onResponse {
		if b.ix.Response.Headers == nil {
			b.ix.Response.Headers = make(map[string][]string)
		}
		b.ix.Response.Headers[name] = []string{value}
	} else {
		if b.ix.Request.Headers == nil {
			b.ix.Request.Headers = make(map[string][]string)
		}
		b.ix.Request.Headers[name] = []string{value}
	}
	return b
}

// WithJSONBody sets the body on the current side. Matchers embedded in
// the value (via [Like], [Term], etc.) are extracted by the writer
// during serialisation and the placeholders are replaced with their
// example values.
func (b *InteractionBuilder) WithJSONBody(body any) *InteractionBuilder {
	if b == nil {
		return b
	}
	if b.onResponse {
		b.ix.Response.Body = body
	} else {
		b.ix.Request.Body = body
	}
	return b
}

// WillRespondWith pins the response status and switches the builder
// to response-side mode. Required — verifiers reject interactions
// without a response status.
func (b *InteractionBuilder) WillRespondWith(status int) *InteractionBuilder {
	if b == nil {
		return b
	}
	b.ix.Response.Status = status
	b.onResponse = true
	// V4 requires every HTTP interaction to declare its type discriminator.
	// V3 leaves the field empty.
	if b.specVersion == SpecV4 && b.ix.Type == "" {
		b.ix.Type = HTTPInteractionType
	}
	return b
}
