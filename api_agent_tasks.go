// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// AgentTaskAPI provides methods for managing AI agent tasks.
type AgentTaskAPI struct {
	client *Client
}

// AgentTask represents an AI agent task.
type AgentTask struct {
	ID        string `json:"id,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Status    string `json:"status,omitempty"`
	Result    any    `json:"result,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// List returns all agent tasks.
func (a *AgentTaskAPI) List(ctx context.Context) ([]AgentTask, error) {
	var tasks []AgentTask
	if err := a.client.do(ctx, "GET", "/api/v1/agent-tasks", nil, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// Get retrieves an agent task by ID.
func (a *AgentTaskAPI) Get(ctx context.Context, id string) (*AgentTask, error) {
	var task AgentTask
	if err := a.client.do(ctx, "GET", "/api/v1/agent-tasks/"+url.PathEscape(id), nil, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

// Submit creates and submits a new agent task.
func (a *AgentTaskAPI) Submit(ctx context.Context, task *AgentTask) (*AgentTask, error) {
	var result AgentTask
	if err := a.client.do(ctx, "POST", "/api/v1/agent-tasks", task, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Cancel cancels a running agent task.
func (a *AgentTaskAPI) Cancel(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/agent-tasks/"+url.PathEscape(id)+"/cancel", nil, nil)
}

// Delete deletes an agent task by ID.
func (a *AgentTaskAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/agent-tasks/"+url.PathEscape(id), nil, nil)
}

// ClearAll deletes all agent tasks.
func (a *AgentTaskAPI) ClearAll(ctx context.Context) error {
	return a.client.do(ctx, "DELETE", "/api/v1/agent-tasks", nil, nil)
}

// Rerun re-executes an agent task by ID.
func (a *AgentTaskAPI) Rerun(ctx context.Context, id string) (*AgentTask, error) {
	var result AgentTask
	if err := a.client.do(ctx, "POST", "/api/v1/agent-tasks/"+url.PathEscape(id)+"/rerun", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Export exports an agent task result as raw bytes.
func (a *AgentTaskAPI) Export(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/api/v1/agent-tasks/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}
