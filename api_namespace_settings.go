// Copyright (c) 2026 Mockarty. All rights reserved.
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
//
// Wire shape: server emits `{users: [...], total: N}` — NOT a bare list.
// The older SDK build decoded into `[]NamespaceUser` directly and every
// call failed with 'cannot unmarshal object into Go value of type
// []mockarty.NamespaceUser'. We now unwrap the envelope.
func (a *NamespaceSettingsAPI) ListUsers(ctx context.Context, ns string) ([]NamespaceUser, error) {
	var env struct {
		Users []NamespaceUser `json:"users"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/users", nil, &env); err != nil {
		return nil, err
	}
	if env.Users == nil {
		return []NamespaceUser{}, nil
	}
	return env.Users, nil
}

// AddUser adds a user to a namespace.
func (a *NamespaceSettingsAPI) AddUser(ctx context.Context, ns string, req *AddNamespaceUserRequest) error {
	return a.client.do(ctx, "POST", "/api/v1/namespaces/"+url.PathEscape(ns)+"/users", req, nil)
}

// RemoveUser removes a user from a namespace.
func (a *NamespaceSettingsAPI) RemoveUser(ctx context.Context, ns string, userID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/namespaces/"+url.PathEscape(ns)+"/users/"+url.PathEscape(userID), nil, nil)
}

// UpdateUserRole updates a user's role within a namespace.
func (a *NamespaceSettingsAPI) UpdateUserRole(ctx context.Context, ns string, userID string, role string) error {
	body := struct {
		Role string `json:"role"`
	}{Role: role}
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(ns)+"/users/"+url.PathEscape(userID)+"/role", body, nil)
}

// GetCleanupPolicy retrieves the cleanup policy for a namespace.
func (a *NamespaceSettingsAPI) GetCleanupPolicy(ctx context.Context, ns string) (*CleanupPolicy, error) {
	var policy CleanupPolicy
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/cleanup-policy", nil, &policy); err != nil {
		return nil, err
	}
	return &policy, nil
}

// UpdateCleanupPolicy updates the cleanup policy for a namespace.
func (a *NamespaceSettingsAPI) UpdateCleanupPolicy(ctx context.Context, ns string, policy *CleanupPolicy) error {
	return a.client.do(ctx, "PUT", "/api/v1/namespaces/"+url.PathEscape(ns)+"/cleanup-policy", policy, nil)
}

// ListWebhooks returns all webhooks configured for a namespace.
//
// Wire shape: server emits `{webhooks: [...]}` — unwrap.
func (a *NamespaceSettingsAPI) ListWebhooks(ctx context.Context, ns string) ([]NamespaceWebhook, error) {
	var env struct {
		Webhooks []NamespaceWebhook `json:"webhooks"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/namespaces/"+url.PathEscape(ns)+"/webhooks", nil, &env); err != nil {
		return nil, err
	}
	if env.Webhooks == nil {
		return []NamespaceWebhook{}, nil
	}
	return env.Webhooks, nil
}

// CreateWebhook creates a new webhook for a namespace.
//
// Wire shape: server replies with `{message: "...", webhook: {...}}` —
// unwrap the inner webhook.
func (a *NamespaceSettingsAPI) CreateWebhook(ctx context.Context, ns string, webhook *NamespaceWebhook) (*NamespaceWebhook, error) {
	var env struct {
		Webhook NamespaceWebhook `json:"webhook"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/namespaces/"+url.PathEscape(ns)+"/webhooks", webhook, &env); err != nil {
		return nil, err
	}
	return &env.Webhook, nil
}

// DeleteWebhook deletes a webhook from a namespace.
func (a *NamespaceSettingsAPI) DeleteWebhook(ctx context.Context, ns string, webhookID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/namespaces/"+url.PathEscape(ns)+"/webhooks/"+url.PathEscape(webhookID), nil, nil)
}
