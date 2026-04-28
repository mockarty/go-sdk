// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// api_testplans_manual.go — T10 cascade closure for the manual-flow surface
// added in T1..T9:
//
//   - RunManual / Pause / Resume / Cancel / Rerun on plan runs
//   - ResolveStep on a TCM case-run step
//   - Me().AwaitingManual() for the topbar bell counter
//
// These are the CI/CD-useful slice — admin-only flows (webhook subscription
// management, schedule CRUD when not driving from CI, attachment uploads
// from a UI) are intentionally NOT exposed here per the SDK/CLI scope rule
// in CLAUDE.md.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// RunManualOptions — POST /test-plans/:id/run with the T6/T7/T8 fields.
// ---------------------------------------------------------------------------

// RunManualOptions tunes a manual-flow plan run. All fields are optional.
//
//   - RecordDetailed (T6): persists per-item HAR-shaped traces to
//     test_plan_run_traces. Useful for diagnostic / golden-value runs;
//     leave false for routine smoke runs.
//   - ExecutionModeOverride (T8): "manual" forces every test_case item
//     to gate for human verdict regardless of the persisted mode; "auto"
//     forces unattended execution; "" keeps the per-item default.
//   - NotifyOnCompletion / NotifyEmails (T7): fire a completion email at
//     the end of the run. Empty NotifyEmails falls back to the plan's
//     subscriber list configured in the notifications panel.
//   - Items: subset of 1-based item orders to execute. Empty = all items.
//   - Mode: legacy execution-mode override (sequential / parallel / dag /
//     timed) applied to the whole run. ExecutionModeOverride and Mode
//     compose: Mode picks the orchestration shape, ExecutionModeOverride
//     picks the per-step gating policy for test_case items.
type RunManualOptions struct {
	ExecutionModeOverride string   `json:"executionModeOverride,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
	NotifyEmails          []string `json:"notifyEmails,omitempty"`
	Items                 []int    `json:"items,omitempty"`
	RecordDetailed        bool     `json:"recordDetailed,omitempty"`
	NotifyOnCompletion    bool     `json:"notifyOnCompletion,omitempty"`
}

// RunManual triggers a plan run with the T6/T7/T8 manual-flow knobs.
//
// Example (CI gate that requires human confirmation per step):
//
//	run, err := c.TestPlans().RunManual(ctx, "#42", mockarty.RunManualOptions{
//	    ExecutionModeOverride: "manual",
//	    NotifyOnCompletion:    true,
//	    NotifyEmails:          []string{"qa-lead@example.com"},
//	    RecordDetailed:        true,
//	})
func (a *TestPlansAPI) RunManual(ctx context.Context, idOrNumeric string, opts RunManualOptions) (*TestPlanRun, error) {
	key := strings.TrimPrefix(strings.TrimSpace(idOrNumeric), "#")
	if key == "" {
		return nil, fmt.Errorf("mockarty: empty plan id")
	}
	var env struct {
		RunID  string `json:"runId"`
		PlanID string `json:"planId"`
		Status string `json:"status"`
	}
	if err := a.client.do(ctx, http.MethodPost, testPlansBase+"/"+url.PathEscape(key)+"/run", opts, &env); err != nil {
		return nil, err
	}
	return &TestPlanRun{
		ID:     env.RunID,
		PlanID: env.PlanID,
		Status: env.Status,
	}, nil
}

// ---------------------------------------------------------------------------
// Plan run lifecycle: only CancelRun exists at the plan-run level (already
// defined on the legacy surface above). Pause / resume / rerun semantics
// live on TCM case-runs — see PauseCaseRun / ResumeCaseRun / CancelCaseRun /
// RerunCaseRun below.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// ResolveStep — POST /tcm/case-runs/:runId/steps/:stepUid/resolve
// ---------------------------------------------------------------------------

// StepResolution is the verdict pushed by the human / agent / scheduled job
// resolving a manual_pending TCM case-run step. Mirrors the server's
// resolution enum.
type StepResolution string

const (
	StepResolutionPass StepResolution = "pass"
	StepResolutionFail StepResolution = "fail"
	StepResolutionSkip StepResolution = "skip"
)

// ResolveStepOptions is the payload for POST .../tcm/case-runs/:runId/
// steps/:stepUid/resolve.
//
// AttachmentIDs must reference attachments already uploaded via the TCM
// attachments endpoint (the SDK does not currently bundle a multipart
// upload helper — manual flows that need fresh uploads should hit
// /tcm/attachments directly first). Extracted is a free-form key/value bag
// the runner persists alongside the resolution and surfaces in the report.
type ResolveStepOptions struct {
	Extracted     map[string]any `json:"extracted,omitempty"`
	Resolution    StepResolution `json:"resolution"`
	Note          string         `json:"note,omitempty"`
	NoteFmt       string         `json:"noteFmt,omitempty"` // markdown (default) | plain
	ResolverKind  string         `json:"resolverKind,omitempty"`
	ErrorText     string         `json:"errorText,omitempty"`
	Namespace     string         `json:"-"` // path segment, not body
	AttachmentIDs []string       `json:"attachments,omitempty"`
}

// ResolveStep posts a verdict for one TCM case-run step. The server validates
// that the step is actually in manual_pending; non-manual steps are rejected
// with KindValidation.
func (a *TestPlansAPI) ResolveStep(ctx context.Context, runID, stepUID string, opts ResolveStepOptions) error {
	if runID == "" {
		return fmt.Errorf("mockarty: empty run id")
	}
	if stepUID == "" {
		return fmt.Errorf("mockarty: empty step uid")
	}
	switch opts.Resolution {
	case StepResolutionPass, StepResolutionFail, StepResolutionSkip:
		// ok
	default:
		return fmt.Errorf("mockarty: invalid resolution %q (want pass|fail|skip)", opts.Resolution)
	}
	path := a.namespaceScopedBase(opts.Namespace) +
		"/tcm/case-runs/" + url.PathEscape(runID) +
		"/steps/" + url.PathEscape(stepUID) +
		"/resolve"
	return a.client.do(ctx, http.MethodPost, path, opts, nil)
}

// PauseCaseRun / ResumeCaseRun / CancelCaseRun / RerunCaseRun control a
// single TCM case-run — useful from CI when a step deadlocks waiting for a
// human or when an over-eager runner needs to be aborted.
func (a *TestPlansAPI) caseRunAction(ctx context.Context, namespace, runID, action string) error {
	if runID == "" {
		return fmt.Errorf("mockarty: empty run id")
	}
	path := a.namespaceScopedBase(namespace) +
		"/tcm/case-runs/" + url.PathEscape(runID) +
		"/" + action
	return a.client.do(ctx, http.MethodPost, path, nil, nil)
}

// PauseCaseRun pauses a single TCM case-run.
func (a *TestPlansAPI) PauseCaseRun(ctx context.Context, namespace, runID string) error {
	return a.caseRunAction(ctx, namespace, runID, "pause")
}

// ResumeCaseRun resumes a paused TCM case-run.
func (a *TestPlansAPI) ResumeCaseRun(ctx context.Context, namespace, runID string) error {
	return a.caseRunAction(ctx, namespace, runID, "resume")
}

// CancelCaseRun cancels a TCM case-run (terminal).
func (a *TestPlansAPI) CancelCaseRun(ctx context.Context, namespace, runID string) error {
	return a.caseRunAction(ctx, namespace, runID, "cancel")
}

// RerunCaseRun starts a new TCM case-run with the same case definition.
func (a *TestPlansAPI) RerunCaseRun(ctx context.Context, namespace, runID string) error {
	return a.caseRunAction(ctx, namespace, runID, "rerun")
}

// ---------------------------------------------------------------------------
// Me / awaiting-manual — bell-badge counter for CI dashboards.
// ---------------------------------------------------------------------------

// MeAPI exposes the /api/v1/me/* endpoints. Currently there's only one
// caller-relevant route here (awaiting-manual); kept as a sub-API for
// future expansion (e.g. /me/preferences, /me/api-keys).
type MeAPI struct {
	client *Client
}

// Me returns the per-caller endpoints. Lazily constructed.
func (c *Client) Me() *MeAPI {
	if c.meAPI == nil {
		c.meAPI = &MeAPI{client: c}
	}
	return c.meAPI
}

// AwaitingManualSummary is one row from the awaiting-manual list.
type AwaitingManualSummary struct {
	AwaitingSince time.Time `json:"awaitingSince"`
	RunID         string    `json:"runId"`
	PlanRunID     string    `json:"planRunId,omitempty"`
	PlanName      string    `json:"planName,omitempty"`
	ItemUID       string    `json:"itemUid,omitempty"`
	StepUID       string    `json:"stepUid"`
	StepName      string    `json:"stepName"`
	StepRunID     string    `json:"stepRunId"`
	Namespace     string    `json:"namespace"`
	DeepLink      string    `json:"deepLink,omitempty"`
}

// AwaitingManualResponse mirrors GET /api/v1/me/awaiting-manual.
type AwaitingManualResponse struct {
	Items []AwaitingManualSummary `json:"items"`
	Count int                     `json:"count"`
}

// AwaitingManual returns the count + bounded preview of every TCM case-run
// step in manual_pending whose namespace the caller can read. Used by CI
// dashboards to fail-fast when a manual gate has been waiting too long.
func (a *MeAPI) AwaitingManual(ctx context.Context) (*AwaitingManualResponse, error) {
	var out AwaitingManualResponse
	if err := a.client.do(ctx, http.MethodGet, "/api/v1/me/awaiting-manual", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
