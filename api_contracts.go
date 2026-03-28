// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
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

// DetectGraphQLDrift detects drift between mock and real GraphQL schema via introspection.
func (a *ContractAPI) DetectGraphQLDrift(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/detect-drift/graphql", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DetectGRPCDrift detects drift between mock and real gRPC services via reflection.
func (a *ContractAPI) DetectGRPCDrift(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/detect-drift/grpc", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ─── API Registry ───────────────────────────────────────────────────────────

// RegistryEntry represents a published API in the internal marketplace.
type RegistryEntry struct {
	ID                string   `json:"id,omitempty"`
	Namespace         string   `json:"namespace,omitempty"`
	ServiceName       string   `json:"serviceName"`
	Description       string   `json:"description,omitempty"`
	SpecType          string   `json:"specType,omitempty"`
	SpecURL           string   `json:"specUrl,omitempty"`
	SpecContent       string   `json:"specContent,omitempty"`
	Version           string   `json:"version,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Visibility        string   `json:"visibility,omitempty"`
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`
	Owner             string   `json:"owner,omitempty"`
	EndpointsCount    int      `json:"endpointsCount,omitempty"`
}

// ListRegistry returns published APIs, optionally filtered by query.
func (a *ContractAPI) ListRegistry(ctx context.Context, query string) ([]RegistryEntry, error) {
	path := "/api/v1/contract/registry"
	if query != "" {
		path += "?q=" + url.QueryEscape(query)
	}
	var entries []RegistryEntry
	if err := a.client.do(ctx, "GET", path, nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// GetRegistryEntry returns a single registry entry by ID.
func (a *ContractAPI) GetRegistryEntry(ctx context.Context, id string) (*RegistryEntry, error) {
	var entry RegistryEntry
	if err := a.client.do(ctx, "GET", "/api/v1/contract/registry/"+id, nil, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// PublishToRegistry publishes an API specification to the internal registry.
func (a *ContractAPI) PublishToRegistry(ctx context.Context, entry *RegistryEntry) (*RegistryEntry, error) {
	var result RegistryEntry
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry", entry, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateRegistryEntry updates an existing registry entry.
func (a *ContractAPI) UpdateRegistryEntry(ctx context.Context, id string, update *RegistryEntry) (*RegistryEntry, error) {
	var result RegistryEntry
	if err := a.client.do(ctx, "PUT", "/api/v1/contract/registry/"+id, update, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteRegistryEntry removes a registry entry.
func (a *ContractAPI) DeleteRegistryEntry(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/contract/registry/"+id, nil, nil)
}

// GenerateMocksFromRegistry generates mocks from a registry entry's specification.
func (a *ContractAPI) GenerateMocksFromRegistry(ctx context.Context, entryID string) (map[string]any, error) {
	var result map[string]any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/generate-mocks", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CheckImpact checks which subscribers would be affected by a spec change.
func (a *ContractAPI) CheckImpact(ctx context.Context, entryID string, newSpecContent string) (map[string]any, error) {
	var result map[string]any
	body := map[string]string{"newSpecContent": newSpecContent}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/check-impact", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ─── Subscriptions ──────────────────────────────────────────────────────────

// Subscription represents a team's dependency on another team's API.
type Subscription struct {
	ID               string   `json:"id,omitempty"`
	Namespace        string   `json:"namespace,omitempty"`
	RegistryEntryID  string   `json:"registryEntryId"`
	ServiceName      string   `json:"serviceName"`
	WatchEndpoints   []string `json:"watchEndpoints,omitempty"`
	NotifyOnBreaking bool     `json:"notifyOnBreaking,omitempty"`
	AutoBlock        bool     `json:"autoBlock,omitempty"`
}

// ListSubscriptions returns current namespace's subscriptions.
func (a *ContractAPI) ListSubscriptions(ctx context.Context) ([]Subscription, error) {
	var subs []Subscription
	if err := a.client.do(ctx, "GET", "/api/v1/contract/subscriptions", nil, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// Subscribe creates a subscription to a registry API.
func (a *ContractAPI) Subscribe(ctx context.Context, registryEntryID string, sub *Subscription) (*Subscription, error) {
	var result Subscription
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+registryEntryID+"/subscribe", sub, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Unsubscribe removes a subscription.
func (a *ContractAPI) Unsubscribe(ctx context.Context, subscriptionID string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/contract/subscriptions/"+subscriptionID, nil, nil)
}

// ListSubscribers returns who subscribes to a specific API.
func (a *ContractAPI) ListSubscribers(ctx context.Context, registryEntryID string) ([]Subscription, error) {
	var subs []Subscription
	if err := a.client.do(ctx, "GET", "/api/v1/contract/registry/"+registryEntryID+"/subscribers", nil, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// ─── Change Requests ────────────────────────────────────────────────────────

// ChangeRequest represents a proposed API spec update pending review.
type ChangeRequest struct {
	ID              string `json:"id,omitempty"`
	RegistryEntryID string `json:"registryEntryId"`
	Namespace       string `json:"namespace,omitempty"`
	SubmittedBy     string `json:"submittedBy,omitempty"`
	NewSpecContent  string `json:"newSpecContent"`
	NewVersion      string `json:"newVersion,omitempty"`
	Status          string `json:"status,omitempty"`
	BreakingChanges int    `json:"breakingChanges,omitempty"`
}

// CreateChangeRequest submits a spec change for review.
func (a *ContractAPI) CreateChangeRequest(ctx context.Context, registryEntryID string, newSpec, newVersion string) (*ChangeRequest, error) {
	var result ChangeRequest
	body := map[string]string{"newSpecContent": newSpec, "newVersion": newVersion}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+registryEntryID+"/change-requests", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListChangeRequests returns change requests for a registry entry.
func (a *ContractAPI) ListChangeRequests(ctx context.Context, registryEntryID string) ([]ChangeRequest, error) {
	var result []ChangeRequest
	if err := a.client.do(ctx, "GET", "/api/v1/contract/registry/"+registryEntryID+"/change-requests", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ApproveChangeRequest approves a change request.
func (a *ContractAPI) ApproveChangeRequest(ctx context.Context, crID, comment string) error {
	body := map[string]string{"comment": comment}
	return a.client.do(ctx, "POST", "/api/v1/contract/change-requests/"+crID+"/approve", body, nil)
}

// RejectChangeRequest rejects a change request.
func (a *ContractAPI) RejectChangeRequest(ctx context.Context, crID, comment string) error {
	body := map[string]string{"comment": comment}
	return a.client.do(ctx, "POST", "/api/v1/contract/change-requests/"+crID+"/reject", body, nil)
}

// PendingChangeRequests returns change requests awaiting my team's approval.
func (a *ContractAPI) PendingChangeRequests(ctx context.Context) ([]ChangeRequest, error) {
	var result []ChangeRequest
	if err := a.client.do(ctx, "GET", "/api/v1/contract/change-requests/pending", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetTrends returns validation trend data for the past N days.
func (a *ContractAPI) GetTrends(ctx context.Context, days int) ([]map[string]any, error) {
	path := fmt.Sprintf("/api/v1/contract/trends?days=%d", days)
	var result []map[string]any
	if err := a.client.do(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ListParticipants returns unique consumer/provider names from pacts (for autocomplete).
func (a *ContractAPI) ListParticipants(ctx context.Context) ([]string, error) {
	var result []string
	if err := a.client.do(ctx, "GET", "/api/v1/contract/pacts/participants", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateFromRegistry validates mocks against a registry entry's specification.
func (a *ContractAPI) ValidateFromRegistry(ctx context.Context, entryID string) (map[string]any, error) {
	var result map[string]any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/validate", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SubmitForReview submits a registry entry for review.
func (a *ContractAPI) SubmitForReview(ctx context.Context, entryID, reviewerID string) (*RegistryEntry, error) {
	var result RegistryEntry
	body := map[string]string{"reviewerId": reviewerID}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/submit-review", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ApproveReview approves a registry entry review.
func (a *ContractAPI) ApproveReview(ctx context.Context, entryID, comment string) (*RegistryEntry, error) {
	var result RegistryEntry
	body := map[string]string{"comment": comment}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/approve-review", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RejectReview rejects a registry entry review.
func (a *ContractAPI) RejectReview(ctx context.Context, entryID, comment string) (*RegistryEntry, error) {
	var result RegistryEntry
	body := map[string]string{"comment": comment}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+entryID+"/reject-review", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AssignReviewer assigns a reviewer to a registry entry.
func (a *ContractAPI) AssignReviewer(ctx context.Context, entryID, reviewerID string) (*RegistryEntry, error) {
	var result RegistryEntry
	body := map[string]string{"reviewerId": reviewerID}
	if err := a.client.do(ctx, "PUT", "/api/v1/contract/registry/"+entryID+"/reviewer", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
