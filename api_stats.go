// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
)

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
