// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// ExternalRunsAPI bridges arbitrary test results (custom in-house
// runners, the fluent Tester DSL, third-party frameworks via the
// Allure adapter) into Mockarty TCM. Mirrors the Python + Java SDK
// surfaces 1:1 so a test harness that targets one SDK can swap to
// another without renaming fields.
//
// Server contract: POST /api/v1/namespaces/:ns/tcm/external-runs and
// POST /api/v1/namespaces/:ns/tcm/external-runs/batch.
type ExternalRunsAPI struct {
	client *Client
}

// ExternalAttachment matches internal/testcase/external_run.go on the
// server side.
type ExternalAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType,omitempty"`
	BodyB64     string `json:"bodyB64,omitempty"`
	BlobURI     string `json:"blobUri,omitempty"`
}

// ExternalStep is one step inside an external run. Metadata is the
// protocol-specific bag (kafka offset/partition, gRPC method, etc).
type ExternalStep struct {
	StartedAt  *time.Time     `json:"startedAt,omitempty"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Name       string         `json:"name"`
	Status     string         `json:"status"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"durationMs,omitempty"`
}

// CustomField is a key/value tag the user attaches to the run. The
// `Type` discriminator is informational ("string" | "number" | "url").
type CustomField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type,omitempty"`
}

// ExternalRunRequest is the per-run upload payload. Field tag matches
// the server's internal/testcase/external_run.go shape verbatim — DO
// NOT rename JSON keys without bumping SchemaVersion.
type ExternalRunRequest struct {
	StartedAt  *time.Time        `json:"startedAt,omitempty"`
	FinishedAt *time.Time        `json:"finishedAt,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	// Parameters carries a data-driven/parametrised test's inputs (Allure
	// `parameters`) as name→value. Mapped onto custom fields server-side just
	// like Labels (labels win on key collision), so a parametrised pytest /
	// JUnit case can promote its params without a separate call. Optional.
	Parameters         map[string]string `json:"parameters,omitempty"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
	Stdout             string            `json:"stdout,omitempty"`
	ExternalID         string            `json:"externalId,omitempty"`
	CaseID             string            `json:"caseId,omitempty"`
	CaseName           string            `json:"caseName,omitempty"`
	PlanID             string            `json:"planId,omitempty"`
	PlanRunID          string            `json:"planRunId,omitempty"`
	Framework          string            `json:"framework,omitempty"`
	FrameworkVersion   string            `json:"frameworkVersion,omitempty"`
	Stderr             string            `json:"stderr,omitempty"`
	TestDisplayName    string            `json:"testDisplayName,omitempty"`
	Status             string            `json:"status"`
	Error              string            `json:"error,omitempty"`
	CaseDescription    string            `json:"caseDescription,omitempty"`
	CaseExpectedResult string            `json:"caseExpectedResult,omitempty"`
	FullName           string            `json:"fullName,omitempty"`
	// TestCaseID is the author-pinned identity (Allure testCaseId / @allure.id).
	// AUTHORITATIVE resolution key — tried before fullName/name — so a method or
	// parameter rename (which changes fullName) still lands on the same case.
	TestCaseID         string               `json:"testCaseId,omitempty"`
	Steps              []ExternalStep       `json:"steps,omitempty"`
	Attachments        []ExternalAttachment `json:"attachments,omitempty"`
	CustomFields       []CustomField        `json:"customFields,omitempty"`
	DurationMs         int64                `json:"durationMs,omitempty"`
	SchemaVersion      int                  `json:"schemaVersion"`
	AutoCreate         bool                 `json:"autoCreate,omitempty"`
	ClaimCaseOwnership bool                 `json:"claimCaseOwnership,omitempty"`
}

// ExternalRunResponse — minimal envelope the server returns after a
// successful submit. Fields are documented in the server's
// internal/webui/tcm_external_runs_handlers.go.
type ExternalRunResponse struct {
	RunID  string `json:"runId,omitempty"`
	CaseID string `json:"caseId,omitempty"`
	Status string `json:"status,omitempty"`
}

