// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
)

// HealthAPI provides health-check methods.
type HealthAPI struct {
	client *Client
}

// HealthStatus represents the overall status of a health check.
type HealthStatus string

const (
	HealthStatusPass HealthStatus = "pass"
	HealthStatusFail HealthStatus = "fail"
)

// HealthCheckDetail represents the result of a single health check.
type HealthCheckDetail struct {
	Status HealthStatus `json:"status"`
	Output string       `json:"output,omitempty"`
	Time   string       `json:"time"`
}

// HealthResponse represents the full health check response from the server.
type HealthResponse struct {
	Status    HealthStatus                   `json:"status"`
	ReleaseID string                         `json:"releaseId,omitempty"`
	Errors    map[string]string              `json:"errors,omitempty"`
	Checks    map[string][]HealthCheckDetail `json:"checks"`
	Output    string                         `json:"output,omitempty"`
}

// Check performs a comprehensive health check.
func (a *HealthAPI) Check(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	if err := a.client.do(ctx, "GET", "/health", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Live performs a liveness probe. Returns nil if the server is alive.
func (a *HealthAPI) Live(ctx context.Context) error {
	resp, err := a.client.doJSON(ctx, "GET", "/health/live", nil)
	if err != nil {
		return err
	}
	_ = resp
	return nil
}

// Ready performs a readiness probe. Returns nil if the server is ready
// to accept traffic. Returns an error if the server is not ready.
func (a *HealthAPI) Ready(ctx context.Context) error {
	resp, err := a.client.doJSON(ctx, "GET", "/health/ready", nil)
	if err != nil {
		return fmt.Errorf("mockarty: server not ready: %w", err)
	}
	_ = resp
	return nil
}
