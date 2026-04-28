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

// TestRunReportFormat enumerates the aggregated report formats served by
// GET /api/v1/api-tester/test-runs/:id/report. Works for every mode
// (functional / load / fuzz / chaos / contract / merged).
const (
	TestRunReportFormatAllureZip   = "allure_zip"
	TestRunReportFormatAllureJSON  = "allure_json"
	TestRunReportFormatJUnit       = "junit"
	TestRunReportFormatMarkdown    = "markdown"
	TestRunReportFormatUnifiedJSON = "unified_json"
	TestRunReportFormatHTML        = "html"
)

// GetTestRunReport fetches the aggregated report for a test run in the
// requested format. Pass the empty string to default to unified_json. The
// returned bytes are the server's raw response body; callers can Unmarshal
// into their own type for JSON formats or consume as-is for the others.
//
// Fuzz / chaos / contract runs expand into per-item AllureResults (one row
// per finding / fault / case); functional / load / merged runs emit a single
// summary row. All six formats produce deterministic byte output so CI
// checksums stay stable across retries.
//
// GET /api/v1/api-tester/test-runs/:id/report?format=...
func (a *TestRunAPI) GetTestRunReport(ctx context.Context, runID, format string) ([]byte, error) {
	if format == "" {
		format = TestRunReportFormatUnifiedJSON
	}
	q := url.Values{}
	q.Set("format", format)
	path := "/api/v1/api-tester/test-runs/" + url.PathEscape(runID) + "/report?" + q.Encode()
	return a.client.doJSON(ctx, "GET", path, nil)
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

// ListMergedRunsResponse is the paginated list envelope returned by the list
// endpoint. Fields alignment-sorted.
type ListMergedRunsResponse struct {
	Items  []MergedRunView `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// ListMergedRunsOptions scopes the list call. Zero values mean server default
// (limit=50, offset=0). The server hard-caps limit at 500.
type ListMergedRunsOptions struct {
	Limit  int
	Offset int
}

// ListMergedRuns returns the paginated list of merged runs in the client's
// namespace, newest first. Each item carries the parent row plus the current
// snapshot of every attached source.
//
// GET /api/v1/test-runs/merges
func (a *TestRunAPI) ListMergedRuns(ctx context.Context, opts ListMergedRunsOptions) (*ListMergedRunsResponse, error) {
	q := url.Values{}
	if opts.Limit > 0 {
		q.Set("limit", intToString(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", intToString(opts.Offset))
	}
	path := "/api/v1/test-runs/merges"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var resp ListMergedRunsResponse
	if err := a.client.do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteMergedRun removes a merge parent row. The source runs are untouched.
// Edges in test_run_merges are dropped by ON DELETE CASCADE at the DB layer.
//
// DELETE /api/v1/test-runs/merges/:id
func (a *TestRunAPI) DeleteMergedRun(ctx context.Context, mergedRunID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/test-runs/merges/"+url.PathEscape(mergedRunID), nil, nil)
}

// MergedRunReportFormat enumerates the report formats supported by the merged
// run aggregator. Only two formats are served — merged runs span heterogeneous
// sources with no plan/DAG shape, so Allure/JUnit/HTML are not available.
const (
	MergedRunReportFormatUnified  = "unified"
	MergedRunReportFormatMarkdown = "markdown"
	// HTML (print-to-PDF) and JUnit XML are supported by the transient
	// aggregate endpoint — see AggregateRunsReport below. Kept separate
	// from the legacy merged endpoint constants above for clarity.
)

// AggregateReportFormat enumerates formats for the transient aggregate
// endpoint POST /test-runs/reports/aggregate — the replacement for the
// removed persistent merge.
type AggregateReportFormat = string

const (
	AggregateReportFormatUnified  AggregateReportFormat = "unified"
	AggregateReportFormatMarkdown AggregateReportFormat = "markdown"
	AggregateReportFormatHTML     AggregateReportFormat = "html"
	AggregateReportFormatJUnit    AggregateReportFormat = "junit"
)

// AggregateRunsReportRequest is the POST body for the aggregate endpoint.
// Name is optional (server falls back to "Aggregate of N runs").
type AggregateRunsReportRequest struct {
	Name   string   `json:"name,omitempty"`
	RunIDs []string `json:"run_ids"`
}

// AggregateRunsReport builds a release-ready aggregated report over the
// provided test-run IDs and returns the raw response bytes. The endpoint
// is transient — nothing is persisted server-side; each call recomputes.
//
// POST /api/v1/test-runs/reports/aggregate?format=<format>
//
// HTML output is self-contained (inline CSS + inline SVG charts) and
// print-friendly, so saving as PDF via the browser print dialog is the
// supported PDF export path (avoids a server-side headless-Chrome
// dependency in distroless builds).
func (a *TestRunAPI) AggregateRunsReport(ctx context.Context, req AggregateRunsReportRequest, format AggregateReportFormat) ([]byte, error) {
	if format == "" {
		format = AggregateReportFormatUnified
	}
	q := url.Values{}
	q.Set("format", format)
	path := "/api/v1/test-runs/reports/aggregate?" + q.Encode()
	return a.client.doJSON(ctx, "POST", path, req)
}

// GetMergedRunReport fetches the aggregated report for a merged run in the
// requested format. Pass the empty string to default to the unified native JSON
// envelope. The return bytes are the server's raw response body; callers can
// Unmarshal into their own type for unified, or consume as-is for markdown.
//
// GET /api/v1/test-runs/merges/:id/report?format=unified|markdown
func (a *TestRunAPI) GetMergedRunReport(ctx context.Context, mergedRunID, format string) ([]byte, error) {
	if format == "" {
		format = MergedRunReportFormatUnified
	}
	q := url.Values{}
	q.Set("format", format)
	path := "/api/v1/test-runs/merges/" + url.PathEscape(mergedRunID) + "/report?" + q.Encode()
	return a.client.doJSON(ctx, "GET", path, nil)
}
