// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AgentTaskAPI provides methods for managing AI agent tasks.
type AgentTaskAPI struct {
	client *Client
}

// AgentTask represents an AI agent task.
//
// Title is required on Submit per server's submitAgentTask handler
// (binding:"required"). The older SDK shape didn't expose Title and
// every Submit 400'd with 'title and prompt are required'.
//
// CreatedAt was int64 in older builds but the server emits RFC3339
// strings via time.Time — every Submit/Get response failed to decode
// with 'cannot unmarshal string into Go struct field
// AgentTask.task.createdAt of type int64'.
type AgentTask struct {
	CreatedAt                         time.Time     `json:"createdAt,omitempty"`
	ID                                string        `json:"id,omitempty"`
	Title                             string        `json:"title,omitempty"`
	Prompt                            string        `json:"prompt,omitempty"`
	Status                            string        `json:"status,omitempty"`
	Result                            any           `json:"result,omitempty"`
	ToolReceipts                      []ToolReceipt `json:"toolReceipts,omitempty"`
	ToolReceiptReconcileBlockedReason string        `json:"toolReceiptReconcileBlockedReason,omitempty"`
	// CanReconcileToolReceipts is true only after the task stops and the task
	// owner's user has Coder Deploy permission in the task namespace. Owners
	// without that permission can inspect receipts but cannot record a decision.
	CanReconcileToolReceipts bool `json:"canReconcileToolReceipts"`
	// ToolReceiptRetryAllowed is false for cancelled tasks even when evidence
	// decisions (already_applied or mark_failed) remain available.
	ToolReceiptRetryAllowed bool `json:"toolReceiptRetryAllowed"`
}

// ToolReceipt is the durable state of one external action issued by an agent.
// awaiting_reconcile means Mockarty deliberately paused automatic recovery:
// inspect the downstream system before choosing an outcome.
type ToolReceipt struct {
	DecisionAt         *time.Time `json:"decisionAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	Namespace          string     `json:"namespace"`
	ReceiptKey         string     `json:"receiptKey"`
	ArgsDigest         string     `json:"argsDigest"`
	TaskID             string     `json:"taskId"`
	AttemptID          string     `json:"attemptId"`
	ToolName           string     `json:"toolName"`
	EffectClass        string     `json:"effectClass"`
	Result             string     `json:"result,omitempty"`
	Status             string     `json:"status"`
	DispatchEventID    string     `json:"dispatchEventId,omitempty"`
	Decision           string     `json:"decision,omitempty"`
	DecisionActor      string     `json:"decisionActor,omitempty"`
	DecisionReason     string     `json:"decisionReason,omitempty"`
	Version            int64      `json:"version"`
	LogicalOrdinal     int        `json:"logicalOrdinal"`
	ReplayCount        int        `json:"replayCount"`
	DispatchGeneration int        `json:"dispatchGeneration"`
	IsError            bool       `json:"isError"`
}

// ToolReceiptReconcileRequest records a version-fenced operator decision.
// IdempotencyKey must remain stable when retrying the same request.
type ToolReceiptReconcileRequest struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Decision        string `json:"decision"`
	Reason          string `json:"reason"`
	Result          string `json:"result,omitempty"`
}

// LegacyAgentSessionSummary describes one pre-namespace session available to
// its authenticated owner for explicit export or recovery.
type LegacyAgentSessionSummary struct {
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	OriginalID string    `json:"originalId"`
	AppName    string    `json:"appName"`
	EventCount int64     `json:"eventCount"`
}

// AgentSession is a durable, namespace-scoped agent conversation.
type AgentSession struct {
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	ExpiresAt time.Time       `json:"expiresAt"`
	ID        string          `json:"id"`
	UserID    string          `json:"userId"`
	Namespace string          `json:"namespace"`
	AppName   string          `json:"appName"`
	State     json.RawMessage `json:"state,omitempty"`
}

// AgentSessionEvent is one recovered conversation turn.
type AgentSessionEvent struct {
	CreatedAt   time.Time       `json:"createdAt"`
	SessionID   string          `json:"sessionId"`
	Role        string          `json:"role"`
	Content     string          `json:"content,omitempty"`
	ToolCalls   json.RawMessage `json:"toolCalls,omitempty"`
	ToolResults json.RawMessage `json:"toolResults,omitempty"`
	ID          int64           `json:"id"`
}

// LegacyAgentSessionPage is a bounded keyset page of recoverable sessions.
type LegacyAgentSessionPage struct {
	Sessions   []LegacyAgentSessionSummary `json:"sessions"`
	NextCursor string                      `json:"nextCursor,omitempty"`
}

// LegacyAgentSessionExport is one bounded transcript page.
type LegacyAgentSessionExport struct {
	Session     AgentSession        `json:"session"`
	Events      []AgentSessionEvent `json:"events"`
	NextEventID int64               `json:"nextEventId,omitempty"`
	Truncated   bool                `json:"truncated"`
}

// LegacyAgentSessionClaimRequest requires explicit acknowledgement because a
// pre-namespace transcript has no trustworthy original workspace assignment.
type LegacyAgentSessionClaimRequest struct {
	Namespace                string `json:"namespace"`
	SessionKey               string `json:"sessionKey,omitempty"`
	AcknowledgeUnknownOrigin bool   `json:"acknowledgeUnknownOrigin"`
}

// List returns all agent tasks.
//
// Wire shape: server emits `{tasks: [...], total, limit, offset}`
// envelope — NOT a bare list. Older SDK builds decoded into
// `[]AgentTask` and 'cannot unmarshal object into ...' on every call.
func (a *AgentTaskAPI) List(ctx context.Context) ([]AgentTask, error) {
	var env struct {
		Tasks []AgentTask `json:"tasks"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/agent/tasks", nil, &env); err != nil {
		return nil, err
	}
	if env.Tasks == nil {
		return []AgentTask{}, nil
	}
	return env.Tasks, nil
}

