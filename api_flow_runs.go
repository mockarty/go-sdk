// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FlowRunsAPI talks to POST /api/v1/api-tester/flow-runs — the
// server-side runner that accepts a canonical Mockarty IR Flow,
// generates JavaScript via the embedded codegen, executes it through
// the script engine, and returns the aggregated RunResult.
//
// Useful when the SDK caller has IR in hand (e.g. produced by the CLI
// `mockarty-cli flow import --from postman <c>.json`) and just wants
// to fire it at the server without dragging the local goja runtime in.
type FlowRunsAPI struct {
	client *Client
}

// FlowRunRequest is the wire shape. Flow is opaque to the SDK (the IR
// type lives in `mockarty/internal/iruir` server-side) so callers can
// pass either a marshalled iruir.Flow or any map[string]any that
// matches the schema.
type FlowRunRequest struct {
	Flow    json.RawMessage `json:"flow"`
	BaseURL string          `json:"base_url,omitempty"`
}

// FlowRunResponse mirrors the server's flowRunResponse. Field names +
// JSON tags must match across SDKs and the admin handler — wire-shape
// is part of the contract.
type FlowRunResponse struct {
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt time.Time      `json:"finishedAt"`
	Variables  map[string]any `json:"variables,omitempty"`
	Status     string         `json:"status"`
	Logs       []string       `json:"logs,omitempty"`
	Errors     []string       `json:"errors,omitempty"`
	DurationMs int64          `json:"durationMs"`
}

// FlowRunOption tweaks a single execution. Currently only BaseURL is
// supported but the variadic-option shape leaves room for future
// knobs (timeouts, label overrides) without breaking callers.
type FlowRunOption func(*FlowRunRequest)

// WithBaseURL injects an HTTP base prefix into the generated JS,
// matching iruir.RunIROptions.BaseURL on the server side.
func WithBaseURL(base string) FlowRunOption {
	return func(r *FlowRunRequest) { r.BaseURL = base }
}

// Execute POSTs the given Flow JSON to the server-side runner and
// returns the aggregated result.
//
// flowJSON is the raw JSON-encoded iruir.Flow shape. Pass nil or empty
// → an explicit error (the server would also 400, but failing fast on
// the client saves a round trip).
func (a *FlowRunsAPI) Execute(ctx context.Context, flowJSON []byte, opts ...FlowRunOption) (*FlowRunResponse, error) {
	if len(flowJSON) == 0 {
		return nil, fmt.Errorf("flow_runs.Execute: flow JSON is empty")
	}
	// Sanity-check shape before sending — we are NOT trying to fully
	// validate the IR (that's the server's job), just refuse obviously
	// non-JSON input so callers get a useful local error.
	var probe map[string]any
	if err := json.Unmarshal(flowJSON, &probe); err != nil {
		return nil, fmt.Errorf("flow_runs.Execute: flow is not a JSON object: %w", err)
	}
	req := FlowRunRequest{Flow: json.RawMessage(flowJSON)}
	for _, o := range opts {
		if o != nil {
			o(&req)
		}
	}
	body, err := a.client.doJSON(ctx, "POST", "/api/v1/api-tester/flow-runs", req)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return &FlowRunResponse{}, nil
	}
	var resp FlowRunResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("flow_runs.Execute: decode response: %w", err)
	}
	return &resp, nil
}
