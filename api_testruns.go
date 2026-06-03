// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
	"time"
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
//
// StartedAt / FinishedAt were `int64` ms timestamps in older SDK
// builds, but the server emits RFC3339 strings via time.Time. The
// older SDK shape failed to decode any TestRun list — every call
// returned 'cannot unmarshal string into Go struct field
// TestRun.startedAt of type int64'. Now using time.Time so the
// envelope round-trips cleanly.
type TestRun struct {
	StartedAt    time.Time `json:"startedAt,omitempty"`
	FinishedAt   time.Time `json:"finishedAt,omitempty"`
	Duration     int64     `json:"duration,omitempty"` // ms
	ID           string    `json:"id,omitempty"`
	CollectionID string    `json:"collectionId,omitempty"`
	Mode         string    `json:"mode,omitempty"`
	ReferenceID  string    `json:"referenceId,omitempty"`
	Status       string    `json:"status,omitempty"` // running, completed, failed, cancelled
	Environment  string    `json:"environment,omitempty"`
	TotalTests   int       `json:"totalTests,omitempty"`
	PassedTests  int       `json:"passedTests,omitempty"`
	FailedTests  int       `json:"failedTests,omitempty"`
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

	// Server emits `{runs: [...], total: N, ...}` envelope. Decode into
	// the envelope unconditionally and return the runs slice (which may
	// be nil/empty when no rows match the filter). The earlier bare-list
	// fallback corrupted SDK behaviour: when envelope.Runs was nil it
	// re-issued a request and tried to decode the object response into
	// []TestRun → 'cannot unmarshal object' errors for filter modes
	// that had no rows.
	var envelope activeTestRunsEnvelope
	if err := a.client.do(ctx, "GET", path, nil, &envelope); err != nil {
		return nil, err
	}
	if envelope.Runs == nil {
		return []TestRun{}, nil
	}
	return envelope.Runs, nil
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
