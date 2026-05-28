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

// CITriggerInput is the write-side shape used by Create / Update.
// Mirrors the admin REST surface (POST/PATCH /api/v1/ci/triggers).
// Empty fields on PATCH mean 'leave existing' on the server side;
// AuthSecret specifically — pass empty to keep the previously-stored
// secret, pass a value to rotate it.
type CITriggerInput struct {
	TriggerHeaders      map[string]string `json:"triggerHeaders,omitempty"`
	StatusHeaders       map[string]string `json:"statusHeaders,omitempty"`
	StatusSuccessValues []string          `json:"statusSuccessValues,omitempty"`
	StatusFailureValues []string          `json:"statusFailureValues,omitempty"`
	StatusPendingValues []string          `json:"statusPendingValues,omitempty"`
	Tags                []string          `json:"tags,omitempty"`

	Name                string `json:"name"`
	Description         string `json:"description,omitempty"`
	TriggerURL          string `json:"triggerUrl"`
	TriggerMethod       string `json:"triggerMethod,omitempty"`
	TriggerBodyTemplate string `json:"triggerBodyTemplate,omitempty"`
	StatusURLTemplate   string `json:"statusUrlTemplate,omitempty"`
	StatusMethod        string `json:"statusMethod,omitempty"`
	StatusIDJSONPath    string `json:"statusIdJsonpath,omitempty"`
	StatusStateJSONPath string `json:"statusStateJsonpath,omitempty"`
	StatusErrorJSONPath string `json:"statusErrorJsonpath,omitempty"`
	AuthKind            string `json:"authKind,omitempty"`
	AuthSecret          string `json:"authSecret,omitempty"`
	TemplateKind        string `json:"templateKind,omitempty"`
	RepositoryURL       string `json:"repositoryUrl,omitempty"`

	PollIntervalSec int32 `json:"pollIntervalSec,omitempty"`
	PollTimeoutSec  int32 `json:"pollTimeoutSec,omitempty"`

	Enabled *bool `json:"enabled,omitempty"`
}

// CITestOutcome carries the dispatcher's reading after a /test call:
// HTTP status the upstream returned, the rendered request body (so the
// caller can verify their template), the extracted externalJobId, the
// upstream's response excerpt (truncated server-side), and a friendly
// userMessage. RetryAfter is set when the upstream signalled a Retry-
// After header.
type CITestOutcome struct {
	HTTPStatus    int    `json:"httpStatus"`
	ExternalJobID string `json:"externalJobId,omitempty"`
	RenderedBody  string `json:"renderedBody,omitempty"`
	ResponseBody  string `json:"responseBody,omitempty"`
	UserMessage   string `json:"userMessage,omitempty"`
	RetryAfter    string `json:"retryAfter,omitempty"`
}

// CITestResult is the wire envelope of POST /api/v1/ci/triggers/:id/test.
// ok=true when the dispatcher's call to the upstream CI succeeded;
// false when ANY step (template render, secret expansion, HTTP call,
// JSONPath extraction) failed. Error carries the dispatcher's
// human-readable error string; Outcome.UserMessage carries the same on
// soft-failures (4xx/5xx upstream).
type CITestResult struct {
	Outcome *CITestOutcome `json:"outcome,omitempty"`
	Error   string         `json:"error,omitempty"`
	OK      bool           `json:"ok"`
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

// Create stores a new trigger in the given namespace. Useful for IaC
// bootstrap scripts that seed a fresh tenant + for integration tests
// where the test itself owns the trigger lifecycle. The returned
// trigger has its AuthSecret redacted as "***" (defence-in-depth — the
// SDK never echoes plaintext secrets back to its caller).
func (a *CITriggersAPI) Create(ctx context.Context, namespace string, in *CITriggerInput) (*CITrigger, error) {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	// Server-side mandatory minima: pollIntervalSec ≥ 2, pollTimeoutSec
	// ≥ 60. CI/CD scripts rarely care about polling cadence (the trigger
	// either fires-and-forgets or the script polls /ci/runs itself), so
	// the SDK fills in sensible defaults on CREATE when the caller
	// leaves these zero. PATCH/Update intentionally does NOT fill them
	// in — preserving the existing values is the right PATCH semantic.
	patched := *in
	if patched.PollIntervalSec == 0 {
		patched.PollIntervalSec = 10
	}
	if patched.PollTimeoutSec == 0 {
		patched.PollTimeoutSec = 1800
	}
	var resp struct {
		Trigger CITrigger `json:"trigger"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/ci/triggers"+q, &patched, &resp); err != nil {
		return nil, err
	}
	return &resp.Trigger, nil
}

// Update PATCH-merges fields into an existing trigger. Empty payload
// fields preserve the existing value on the server side (so a caller
// rotating only the URL does NOT have to re-send the secret). To
// CLEAR a previously-set value (e.g. drop the description) the
// server requires an explicit explicit-clear path — not yet exposed
// in the SDK because the use-case is rare; route to the REST API
// directly if you need it.
func (a *CITriggersAPI) Update(ctx context.Context, id, namespace string, in *CITriggerInput) (*CITrigger, error) {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	var resp struct {
		Trigger CITrigger `json:"trigger"`
	}
	if err := a.client.do(ctx, "PATCH", "/api/v1/ci/triggers/"+url.PathEscape(id)+q, in, &resp); err != nil {
		return nil, err
	}
	return &resp.Trigger, nil
}

// Delete soft-deletes a trigger (moves to the trash registry). Existing
// in-flight runs continue to be tracked — the trigger row stays in
// the DB with closed_at set, so the runs FK is preserved.
func (a *CITriggersAPI) Delete(ctx context.Context, id, namespace string) error {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	return a.client.do(ctx, "DELETE", "/api/v1/ci/triggers/"+url.PathEscape(id)+q, nil, nil)
}

// TestDispatch fires a saved trigger against its configured URL with
// a synthetic task ID — no ci_runs row is created, no runner task is
// claimed. Useful as a smoke probe before scheduling real work: the
// returned outcome tells the caller whether the upstream CI accepted
// the request and what externalJobId would have been extracted.
//
// Rate-limited 20/min/user by the server. The dispatcher's call to
// the upstream is best-effort: if the upstream returns 5xx, the
// returned result still has OK=false but the Outcome carries
// HTTPStatus + ResponseBody so the caller can diagnose.
func (a *CITriggersAPI) TestDispatch(ctx context.Context, id, namespace string) (*CITestResult, error) {
	q := ""
	if namespace != "" {
		q = "?namespace=" + url.QueryEscape(namespace)
	}
	var out CITestResult
	if err := a.client.do(ctx, "POST", "/api/v1/ci/triggers/"+url.PathEscape(id)+"/test"+q, struct{}{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
