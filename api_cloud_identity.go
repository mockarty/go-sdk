// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// CloudIdentityAPI manages the current Cloud user's external sign-in methods.
// Use a Cloud session token, not an API token. The default Go client keeps the
// short-lived HttpOnly step-up cookie between StepUp and Unlink calls.
type CloudIdentityAPI struct{ client *Client }

type CloudOAuthIdentity struct {
	CreatedAt time.Time `json:"created_at,omitempty"`
	Provider  string    `json:"provider"`
}

type CloudStepUpRequest struct {
	Action          string `json:"action"`
	Credential      string `json:"credential,omitempty"`
	ForceCredential bool   `json:"force_credential,omitempty"`
}

type CloudStepUpResult struct {
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
	Action    string    `json:"action"`
}

func (a *CloudIdentityAPI) List(ctx context.Context) ([]CloudOAuthIdentity, error) {
	var out struct {
		Identities []CloudOAuthIdentity `json:"identities"`
	}
	err := a.client.do(a.client.cloudContext(ctx), "GET", "/api/v1/cloud/auth/oauth/identities", nil, &out)
	return out.Identities, err
}

func (a *CloudIdentityAPI) StepUp(ctx context.Context, request CloudStepUpRequest) (*CloudStepUpResult, error) {
	if request.Action == "" {
		return nil, fmt.Errorf("mockarty: step-up action is required")
	}
	var out CloudStepUpResult
	err := a.client.do(a.client.cloudContext(ctx), "POST", "/api/v1/cloud/auth/step-up", request, &out)
	return &out, err
}

func (a *CloudIdentityAPI) Unlink(ctx context.Context, provider, idempotencyKey string) error {
	if provider == "" || idempotencyKey == "" {
		return fmt.Errorf("mockarty: provider and idempotency key are required")
	}
	ctx = withRequestHeaders(ctx, map[string]string{
		headerAPIKey: "", "Authorization": "Bearer " + a.client.apiKey, "Idempotency-Key": idempotencyKey,
	})
	return a.client.do(ctx, "DELETE", "/api/v1/cloud/auth/oauth/identities/"+url.PathEscape(provider), nil, nil)
}

// LinkURL returns the authenticated redirect endpoint to open after a
// successful oauth_identity_link step-up. It does not perform the redirect.
func (a *CloudIdentityAPI) LinkURL(provider string) (string, error) {
	if provider == "" {
		return "", fmt.Errorf("mockarty: provider is required")
	}
	return a.client.baseURL + "/api/v1/cloud/auth/oauth/" + url.PathEscape(provider) + "/link", nil
}
