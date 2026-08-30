// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CloudConnectorsAPI manages operator-owned SMTP, OAuth and payment connector
// versions. Secret values are write-only and never appear in response models.
type CloudConnectorsAPI struct{ client *Client }

type CloudConnector struct {
	UpdatedAt        time.Time         `json:"updated_at"`
	LastTestedAt     *time.Time        `json:"last_tested_at,omitempty"`
	Config           map[string]string `json:"config"`
	SecretFields     []string          `json:"secret_fields"`
	ID               string            `json:"id"`
	VersionID        string            `json:"version_id"`
	Key              string            `json:"key"`
	Kind             string            `json:"kind"`
	Provider         string            `json:"provider"`
	DisplayName      string            `json:"display_name"`
	LastTestStatus   string            `json:"last_test_status"`
	LastTestCode     string            `json:"last_test_code,omitempty"`
	Revision         int64             `json:"revision"`
	Enabled          bool              `json:"enabled"`
	Default          bool              `json:"default"`
	SecretConfigured bool              `json:"secret_configured"`
	SecretRevoked    bool              `json:"secret_revoked"`
}

type CloudConnectorUpdate struct {
	Config           map[string]string `json:"config"`
	Secrets          map[string]string `json:"secrets,omitempty"`
	ClearSecrets     []string          `json:"clear_secrets,omitempty"`
	ExpectedRevision int64             `json:"expected_revision"`
	Enabled          bool              `json:"enabled"`
	Default          bool              `json:"default"`
}

type CloudConnectorTestResult struct {
	AttemptID string `json:"attempt_id"`
	Status    string `json:"status"`
	Code      string `json:"code"`
}

func cloudConnectorPath(kind, provider, slot string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	provider = strings.ToLower(strings.TrimSpace(provider))
	slot = strings.ToLower(strings.TrimSpace(slot))
	valid := kind == "smtp" && provider == "smtp" && slot == "" ||
		kind == "oauth" && (provider == "yandex" || provider == "vk" || provider == "github") && slot == "" ||
		kind == "payment" && (provider == "yookassa" || provider == "stripe") && slot == "main"
	if !valid {
		return "", fmt.Errorf("mockarty: unsupported Cloud connector key")
	}
	path := "/api/v1/cloud/operator/connectors/" + url.PathEscape(kind) + "/" + url.PathEscape(provider)
	if slot != "" {
		path += "/" + url.PathEscape(slot)
	}
	return path, nil
}

func (a *CloudConnectorsAPI) List(ctx context.Context) ([]CloudConnector, error) {
	var out struct {
		Connectors []CloudConnector `json:"connectors"`
	}
	err := a.client.do(a.client.cloudContext(ctx), "GET", "/api/v1/cloud/operator/connectors", nil, &out)
	return out.Connectors, err
}

func (a *CloudConnectorsAPI) Update(ctx context.Context, kind, provider, slot string, update CloudConnectorUpdate, idempotencyKey string) (*CloudConnector, error) {
	path, err := cloudConnectorPath(kind, provider, slot)
	if err != nil {
		return nil, err
	}
	if update.Config == nil || update.ExpectedRevision < 1 || strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: config, positive expected revision, and idempotency key are required")
	}
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": idempotencyKey})
	var out CloudConnector
	err = a.client.do(ctx, "PUT", path, update, &out)
	return &out, err
}

func (a *CloudConnectorsAPI) Test(ctx context.Context, kind, provider, slot, idempotencyKey string) (*CloudConnectorTestResult, error) {
	path, err := cloudConnectorPath(kind, provider, slot)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": idempotencyKey})
	var out CloudConnectorTestResult
	err = a.client.do(ctx, "POST", path+"/test", map[string]any{}, &out)
	return &out, err
}

func (a *CloudConnectorsAPI) Revoke(ctx context.Context, versionID, idempotencyKey string) error {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" || strings.TrimSpace(idempotencyKey) == "" {
		return fmt.Errorf("mockarty: version id and idempotency key are required")
	}
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": idempotencyKey})
	return a.client.do(ctx, "POST", "/api/v1/cloud/operator/connector-versions/"+url.PathEscape(versionID)+"/revoke", map[string]any{}, nil)
}
