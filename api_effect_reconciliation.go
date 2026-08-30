package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// EffectReconciliationAPI exposes the admin queue for unresolved external effects.
type EffectReconciliationAPI struct{ client *Client }

type EffectReconciliationListOptions struct {
	ProjectID    string
	EffectFamily string
	Reason       string
	Cursor       string
	MinAge       time.Duration
	Limit        int
}

type EffectReconciliationClaim struct {
	ExpiresAt       time.Time `json:"expiresAt"`
	ClaimedBy       string    `json:"claimedBy"`
	ClaimGeneration int64     `json:"claimGeneration"`
}

type EffectReconciliationItem struct {
	CreatedAt         time.Time                  `json:"createdAt"`
	UpdatedAt         time.Time                  `json:"updatedAt"`
	Claim             *EffectReconciliationClaim `json:"claim,omitempty"`
	Namespace         string                     `json:"namespace"`
	ProjectID         string                     `json:"projectId,omitempty"`
	MissionID         string                     `json:"missionId"`
	RunID             string                     `json:"runId"`
	ExecutionID       string                     `json:"executionId"`
	EffectFamily      string                     `json:"effectFamily"`
	Status            string                     `json:"status"`
	Reason            string                     `json:"reason"`
	ExternalEffectRef string                     `json:"externalEffectRef,omitempty"`
	RecoveryAttempts  int64                      `json:"recoveryAttempts"`
}

type EffectReconciliationPage struct {
	Items      []EffectReconciliationItem `json:"items"`
	NextCursor string                     `json:"nextCursor"`
}

type EffectReconciliationResult struct {
	ExecutionID  string `json:"executionId"`
	Status       string `json:"status"`
	Reason       string `json:"reason"`
	EffectFamily string `json:"effectFamily"`
}

func (a *EffectReconciliationAPI) ListQueue(ctx context.Context, options EffectReconciliationListOptions) (*EffectReconciliationPage, error) {
	query := url.Values{"namespace": []string{a.client.namespace}}
	if options.ProjectID != "" {
		query.Set("project", options.ProjectID)
	}
	if options.EffectFamily != "" {
		query.Set("family", options.EffectFamily)
	}
	if options.Reason != "" {
		query.Set("reason", options.Reason)
	}
	if options.Cursor != "" {
		query.Set("cursor", options.Cursor)
	}
	if options.MinAge < 0 || options.MinAge%time.Second != 0 {
		return nil, fmt.Errorf("mockarty: effect reconciliation min age must be a non-negative whole number of seconds")
	}
	if options.MinAge > 0 {
		query.Set("minAgeSeconds", strconv.FormatInt(int64(options.MinAge/time.Second), 10))
	}
	if options.Limit < 0 || options.Limit > 100 {
		return nil, fmt.Errorf("mockarty: effect reconciliation limit must be between 1 and 100")
	}
	if options.Limit > 0 {
		query.Set("limit", strconv.Itoa(options.Limit))
	}
	var result EffectReconciliationPage
	err := a.client.do(ctx, http.MethodGet, "/api/v1/admin/effects/reconciliation?"+query.Encode(), nil, &result)
	return &result, err
}

func (a *EffectReconciliationAPI) ReconcileNoEffect(ctx context.Context, executionID, providerReference, evidenceSource string) (*EffectReconciliationResult, error) {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, fmt.Errorf("mockarty: effect reconciliation execution id is required")
	}
	body := map[string]any{
		"namespace": a.client.namespace, "executionId": executionID,
		"decision": "no_effect", "autoClaim": true,
		"providerReference": strings.TrimSpace(providerReference),
		"evidenceSource":    strings.TrimSpace(evidenceSource),
	}
	var result EffectReconciliationResult
	err := a.client.do(ctx, http.MethodPost, "/api/v1/admin/effects/reconciliation/reconcile", body, &result)
	return &result, err
}
