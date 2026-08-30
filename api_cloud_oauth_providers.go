// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
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
	ConfigRevision   int64     `json:"config_revision"`
	Enabled          bool      `json:"enabled"`
	SecretConfigured bool      `json:"secret_configured"`
}

type CloudOAuthProviderUpdate struct {
	ClientID         string `json:"client_id"`
	ClientSecretRef  string `json:"client_secret_ref,omitempty"`
	ExpectedRevision int64  `json:"expected_revision"`
	Enabled          bool   `json:"enabled"`
}

func (a *CloudOAuthProvidersAPI) List(ctx context.Context) ([]CloudOAuthProvider, error) {
	var out struct {
		Providers []CloudOAuthProvider `json:"providers"`
	}
	err := a.client.do(ctx, "GET", "/api/v1/cloud/operator/oauth/providers", nil, &out)
	return out.Providers, err
}

func (a *CloudOAuthProvidersAPI) Update(ctx context.Context, provider string, update CloudOAuthProviderUpdate, idempotencyKey string) (*CloudOAuthProvider, error) {
	if provider == "" || update.ClientID == "" || idempotencyKey == "" || update.ExpectedRevision < 0 {
		return nil, fmt.Errorf("mockarty: provider, client id, non-negative revision, and idempotency key are required")
	}
	headers := map[string]string{"Idempotency-Key": idempotencyKey}
	ctx = withRequestHeaders(ctx, headers)
	var out CloudOAuthProvider
	err := a.client.do(ctx, "PUT", "/api/v1/cloud/operator/oauth/providers/"+url.PathEscape(provider), update, &out)
	return &out, err
}