// Get retrieves an agent task by ID.
//
// Wire shape: server emits `{task: <AgentTask>}` envelope.
func (a *AgentTaskAPI) Get(ctx context.Context, id string) (*AgentTask, error) {
	var env struct {
		Task                              AgentTask     `json:"task"`
		ToolReceipts                      []ToolReceipt `json:"toolReceipts"`
		ToolReceiptReconcileBlockedReason string        `json:"toolReceiptReconcileBlockedReason"`
		CanReconcileToolReceipts          bool          `json:"canReconcileToolReceipts"`
		ToolReceiptRetryAllowed           bool          `json:"toolReceiptRetryAllowed"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/agent/tasks/"+url.PathEscape(id), nil, &env); err != nil {
		return nil, err
	}
	env.Task.ToolReceipts = env.ToolReceipts
	env.Task.CanReconcileToolReceipts = env.CanReconcileToolReceipts
	env.Task.ToolReceiptRetryAllowed = env.ToolReceiptRetryAllowed
	env.Task.ToolReceiptReconcileBlockedReason = env.ToolReceiptReconcileBlockedReason
	return &env.Task, nil
}

// ReconcileToolReceipt resolves one awaiting_reconcile external action after
// the caller has inspected the real downstream system. Decisions are
// already_applied, retry_once, or mark_failed. retry_once must use an empty
// Result and authorizes exactly one new physical dispatch generation.
func (a *AgentTaskAPI) ReconcileToolReceipt(ctx context.Context, taskID, receiptKey string, request ToolReceiptReconcileRequest) (*ToolReceipt, error) {
	var envelope struct {
		Receipt ToolReceipt `json:"receipt"`
	}
	path := "/api/v1/agent/tasks/" + url.PathEscape(taskID) + "/tool-receipts/" +
		url.PathEscape(receiptKey) + "/reconcile"
	if err := a.client.do(ctx, "POST", path, request, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Receipt, nil
}

// Submit creates and submits a new agent task.
//
// Wire shape: server replies with `{task: <AgentTask>, message: "..."}` —
// older SDK builds decoded into a bare AgentTask and returned an empty
// struct with no ID, breaking the immediate Cancel / Delete follow-up
// that every CI/CD wrapper relies on.
func (a *AgentTaskAPI) Submit(ctx context.Context, task *AgentTask) (*AgentTask, error) {
	var env struct {
		Task AgentTask `json:"task"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/agent/tasks", task, &env); err != nil {
		return nil, err
	}
	return &env.Task, nil
}

// Cancel cancels a running agent task.
func (a *AgentTaskAPI) Cancel(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/agent/tasks/"+url.PathEscape(id)+"/cancel", nil, nil)
}

// Delete deletes an agent task by ID.
func (a *AgentTaskAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/agent/tasks/"+url.PathEscape(id), nil, nil)
}

// ClearAll deletes all agent tasks.
func (a *AgentTaskAPI) ClearAll(ctx context.Context) error {
	return a.client.do(ctx, "DELETE", "/api/v1/agent/tasks", nil, nil)
}

