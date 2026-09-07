// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CoderObservabilityCorrelation binds a reading to the release or run it describes.
type CoderObservabilityCorrelation struct {
	MissionID       string `json:"missionId,omitempty"`
	DeploymentRunID string `json:"deploymentRunId,omitempty"`
	ReleaseDigest   string `json:"releaseDigest,omitempty"`
	TestRunID       string `json:"testRunId,omitempty"`
	TraceID         string `json:"traceId,omitempty"`
	Environment     string `json:"environment,omitempty"`
}

// CoderObservabilityQuery is one bounded, read-only deployed-system query.
type CoderObservabilityQuery struct {
	Start       *time.Time                    `json:"start,omitempty"`
	End         *time.Time                    `json:"end,omitempty"`
	Correlation CoderObservabilityCorrelation `json:"correlation"`
	Source      string                        `json:"source"`
	Kind        string                        `json:"kind,omitempty"`
	Expression  string                        `json:"expression"`
	Limit       int                           `json:"limit,omitempty"`
	MaxBytes    int                           `json:"maxBytes,omitempty"`
}

type CoderObservabilitySource struct {
	Kinds        []string `json:"kinds"`
	Source       string   `json:"source"`
	ConnectionID string   `json:"connectionId"`
	Revision     int64    `json:"revision"`
}

type CoderObservabilitySources struct {
	Defaults        map[string]int             `json:"defaults"`
	Sources         []CoderObservabilitySource `json:"sources"`
	ContractVersion string                     `json:"contractVersion"`
}

type CoderObservabilityResponse struct {
	Result          map[string]any `json:"result"`
	ContractVersion string         `json:"contractVersion"`
}

func (a *CoderDeliveryAPI) observabilityPath(suffix string) string {
	return "/api/v1/observability/" + suffix + "?" + url.Values{"namespace": {a.client.namespace}}.Encode()
}

// ObservabilitySources lists only sources bound to this client's namespace.
func (a *CoderDeliveryAPI) ObservabilitySources(ctx context.Context) (*CoderObservabilitySources, error) {
	var out CoderObservabilitySources
	err := a.client.do(ctx, http.MethodGet, a.observabilityPath("sources"), nil, &out)
	if out.Sources == nil {
		out.Sources = []CoderObservabilitySource{}
	}
	return &out, err
}

// QueryObservability returns frozen evidence from one configured source.
func (a *CoderDeliveryAPI) QueryObservability(ctx context.Context, query CoderObservabilityQuery) (*CoderObservabilityResponse, error) {
	if strings.TrimSpace(query.Source) == "" || strings.TrimSpace(query.Expression) == "" {
		return nil, fmt.Errorf("mockarty: observability source and expression are required")
	}
	var out CoderObservabilityResponse
	err := a.client.do(ctx, http.MethodPost, a.observabilityPath("query"), query, &out)
	return &out, err
}
