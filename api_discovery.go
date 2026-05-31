// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// DiscoveryAPI mirrors a test framework's "collect-only" pass into
// Mockarty TCM: a CI step enumerates the test cases it owns (pytest
// --collect-only, `go test -list`, JUnit dry-run, etc.) and syncs that
// manifest so the catalogue stays in lock-step with the source tree.
// Cases are matched by FullName (deterministic identity); orphans — cases
// previously synced under the same Source but absent from this manifest —
// are pruned when PruneMissing is set.
//
// Mirrors the Python + Java SDK surfaces 1:1 so a harness that targets one
// SDK can swap to another without renaming fields.
//
// Server contract: POST /api/v1/namespaces/:ns/tcm/discovery.
type DiscoveryAPI struct {
	client *Client
}

// DiscoveryManifestCase is one test case in a discovery manifest. FullName
// is REQUIRED and is the case's deterministic identity within a Source —
// the server upserts on it. Field tags match the server's wire shape
// verbatim; DO NOT rename JSON keys without coordinating across SDKs.
type DiscoveryManifestCase struct {
	// TestCaseID is the OPTIONAL explicit, author-pinned identity (Allure
	// testCaseId / @allure.id). When set the server matches on it FIRST
	// (authoritative — survives a rename of the test method/parameters);
	// otherwise FullName is the join key.
	TestCaseID  string   `json:"testCaseId,omitempty"`
	FullName    string   `json:"fullName"`
	Name        string   `json:"name,omitempty"`
	Suite       string   `json:"suite,omitempty"`
	Description string   `json:"description,omitempty"`
	SourceRef   string   `json:"sourceRef,omitempty"`
	Labels      []string `json:"labels,omitempty"`
}

// DiscoveryManifest is the upload payload. Source is REQUIRED — it is the
// scope key, and pruning is scoped to it (orphans are only computed within
// the same Source). PruneMissing true marks cases absent from this manifest
// as orphaned.
type DiscoveryManifest struct {
	Source       string                  `json:"source"`
	Framework    string                  `json:"framework,omitempty"`
	Cases        []DiscoveryManifestCase `json:"cases,omitempty"`
	PruneMissing bool                    `json:"pruneMissing,omitempty"`
}

// DiscoverySyncResult is the server's response after a successful sync.
// Created / Updated / Orphaned partition the affected cases; Total is the
// number of cases in the manifest.
type DiscoverySyncResult struct {
	Source   string `json:"source"`
	Created  int    `json:"created"`
	Updated  int    `json:"updated"`
	Orphaned int    `json:"orphaned"`
	Total    int    `json:"total"`
}

// Sync uploads a discovery manifest, upserting cases by FullName and
// (when PruneMissing is set) marking absent cases as orphaned. The
// namespace argument overrides the client's default — pass "" to use the
// client default.
func (a *DiscoveryAPI) Sync(ctx context.Context, namespace string, manifest DiscoveryManifest) (*DiscoverySyncResult, error) {
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return nil, fmt.Errorf("discovery.Sync: namespace required")
	}
	path := "/api/v1/namespaces/" + url.PathEscape(ns) + "/tcm/discovery"
	body, err := a.client.doJSON(ctx, "POST", path, manifest)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return &DiscoverySyncResult{}, nil
	}
	var resp DiscoverySyncResult
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}
