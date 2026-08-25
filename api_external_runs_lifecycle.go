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
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     *time.Time        `json:"finished_at,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	ID             string            `json:"id"`
	Namespace      string            `json:"namespace"`
	Name           string            `json:"name"`
	Framework      string            `json:"framework,omitempty"`
	SuiteID        string            `json:"suite_id,omitempty"`
	ExternalID     string            `json:"external_id,omitempty"`
	FullName       string            `json:"full_name,omitempty"`
	Status         string            `json:"status"`
	ResolvedCaseID string            `json:"resolved_case_id,omitempty"`
	ResolvedRunID  string            `json:"resolved_run_id,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Steps          []LifecycleStep   `json:"steps,omitempty"`
	StepCount      int               `json:"step_count"`
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
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"steps": steps}
	return a.lifecycleDo(ctx, "POST", base+"/"+url.PathEscape(runID)+"/steps", body)
}

// FinishRun closes an open run; the returned view carries the resolved TCM
// case/run ids the ingest matched or created.
func (a *ExternalRunsAPI) FinishRun(ctx context.Context, namespace, runID string, req FinishRunRequest) (*LifecycleRun, error) {
	base, err := a.lifecycleBase(namespace)
	if err != nil {
		return nil, err
	}
	return a.lifecycleDo(ctx, "POST", base+"/"+url.PathEscape(runID)+"/finish", req)
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
