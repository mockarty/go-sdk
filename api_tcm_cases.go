// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// TCM cases / case-runs / defects — the test-case-management automation core.
//
// Cases live under /api/v1/namespaces/:ns/test-cases; case-runs and defects
// under /api/v1/namespaces/:ns/tcm/... . Payloads are rich and evolve, so this
// surface uses loosely-typed map I/O (mirrored by the Python dict and Java
// JsonNode SDKs), matching the IssueTracker API. Pass "" for namespace to use
// the client default.

// TCMObject is a loosely-typed TCM record (case / case-run / defect). Field
// names match the server JSON (e.g. obj["title"], obj["status"]).
type TCMObject = map[string]any

func (a *TCMAPI) nsBase(namespace, suffix string) (string, error) {
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return "", fmt.Errorf("tcm: namespace required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + suffix, nil
}

func (a *TCMAPI) sendObj(ctx context.Context, method, path string, body any) (TCMObject, error) {
	data, err := a.client.doJSON(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return TCMObject{}, nil
	}
	var out TCMObject
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("tcm: decode: %w", err)
	}
	return out, nil
}

// listObjs GETs path and unwraps whichever of keys[] holds the array (the TCM
// list endpoints vary: test_cases / runs / defects), falling back to a bare
// top-level array.
func (a *TCMAPI) listObjs(ctx context.Context, path string, keys ...string) ([]TCMObject, error) {
	data, err := a.client.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	// Bare array?
	var arr []TCMObject
	if json.Unmarshal(data, &arr) == nil && arr != nil {
		return arr, nil
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("tcm: decode list: %w", err)
	}
	for _, k := range keys {
		raw, ok := env[k]
		// Skip a present-but-null key (e.g. {"cases":null,"items":[...]}) so we
		// fall through to the next candidate instead of returning [] — matches
		// the Python/Java behaviour of only accepting an array value.
		if !ok || len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var out []TCMObject
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("tcm: decode %s: %w", k, err)
		}
		return out, nil
	}
	return []TCMObject{}, nil
}

// ── Cases ──────────────────────────────────────────────────────────────

// CreateCase creates a test case (fields: title, folderId, steps, …).
func (a *TCMAPI) CreateCase(ctx context.Context, namespace string, testCase TCMObject) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases")
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodPost, base, testCase)
}

// GetCase fetches a test case by id.
func (a *TCMAPI) GetCase(ctx context.Context, namespace, caseID string) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases/"+url.PathEscape(caseID))
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodGet, base, nil)
}

// ListCases lists test cases, optionally filtered via query params.
func (a *TCMAPI) ListCases(ctx context.Context, namespace string, filters map[string]string) ([]TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases")
	if err != nil {
		return nil, err
	}
	return a.listObjs(ctx, base+encodeQuery(filters), "test_cases", "cases", "items")
}

// UpdateCase applies an update to a test case.
func (a *TCMAPI) UpdateCase(ctx context.Context, namespace, caseID string, fields TCMObject) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases/"+url.PathEscape(caseID))
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodPut, base, fields)
}

// DeleteCase soft-deletes a test case.
func (a *TCMAPI) DeleteCase(ctx context.Context, namespace, caseID string) error {
	base, err := a.nsBase(namespace, "/test-cases/"+url.PathEscape(caseID))
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, base, nil, nil)
}

// RunCase starts a run of a test case and returns the run descriptor (with the
// run id to poll via GetCaseRun). opts may be nil.
func (a *TCMAPI) RunCase(ctx context.Context, namespace, caseID string, opts TCMObject) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases/"+url.PathEscape(caseID)+"/run")
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodPost, base, opts)
}

// ListCaseRuns lists prior runs of a test case.
func (a *TCMAPI) ListCaseRuns(ctx context.Context, namespace, caseID string) ([]TCMObject, error) {
	base, err := a.nsBase(namespace, "/test-cases/"+url.PathEscape(caseID)+"/runs")
	if err != nil {
		return nil, err
	}
	return a.listObjs(ctx, base, "runs", "caseRuns", "case_runs", "items")
}

// ── Case-runs ──────────────────────────────────────────────────────────

// GetCaseRun fetches a single case-run by id.
func (a *TCMAPI) GetCaseRun(ctx context.Context, namespace, runID string) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/tcm/case-runs/"+url.PathEscape(runID))
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodGet, base, nil)
}

// CancelCaseRun cancels an in-flight case-run.
func (a *TCMAPI) CancelCaseRun(ctx context.Context, namespace, runID string) error {
	base, err := a.nsBase(namespace, "/tcm/case-runs/"+url.PathEscape(runID)+"/cancel")
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodPost, base, nil, nil)
}

// ── Defects ────────────────────────────────────────────────────────────

// CreateDefect files a defect (fields: title, description, caseRunId, …).
func (a *TCMAPI) CreateDefect(ctx context.Context, namespace string, defect TCMObject) (TCMObject, error) {
	base, err := a.nsBase(namespace, "/tcm/defects")
	if err != nil {
		return nil, err
	}
	return a.sendObj(ctx, http.MethodPost, base, defect)
}

// ListDefects lists defects, optionally filtered.
func (a *TCMAPI) ListDefects(ctx context.Context, namespace string, filters map[string]string) ([]TCMObject, error) {
	base, err := a.nsBase(namespace, "/tcm/defects")
	if err != nil {
		return nil, err
	}
	return a.listObjs(ctx, base+encodeQuery(filters), "defects", "items")
}

// DeleteDefect deletes a defect by id.
func (a *TCMAPI) DeleteDefect(ctx context.Context, namespace, defectID string) error {
	base, err := a.nsBase(namespace, "/tcm/defects/"+url.PathEscape(defectID))
	if err != nil {
		return err
	}
	return a.client.do(ctx, http.MethodDelete, base, nil, nil)
}
