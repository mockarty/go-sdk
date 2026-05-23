// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockarty

import (
	"context"
	"net/url"
)

// CITriggersAPI is the minimal SDK surface for the Phase 4 CI Triggers
// feature. Per SDK scope policy (feedback_sdk_cli_scope.md): only the
// surface that's genuinely useful from CI/CD pipelines + scripts. CRUD
// of triggers themselves is an administrative UI concern and lives on
// the REST API only — this SDK covers list + run-with-trigger
// (look-up + status polling).
type CITriggersAPI struct {
	client *Client
}

// CITriggers gates the SDK's CI Triggers methods.
func (c *Client) CITriggers() *CITriggersAPI { return &CITriggersAPI{client: c} }

// CITrigger is the read-side shape of a saved CI trigger. Fields are
// trimmed to what a CI/CD script typically needs: enough to pick the
// right one out of a list, no internal columns. AuthSecret is always
// returned as "***" by the server.
type CITrigger struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	TriggerURL   string `json:"triggerUrl"`
	TemplateKind string `json:"templateKind"`
	Enabled      bool   `json:"enabled"`
}

// CIRun is the read-side shape of one run through a trigger. Joined to a
// runner_task via DispatchToken. The state vocabulary is per-trigger;
// the SDK exposes whatever string the server returns so the script can
// poll until it sees its preferred terminal value.
type CIRun struct {
	ID            string `json:"id"`
	TaskID        string `json:"taskId"`
	TriggerID     string `json:"triggerId"`
	DispatchToken string `json:"dispatchToken"`
	ExternalJobID string `json:"externalJobId"`
	ExternalState string `json:"externalState"`
	ExternalError string `json:"externalError,omitempty"`
	Namespace     string `json:"namespace"`
}

// List returns enabled + disabled triggers in a namespace. Empty
// namespace falls back to the caller's authenticated namespace
// claim on the server side.
func (a *CITriggersAPI) List(ctx context.Context, namespace string) ([]CITrigger, error) {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	var resp struct {
		Triggers []CITrigger `json:"triggers"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/ci/triggers"+q, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Triggers, nil
}

// Get returns a single trigger by ID. The caller's namespace must match
// the trigger's namespace — cross-NS lookups return 404 (the server
// hides existence across NS boundaries).
func (a *CITriggersAPI) Get(ctx context.Context, id, namespace string) (*CITrigger, error) {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	var resp struct {
		Trigger CITrigger `json:"trigger"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/ci/triggers/"+url.PathEscape(id)+q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Trigger, nil
}

// GetRun returns the run linked to a runner task. The id parameter
// matches the runner.Task.ID; the returned run carries the external
// CI job ID + the current external state. Returns ErrNotFound when no
// CI run is associated with the task.
func (a *CITriggersAPI) GetRun(ctx context.Context, taskID string) (*CIRun, error) {
	var resp struct {
		Run CIRun `json:"run"`
	}
	q := "?taskId=" + url.QueryEscape(taskID)
	if err := a.client.do(ctx, "GET", "/api/v1/ci/runs"+q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

// CancelRun marks a run as cancelled and fails the linked runner task.
// Best-effort — does not call a remote CI cancel API.
func (a *CITriggersAPI) CancelRun(ctx context.Context, runID string) error {
	return a.client.do(ctx, "POST", "/api/v1/ci/runs/"+url.PathEscape(runID)+"/cancel", nil, nil)
}
