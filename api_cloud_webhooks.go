// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type CloudWebhooksAPI struct{ client *Client }

type CloudWebhook struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Events        []string  `json:"events"`
	SigningStatus string    `json:"signing_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Active        bool      `json:"active"`
	SigningReady  bool      `json:"signing_ready"`
}

type CloudWebhookDelivery struct {
	ID            string     `json:"id"`
	WebhookID     string     `json:"webhook_id"`
	WorkspaceID   string     `json:"workspace_id"`
	Event         string     `json:"event"`
	Status        string     `json:"status"`
	ResponseBody  string     `json:"response_body,omitempty"`
	LastAttemptAt time.Time  `json:"last_attempt_at"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	Attempt       int        `json:"attempt"`
	StatusCode    *int       `json:"status_code,omitempty"`
}

type CloudWebhookCredential struct {
	Webhook *CloudWebhook `json:"webhook"`
	Secret  string        `json:"secret"`
}

func (a *CloudWebhooksAPI) cloudContext(ctx context.Context, idempotencyKey string) context.Context {
	headers := map[string]string{headerAPIKey: ""}
	if a.client.apiKey != "" {
		headers["Authorization"] = "Bearer " + a.client.apiKey
	}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	return withRequestHeaders(ctx, headers)
}

func cloudWorkspaceQuery(workspaceID string) string {
	return "workspace_id=" + url.QueryEscape(workspaceID)
}

func (a *CloudWebhooksAPI) List(ctx context.Context, workspaceID string) ([]CloudWebhook, error) {
	var out struct {
		Webhooks []CloudWebhook `json:"webhooks"`
	}
	err := a.client.do(a.cloudContext(ctx, ""), "GET", "/api/v1/cloud/webhooks?"+cloudWorkspaceQuery(workspaceID), nil, &out)
	return out.Webhooks, err
}

func (a *CloudWebhooksAPI) Create(ctx context.Context, workspaceID, name, targetURL string, events []string) (*CloudWebhookCredential, error) {
	var out CloudWebhookCredential
	err := a.client.do(a.cloudContext(ctx, ""), "POST", "/api/v1/cloud/webhooks?"+cloudWorkspaceQuery(workspaceID), map[string]any{
		"name": name, "url": targetURL, "events": events,
	}, &out)
	return &out, err
}

func (a *CloudWebhooksAPI) Deactivate(ctx context.Context, workspaceID, webhookID string) error {
	if webhookID == "" {
		return fmt.Errorf("mockarty: webhook id is required")
	}
	return a.client.do(a.cloudContext(ctx, ""), "DELETE", "/api/v1/cloud/webhooks/"+url.PathEscape(webhookID)+"?"+cloudWorkspaceQuery(workspaceID), nil, nil)
}

func (a *CloudWebhooksAPI) Test(ctx context.Context, workspaceID, webhookID string) error {
	if webhookID == "" {
		return fmt.Errorf("mockarty: webhook id is required")
	}
	return a.client.do(a.cloudContext(ctx, ""), "POST", "/api/v1/cloud/webhooks/"+url.PathEscape(webhookID)+"/test?"+cloudWorkspaceQuery(workspaceID), map[string]any{}, nil)
}

func (a *CloudWebhooksAPI) ListDeliveries(ctx context.Context, workspaceID, webhookID string, limit int) ([]CloudWebhookDelivery, error) {
	if webhookID == "" {
		return nil, fmt.Errorf("mockarty: webhook id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var out struct {
		Deliveries []CloudWebhookDelivery `json:"deliveries"`
	}
	path := "/api/v1/cloud/webhooks/" + url.PathEscape(webhookID) + "/deliveries?" + cloudWorkspaceQuery(workspaceID) + "&limit=" + strconv.Itoa(limit)
	err := a.client.do(a.cloudContext(ctx, ""), "GET", path, nil, &out)
	return out.Deliveries, err
}

func (a *CloudWebhooksAPI) RotateSecret(ctx context.Context, workspaceID, webhookID, idempotencyKey string) (*CloudWebhookCredential, error) {
	if webhookID == "" || idempotencyKey == "" {
		return nil, fmt.Errorf("mockarty: webhook id and idempotency key are required")
	}
	var out CloudWebhookCredential
	path := "/api/v1/cloud/webhooks/" + url.PathEscape(webhookID) + "/rotate-secret?" + cloudWorkspaceQuery(workspaceID)
	err := a.client.do(a.cloudContext(ctx, idempotencyKey), "POST", path, map[string]any{}, &out)
	return &out, err
}
