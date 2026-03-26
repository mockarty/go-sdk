// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// EnvironmentAPI provides methods for managing API Tester environments.
type EnvironmentAPI struct {
	client *Client
}

// Environment represents an API Tester environment with variables.
type Environment struct {
	ID        string            `json:"id,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Name      string            `json:"name"`
	Variables map[string]string `json:"variables,omitempty"`
	IsActive  bool              `json:"isActive,omitempty"`
}

// List returns all environments.
func (a *EnvironmentAPI) List(ctx context.Context) ([]Environment, error) {
	var envs []Environment
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/environments", nil, &envs); err != nil {
		return nil, err
	}
	return envs, nil
}

// GetActive returns the currently active environment.
func (a *EnvironmentAPI) GetActive(ctx context.Context) (*Environment, error) {
	var env Environment
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/environments/active", nil, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// Get retrieves an environment by ID.
func (a *EnvironmentAPI) Get(ctx context.Context, id string) (*Environment, error) {
	var env Environment
	if err := a.client.do(ctx, "GET", "/api/v1/api-tester/environments/"+url.PathEscape(id), nil, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// Create creates a new environment.
func (a *EnvironmentAPI) Create(ctx context.Context, env *Environment) (*Environment, error) {
	if env.Namespace == "" && a.client.namespace != "" {
		env.Namespace = a.client.namespace
	}
	var result Environment
	if err := a.client.do(ctx, "POST", "/api/v1/api-tester/environments", env, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update updates an existing environment by ID.
func (a *EnvironmentAPI) Update(ctx context.Context, id string, env *Environment) (*Environment, error) {
	var result Environment
	if err := a.client.do(ctx, "PUT", "/api/v1/api-tester/environments/"+url.PathEscape(id), env, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes an environment by ID.
func (a *EnvironmentAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/api-tester/environments/"+url.PathEscape(id), nil, nil)
}

// Activate sets an environment as the active one.
func (a *EnvironmentAPI) Activate(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/api/v1/api-tester/environments/"+url.PathEscape(id)+"/activate", nil, nil)
}
