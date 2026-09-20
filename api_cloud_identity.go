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

// CloudEmailChangeResult reports which mailbox must be confirmed next. An
// account's existing sign-in address remains valid until the final proof.
type CloudEmailChangeResult struct {
	Status    string `json:"status"`
	Stage     string `json:"stage,omitempty"`
	NewEmail  string `json:"new_email,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type CloudEmailChangeRequest struct {
	NewEmail        string `json:"new_email"`
	CurrentPassword string `json:"current_password,omitempty"`
	TOTPCode        string `json:"totp_code,omitempty"`
}

func (a *CloudIdentityAPI) RequestEmailChange(ctx context.Context, request CloudEmailChangeRequest) (*CloudEmailChangeResult, error) {
	if request.NewEmail == "" {
		return nil, fmt.Errorf("mockarty: new email is required")
	}
	var out CloudEmailChangeResult
	err := a.client.do(a.client.cloudContext(ctx), "PATCH", "/api/v1/cloud/email", request, &out)
	return &out, err
}

func (a *CloudIdentityAPI) AuthorizeCurrentEmail(ctx context.Context, token string) (*CloudEmailChangeResult, error) {
	return a.consumeEmailChangeProof(ctx, "/api/v1/cloud/auth/email/change/authorize", token)
}

func (a *CloudIdentityAPI) ConfirmNewEmail(ctx context.Context, token string) (*CloudEmailChangeResult, error) {
	return a.consumeEmailChangeProof(ctx, "/api/v1/cloud/auth/email/change/confirm", token)
}

func (a *CloudIdentityAPI) consumeEmailChangeProof(ctx context.Context, path, token string) (*CloudEmailChangeResult, error) {
	if token == "" {
		return nil, fmt.Errorf("mockarty: email confirmation token is required")
	}
	var out CloudEmailChangeResult
	err := a.client.do(a.client.cloudContext(ctx), "POST", path, map[string]string{"token": token}, &out)
	return &out, err
}

func (a *CloudIdentityAPI) CancelEmailChange(ctx context.Context) error {
	return a.client.do(a.client.cloudContext(ctx), "DELETE", "/api/v1/cloud/email/pending", nil, nil)
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
