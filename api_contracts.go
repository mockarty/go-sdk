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

// DetectWSDLDrift detects drift between mock and real SOAP/WSDL services.
func (a *ContractAPI) DetectWSDLDrift(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/detect-drift/wsdl", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DetectMCPDrift detects drift between mock and real MCP servers.
func (a *ContractAPI) DetectMCPDrift(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/contract/detect-drift/mcp", req, &result); err != nil {
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

// ---------------------------------------------------------------------------
// Consumer Contracts (Dependency Bundles)
// ---------------------------------------------------------------------------

// ConsumerContract represents a consumer's dependency bundle.
type ConsumerContract struct {
	ID           string               `json:"id,omitempty"`
	Name         string               `json:"name"`
	Namespace    string               `json:"namespace,omitempty"`
	Dependencies []ContractDependency `json:"dependencies"`
	Version      int                  `json:"version,omitempty"`
	Tags         []string             `json:"tags,omitempty"`
	Hash         string               `json:"hash,omitempty"`
	CreatedBy    string               `json:"createdBy,omitempty"`
	CreatedAt    string               `json:"createdAt,omitempty"`
	UpdatedAt    string               `json:"updatedAt,omitempty"`
}

// ContractDependency links to a registry API.
type ContractDependency struct {
	RegistryEntryID string             `json:"registryEntryId"`
	ProviderName    string             `json:"providerName"`
	ProviderVersion string             `json:"providerVersion,omitempty"`
	SpecHash        string             `json:"specHash,omitempty"`
	Endpoints       []ContractEndpoint `json:"endpoints"`
}

// ContractEndpoint describes what the consumer needs from a specific endpoint.
type ContractEndpoint struct {
	Route           string            `json:"route"`
	Protocol        string            `json:"protocol,omitempty"`
	ExpectedStatus  []int             `json:"expectedStatus,omitempty"`
	RequiredFields  []ContractField   `json:"requiredFields,omitempty"`
	RequiredHeaders map[string]string `json:"requiredHeaders,omitempty"`
}

// ContractField describes a field the consumer depends on.
type ContractField struct {
	Path     string `json:"path"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required"`
	Pattern  string `json:"pattern,omitempty"`
}

// ListConsumerContracts returns all consumer contracts in the current namespace.
func (a *ContractAPI) ListConsumerContracts(ctx context.Context) ([]ConsumerContract, error) {
	var result []ConsumerContract
	if err := a.client.do(ctx, "GET", "/api/v1/contract/consumer-contracts", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConsumerContract returns a consumer contract by ID.
func (a *ContractAPI) GetConsumerContract(ctx context.Context, id string) (*ConsumerContract, error) {
	var result ConsumerContract
	if err := a.client.do(ctx, "GET", "/api/v1/contract/consumer-contracts/"+id, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateConsumerContract creates or updates a consumer contract.
func (a *ContractAPI) CreateConsumerContract(ctx context.Context, contract ConsumerContract) (*ConsumerContract, error) {
	var result ConsumerContract
	if err := a.client.do(ctx, "POST", "/api/v1/contract/consumer-contracts", contract, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteConsumerContract removes a consumer contract.
func (a *ContractAPI) DeleteConsumerContract(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/contract/consumer-contracts/"+id, nil, nil)
}

// ---------------------------------------------------------------------------
// Can I Deploy V2 (Bidirectional)
// ---------------------------------------------------------------------------

// CanIDeployV2Request is the request for the bidirectional deployment check.
type CanIDeployV2Request struct {
	Role            string `json:"role"`                      // "consumer" or "provider"
	ContractID      string `json:"contractId,omitempty"`      // For consumer check
	RegistryEntryID string `json:"registryEntryId,omitempty"` // For provider check
	NewSpec         string `json:"newSpec,omitempty"`          // Future spec (pre-deploy)
	NewSpecURL      string `json:"newSpecUrl,omitempty"`
}

// CanIDeployV2Result is the result of a bidirectional deployment check.
type CanIDeployV2Result struct {
	Deployable        bool                    `json:"deployable"`
	Role              string                  `json:"role"`
	Summary           string                  `json:"summary"`
	DependencyResults []DependencyCheckResult `json:"dependencyResults,omitempty"`
	AffectedConsumers []ConsumerImpact        `json:"affectedConsumers,omitempty"`
}

// DependencyCheckResult is the result of checking one provider dependency.
type DependencyCheckResult struct {
	ProviderName string `json:"providerName"`
	Compatible   bool   `json:"compatible"`
}

// ConsumerImpact describes how a provider change affects one consumer.
type ConsumerImpact struct {
	ContractID   string   `json:"contractId"`
	ConsumerName string   `json:"consumerName"`
	Namespace    string   `json:"namespace"`
	Compatible   bool     `json:"compatible"`
	BrokenFields []string `json:"brokenFields,omitempty"`
}

// CanIDeployV2 performs a bidirectional deployment readiness check.
func (a *ContractAPI) CanIDeployV2(ctx context.Context, req CanIDeployV2Request) (*CanIDeployV2Result, error) {
	var result CanIDeployV2Result
	if err := a.client.do(ctx, "POST", "/api/v1/contract/can-i-deploy", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Spec Parsing (Wizard Support)
// ---------------------------------------------------------------------------

// ParseEndpointsResult is the result of parsing endpoints from a spec.
type ParseEndpointsResult struct {
	RegistryEntryID string           `json:"registryEntryId"`
	SpecType        string           `json:"specType"`
	Endpoints       []ParsedEndpoint `json:"endpoints"`
}

// ParsedEndpoint is a single endpoint extracted from a spec.
type ParsedEndpoint struct {
	Route       string   `json:"route"`
	Protocol    string   `json:"protocol"`
	Summary     string   `json:"summary,omitempty"`
	StatusCodes []int    `json:"statusCodes,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}

// ParseFieldsResult is the result of parsing response fields for an endpoint.
type ParseFieldsResult struct {
	Route  string          `json:"route"`
	Fields []FieldNode     `json:"fields"`
}

// FieldNode is a tree node representing a response field.
type FieldNode struct {
	Path     string      `json:"path"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	Required bool        `json:"required"`
	Checked  bool        `json:"checked"`
	Children []FieldNode `json:"children,omitempty"`
}

// ParseEndpoints parses a registry entry's spec and returns available endpoints.
func (a *ContractAPI) ParseEndpoints(ctx context.Context, registryEntryID string) (*ParseEndpointsResult, error) {
	var result ParseEndpointsResult
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+registryEntryID+"/parse-endpoints", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ParseFields parses response fields for a specific endpoint in a registry entry.
func (a *ContractAPI) ParseFields(ctx context.Context, registryEntryID, route string, statusCode int) (*ParseFieldsResult, error) {
	var result ParseFieldsResult
	body := map[string]interface{}{"route": route, "statusCode": statusCode}
	if err := a.client.do(ctx, "POST", "/api/v1/contract/registry/"+registryEntryID+"/parse-fields", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Versioning
// ---------------------------------------------------------------------------

// VersionEntry represents one version of a registry entry or consumer contract.
type VersionEntry struct {
	ID         string `json:"id"`
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	Version    int    `json:"version"`
	Content    string `json:"content,omitempty"`
	Hash       string `json:"hash,omitempty"`
	CreatedBy  string `json:"createdBy,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	ChangeNote string `json:"changeNote,omitempty"`
	IsCurrent  bool   `json:"isCurrent"`
}

// VersionDiff represents a structural diff between two versions.
type VersionDiff struct {
	OldVersion int         `json:"oldVersion"`
	NewVersion int         `json:"newVersion"`
	Changes    []DiffEntry `json:"changes"`
	Summary    DiffSummary `json:"summary"`
}

// DiffEntry is a single change between versions.
type DiffEntry struct {
	ChangeType string `json:"changeType"`
	Category   string `json:"category"`
	Path       string `json:"path"`
	OldValue   string `json:"oldValue,omitempty"`
	NewValue   string `json:"newValue,omitempty"`
	Breaking   bool   `json:"breaking"`
	Message    string `json:"message"`
}

// DiffSummary aggregates the diff.
type DiffSummary struct {
	TotalChanges    int `json:"totalChanges"`
	Added           int `json:"added"`
	Removed         int `json:"removed"`
	Changed         int `json:"changed"`
	BreakingChanges int `json:"breakingChanges"`
}

// ListRegistryVersions returns version history for a registry entry.
func (a *ContractAPI) ListRegistryVersions(ctx context.Context, entryID string) ([]VersionEntry, error) {
	var result []VersionEntry
	if err := a.client.do(ctx, "GET", "/api/v1/contract/registry/"+entryID+"/versions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetRegistryVersion returns a specific version of a registry entry.
func (a *ContractAPI) GetRegistryVersion(ctx context.Context, entryID string, version int) (*VersionEntry, error) {
	var result VersionEntry
	if err := a.client.do(ctx, "GET", fmt.Sprintf("/api/v1/contract/registry/%s/versions/%d", entryID, version), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RollbackRegistryVersion rolls back a registry entry to a previous version.
func (a *ContractAPI) RollbackRegistryVersion(ctx context.Context, entryID string, version int) error {
	return a.client.do(ctx, "POST", fmt.Sprintf("/api/v1/contract/registry/%s/versions/%d/rollback", entryID, version), nil, nil)
}

// DiffRegistryVersions computes a diff between two versions of a registry entry.
func (a *ContractAPI) DiffRegistryVersions(ctx context.Context, entryID string, v1, v2 int) (*VersionDiff, error) {
	var result VersionDiff
	if err := a.client.do(ctx, "GET", fmt.Sprintf("/api/v1/contract/registry/%s/versions/%d/diff/%d", entryID, v1, v2), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListConsumerContractVersions returns version history for a consumer contract.
func (a *ContractAPI) ListConsumerContractVersions(ctx context.Context, contractID string) ([]VersionEntry, error) {
	var result []VersionEntry
	if err := a.client.do(ctx, "GET", "/api/v1/contract/consumer-contracts/"+contractID+"/versions", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetConsumerContractVersion returns a specific version of a consumer contract.
func (a *ContractAPI) GetConsumerContractVersion(ctx context.Context, contractID string, version int) (*VersionEntry, error) {
	var result VersionEntry
	if err := a.client.do(ctx, "GET", fmt.Sprintf("/api/v1/contract/consumer-contracts/%s/versions/%d", contractID, version), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// RollbackConsumerContractVersion rolls back a consumer contract to a previous version.
func (a *ContractAPI) RollbackConsumerContractVersion(ctx context.Context, contractID string, version int) error {
	return a.client.do(ctx, "POST", fmt.Sprintf("/api/v1/contract/consumer-contracts/%s/versions/%d/rollback", contractID, version), nil, nil)
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// ContractHealth represents the health status of contracts in a namespace.
type ContractHealth struct {
	Namespace string                 `json:"namespace"`
	Overall   string                 `json:"overall"`
	Items     []ContractHealthItem   `json:"items"`
	Summary   map[string]interface{} `json:"summary"`
}

// ContractHealthItem represents a single contract's health.
type ContractHealthItem struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	ProvidersCount int    `json:"providersCount,omitempty"`
	EndpointsCount int    `json:"endpointsCount,omitempty"`
	Status         string `json:"status"`
}

// Health returns the health status of all contracts in the current namespace.
func (a *ContractAPI) Health(ctx context.Context) (*ContractHealth, error) {
	var result ContractHealth
	if err := a.client.do(ctx, "GET", "/api/v1/contract/health", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
