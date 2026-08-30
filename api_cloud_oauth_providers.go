// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// CloudOAuthProvidersAPI manages the global Cloud cabinet sign-in providers.
// It requires a Cloud operator session/token. Secret references are accepted
// on write but intentionally never returned by the service.
type CloudOAuthProvidersAPI struct{ client *Client }

type CloudOAuthProvider struct {
	UpdatedAt        time.Time `json:"updated_at"`
	Provider         string    `json:"provider"`
	ClientID         string    `json:"client_id"`
	Source           string    `json:"source"`
	VersionID        string    `json:"version_id"`
	ConfigRevision   int64     `json:"config_revision"`
	Enabled          bool      `json:"enabled"`
	SecretConfigured bool      `json:"secret_configured"`
}

type CloudOAuthProviderUpdate struct {
	ClientID         string `json:"client_id"`
	ClientSecret     string `json:"client_secret,omitempty"`
	ClientSecretRef  string `json:"-"` // Deprecated: use ClientSecret or CloudConnectorsAPI.
	ExpectedRevision int64  `json:"expected_revision"`
	Enabled          bool   `json:"enabled"`
	ClearSecret      bool   `json:"clear_secret,omitempty"`
}

func (update CloudOAuthProviderUpdate) request() (CloudOAuthProviderUpdate, error) {
	if update.ClientSecret == "" && update.ClientSecretRef != "" {
		const prefix = "env://"
		if !strings.HasPrefix(update.ClientSecretRef, prefix) || len(update.ClientSecretRef) == len(prefix) {
			return CloudOAuthProviderUpdate{}, fmt.Errorf("mockarty: client secret reference must use env://NAME")
		}
		value, exists := os.LookupEnv(strings.TrimPrefix(update.ClientSecretRef, prefix))
		if !exists || value == "" {
			return CloudOAuthProviderUpdate{}, fmt.Errorf("mockarty: referenced client secret environment variable is empty or unset")
		}
		update.ClientSecret = value
	}
	update.ClientSecretRef = ""
	return update, nil
}

func (a *CloudOAuthProvidersAPI) List(ctx context.Context) ([]CloudOAuthProvider, error) {
	var out struct {
		Providers []CloudOAuthProvider `json:"providers"`
	}
	err := a.client.do(ctx, "GET", "/api/v1/cloud/operator/oauth/providers", nil, &out)
	return out.Providers, err
}

func (a *CloudOAuthProvidersAPI) Update(ctx context.Context, provider string, update CloudOAuthProviderUpdate, idempotencyKey string) (*CloudOAuthProvider, error) {
	if provider == "" || update.ClientID == "" || idempotencyKey == "" || update.ExpectedRevision < 1 || update.ClearSecret && update.ClientSecret != "" {
		return nil, fmt.Errorf("mockarty: provider, client id, positive revision, and idempotency key are required")
	}
	request, err := update.request()
	if err != nil {
		return nil, err
	}
	if request.ClearSecret && request.ClientSecret != "" {
		return nil, fmt.Errorf("mockarty: client secret and clear secret are mutually exclusive")
	}
	headers := map[string]string{"Idempotency-Key": idempotencyKey}
	ctx = withRequestHeaders(ctx, headers)
	var out CloudOAuthProvider
	err = a.client.do(ctx, "PUT", "/api/v1/cloud/operator/oauth/providers/"+url.PathEscape(provider), request, &out)
	return &out, err
}
