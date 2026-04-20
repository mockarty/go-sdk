// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// EntitySearchAPI exposes the unified entity-picker endpoint
// (GET /api/v1/entity-search) that powers UI pickers and CI/CD automation
// that needs to look up Mockarty resources by case-insensitive name match.
//
// Supported entity types (mirrors the server-side
// internal/webui/entity_search_handlers.go constants):
//
//   - "mock"
//   - "test_plan"
//   - "perf_config"
//   - "fuzz_config"
//   - "chaos_experiment"
//   - "contract_pact"
//
// Use the EntityType* constants below to avoid typos.
type EntitySearchAPI struct {
	client *Client
}

// Supported entity types — kept in sync with the server-side handler so a
// typo at the call site is caught at compile time.
const (
	EntityTypeMock            = "mock"
	EntityTypeTestPlan        = "test_plan"
	EntityTypePerfConfig      = "perf_config"
	EntityTypeFuzzConfig      = "fuzz_config"
	EntityTypeChaosExperiment = "chaos_experiment"
	EntityTypeContractPact    = "contract_pact"
)

// EntitySearchPaging is the default and ceiling for the `limit` query
// parameter — mirrors the server constants. Documented so callers know
// the upper bound up front.
const (
	EntitySearchDefaultLimit = 50
	EntitySearchMaxLimit     = 200
)

// EntitySearchResult is one row in the picker response. NumericID is a
// pointer because most entity types do not carry a numeric identifier;
// nil simply means "not applicable for this row".
//
// Field ordering follows the project descending-alignment rule: 8-byte
// strings + pointer first, then nothing smaller.
type EntitySearchResult struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	CreatedAt string `json:"createdAt"`
	NumericID *int64 `json:"numericId,omitempty"`
}

// EntitySearchResponse is the response envelope. Total reflects the total
// number of matches across the (type, namespace, q) triplet BEFORE
// pagination — useful for rendering "showing 50 of 132" hints in custom
// pickers.
type EntitySearchResponse struct {
	Items []EntitySearchResult `json:"items"`
	Total int                  `json:"total"`
}

// EntitySearchRequest narrows an EntitySearch call. Only Type is required.
//
// Namespace honours the same tenant-scoping rule the server enforces:
// a tenant-scoped token cannot search a different namespace via
// Namespace, even if it sends one — the server silently ignores the
// override and uses the token's bound namespace. Global admins may
// pass any namespace (or leave it empty for cross-namespace search).
//
// Field ordering: 8-byte strings first, then ints.
type EntitySearchRequest struct {
	Type      string
	Namespace string
	Query     string
	Limit     int
	Offset    int
}

// EntitySearch performs the unified entity lookup. Returns an
// EntitySearchResponse with up to req.Limit matching rows (server caps at
// EntitySearchMaxLimit when the caller asks for more).
//
// Example:
//
//	resp, err := client.EntitySearch().Search(ctx, mockarty.EntitySearchRequest{
//	    Type:  mockarty.EntityTypeTestPlan,
//	    Query: "smoke",
//	    Limit: 25,
//	})
func (a *EntitySearchAPI) Search(ctx context.Context, req EntitySearchRequest) (EntitySearchResponse, error) {
	t := strings.TrimSpace(req.Type)
	if t == "" {
		return EntitySearchResponse{}, fmt.Errorf("mockarty: entity_search: type is required")
	}
	q := url.Values{}
	q.Set("type", t)
	if ns := strings.TrimSpace(req.Namespace); ns != "" {
		q.Set("namespace", ns)
	}
	if query := strings.TrimSpace(req.Query); query != "" {
		q.Set("q", query)
	}
	if req.Limit > 0 {
		q.Set("limit", intToString(req.Limit))
	}
	if req.Offset > 0 {
		q.Set("offset", intToString(req.Offset))
	}
	path := "/api/v1/entity-search?" + q.Encode()
	var out EntitySearchResponse
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return EntitySearchResponse{}, err
	}
	if out.Items == nil {
		// Server contract: Items is always a non-nil slice. Defensive
		// normalisation keeps caller code free of nil-guards.
		out.Items = []EntitySearchResult{}
	}
	return out, nil
}
