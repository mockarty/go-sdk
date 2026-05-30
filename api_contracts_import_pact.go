// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
)

// ImportedPact is the contract returned by the pact import bridge. Its
// shape mirrors what the server's pactPublish handler returns — the
// participants are nested objects ({"name": ...}), unlike the flat
// strings on the legacy Pact type used by the broker-style endpoints.
type ImportedPact struct {
	ID          string          `json:"id,omitempty"`
	Consumer    PactParticipant `json:"consumer"`
	Provider    PactParticipant `json:"provider"`
	Namespace   string          `json:"namespace,omitempty"`
	Version     string          `json:"version,omitempty"`
	PublishedBy string          `json:"publishedBy,omitempty"`
	PublishedAt string          `json:"publishedAt,omitempty"`
}

// PactParticipant is a named service (consumer or provider) in a pact.
type PactParticipant struct {
	Name string `json:"name"`
}

// ImportPactOptions tunes how a pact file is published to Mockarty as a
// consumer-driven contract.
//
// Version is the consumer application version recorded against the pact
// (pact-broker semantics). When empty, the version is taken from the
// pact body's metadata.pactSpecification.version; if that is also empty
// the server rejects the publish, so callers SHOULD set it explicitly
// from CI (e.g. the git SHA).
//
// Namespace overrides the client's default namespace for this one call.
type ImportPactOptions struct {
	Version   string
	Namespace string
}

// ImportPactFile reads a Pact contract file from disk and publishes it to
// Mockarty as a consumer-driven contract (the pact → Mockarty-contract
// bridge).
//
// The file is the standard Pact JSON document written by any pact
// framework (mockarty-go pact.WritePactFile, pact-python, pact-js,
// pact-jvm, …). Mockarty's server parses Pact v2/v3/v4 — HTTP and
// message interactions alike — and stores the result as a first-class
// contract that surfaces in the contract dashboard, can-i-deploy matrix
// and provider verification.
//
// This is the one-call replacement for the previous manual flow of
// "write pact file → open the UI → paste the JSON".
func (a *ContractAPI) ImportPactFile(ctx context.Context, path string, opts *ImportPactOptions) (*ImportedPact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("import pact: read %s: %w", path, err)
	}
	return a.ImportPact(ctx, data, opts)
}

// ImportPact publishes raw Pact JSON bytes to Mockarty as a
// consumer-driven contract. Prefer ImportPactFile when the pact lives on
// disk; this variant suits in-memory pacts (e.g. bytes produced by a
// pact writer or fetched from a broker).
//
// The bytes are validated to be a well-formed pact (parseable JSON with
// non-empty consumer and provider names) before the round-trip so a
// malformed file fails fast with a clear error instead of a server-side
// 400.
func (a *ContractAPI) ImportPact(ctx context.Context, pactJSON []byte, opts *ImportPactOptions) (*ImportedPact, error) {
	version, namespace := "", ""
	if opts != nil {
		version = opts.Version
		namespace = opts.Namespace
	}

	parsed, err := parsePactMeta(pactJSON)
	if err != nil {
		return nil, err
	}
	if version == "" {
		version = parsed.specVersion
	}
	if namespace == "" {
		namespace = a.client.namespace
	}

	body := map[string]any{"pactContent": string(pactJSON)}
	if version != "" {
		body["version"] = version
	}

	path := "/api/v1/contract/pacts"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}

	var result ImportedPact
	if err := a.client.do(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// pactMeta is the minimal slice of a pact document the bridge needs to
// validate the file and derive a default version.
type pactMeta struct {
	consumer    string
	provider    string
	specVersion string
}

// parsePactMeta does a light, dependency-free validation of pact bytes:
// it must be a JSON object naming a consumer and a provider. The full
// interaction-shape validation is left to the server (which supports
// v2/v3/v4 HTTP + message pacts) — duplicating that here would drift.
func parsePactMeta(pactJSON []byte) (pactMeta, error) {
	var doc struct {
		Consumer struct {
			Name string `json:"name"`
		} `json:"consumer"`
		Provider struct {
			Name string `json:"name"`
		} `json:"provider"`
		Metadata struct {
			PactSpecification struct {
				Version string `json:"version"`
			} `json:"pactSpecification"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(pactJSON, &doc); err != nil {
		return pactMeta{}, fmt.Errorf("import pact: not valid JSON: %w", err)
	}
	if doc.Consumer.Name == "" {
		return pactMeta{}, fmt.Errorf("import pact: pact consumer name is required")
	}
	if doc.Provider.Name == "" {
		return pactMeta{}, fmt.Errorf("import pact: pact provider name is required")
	}
	return pactMeta{
		consumer:    doc.Consumer.Name,
		provider:    doc.Provider.Name,
		specVersion: doc.Metadata.PactSpecification.Version,
	}, nil
}
