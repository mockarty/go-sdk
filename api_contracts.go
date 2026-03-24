// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// ContractAPI provides methods for managing contract tests.
type ContractAPI struct {
	client *Client
}

// Contract represents a contract test definition.
type Contract struct {
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

// ContractValidationResult holds the result of a contract validation.
type ContractValidationResult struct {
	ID          string              `json:"id,omitempty"`
	ContractID  string              `json:"contractId,omitempty"`
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

// Create creates a new contract.
func (a *ContractAPI) Create(ctx context.Context, contract *Contract) (*Contract, error) {
	if contract.Namespace == "" && a.client.namespace != "" {
		contract.Namespace = a.client.namespace
	}
	var result Contract
	if err := a.client.do(ctx, "POST", "/ui/api/contract", contract, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a contract by ID.
func (a *ContractAPI) Get(ctx context.Context, id string) (*Contract, error) {
	var contract Contract
	if err := a.client.do(ctx, "GET", "/ui/api/contract/"+url.PathEscape(id), nil, &contract); err != nil {
		return nil, err
	}
	return &contract, nil
}

// Update updates an existing contract by ID.
func (a *ContractAPI) Update(ctx context.Context, id string, contract *Contract) (*Contract, error) {
	if contract.Namespace == "" && a.client.namespace != "" {
		contract.Namespace = a.client.namespace
	}
	var result Contract
	if err := a.client.do(ctx, "PUT", "/ui/api/contract/"+url.PathEscape(id), contract, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes a contract by ID.
func (a *ContractAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/ui/api/contract/"+url.PathEscape(id), nil, nil)
}

// List returns all contracts.
func (a *ContractAPI) List(ctx context.Context) ([]Contract, error) {
	var contracts []Contract
	if err := a.client.do(ctx, "GET", "/ui/api/contract", nil, &contracts); err != nil {
		return nil, err
	}
	return contracts, nil
}

// Validate triggers validation of a contract by ID.
func (a *ContractAPI) Validate(ctx context.Context, id string) (*ContractValidationResult, error) {
	var result ContractValidationResult
	if err := a.client.do(ctx, "POST", "/ui/api/contract/"+url.PathEscape(id)+"/validate", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Results retrieves all validation results for a contract by ID.
func (a *ContractAPI) Results(ctx context.Context, id string) ([]ContractValidationResult, error) {
	var results []ContractValidationResult
	if err := a.client.do(ctx, "GET", "/ui/api/contract/"+url.PathEscape(id)+"/results", nil, &results); err != nil {
		return nil, err
	}
	return results, nil
}
