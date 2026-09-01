// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// External-run LIFECYCLE (streaming) API.
//
// Unlike Report/ReportBatch (one-shot upload of a finished run), the lifecycle
// API lets a long test report incrementally: StartRun → AppendSteps (repeatedly
// as the test progresses) → FinishRun. Finish converges on the SAME ingest as
// the one-shot path, so the run is matched/auto-created against a TCM case and
// its resolved case/run ids come back on the finished view.
//
// Server contract (mounted under the namespace):
//
//	POST /api/v1/namespaces/:ns/tcm/external-runs/lifecycle              StartRun
//	GET  /api/v1/namespaces/:ns/tcm/external-runs/lifecycle              ListRuns
//	GET  /api/v1/namespaces/:ns/tcm/external-runs/lifecycle/:runId       GetRun
//	POST /api/v1/namespaces/:ns/tcm/external-runs/lifecycle/:runId/steps AppendSteps
//	POST /api/v1/namespaces/:ns/tcm/external-runs/lifecycle/:runId/finish FinishRun

// LifecycleStep is one step of a streaming external run.
type LifecycleStep struct {
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
	StepKey    string            `json:"step_key"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	Message    string            `json:"message,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
	ParentKey  string            `json:"parent_key,omitempty"`
	DurationMS int64             `json:"duration_ms,omitempty"`
}

// LifecycleRun is the server view of a streaming external run.
type LifecycleRun struct {
	StartedAt      time.Time             `json:"started_at"`
	FinishedAt     *time.Time            `json:"finished_at,omitempty"`
	Environment    map[string]string     `json:"environment,omitempty"`
	ID             string                `json:"id"`
	Namespace      string                `json:"namespace"`
	Name           string                `json:"name"`
	Framework      string                `json:"framework,omitempty"`
	SuiteID        string                `json:"suite_id,omitempty"`
	ExternalID     string                `json:"external_id,omitempty"`
	FullName       string                `json:"full_name,omitempty"`
	Status         string                `json:"status"`
	ResolvedCaseID string                `json:"resolved_case_id,omitempty"`
	ResolvedRunID  string                `json:"resolved_run_id,omitempty"`
	Tags           []string              `json:"tags,omitempty"`
	Steps          []LifecycleStep       `json:"steps,omitempty"`
	Attachments    []LifecycleAttachment `json:"attachments,omitempty"`
	Revision       int64                 `json:"revision"`
	StepCount      int                   `json:"step_count"`
}