// Submit uploads a single run. The namespace argument overrides the
// client's default — pass "" to use the client default.
func (a *ExternalRunsAPI) Report(ctx context.Context, namespace string, run ExternalRunRequest) (*ExternalRunResponse, error) {
	if run.SchemaVersion == 0 {
		run.SchemaVersion = ExternalRunSchemaVersion
	}
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return nil, fmt.Errorf("external_runs.Submit: namespace required")
	}
	path := "/api/v1/namespaces/" + url.PathEscape(ns) + "/tcm/external-runs"
	body, err := a.client.doJSON(ctx, "POST", path, run)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return &ExternalRunResponse{}, nil
	}
	var resp ExternalRunResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

// ExternalRunsBatchResponse is what the /batch endpoint returns: per-row
// success / per-row error. Rows arrive in the same order as the request.
//
// Wire shape: server emits each row as `{result: <Run>, error?: "...",
// index: N}`. Older SDK builds expected the run fields at the top of
// each row and silently zeroed every RunID/CaseID/Status. Custom
// UnmarshalJSON below flattens the server's nested `result` block onto
// the SDK row so callers see the run identifiers directly while still
// being able to read `Error` for per-row failures.
type ExternalRunsBatchRow struct {
	RunID  string `json:"runId,omitempty"`
	CaseID string `json:"caseId,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Index  int    `json:"index,omitempty"`
}

// UnmarshalJSON flattens the server's `{result:{runId,caseId,status},
// error, index}` row into a flat ExternalRunsBatchRow. Both the
// nested `result` shape and the historical flat shape are accepted.
func (r *ExternalRunsBatchRow) UnmarshalJSON(data []byte) error {
	var wire struct {
		Result *struct {
			RunID  string `json:"runId"`
			CaseID string `json:"caseId"`
			Status string `json:"status"`
		} `json:"result"`
		Error  string `json:"error"`
		Index  int    `json:"index"`
		RunID  string `json:"runId"`
		CaseID string `json:"caseId"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.Error = wire.Error
	r.Index = wire.Index
	if wire.Result != nil {
		r.RunID = wire.Result.RunID
		r.CaseID = wire.Result.CaseID
		r.Status = wire.Result.Status
		return nil
	}
	r.RunID = wire.RunID
	r.CaseID = wire.CaseID
	r.Status = wire.Status
	return nil
}

type ExternalRunsBatchResponse struct {
	Results []ExternalRunsBatchRow `json:"results"`
}

// SubmitBatch uploads up to 100 runs in one POST (the server caps per
// batch). When more rows are supplied, the slice is chunked and the
// returned response merges Results in original order.
func (a *ExternalRunsAPI) ReportBatch(ctx context.Context, namespace string, runs []ExternalRunRequest) (*ExternalRunsBatchResponse, error) {
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return nil, fmt.Errorf("external_runs.SubmitBatch: namespace required")
	}
	path := "/api/v1/namespaces/" + url.PathEscape(ns) + "/tcm/external-runs/batch"

	out := &ExternalRunsBatchResponse{}
	for i := 0; i < len(runs); i += externalRunsBatchCap {
		end := i + externalRunsBatchCap
		if end > len(runs) {
			end = len(runs)
		}
		chunk := runs[i:end]
		// Default schemaVersion per row.
		for j := range chunk {
			if chunk[j].SchemaVersion == 0 {
				chunk[j].SchemaVersion = ExternalRunSchemaVersion
			}
		}
		body, err := a.client.doJSON(ctx, "POST", path, map[string]any{"runs": chunk})
		if err != nil {
			return out, err
		}
		var resp ExternalRunsBatchResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return out, fmt.Errorf("decode batch response: %w", err)
		}
		out.Results = append(out.Results, resp.Results...)
	}
	return out, nil
}

// ExternalRunSchemaVersion is the wire-protocol version. Bump when the
// server-side payload contract changes incompatibly.
const ExternalRunSchemaVersion = 1

// externalRunsBatchCap mirrors the server-side per-batch limit (see
// internal/webui/tcm_external_runs_handlers.go batchSize).
const externalRunsBatchCap = 100

// ── Status constants ─────────────────────────────────────────────────
// Mirror the Python SDK so cross-language ports use the same strings.

const (
	ExternalStatusPassed    = "passed"
	ExternalStatusFailed    = "failed"
	ExternalStatusBroken    = "broken"
	ExternalStatusSkipped   = "skipped"
	ExternalStatusCancelled = "cancelled"
)
