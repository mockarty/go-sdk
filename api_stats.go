// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"net/url"
)

type CapabilityCatalog struct {
	Capabilities []CapabilityDescriptor `json:"capabilities"`
	Count        int                    `json:"count"`
	Skipped      int                    `json:"skipped"`
}

type CapabilityDescriptor struct {
	Schemas         CapabilitySchemas       `json:"schemas,omitempty"`
	Policy          CapabilityPolicy        `json:"policy"`
	Resource        CapabilityResource      `json:"resource"`
	Provenance      CapabilityProvenance    `json:"provenance"`
	Trust           CapabilityTrust         `json:"trust"`
	Executor        CapabilityExecutor      `json:"executor"`
	Health          CapabilityHealth        `json:"health"`
	Compatibility   CapabilityCompatibility `json:"compatibility"`
	Availability    CapabilityAvailability  `json:"availability"`
	ContractVersion string                  `json:"contractVersion"`
	Key             string                  `json:"key"`
	Version         string                  `json:"version"`
	Provider        string                  `json:"provider"`
	Kind            string                  `json:"kind"`
	Title           string                  `json:"title"`
	Description     string                  `json:"description"`
	FeatureKey      string                  `json:"featureKey,omitempty"`
	Hosts           []string                `json:"hosts"`
	Builtin         bool                    `json:"builtin"`
}

type CapabilitySchemas struct {
	Input     json.RawMessage `json:"input,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	Settings  json.RawMessage `json:"settings,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Evidence  json.RawMessage `json:"evidence,omitempty"`
	Admission json.RawMessage `json:"admission,omitempty"`
}

type CapabilityPolicy struct {
	DataBoundary        CapabilityDataBoundary `json:"dataBoundary,omitempty"`
	RequiredRoles       []string               `json:"requiredRoles,omitempty"`
	RequiredPermissions []string               `json:"requiredPermissions,omitempty"`
	SideEffect          string                 `json:"sideEffect"`
	Idempotency         string                 `json:"idempotency"`
	TimeoutMillis       int64                  `json:"timeoutMillis,omitempty"`
	MaxRetries          int32                  `json:"maxRetries,omitempty"`
}

type CapabilityDataBoundary struct {
	Classes      []string `json:"classes,omitempty"`
	Residencies  []string `json:"residencies,omitempty"`
	AllowedHosts []string `json:"allowedHosts,omitempty"`
	NetworkScope string   `json:"networkScope,omitempty"`
}

type CapabilityResource struct {
	MemoryBytes          int64 `json:"memoryBytes,omitempty"`
	ScratchBytes         int64 `json:"scratchBytes,omitempty"`
	CostUpperBoundMicros int64 `json:"costUpperBoundMicros,omitempty"`
	CPUUnits             int32 `json:"cpuUnits,omitempty"`
	MaxConcurrency       int32 `json:"maxConcurrency,omitempty"`
}

type CapabilityProvenance struct {
	SourceKind string `json:"sourceKind"`
	SourceRef  string `json:"sourceRef"`
	Digest     string `json:"digest,omitempty"`
	Publisher  string `json:"publisher"`
}

type CapabilityTrust struct {
	Level          string `json:"level"`
	Isolation      string `json:"isolation"`
	SignatureKeyID string `json:"signatureKeyId,omitempty"`
	Verified       bool   `json:"verified"`
}

type CapabilityExecutor struct {
	Kind    string `json:"kind"`
	Binding string `json:"binding"`
}

type CapabilityHealth struct {
	Kind          string `json:"kind"`
	Probe         string `json:"probe,omitempty"`
	TimeoutMillis int64  `json:"timeoutMillis,omitempty"`
}

type CapabilityCompatibility struct {
	MinHostVersion string `json:"minHostVersion,omitempty"`
	MaxHostVersion string `json:"maxHostVersion,omitempty"`
}

type CapabilityAvailability struct {
	Reason    string `json:"reason,omitempty"`
	Available bool   `json:"available"`
}

// StatsAPI provides methods for retrieving platform statistics.
type StatsAPI struct {
	client *Client
}

// GetStats returns general platform statistics.
func (a *StatsAPI) GetStats(ctx context.Context) (map[string]any, error) {
	var stats map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/stats", nil, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

// GetCounts returns resource counts (mocks, namespaces, etc.).
func (a *StatsAPI) GetCounts(ctx context.Context) (map[string]any, error) {
	var counts map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/counts", nil, &counts); err != nil {
		return nil, err
	}
	return counts, nil
}

// GetStatus returns the current system status.
func (a *StatsAPI) GetStatus(ctx context.Context) (map[string]any, error) {
	var status map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/status", nil, &status); err != nil {
		return nil, err
	}
	return status, nil
}

// GetFeatures returns the available feature flags and capabilities.
func (a *StatsAPI) GetFeatures(ctx context.Context) (map[string]any, error) {
	var features map[string]any
	if err := a.client.do(ctx, "GET", "/api/v1/features", nil, &features); err != nil {
		return nil, err
	}
	return features, nil
}

// ListCapabilities returns the canonical, caller-scoped capability catalogue.
func (a *StatsAPI) ListCapabilities(ctx context.Context) (*CapabilityCatalog, error) {
	var catalog CapabilityCatalog
	path := "/api/v1/capabilities?namespace=" + url.QueryEscape(a.client.namespace)
	if err := a.client.do(ctx, "GET", path, nil, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}
