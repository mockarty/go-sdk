// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CoderDeliveryAPI manages the autonomous coder's admitted repositories,
// delivery targets, and deployment-bearing missions.
type CoderDeliveryAPI struct{ client *Client }

type CoderRepoRef struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	VCS     string `json:"vcs,omitempty"`
	CredRef string `json:"credRef,omitempty"`
	Default bool   `json:"default,omitempty"`
}

type CoderDeployTarget struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Env      string          `json:"env,omitempty"`
	CredRef  string          `json:"credRef,omitempty"`
	Approval string          `json:"approval,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
}

type CoderCIConfig struct {
	System        string `json:"system,omitempty"`
	BaseURL       string `json:"baseUrl,omitempty"`
	CredRef       string `json:"credRef,omitempty"`
	OpenMR        string `json:"openMr,omitempty"`
	AutoReviewMRs bool   `json:"autoReviewMRs,omitempty"`
}

type CoderRegistryConfig struct {
	URL              string `json:"url,omitempty"`
	ImageNamePattern string `json:"imageNamePattern,omitempty"`
	CredRef          string `json:"credRef,omitempty"`
}

type CoderGitOpsConfig struct {
	OverlayPaths map[string]string `json:"overlayPaths,omitempty"`
	RepoURL      string            `json:"repoUrl,omitempty"`
	CredRef      string            `json:"credRef,omitempty"`
	ArgoApp      string            `json:"argoApp,omitempty"`
}

type CoderGuardrailPolicy struct {
	ClassOverrides    map[string]string `json:"classOverrides,omitempty"`
	ProtectedBranches []string          `json:"protectedBranches,omitempty"`
	EgressAllowlist   []string          `json:"egressAllowlist,omitempty"`
	ApproverNotify    string            `json:"approverNotify,omitempty"`
}

type CoderIntakeMatchers struct {
	Labels     map[string]string `json:"labels,omitempty"`
	ProjectIDs []string          `json:"projectIds,omitempty"`
	ClientIDs  []string          `json:"clientIds,omitempty"`
	RepoURLs   []string          `json:"repoUrls,omitempty"`
	Hosts      []string          `json:"hosts,omitempty"`
}

type CoderDeliveryConfig struct {
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	MissionSettings map[string]string    `json:"missionSettings,omitempty"`
	Repos           []CoderRepoRef       `json:"repos,omitempty"`
	Targets         []CoderDeployTarget  `json:"targets,omitempty"`
	CI              CoderCIConfig        `json:"ci"`
	Registry        CoderRegistryConfig  `json:"registry"`
	GitOps          CoderGitOpsConfig    `json:"gitops"`
	Policy          CoderGuardrailPolicy `json:"policy"`
	Intake          CoderIntakeMatchers  `json:"intake"`
	Namespace       string               `json:"namespace"`
	ProductID       string               `json:"productId,omitempty"`
	InfraNotes      string               `json:"infraNotes,omitempty"`
	Quality         string               `json:"quality,omitempty"`
	UpdatedBy       string               `json:"updatedBy,omitempty"`
}

type CoderMissionStartRequest struct {
	Goal         string   `json:"goal"`
	RepoURL      string   `json:"repoUrl"`
	Engine       string   `json:"engine,omitempty"`
	DeployTarget string   `json:"deployTarget,omitempty"`
	ProductID    string   `json:"productId,omitempty"`
	Answers      string   `json:"answers,omitempty"`
	IssueKey     string   `json:"issueKey,omitempty"`
	Autonomy     string   `json:"autonomy,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	MaxAttempts  int      `json:"maxAttempts,omitempty"`
	Analyze      bool     `json:"analyze,omitempty"`
}

type CoderMission struct {
	ID              string         `json:"id"`
	Namespace       string         `json:"namespace"`
	Goal            string         `json:"goal"`
	RepoURL         string         `json:"repoUrl"`
	Status          string         `json:"status"`
	Error           string         `json:"error,omitempty"`
	DeployTarget    string         `json:"deployTarget,omitempty"`
	AcceptedCommit  string         `json:"acceptedCommit,omitempty"`
	Approval        string         `json:"approval,omitempty"`
	DeployResult    map[string]any `json:"deployResult,omitempty"`
	DeployStopState string         `json:"deployStopState,omitempty"`
	UnverifiedJobs  int            `json:"unverifiedJobs,omitempty"`
}

