// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// PromptsAPI provides methods for the Prompts Storage feature —
// centrally-managed AI prompt templates with FIFO-20 version history and
// rollback. Prompts are attached to AI buttons / TCM steps by ID, so
// editing a prompt propagates to every consumer without code changes.
type PromptsAPI struct {
	client *Client
}

// Prompt is a single managed prompt template.
type Prompt struct {
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Description string            `json:"description,omitempty"`
	Body        string            `json:"body"`
	Model       string            `json:"model,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	Version     int               `json:"version"`
}

// PromptVersion is one entry from a prompt's FIFO-20 history.
type PromptVersion struct {
	CreatedAt time.Time `json:"createdAt"`
	Author    string    `json:"author,omitempty"`
	Body      string    `json:"body"`
	Model     string    `json:"model,omitempty"`
	Note      string    `json:"note,omitempty"`
	Version   int       `json:"version"`
}

// ListPrompts returns every prompt in the client's default namespace.
func (a *PromptsAPI) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var out []Prompt
	if err := a.client.do(ctx, "GET", "/api/v1/stores/prompts?namespace="+url.QueryEscape(a.client.namespace), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreatePrompt creates a new prompt (becomes version 1).
func (a *PromptsAPI) CreatePrompt(ctx context.Context, p Prompt) (*Prompt, error) {
	if p.Namespace == "" {
		p.Namespace = a.client.namespace
	}
	var out Prompt
	if err := a.client.do(ctx, "POST", "/api/v1/stores/prompts", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPrompt fetches the current version of a prompt.
func (a *PromptsAPI) GetPrompt(ctx context.Context, id string) (*Prompt, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: prompt id is required")
	}
	var out Prompt
	if err := a.client.do(ctx, "GET", "/api/v1/stores/prompts/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePrompt saves a new version of the prompt. The previous body is
// appended to version history (FIFO-capped at 20).
func (a *PromptsAPI) UpdatePrompt(ctx context.Context, id string, p Prompt) (*Prompt, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: prompt id is required")
	}
	var out Prompt
	if err := a.client.do(ctx, "PUT", "/api/v1/stores/prompts/"+url.PathEscape(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePrompt removes a prompt along with its version history.
func (a *PromptsAPI) DeletePrompt(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("mockarty: prompt id is required")
	}
	return a.client.do(ctx, "DELETE", "/api/v1/stores/prompts/"+url.PathEscape(id), nil, nil)
}

// ListVersions returns the prompt's last 20 versions (newest first).
func (a *PromptsAPI) ListVersions(ctx context.Context, id string) ([]PromptVersion, error) {
	if id == "" {
		return nil, fmt.Errorf("mockarty: prompt id is required")
	}
	var out []PromptVersion
	if err := a.client.do(ctx, "GET", "/api/v1/stores/prompts/"+url.PathEscape(id)+"/versions", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetVersion fetches a specific historical version.
func (a *PromptsAPI) GetVersion(ctx context.Context, id string, version int) (*PromptVersion, error) {
	if id == "" || version <= 0 {
		return nil, fmt.Errorf("mockarty: prompt id and positive version are required")
	}
	var out PromptVersion
	path := fmt.Sprintf("/api/v1/stores/prompts/%s/versions/%d", url.PathEscape(id), version)
	if err := a.client.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Rollback restores the prompt body from a previous version. The current
// body is itself pushed onto the history stack first — nothing is lost.
func (a *PromptsAPI) Rollback(ctx context.Context, id string, toVersion int) (*Prompt, error) {
	if id == "" || toVersion <= 0 {
		return nil, fmt.Errorf("mockarty: prompt id and positive toVersion are required")
	}
	var out Prompt
	path := fmt.Sprintf("/api/v1/stores/prompts/%s/rollback?to=%d", url.PathEscape(id), toVersion)
	if err := a.client.do(ctx, "POST", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
