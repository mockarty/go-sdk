// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// NamespaceSettingsAPI provides methods for managing namespace-level settings.
type NamespaceSettingsAPI struct {
	client *Client
}

// NamespaceWebhook represents a namespace-scoped webhook configuration.
type NamespaceWebhook struct {
	ID        string            `json:"id,omitempty"`
	Operation string            `json:"operation,omitempty"`
	URL       string            `json:"url"`
	Method    string            `json:"method,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Enabled   bool              `json:"enabled"`
}

// ListUsers returns all users in a namespace.
func (a *NamespaceSettingsAPI) ListUsers(ctx context.Context, ns string) ([]NamespaceUser, error) {
	var users []NamespaceUser
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/users", nil, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// AddUser adds a user to a namespace.
func (a *NamespaceSettingsAPI) AddUser(ctx context.Context, ns string, req *AddNamespaceUserRequest) error {
	return a.client.do(ctx, "POST", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/users", req, nil)
}

// RemoveUser removes a user from a namespace.
func (a *NamespaceSettingsAPI) RemoveUser(ctx context.Context, ns string, userID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/users/"+url.PathEscape(userID), nil, nil)
}

// UpdateUserRole updates a user's role within a namespace.
func (a *NamespaceSettingsAPI) UpdateUserRole(ctx context.Context, ns string, userID string, role string) error {
	body := struct {
		Role string `json:"role"`
	}{Role: role}
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/users/"+url.PathEscape(userID)+"/role", body, nil)
}

// GetCleanupPolicy retrieves the cleanup policy for a namespace.
func (a *NamespaceSettingsAPI) GetCleanupPolicy(ctx context.Context, ns string) (*CleanupPolicy, error) {
	var policy CleanupPolicy
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/cleanup-policy", nil, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpdateCleanupPolicy updates the cleanup policy for a namespace.
func (a *NamespaceSettingsAPI) UpdateCleanupPolicy(ctx context.Context, ns string, policy *CleanupPolicy) error {
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/cleanup-policy", policy, nil)
}

// ListWebhooks returns all webhooks configured for a namespace.
func (a *NamespaceSettingsAPI) ListWebhooks(ctx context.Context, ns string) ([]NamespaceWebhook, error) {
	var webhooks []NamespaceWebhook
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/webhooks", nil, &webhooks); err != nil {
		return nil, err
	}
	return webhooks, nil
}

// CreateWebhook creates a new webhook for a namespace.
func (a *NamespaceSettingsAPI) CreateWebhook(ctx context.Context, ns string, webhook *NamespaceWebhook) (*NamespaceWebhook, error) {
	var result NamespaceWebhook
	if err := a.client.do(ctx, "POST", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/webhooks", webhook, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteWebhook deletes a webhook from a namespace.
func (a *NamespaceSettingsAPI) DeleteWebhook(ctx context.Context, ns string, webhookID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/namespaces/"+url.PathEscape(ns)+"/settings/webhooks/"+url.PathEscape(webhookID), nil, nil)
}
