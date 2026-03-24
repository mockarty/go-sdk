// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// FuzzingAPI provides methods for managing fuzzing tests.
type FuzzingAPI struct {
	client *Client
}

// FuzzingConfig defines the configuration for a fuzzing run.
type FuzzingConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
	SpecURL   string `json:"specUrl,omitempty"`
	Spec      string `json:"spec,omitempty"`
	Duration  string `json:"duration,omitempty"`
	Workers   int    `json:"workers,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// FuzzingRun represents a started fuzzing run.
type FuzzingRun struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

// FuzzingResult holds the results of a completed or in-progress fuzzing run.
type FuzzingResult struct {
	ID            string `json:"id,omitempty"`
	ConfigID      string `json:"configId,omitempty"`
	Status        string `json:"status,omitempty"`
	StartedAt     int64  `json:"startedAt,omitempty"`
	FinishedAt    int64  `json:"finishedAt,omitempty"`
	TotalRequests int    `json:"totalRequests,omitempty"`
	Findings      int    `json:"findings,omitempty"`
}

// Start starts a new fuzzing run with the given configuration.
func (a *FuzzingAPI) Start(ctx context.Context, config *FuzzingConfig) (*FuzzingRun, error) {
	if config.Namespace == "" && a.client.namespace != "" {
		config.Namespace = a.client.namespace
	}
	var run FuzzingRun
	if err := a.client.do(ctx, "POST", "/ui/api/fuzzing/start", config, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// Stop stops a running fuzzing run by ID.
func (a *FuzzingAPI) Stop(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/ui/api/fuzzing/"+url.PathEscape(id)+"/stop", nil, nil)
}

// GetResult retrieves the result of a fuzzing run by ID.
func (a *FuzzingAPI) GetResult(ctx context.Context, id string) (*FuzzingResult, error) {
	var result FuzzingResult
	if err := a.client.do(ctx, "GET", "/ui/api/fuzzing/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListResults returns all fuzzing results.
func (a *FuzzingAPI) ListResults(ctx context.Context) ([]FuzzingResult, error) {
	var results []FuzzingResult
	if err := a.client.do(ctx, "GET", "/ui/api/fuzzing/results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteResult deletes a fuzzing result by ID.
func (a *FuzzingAPI) DeleteResult(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/ui/api/fuzzing/"+url.PathEscape(id), nil, nil)
}

// CreateConfig creates a new fuzzing configuration.
func (a *FuzzingAPI) CreateConfig(ctx context.Context, config *FuzzingConfig) (*FuzzingConfig, error) {
	if config.Namespace == "" && a.client.namespace != "" {
		config.Namespace = a.client.namespace
	}
	var result FuzzingConfig
	if err := a.client.do(ctx, "POST", "/ui/api/fuzzing/config", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConfig retrieves a fuzzing configuration by ID.
func (a *FuzzingAPI) GetConfig(ctx context.Context, id string) (*FuzzingConfig, error) {
	var config FuzzingConfig
	if err := a.client.do(ctx, "GET", "/ui/api/fuzzing/config/"+url.PathEscape(id), nil, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
