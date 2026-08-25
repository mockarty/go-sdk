// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"

	"github.com/google/uuid"
)

// NamespaceSettingsAPI provides methods for managing namespace-level settings.
type NamespaceSettingsAPI struct {
	client *Client
}

// AutonomyNamespaceSettings is the user-manageable namespace policy for new
// autonomous missions and retained mission evidence. A nil run-window pointer
// inherits the lower policy layer; nil retention pointers inherit the instance
// policy. Run windows are 1..20160 minutes; retention is 1..3650 days.
type AutonomyNamespaceSettings struct {
	DefaultBudget               AutonomyDefaultBudget `json:"defaultBudget"`
	DefaultContextRefs          []AutonomyContextRef  `json:"defaultContextRefs"`
	UpdatedAt                   string                `json:"updatedAt,omitempty"`
	DefaultAutonomy             string                `json:"defaultAutonomy"`
	JournalEventRetentionDays   *int                  `json:"journalEventRetentionDays,omitempty"`
	JournalPayloadRetentionDays *int                  `json:"journalPayloadRetentionDays,omitempty"`
	RunWindowMinutes            *int                  `json:"runWindowMinutes,omitempty"`
	ETag                        string                `json:"etag,omitempty"`
}

type AutonomyDefaultBudget struct {
	TokensTotal  int64   `json:"tokensTotal"`
	TokensPerDay int64   `json:"tokensPerDay"`
	USDCap       float64 `json:"usdCap"`
}

type AutonomyContextRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// AutonomySettingsSaveOptions controls retry identity and presence-sensitive
// fields without breaking the ergonomic partial-update method. Reuse RequestID
// after a timeout or connection loss. ReplaceDefaultBudget makes an all-zero
// budget an explicit update instead of the legacy "omitted" signal.
type AutonomySettingsSaveOptions struct {
	RequestID            string
	ReplaceDefaultBudget bool
}

// GetAutonomySettings returns the autonomous-mission defaults for the client's
// configured namespace.
func (a *NamespaceSettingsAPI) GetAutonomySettings(ctx context.Context) (*AutonomyNamespaceSettings, error) {
	var out AutonomyNamespaceSettings
	if err := a.client.do(ctx, "GET", "/api/v1/autotester/settings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SaveAutonomySettings replaces autonomous-mission defaults for the client's
// namespace. Nil run-window and retention pointers are omitted and therefore
// preserve current overrides. Use ClearAutonomyRunWindow or
// ClearAutonomyRetention for explicit inheritance.
// Legal holds always take precedence over retention cleanup.
func (a *NamespaceSettingsAPI) SaveAutonomySettings(ctx context.Context, settings *AutonomyNamespaceSettings) (*AutonomyNamespaceSettings, error) {
	return a.SaveAutonomySettingsWithOptions(ctx, settings, AutonomySettingsSaveOptions{})
}

// SaveAutonomySettingsWithOptions is the retry-safe, presence-aware variant of
// SaveAutonomySettings. An empty RequestID generates a new UUID for this call.
func (a *NamespaceSettingsAPI) SaveAutonomySettingsWithOptions(ctx context.Context, settings *AutonomyNamespaceSettings, options AutonomySettingsSaveOptions) (*AutonomyNamespaceSettings, error) {
	if settings == nil {
		return nil, fmt.Errorf("autonomy settings are required")
	}
	current, err := a.GetAutonomySettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings.DefaultAutonomy != "" {
		current.DefaultAutonomy = settings.DefaultAutonomy
	}
	if options.ReplaceDefaultBudget || settings.DefaultBudget != (AutonomyDefaultBudget{}) {
		current.DefaultBudget = settings.DefaultBudget
	}
	if settings.DefaultContextRefs != nil {
		current.DefaultContextRefs = settings.DefaultContextRefs
	}
	if settings.JournalEventRetentionDays != nil {
		current.JournalEventRetentionDays = settings.JournalEventRetentionDays
	}
	if settings.JournalPayloadRetentionDays != nil {
		current.JournalPayloadRetentionDays = settings.JournalPayloadRetentionDays
	}
	if settings.RunWindowMinutes != nil {
		current.RunWindowMinutes = settings.RunWindowMinutes
	}
	var out AutonomyNamespaceSettings
	ctx = withRequestHeaders(ctx, map[string]string{"Idempotency-Key": autonomyRequestID(options.RequestID), "If-Match": current.ETag})
	if err := a.client.do(ctx, "PUT", "/api/v1/autotester/settings", current, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearAutonomyRunWindow clears the namespace override so the product,
// instance or built-in 480-minute wall applies.
func (a *NamespaceSettingsAPI) ClearAutonomyRunWindow(ctx context.Context, options AutonomySettingsSaveOptions) (*AutonomyNamespaceSettings, error) {
	current, err := a.GetAutonomySettings(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"defaultAutonomy": current.DefaultAutonomy, "defaultBudget": current.DefaultBudget,
		"defaultContextRefs": current.DefaultContextRefs, "runWindowMinutes": nil,
	}
	if current.JournalEventRetentionDays != nil {
		body["journalEventRetentionDays"] = *current.JournalEventRetentionDays
	}
	if current.JournalPayloadRetentionDays != nil {
		body["journalPayloadRetentionDays"] = *current.JournalPayloadRetentionDays
	}
	var out AutonomyNamespaceSettings
	ctx = withRequestHeaders(ctx, map[string]string{"Idempotency-Key": autonomyRequestID(options.RequestID), "If-Match": current.ETag})
	if err = a.client.do(ctx, "PUT", "/api/v1/autotester/settings", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ClearAutonomyRetention explicitly clears selected namespace overrides so
// they inherit instance retention. Unselected overrides are preserved.
func (a *NamespaceSettingsAPI) ClearAutonomyRetention(ctx context.Context, clearEvent, clearPayload bool) (*AutonomyNamespaceSettings, error) {
	return a.ClearAutonomyRetentionWithOptions(ctx, clearEvent, clearPayload, AutonomySettingsSaveOptions{})
}

// ClearAutonomyRetentionWithOptions supports a stable retry identity after an
// ambiguous network outcome. ReplaceDefaultBudget is ignored for this helper.
func (a *NamespaceSettingsAPI) ClearAutonomyRetentionWithOptions(ctx context.Context, clearEvent, clearPayload bool, options AutonomySettingsSaveOptions) (*AutonomyNamespaceSettings, error) {
	if !clearEvent && !clearPayload {
		return nil, fmt.Errorf("at least one retention override must be selected")
	}
	current, err := a.GetAutonomySettings(ctx)
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"defaultAutonomy":    current.DefaultAutonomy,
		"defaultBudget":      current.DefaultBudget,
		"defaultContextRefs": current.DefaultContextRefs,
	}
	if clearEvent {
		body["journalEventRetentionDays"] = nil
	} else if current.JournalEventRetentionDays != nil {
		body["journalEventRetentionDays"] = *current.JournalEventRetentionDays
	}
	if clearPayload {
		body["journalPayloadRetentionDays"] = nil
	} else if current.JournalPayloadRetentionDays != nil {
		body["journalPayloadRetentionDays"] = *current.JournalPayloadRetentionDays
	}
	var out AutonomyNamespaceSettings
	ctx = withRequestHeaders(ctx, map[string]string{"Idempotency-Key": autonomyRequestID(options.RequestID), "If-Match": current.ETag})
	if err := a.client.do(ctx, "PUT", "/api/v1/autotester/settings", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func autonomyRequestID(value string) string {
	if value != "" {
		return value
	}
	return uuid.NewString()
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
