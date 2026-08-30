// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// CloudInstancesAPI manages dedicated Mockarty Cloud contours. Provisioning
// operations are asynchronous; callers should poll Get until the projected
// status is terminal.
type CloudInstancesAPI struct{ client *Client }

type CloudInstance struct {
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	Plan        string     `json:"plan"`
	Status      string     `json:"status"`
	Provider    string     `json:"provider"`
	ExternalRef string     `json:"external_ref,omitempty"`
	ConnectURL  string     `json:"connect_url,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	Features    []string   `json:"features"`
}

type CloudInstanceCapabilities struct {
	Reason                string `json:"reason,omitempty"`
	ProvisioningAvailable bool   `json:"provisioning_available"`
	RuntimeReal           bool   `json:"runtime_real"`
}

type CloudInstanceBootstrap struct {
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Available bool   `json:"available"`
	OneTime   bool   `json:"one_time"`
}

type CloudInstanceCreateResult struct {
	Instance  *CloudInstance          `json:"instance"`
	Bootstrap *CloudInstanceBootstrap `json:"bootstrap,omitempty"`
	RequestID string                  `json:"request_id"`
}

type CloudInstancesPage struct {
	Instances    []CloudInstance           `json:"instances"`
	Capabilities CloudInstanceCapabilities `json:"capabilities"`
	Workspace    string                    `json:"workspace,omitempty"`
	RequestID    string                    `json:"request_id"`
	Total        int                       `json:"total"`
}

func (a *CloudInstancesAPI) context(ctx context.Context, idempotencyKey string) context.Context {
	headers := map[string]string{headerAPIKey: ""}
	if a.client.apiKey != "" {
		headers["Authorization"] = "Bearer " + a.client.apiKey
	}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	return withRequestHeaders(ctx, headers)
}

func (a *CloudInstancesAPI) List(ctx context.Context, workspaceID string) (*CloudInstancesPage, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("mockarty: workspace id is required")
	}
	var out CloudInstancesPage
	err := a.client.do(a.context(ctx, ""), "GET", "/api/v1/cloud/instances?workspace_id="+url.QueryEscape(workspaceID), nil, &out)
	return &out, err
}

func (a *CloudInstancesAPI) Get(ctx context.Context, instanceID string) (*CloudInstance, error) {
	if instanceID == "" {
		return nil, fmt.Errorf("mockarty: instance id is required")
	}
	var out struct {
		Instance *CloudInstance `json:"instance"`
	}
	err := a.client.do(a.context(ctx, ""), "GET", "/api/v1/cloud/instances/"+url.PathEscape(instanceID), nil, &out)
	return out.Instance, err
}

// Create returns a bootstrap administrator password only when the server first
// admits this idempotency key. Persist it immediately and never log it.
func (a *CloudInstancesAPI) Create(ctx context.Context, workspaceID, name, idempotencyKey string) (*CloudInstanceCreateResult, error) {
	if workspaceID == "" || name == "" || idempotencyKey == "" {
		return nil, fmt.Errorf("mockarty: workspace id, name, and idempotency key are required")
	}
	var out CloudInstanceCreateResult
	err := a.client.do(a.context(ctx, idempotencyKey), "POST", "/api/v1/cloud/instances", map[string]any{
		"workspace_id": workspaceID, "name": name,
	}, &out)
	return &out, err
}

func (a *CloudInstancesAPI) Delete(ctx context.Context, instanceID, idempotencyKey string) error {
	return a.mutate(ctx, "DELETE", instanceID, "", idempotencyKey)
}

func (a *CloudInstancesAPI) Start(ctx context.Context, instanceID, idempotencyKey string) error {
	return a.mutate(ctx, "POST", instanceID, "start", idempotencyKey)
}

func (a *CloudInstancesAPI) Stop(ctx context.Context, instanceID, idempotencyKey string) error {
	return a.mutate(ctx, "POST", instanceID, "stop", idempotencyKey)
}

func (a *CloudInstancesAPI) mutate(ctx context.Context, method, instanceID, action, idempotencyKey string) error {
	if instanceID == "" || idempotencyKey == "" {
		return fmt.Errorf("mockarty: instance id and idempotency key are required")
	}
	path := "/api/v1/cloud/instances/" + url.PathEscape(instanceID)
	if action != "" {
		path += "/" + action
	}
	return a.client.do(a.context(ctx, idempotencyKey), method, path, nil, nil)
}