// LifecycleAttachment is attachment metadata returned by a streaming run.
type LifecycleAttachment struct {
	UploadedAt time.Time `json:"uploaded_at"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MimeType   string    `json:"mime_type"`
	URL        string    `json:"url"`
	SHA256     string    `json:"sha256"`
	SizeBytes  int64     `json:"size_bytes"`
}

// StartRunRequest opens a streaming external run.
type StartRunRequest struct {
	StartedAt   time.Time         `json:"started_at,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Name        string            `json:"name"`
	FullName    string            `json:"full_name,omitempty"`
	Framework   string            `json:"framework,omitempty"`
	SuiteID     string            `json:"suite_id,omitempty"`
	ExternalID  string            `json:"external_id,omitempty"`
	TestCaseID  string            `json:"test_case_id,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// FinishRunRequest closes a streaming external run.
type FinishRunRequest struct {
	FinishedAt time.Time `json:"finished_at,omitempty"`
	Status     string    `json:"status"`
	Summary    string    `json:"summary,omitempty"`
}

func (a *ExternalRunsAPI) lifecycleBase(namespace string) (string, error) {
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return "", fmt.Errorf("external_runs: namespace required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/tcm/external-runs/lifecycle", nil
}

// StartRun opens a streaming external run and returns its server view (with the
// run id to feed AppendSteps / FinishRun). Pass "" for namespace to use the
// client default.
func (a *ExternalRunsAPI) StartRun(ctx context.Context, namespace string, req StartRunRequest) (*LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	return a.lifecycleDo(ctx, "POST", base, req)
}

// AppendSteps streams one or more steps into an open run.
func (a *ExternalRunsAPI) AppendSteps(ctx context.Context, namespace, runID string, steps []LifecycleStep) (*LifecycleRun, error) {
	return a.appendSteps(ctx, namespace, runID, 0, steps)
}

// AppendStepsAtRevision streams steps only if revision still identifies the
// run projection the caller observed. A stale revision is rejected by the
// server with HTTP 409.
func (a *ExternalRunsAPI) AppendStepsAtRevision(ctx context.Context, namespace, runID string, revision int64, steps []LifecycleStep) (*LifecycleRun, error) {
	if revision < 1 {
		return nil, fmt.Errorf("external_runs: revision must be positive")
	}
	return a.appendSteps(ctx, namespace, runID, revision, steps)
}

func (a *ExternalRunsAPI) appendSteps(ctx context.Context, namespace, runID string, revision int64, steps []LifecycleStep) (*LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	ctx = lifecycleRevisionContext(ctx, revision)
	body := map[string]any{"steps": steps}
	return a.lifecycleDo(ctx, "POST", base+"/"+url.PathEscape(runID)+"/steps", body)
}

// UploadAttachment uploads one bounded multipart attachment using the legacy
// unfenced lane. Prefer UploadAttachmentAtRevision in concurrent CI reporters.
func (a *ExternalRunsAPI) UploadAttachment(ctx context.Context, namespace, runID, name string, data []byte) (*LifecycleRun, error) {
	return a.uploadAttachment(ctx, namespace, runID, 0, name, data)
}

// UploadAttachmentAtRevision uploads an attachment only while revision still
// identifies the current run projection.
func (a *ExternalRunsAPI) UploadAttachmentAtRevision(ctx context.Context, namespace, runID string, revision int64, name string, data []byte) (*LifecycleRun, error) {
	if revision < 1 {
		return nil, fmt.Errorf("external_runs: revision must be positive")
	}
	return a.uploadAttachment(ctx, namespace, runID, revision, name, data)
}

func (a *ExternalRunsAPI) uploadAttachment(ctx context.Context, namespace, runID string, revision int64, name string, data []byte) (*LifecycleRun, error) {
	if name == "" || strings.ContainsAny(name, "\r\n") {
		return nil, fmt.Errorf("external_runs: attachment name must be non-empty and single-line")
	}
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	if err != nil {
		return nil, fmt.Errorf("external_runs: create attachment part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("external_runs: write attachment part: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("external_runs: finish attachment body: %w", err)
	}
	ctx = lifecycleRevisionContext(ctx, revision)
	resp, err := a.client.doRawCT(ctx, "POST", base+"/"+url.PathEscape(runID)+"/attachments", body.Bytes(), w.FormDataContentType())
	if err != nil {
		return nil, err
	}
	defer resp.Close()
	var run LifecycleRun
	if err := json.NewDecoder(resp).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode lifecycle run: %w", err)
	}
	return &run, nil
}

// FinishRun closes an open run; the returned view carries the resolved TCM
// case/run ids the ingest matched or created.
func (a *ExternalRunsAPI) FinishRun(ctx context.Context, namespace, runID string, req FinishRunRequest) (*LifecycleRun, error) {
	return a.finishRun(ctx, namespace, runID, 0, req)
}

// FinishRunAtRevision closes the run only if revision still identifies the
// accumulated steps and attachments the caller intends to finalise.
func (a *ExternalRunsAPI) FinishRunAtRevision(ctx context.Context, namespace, runID string, revision int64, req FinishRunRequest) (*LifecycleRun, error) {
	if revision < 1 {
		return nil, fmt.Errorf("external_runs: revision must be positive")
	}
	return a.finishRun(ctx, namespace, runID, revision, req)
}

func (a *ExternalRunsAPI) finishRun(ctx context.Context, namespace, runID string, revision int64, req FinishRunRequest) (*LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	ctx = lifecycleRevisionContext(ctx, revision)
	return a.lifecycleDo(ctx, "POST", base+"/"+url.PathEscape(runID)+"/finish", req)
}

func lifecycleRevisionContext(ctx context.Context, revision int64) context.Context {
	if revision < 1 {
		return ctx
	}
	return withRequestHeaders(ctx, map[string]string{"If-Match": `"` + strconv.FormatInt(revision, 10) + `"`})
}

// GetRun fetches the current view of a streaming run.
func (a *ExternalRunsAPI) GetRun(ctx context.Context, namespace, runID string) (*LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	return a.lifecycleDo(ctx, "GET", base+"/"+url.PathEscape(runID), nil)
}

// ListRuns lists streaming runs in the namespace.
func (a *ExternalRunsAPI) ListRuns(ctx context.Context, namespace string) ([]LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	data, err := a.client.doJSON(ctx, "GET", base, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Runs []LifecycleRun `json:"runs"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode lifecycle run list: %w", err)
	}
	return out.Runs, nil
}

func (a *ExternalRunsAPI) lifecycleDo(ctx context.Context, method, path string, body any) (*LifecycleRun, error) {
	data, err := a.client.doJSON(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	var run LifecycleRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, fmt.Errorf("decode lifecycle run: %w", err)
	}
	return &run, nil
}