type CoderDeployReconciliationOutcome string

const (
	CoderDeployApplied    CoderDeployReconciliationOutcome = "applied"
	CoderDeployNotApplied CoderDeployReconciliationOutcome = "not_applied"
)

type CoderMissionList struct {
	Missions []CoderMission `json:"missions"`
}

func (a *CoderDeliveryAPI) path(suffix string, productID string) string {
	q := url.Values{"namespace": {a.client.namespace}}
	if productID = strings.TrimSpace(productID); productID != "" {
		q.Set("productId", productID)
	}
	return "/api/v1/coder/" + suffix + "?" + q.Encode()
}

func (a *CoderDeliveryAPI) GetConfig(ctx context.Context, productID string) (*CoderDeliveryConfig, error) {
	var out CoderDeliveryConfig
	err := a.client.do(ctx, http.MethodGet, a.path("delivery-config", productID), nil, &out)
	return &out, err
}

func (a *CoderDeliveryAPI) PutConfig(ctx context.Context, config CoderDeliveryConfig) (*CoderDeliveryConfig, error) {
	var out CoderDeliveryConfig
	err := a.client.do(ctx, http.MethodPut, a.path("delivery-config", ""), config, &out)
	return &out, err
}

func (a *CoderDeliveryAPI) DeleteConfig(ctx context.Context, productID string) error {
	return a.client.do(ctx, http.MethodDelete, a.path("delivery-config", productID), nil, nil)
}

func (a *CoderDeliveryAPI) StartMission(ctx context.Context, request CoderMissionStartRequest) (*CoderMission, error) {
	if strings.TrimSpace(request.Goal) == "" || strings.TrimSpace(request.RepoURL) == "" {
		return nil, fmt.Errorf("mockarty: coder mission goal and repo URL are required")
	}
	var out CoderMission
	err := a.client.do(ctx, http.MethodPost, a.path("missions", ""), request, &out)
	return &out, err
}

func (a *CoderDeliveryAPI) ListMissions(ctx context.Context) (*CoderMissionList, error) {
	var out CoderMissionList
	err := a.client.do(ctx, http.MethodGet, a.path("missions", ""), nil, &out)
	if out.Missions == nil {
		out.Missions = []CoderMission{}
	}
	return &out, err
}

func (a *CoderDeliveryAPI) GetMission(ctx context.Context, missionID string) (*CoderMission, error) {
	if strings.TrimSpace(missionID) == "" {
		return nil, fmt.Errorf("mockarty: coder mission id is required")
	}
	var out CoderMission
	err := a.client.do(ctx, http.MethodGet, a.path("missions/"+url.PathEscape(missionID), ""), nil, &out)
	return &out, err
}

func (a *CoderDeliveryAPI) ApproveMission(ctx context.Context, missionID string, approve bool) (*CoderMission, error) {
	if strings.TrimSpace(missionID) == "" {
		return nil, fmt.Errorf("mockarty: coder mission id is required")
	}
	var out CoderMission
	err := a.client.do(ctx, http.MethodPost, a.path("missions/"+url.PathEscape(missionID)+"/approve", ""), map[string]bool{"approve": approve}, &out)
	return &out, err
}

func (a *CoderDeliveryAPI) ReconcileDeploy(ctx context.Context, missionID string, outcome CoderDeployReconciliationOutcome) (*CoderMission, error) {
	if strings.TrimSpace(missionID) == "" {
		return nil, fmt.Errorf("mockarty: coder mission id is required")
	}
	if outcome != CoderDeployApplied && outcome != CoderDeployNotApplied {
		return nil, fmt.Errorf("mockarty: coder deployment outcome must be applied or not_applied")
	}
	var out CoderMission
	err := a.client.do(ctx, http.MethodPost, a.path("missions/"+url.PathEscape(missionID)+"/deploy-outcome", ""), map[string]CoderDeployReconciliationOutcome{"outcome": outcome}, &out)
	return &out, err
}
