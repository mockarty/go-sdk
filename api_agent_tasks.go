// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
	CreatedAt time.Time `json:"createdAt,omitempty"`
	ID        string    `json:"id,omitempty"`
	Title     string    `json:"title,omitempty"`
	Prompt    string    `json:"prompt,omitempty"`
	Status    string    `json:"status,omitempty"`
	Result    any       `json:"result,omitempty"`
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
		Task AgentTask `json:"task"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/agent/tasks/"+url.PathEscape(id), nil, &env); err != nil {
		return nil, err
	}
	return &env.Task, nil
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
