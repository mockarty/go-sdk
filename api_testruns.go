// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// TestRunAPI provides methods for managing test runs.
type TestRunAPI struct {
	client *Client
}

// TestRun represents a test run execution and its results.
//
// Mode and ReferenceID (added by server-side migration 033) identify which
// execution surface the run belongs to. Supported modes:
//
//	"functional"  — API-tester collection run (default; legacy shape)
//	"load"        — performance/load test run
//	"fuzz"        — fuzz campaign (ReferenceID -> fuzz_configs.id)
//	"chaos"       — chaos experiment (ReferenceID -> chaos_experiments.id)
//	"contract"    — contract verification (ReferenceID -> contract_registry.id)
//
// Fields are alignment-sorted (8-byte first) to minimise struct padding.
type TestRun struct {
	Duration     int64  `json:"duration,omitempty"` // ms
	StartedAt    int64  `json:"startedAt,omitempty"`
	FinishedAt   int64  `json:"finishedAt,omitempty"`
	ID           string `json:"id,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	Mode         string `json:"mode,omitempty"`
	ReferenceID  string `json:"referenceId,omitempty"`
	Status       string `json:"status,omitempty"` // running, completed, failed, cancelled
	Environment  string `json:"environment,omitempty"`
	TotalTests   int    `json:"totalTests,omitempty"`
	PassedTests  int    `json:"passedTests,omitempty"`
	FailedTests  int    `json:"failedTests,omitempty"`
}

// ListTestRunsOptions filters a ListTestRuns call. All fields are optional.
// Mode + ReferenceID surface the unified view added by migration 033 — e.g.
// pass Mode="fuzz", ReferenceID="<uuid>" to see every run for one fuzz config.
type ListTestRunsOptions struct {
	Mode        string
	ReferenceID string
	Limit       int
	Offset      int
}

// List returns all test runs.
func (a *TestRunAPI) List(ctx context.Context) ([]TestRun, error) {
	return a.ListWithOptions(ctx, ListTestRunsOptions{})
}

// ListWithOptions returns test runs with server-side filters (mode / reference
// id / pagination). The server returns an envelope { runs: [...], total: N };
// this method returns just the slice. Use List for the default zero-filter
// shape.
func (a *TestRunAPI) ListWithOptions(ctx context.Context, opts ListTestRunsOptions) ([]TestRun, error) {
	q := url.Values{}
	if opts.Mode != "" {
		q.Set("mode", opts.Mode)
	}
	if opts.ReferenceID != "" {
		q.Set("referenceId", opts.ReferenceID)
	}
	if opts.Limit > 0 {
		q.Set("limit", intToString(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", intToString(opts.Offset))
	}
	path := "/api/v1/api-tester/test-runs"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	// Server responses may be either a bare list (legacy) or an envelope
	// `{ runs: [...] }`. Decode into an envelope first; if the `runs` field is
	// missing, fall back to treating the payload as a bare list.
	var envelope activeTestRunsEnvelope
	if err := a.client.do(ctx, "GET", path, nil, &envelope); err == nil && envelope.Runs != nil {
		return envelope.Runs, nil
	}
	var runs []TestRun
	if err := a.client.do(ctx, "GET", path, nil, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

// intToString avoids pulling in strconv just for a single digit-encoder.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// activeTestRunsEnvelope mirrors the `{"runs": [...]}` envelope emitted by
// the /api/v1/test-runs/active endpoint used by the Runs Tray UI.
type activeTestRunsEnvelope struct {
	Runs []TestRun `json:"runs"`
}

// ListActive returns the list of pending/running test runs visible to the caller
// in the current namespace. Useful for CI/CD gating on parallel runs.
func (a *TestRunAPI) ListActive(ctx context.Context) ([]TestRun, error) {
	var envelope activeTestRunsEnvelope
	if err := a.client.do(ctx, "GET", "/api/v1/test-runs/active", nil, &envelope); err != nil {
		return nil, err
	}
	return envelope.Runs, nil
}

// Get retrieves a test run by ID.
func (a *TestRunAPI) Get(ctx context.Context, id string) (*TestRun, error) {
	var run TestRun
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/test-runs/"+url.PathEscape(id), nil, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// Cancel cancels a running test run by ID.
func (a *TestRunAPI) Cancel(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/api-tester/test-runs/"+url.PathEscape(id)+"/cancel", nil, nil)
}

// Delete deletes a test run by ID.
func (a *TestRunAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/api-tester/test-runs/"+url.PathEscape(id), nil, nil)
}

// Export exports a test run result as raw bytes.
func (a *TestRunAPI) Export(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/api-tester/test-runs/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ImportReport imports a test report.
func (a *TestRunAPI) ImportReport(ctx context.Context, data []byte) error {
	body := struct {
		Data string `json:"data"`
	}{Data: string(data)}
	return a.client.do(ctx, "POST", "/api/v1/api-tester/reports/import", body, nil)
}

// MergedTestRun is the parent row of a test_runs row with mode='merged'. It is
// the same shape as a regular TestRun projected onto the cluster-wide
// ActiveTestRunRow, but the JSON keys come from the server's Go field names
// (no json tags on the repository struct), so capitalised keys are correct.
//
// Fields alignment-sorted (16-byte strings first, then smaller).
type MergedTestRun struct {
	StartedAt   string `json:"StartedAt,omitempty"`
	UpdatedAt   string `json:"UpdatedAt,omitempty"`
	CompletedAt string `json:"CompletedAt,omitempty"`
	ID          string `json:"ID,omitempty"`
	Namespace   string `json:"Namespace,omitempty"`
	NodeID      string `json:"NodeID,omitempty"`
	RunType     string `json:"RunType,omitempty"`
	Status      string `json:"Status,omitempty"`
	Name        string `json:"Name,omitempty"`
	Message     string `json:"Message,omitempty"`
	TaskID      string `json:"TaskID,omitempty"`
	MetaJSON    string `json:"MetaJSON,omitempty"`
	Mode        string `json:"Mode,omitempty"`
	ReferenceID string `json:"ReferenceID,omitempty"`
	UserID      string `json:"UserID,omitempty"`
	Progress    int    `json:"Progress,omitempty"`
}

// MergedRunView is the response shape for merge endpoints — parent row plus the
// current snapshot of every attached source.
type MergedRunView struct {
	Run     *MergedTestRun   `json:"run"`
	Sources []*MergedTestRun `json:"sources"`
}

// CreateMergedRun creates a new mode='merged' parent run aggregating the
// supplied source runs. Source IDs must all live in the caller's namespace
// (or the caller must have admin/support). Returns the freshly-created parent
// row plus the initial source snapshots.
//
// POST /api/v1/test-runs/merges
func (a *TestRunAPI) CreateMergedRun(ctx context.Context, name string, sourceRunIDs []string) (*MergedRunView, error) {
	body := struct {
		Name         string   `json:"name"`
		SourceRunIDs []string `json:"sourceRunIds"`
	}{Name: name, SourceRunIDs: sourceRunIDs}
	var view MergedRunView
	if err := a.client.do(ctx, "POST", "/api/v1/test-runs/merges", body, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

// GetMergedRun returns the parent row plus the current snapshots of every
// attached source. Aggregate counters on the parent are kept fresh server-side
// by a terminal-transition hook — the response never recomputes on the fly.
//
// GET /api/v1/test-runs/merges/:id
func (a *TestRunAPI) GetMergedRun(ctx context.Context, mergedRunID string) (*MergedRunView, error) {
	var view MergedRunView
	if err := a.client.do(ctx, "GET", "/api/v1/test-runs/merges/"+url.PathEscape(mergedRunID), nil, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

// AddMergeSource attaches an existing run to an existing merge. Idempotent —
// duplicate (merge, source) pairs are absorbed by a UNIQUE constraint.
//
// POST /api/v1/test-runs/merges/:id/sources
func (a *TestRunAPI) AddMergeSource(ctx context.Context, mergedRunID, sourceRunID string) error {
	body := struct {
		SourceRunID string `json:"sourceRunId"`
	}{SourceRunID: sourceRunID}
	return a.client.do(ctx, "POST", "/api/v1/test-runs/merges/"+url.PathEscape(mergedRunID)+"/sources", body, nil)
}

// RemoveMergeSource detaches a single source from a merge. Idempotent — a
// missing edge is a no-op.
//
// DELETE /api/v1/test-runs/merges/:id/sources/:sourceRunId
func (a *TestRunAPI) RemoveMergeSource(ctx context.Context, mergedRunID, sourceRunID string) error {
	path := "/api/v1/test-runs/merges/" + url.PathEscape(mergedRunID) + "/sources/" + url.PathEscape(sourceRunID)
	return a.client.do(ctx, "DELETE", path, nil, nil)
}
