// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type WorkflowDefinitionsAPI struct{ client *Client }

type WorkflowCapabilityID struct {
	Key     string `json:"key"`
	Version string `json:"version"`
}

type WorkflowConnectionRef struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

type WorkflowSecretRef struct {
	StoreID string `json:"storeId"`
	Key     string `json:"key"`
	Version int64  `json:"version"`
}

type WorkflowNode struct {
	Connections []WorkflowConnectionRef `json:"connections,omitempty"`
	Secrets     []WorkflowSecretRef     `json:"secrets,omitempty"`
	Capability  WorkflowCapabilityID    `json:"capability"`
	ID          string                  `json:"id"`
}

type WorkflowTransition struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Branch    string `json:"branch"`
	Condition string `json:"condition,omitempty"`
	Default   bool   `json:"default,omitempty"`
}

type WorkflowDefinition struct {
	Nodes           []WorkflowNode       `json:"nodes"`
	Transitions     []WorkflowTransition `json:"transitions"`
	ContractVersion string               `json:"contractVersion"`
	ID              string               `json:"id"`
	Namespace       string               `json:"namespace"`
	Version         string               `json:"version"`
	EntryNode       string               `json:"entryNode"`
	Status          string               `json:"status"`
}

type WorkflowBlocker struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
}

type WorkflowDryRunResult struct {
	Capabilities         []WorkflowCapabilityID  `json:"capabilities"`
	Connections          []WorkflowConnectionRef `json:"connections"`
	Secrets              []WorkflowSecretRef     `json:"secrets"`
	Branches             []WorkflowTransition    `json:"branches"`
	Blockers             []WorkflowBlocker       `json:"blockers"`
	DefinitionDigest     string                  `json:"definitionDigest"`
	CostUpperBoundMicros int64                   `json:"costUpperBoundMicros"`
	Ready                bool                    `json:"ready"`
}

type StoredWorkflowDefinition struct {
	Definition       WorkflowDefinition    `json:"definition"`
	DryRun           *WorkflowDryRunResult `json:"dryRun,omitempty"`
	CreatedAt        time.Time             `json:"createdAt"`
	UpdatedAt        time.Time             `json:"updatedAt"`
	DryRunAt         time.Time             `json:"dryRunAt,omitempty"`
	PublishedAt      time.Time             `json:"publishedAt,omitempty"`
	DefinitionDigest string                `json:"definitionDigest"`
	DryRunDigest     string                `json:"dryRunDigest,omitempty"`
	CreatedBy        string                `json:"createdBy"`
	UpdatedBy        string                `json:"updatedBy"`
	DryRunBy         string                `json:"dryRunBy,omitempty"`
	PublishedBy      string                `json:"publishedBy,omitempty"`
	Revision         int64                 `json:"revision"`
}

type WorkflowDefinitionSummary struct {
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	PublishedAt      time.Time `json:"publishedAt,omitempty"`
	Namespace        string    `json:"namespace"`
	ID               string    `json:"id"`
	Version          string    `json:"version"`
	DefinitionDigest string    `json:"definitionDigest"`
	CreatedBy        string    `json:"createdBy"`
	UpdatedBy        string    `json:"updatedBy"`
	PublishedBy      string    `json:"publishedBy,omitempty"`
	Status           string    `json:"status"`
	Revision         int64     `json:"revision"`
}

type WorkflowDefinitionListOptions struct {
	ID     string
	Status string
	Cursor string
	Limit  int
}

type WorkflowDefinitionList struct {
	Definitions []WorkflowDefinitionSummary `json:"definitions"`
	NextCursor  string                      `json:"nextCursor,omitempty"`
}

func (a *WorkflowDefinitionsAPI) basePath(namespace string) (string, error) {
	if a == nil || a.client == nil {
		return "", fmt.Errorf("workflow definitions API is unavailable")
	}
	if namespace == "" {
		namespace = a.client.namespace
	}
	if namespace == "" || namespace == "*" {
		return "", fmt.Errorf("a concrete namespace is required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(namespace) + "/workflow-definitions", nil
}

func (a *WorkflowDefinitionsAPI) List(ctx context.Context, namespace string, options WorkflowDefinitionListOptions) (*WorkflowDefinitionList, error) {
	base, err := a.basePath(namespace)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	if options.ID != "" {
		query.Set("id", options.ID)
	}
	if options.Status != "" {
		query.Set("status", options.Status)
	}
	if options.Cursor != "" {
		query.Set("cursor", options.Cursor)
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	if encoded := query.Encode(); encoded != "" {
		base += "?" + encoded
	}
	var out WorkflowDefinitionList
	if err = a.client.do(ctx, "GET", base, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) CreateDraft(ctx context.Context, definition WorkflowDefinition) (*StoredWorkflowDefinition, error) {
	if definition.Namespace == "" && a != nil && a.client != nil {
		definition.Namespace = a.client.namespace
	}
	base, err := a.basePath(definition.Namespace)
	if err != nil {
		return nil, err
	}
	var out StoredWorkflowDefinition
	if err = a.client.do(ctx, "POST", base, definition, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) Get(ctx context.Context, namespace, id, version string) (*StoredWorkflowDefinition, error) {
	path, err := a.versionPath(namespace, id, version)
	if err != nil {
		return nil, err
	}
	var out StoredWorkflowDefinition
	if err = a.client.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) UpdateDraft(ctx context.Context, definition WorkflowDefinition, expectedRevision int64) (*StoredWorkflowDefinition, error) {
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("expected revision must be positive")
	}
	if definition.Namespace == "" && a != nil && a.client != nil {
		definition.Namespace = a.client.namespace
	}
	path, err := a.versionPath(definition.Namespace, definition.ID, definition.Version)
	if err != nil {
		return nil, err
	}
	var out StoredWorkflowDefinition
	body := struct {
		Definition       WorkflowDefinition `json:"definition"`
		ExpectedRevision int64              `json:"expectedRevision"`
	}{definition, expectedRevision}
	if err = a.client.do(ctx, "PUT", path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) DryRun(ctx context.Context, namespace, id, version string, expectedRevision int64) (*WorkflowDryRunResult, error) {
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("expected revision must be positive")
	}
	path, err := a.versionPath(namespace, id, version)
	if err != nil {
		return nil, err
	}
	var out WorkflowDryRunResult
	if err = a.client.do(ctx, "POST", path+"/dry-run", map[string]int64{"expectedRevision": expectedRevision}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) Publish(ctx context.Context, namespace, id, version string, expectedRevision int64) (*StoredWorkflowDefinition, error) {
	if expectedRevision <= 0 {
		return nil, fmt.Errorf("expected revision must be positive")
	}
	path, err := a.versionPath(namespace, id, version)
	if err != nil {
		return nil, err
	}
	var out StoredWorkflowDefinition
	if err = a.client.do(ctx, "POST", path+"/publish", map[string]int64{"expectedRevision": expectedRevision}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *WorkflowDefinitionsAPI) versionPath(namespace, id, version string) (string, error) {
	base, err := a.basePath(namespace)
	if err != nil {
		return "", err
	}
	if id == "" || version == "" {
		return "", fmt.Errorf("workflow id and version are required")
	}
	return base + "/" + url.PathEscape(id) + "/versions/" + url.PathEscape(version), nil
}
