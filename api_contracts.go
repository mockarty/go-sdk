// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// ContractAPI provides methods for contract testing.
type ContractAPI struct {
	client *Client
}

// ContractConfig represents a saved contract testing configuration.
type ContractConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Spec      string `json:"spec,omitempty"`
	SpecURL   string `json:"specUrl,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Schedule  string `json:"schedule,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// ContractValidationRequest is the payload for validation endpoints.
type ContractValidationRequest struct {
	Spec      string `json:"spec,omitempty"`
	SpecURL   string `json:"specUrl,omitempty"`
	TargetURL string `json:"targetUrl,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// ContractValidationResult holds the result of a contract validation.
type ContractValidationResult struct {
	ID          string              `json:"id,omitempty"`
	ConfigID    string              `json:"configId,omitempty"`
	Status      string              `json:"status,omitempty"` // pass, fail
	Violations  int                 `json:"violations,omitempty"`
	Details     []ContractViolation `json:"details,omitempty"`
	ValidatedAt int64               `json:"validatedAt,omitempty"`
}

// ContractViolation represents a single violation found during contract validation.
type ContractViolation struct {
	Path     string `json:"path,omitempty"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity,omitempty"` // error, warning
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// ---------------------------------------------------------------------------
// Validation endpoints
// ---------------------------------------------------------------------------

// ValidateMocks validates mocks against a spec.
func (a *ContractAPI) ValidateMocks(ctx context.Context, req *ContractValidationRequest) (*ContractValidationResult, error) {
	if req.Namespace == "" && a.client.namespace != "" {
		req.Namespace = a.client.namespace
	}
	var result ContractValidationResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/validate-mocks", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyProvider verifies a provider against a contract spec.
func (a *ContractAPI) VerifyProvider(ctx context.Context, req *ContractValidationRequest) (*ContractValidationResult, error) {
	if req.Namespace == "" && a.client.namespace != "" {
		req.Namespace = a.client.namespace
	}
	var result ContractValidationResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/verify-provider", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CheckCompatibility checks compatibility between specs.
func (a *ContractAPI) CheckCompatibility(ctx context.Context, req any) (*ContractValidationResult, error) {
	var result ContractValidationResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/check-compatibility", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ValidatePayload validates a payload against a spec.
func (a *ContractAPI) ValidatePayload(ctx context.Context, req any) (*ContractValidationResult, error) {
	var result ContractValidationResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/validate-payload", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Config CRUD
// ---------------------------------------------------------------------------

// ListConfigs returns all contract testing configurations.
func (a *ContractAPI) ListConfigs(ctx context.Context) ([]ContractConfig, error) {
	var configs []ContractConfig
	if err := a.client.do(ctx, "GET", "/api/v1/contract/configs", nil, &configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// SaveConfig creates or updates a contract testing configuration.
func (a *ContractAPI) SaveConfig(ctx context.Context, config *ContractConfig) (*ContractConfig, error) {
	if config.Namespace == "" && a.client.namespace != "" {
		config.Namespace = a.client.namespace
	}
	var result ContractConfig
	if err := a.client.do(ctx, "POST", "/api/v1/contract/configs", config, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteConfig deletes a contract testing configuration by ID.
func (a *ContractAPI) DeleteConfig(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/contract/configs/"+url.PathEscape(id), nil, nil)
}

// ---------------------------------------------------------------------------
// Results
// ---------------------------------------------------------------------------

// ListResults returns all contract validation results.
func (a *ContractAPI) ListResults(ctx context.Context) ([]ContractValidationResult, error) {
	var results []ContractValidationResult
	if err := a.client.do(ctx, "GET", "/api/v1/contract/results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetResult retrieves a specific validation result by ID.
func (a *ContractAPI) GetResult(ctx context.Context, id string) (*ContractValidationResult, error) {
	var result ContractValidationResult
	if err := a.client.do(ctx, "GET", "/api/v1/contract/results/"+url.PathEscape(id), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Pact types
// ---------------------------------------------------------------------------

// Pact represents a consumer-driven contract (pact).
type Pact struct {
	ID        string `json:"id,omitempty"`
	Consumer  string `json:"consumer"`
	Provider  string `json:"provider"`
	Version   string `json:"version,omitempty"`
	Spec      string `json:"spec,omitempty"`
	SpecURL   string `json:"specUrl,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// PactVerificationResult holds the result of a pact verification.
type PactVerificationResult struct {
	ID         string              `json:"id,omitempty"`
	PactID     string              `json:"pactId,omitempty"`
	Status     string              `json:"status,omitempty"`
	Violations []ContractViolation `json:"violations,omitempty"`
	VerifiedAt int64               `json:"verifiedAt,omitempty"`
}

// CanIDeployResult holds the result of a can-i-deploy check.
type CanIDeployResult struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// ---------------------------------------------------------------------------
// Pact endpoints
// ---------------------------------------------------------------------------

// ListPacts returns all pacts.
func (a *ContractAPI) ListPacts(ctx context.Context) ([]Pact, error) {
	var pacts []Pact
	if err := a.client.do(ctx, "GET", "/api/v1/contract/pacts", nil, &pacts); err != nil {
		return nil, err
	}
	return pacts, nil
}

// GetPact retrieves a pact by ID.
func (a *ContractAPI) GetPact(ctx context.Context, id string) (*Pact, error) {
	var pact Pact
	if err := a.client.do(ctx, "GET", "/api/v1/contract/pacts/"+url.PathEscape(id), nil, &pact); err != nil {
		return nil, err
	}
	return &pact, nil
}

// PublishPact publishes a new pact.
func (a *ContractAPI) PublishPact(ctx context.Context, pact *Pact) (*Pact, error) {
	if pact.Namespace == "" && a.client.namespace != "" {
		pact.Namespace = a.client.namespace
	}
	var result Pact
	if err := a.client.do(ctx, "POST", "/api/v1/contract/pacts", pact, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// VerifyPact verifies a pact against a provider.
func (a *ContractAPI) VerifyPact(ctx context.Context, req any) (*PactVerificationResult, error) {
	var result PactVerificationResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/pacts/verify", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CanIDeploy checks whether a service can be safely deployed.
func (a *ContractAPI) CanIDeploy(ctx context.Context, req any) (*CanIDeployResult, error) {
	var result CanIDeployResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/pacts/can-i-deploy", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeletePact deletes a pact by ID.
func (a *ContractAPI) DeletePact(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/contract/pacts/"+url.PathEscape(id), nil, nil)
}

// GenerateMocksFromPact generates mocks from a pact.
func (a *ContractAPI) GenerateMocksFromPact(ctx context.Context, pactID string) ([]Mock, error) {
	var mocks []Mock
	if err := a.client.do(ctx, "POST", "/api/v1/contract/pacts/"+url.PathEscape(pactID)+"/mocks", nil, &mocks); err != nil {
		return nil, err
	}
	return mocks, nil
}

// ListVerifications returns all pact verification results.
func (a *ContractAPI) ListVerifications(ctx context.Context) ([]PactVerificationResult, error) {
	var results []PactVerificationResult
	if err := a.client.do(ctx, "GET", "/api/v1/contract/pacts/verifications", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// DetectDrift detects drift between a contract and the live service.
func (a *ContractAPI) DetectDrift(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/detect-drift", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}
