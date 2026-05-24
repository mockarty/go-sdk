// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

// ExternalRunOptions controls the case/plan binding when converting a
// Tester report into an ExternalRunRequest. All fields are optional —
// pass {} to upload as a synthetic case auto-created by the server.
type ExternalRunOptions struct {
	// CaseID or CaseName — at least one must be set for the server to
	// route the run to the right TCM case. CaseID wins when both
	// supplied.
	CaseID   string
	CaseName string

	// FullName, TestDisplayName — surfaced in the run header.
	FullName        string
	TestDisplayName string

	// Plan binding (optional). PlanRunID joins this submission to an
	// orchestrated plan run; PlanID is the plan template.
	PlanID    string
	PlanRunID string

	// Framework — informational, shown in the run header.
	Framework        string
	FrameworkVersion string

	// AutoCreate — when CaseName is supplied and no case matches,
	// create one server-side.
	AutoCreate bool

	// ClaimCaseOwnership — claim the case for the calling principal
	// (admin operation; server enforces).
	ClaimCaseOwnership bool

	// Extra labels + metadata merged into the request envelope. Tester
	// chain results contribute their own metadata under each step —
	// these are for the run-level labels (suite / feature / severity).
	Labels   map[string]string
	Metadata map[string]any
}

// ToExternalRun materialises the Tester report into an
// ExternalRunRequest ready for client.ExternalRuns().Submit(). Every
// StepRecord becomes one ExternalStep; protocol-specific fields
// (Protocol, Method, URL, StatusOrCode) land in the per-step Metadata
// map keyed as "protocol" / "method" / "url" / "statusOrCode" so the
// server's UI can render them without a schema migration.
//
// Run-level status:
//   - "passed" when t.OK() is true
//   - "failed" when any step has Failures
//   - First step error message is surfaced as Error
//
// StartedAt / FinishedAt are pulled from the first and last step
// timestamps respectively; both are nil-safe (omitted from the
// request when no steps fired).
func (t *Tester) ToExternalRun(opts ExternalRunOptions) mockarty.ExternalRunRequest {
	report := t.Report()
	req := mockarty.ExternalRunRequest{
		CaseID:             opts.CaseID,
		CaseName:           opts.CaseName,
		FullName:           opts.FullName,
		TestDisplayName:    opts.TestDisplayName,
		PlanID:             opts.PlanID,
		PlanRunID:          opts.PlanRunID,
		Framework:          firstNonEmpty(opts.Framework, "mockarty-tester-go"),
		FrameworkVersion:   opts.FrameworkVersion,
		AutoCreate:         opts.AutoCreate,
		ClaimCaseOwnership: opts.ClaimCaseOwnership,
		Labels:             opts.Labels,
		Metadata:           opts.Metadata,
		SchemaVersion:      mockarty.ExternalRunSchemaVersion,
	}

	if len(report) > 0 {
		startedAt := report[0].StartedAt
		endedAt := report[len(report)-1].EndedAt
		if !startedAt.IsZero() {
			req.StartedAt = &startedAt
		}
		if !endedAt.IsZero() {
			req.FinishedAt = &endedAt
			if req.StartedAt != nil {
				req.DurationMs = endedAt.Sub(*req.StartedAt).Milliseconds()
			}
		}

		req.Steps = make([]mockarty.ExternalStep, 0, len(report))
		for _, r := range report {
			step := mockarty.ExternalStep{
				Name:     r.Name,
				Metadata: stepMetadata(r),
			}
			if !r.StartedAt.IsZero() {
				ts := r.StartedAt
				step.StartedAt = &ts
			}
			if !r.EndedAt.IsZero() {
				ts := r.EndedAt
				step.FinishedAt = &ts
				if step.StartedAt != nil {
					step.DurationMs = r.EndedAt.Sub(*step.StartedAt).Milliseconds()
				}
			}
			if len(r.Failures) == 0 {
				step.Status = mockarty.ExternalStatusPassed
			} else {
				step.Status = mockarty.ExternalStatusFailed
				step.Error = joinFailures(r.Failures)
			}
			req.Steps = append(req.Steps, step)
		}
	}

	if t.OK() {
		req.Status = mockarty.ExternalStatusPassed
	} else {
		req.Status = mockarty.ExternalStatusFailed
		errs := t.Errors()
		if len(errs) > 0 {
			req.Error = errs[0].Error()
		}
	}

	return req
}

func stepMetadata(r StepRecord) map[string]any {
	m := map[string]any{
		"protocol": r.Protocol,
		"method":   r.Method,
	}
	if r.URL != "" {
		m["url"] = r.URL
	}
	if r.StatusOrCode != 0 {
		m["statusOrCode"] = r.StatusOrCode
	}
	return m
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func joinFailures(fs []string) string {
	if len(fs) == 0 {
		return ""
	}
	if len(fs) == 1 {
		return fs[0]
	}
	// Multiple — concat with "; " separator so a UI single-line
	// truncation still shows the first failure clearly.
	out := fs[0]
	for _, f := range fs[1:] {
		out += "; " + f
	}
	return out
}

// Ensure the time package import is used even if the body changes.
var _ = time.Time{}