// Rerun re-executes an agent task by ID.
//
// Wire shape: server replies with `{task: <AgentTask>, message: "..."}`.
func (a *AgentTaskAPI) Rerun(ctx context.Context, id string) (*AgentTask, error) {
	var env struct {
		Task AgentTask `json:"task"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/agent/tasks/"+url.PathEscape(id)+"/rerun", nil, &env); err != nil {
		return nil, err
	}
	return &env.Task, nil
}

// Export exports an agent task result as raw bytes.
func (a *AgentTaskAPI) Export(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/agent/tasks/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ListLegacySessions lists owner-scoped recoverable sessions using a bounded
// keyset page. Pass the returned NextCursor to continue.
func (a *AgentTaskAPI) ListLegacySessions(ctx context.Context, limit int, cursor string) (*LegacyAgentSessionPage, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("legacy session page limit must be between 1 and 100")
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	path := "/api/v1/agent/sessions/legacy"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var page LegacyAgentSessionPage
	if err := a.client.do(ctx, "GET", path, nil, &page); err != nil {
		return nil, err
	}
	if page.Sessions == nil {
		page.Sessions = []LegacyAgentSessionSummary{}
	}
	return &page, nil
}

// ExportLegacySession returns one bounded page of a quarantined transcript.
func (a *AgentTaskAPI) ExportLegacySession(ctx context.Context, id string, limit int, afterEventID int64) (*LegacyAgentSessionExport, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("legacy session id is required")
	}
	if limit < 1 || limit > 2000 {
		return nil, fmt.Errorf("legacy session export limit must be between 1 and 2000")
	}
	if afterEventID < 0 {
		return nil, fmt.Errorf("legacy session event cursor must be non-negative")
	}
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("afterEventId", strconv.FormatInt(afterEventID, 10))
	path := "/api/v1/agent/sessions/legacy/" + url.PathEscape(id) + "/export"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var exported LegacyAgentSessionExport
	if err := a.client.do(ctx, "GET", path, nil, &exported); err != nil {
		return nil, err
	}
	if exported.Events == nil {
		exported.Events = []AgentSessionEvent{}
	}
	return &exported, nil
}

// ClaimLegacySession atomically moves an owner-scoped transcript into a
// write-authorized namespace. AcknowledgeUnknownOrigin must be true.
func (a *AgentTaskAPI) ClaimLegacySession(ctx context.Context, id string, request LegacyAgentSessionClaimRequest) (*AgentSession, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("legacy session id is required")
	}
	if strings.TrimSpace(request.Namespace) == "" {
		return nil, fmt.Errorf("legacy session target namespace is required")
	}
	if !request.AcknowledgeUnknownOrigin {
		return nil, fmt.Errorf("legacy session recovery requires acknowledging the unknown origin")
	}
	var envelope struct {
		Session AgentSession `json:"session"`
	}
	path := "/api/v1/agent/sessions/legacy/" + url.PathEscape(id) + "/claim"
	if err := a.client.do(ctx, "POST", path, request, &envelope); err != nil {
		return nil, err
	}
	return &envelope.Session, nil
}

// Terminal errors returned by WaitForResult when an agent task ends without
// succeeding. Callers match with errors.Is.
var (
	ErrAgentTaskFailed    = errors.New("mockarty: agent task failed")
	ErrAgentTaskCancelled = errors.New("mockarty: agent task cancelled")
)

// WaitForResult polls an agent task until it reaches a terminal state or ctx
// expires. On "completed" it returns the finished task (with .Result); on
// "failed"/"cancelled" it returns the task wrapped in ErrAgentTaskFailed /
// ErrAgentTaskCancelled. pollInterval <= 0 defaults to 2s. This is the
// automation counterpart to Submit — dispatch a task into the agent network
// and block for its result without hand-rolling a poll loop.
func (a *AgentTaskAPI) WaitForResult(ctx context.Context, id string, pollInterval time.Duration) (*AgentTask, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	for {
		task, err := a.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		// Case-insensitive to match the Python/Java SDKs and survive a server
		// that ever capitalises a terminal status.
		switch strings.ToLower(task.Status) {
		case "completed", "done", "succeeded":
			return task, nil
		case "failed", "error":
			return task, fmt.Errorf("%w: task %s", ErrAgentTaskFailed, id)
		case "cancelled", "canceled":
			return task, fmt.Errorf("%w: task %s", ErrAgentTaskCancelled, id)
		}
		select {
		case <-ctx.Done():
			return task, ctx.Err()
		case <-t.C:
		}
	}
}

// SubmitAndWait submits a task and blocks until it reaches a terminal state,
// combining Submit + WaitForResult — the one-call automation entry point for
// "run this in the agent network and give me the result".
func (a *AgentTaskAPI) SubmitAndWait(ctx context.Context, task *AgentTask, pollInterval time.Duration) (*AgentTask, error) {
	submitted, err := a.Submit(ctx, task)
	if err != nil {
		return nil, err
	}
	if submitted.ID == "" {
		return submitted, fmt.Errorf("mockarty: agent task submitted without an id")
	}
	return a.WaitForResult(ctx, submitted.ID, pollInterval)
}
