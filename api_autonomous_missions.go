// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AutonomousMissionsAPI submits and supervises durable goal-first autonomous missions.
type AutonomousMissionsAPI struct{ client *Client }

var missionSettingsDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// MissionRevisionReference pins one target or artifact to the exact revision
// and content digest observed before mission admission.
type MissionRevisionReference struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	Revision int64  `json:"revision"`
}

// MissionEffectiveSettingsOptions selects the layered policy preview used for a
// unified mission start. A zero RunWindowMinutes means no mission override.
type MissionEffectiveSettingsOptions struct {
	ProductID        string
	MissionID        string
	RunWindowMinutes int
}

// MissionEffectiveSetting is one resolved autonomy setting and its provenance.
type MissionEffectiveSetting struct {
	Key            string `json:"key"`
	Value          string `json:"value"`
	Layer          string `json:"layer"`
	Builtin        string `json:"builtin"`
	Frozen         bool   `json:"frozen,omitempty"`
	RuntimeApplied bool   `json:"runtimeApplied"`
}

// MissionEffectiveSettings is the authoritative preview returned before start.
type MissionEffectiveSettings struct {
	Settings       []MissionEffectiveSetting `json:"settings"`
	Namespace      string                    `json:"namespace"`
	ProductID      string                    `json:"productId,omitempty"`
	MissionID      string                    `json:"missionId,omitempty"`
	SettingsDigest string                    `json:"settingsDigest"`
	Count          int                       `json:"count"`
}

// MissionStartRequest starts a goal-first mission in the unified ledger. Kind
// and Chain are compatibility overrides; omit them to let Mockarty plan the
// required capability graph from Goal.
type MissionStartRequest struct {
	Data                   map[string]any             `json:"data,omitempty"`
	Namespace              string                     `json:"namespace,omitempty"`
	ProductID              string                     `json:"productId,omitempty"`
	Subject                string                     `json:"subject,omitempty"`
	Kind                   string                     `json:"kind,omitempty"`
	Goal                   string                     `json:"goal"`
	Autonomy               string                     `json:"autonomy,omitempty"`
	OriginRef              string                     `json:"originRef,omitempty"`
	ExpectedSettingsDigest string                     `json:"expectedSettingsDigest,omitempty"`
	Targets                []MissionRevisionReference `json:"targets,omitempty"`
	Artifacts              []MissionRevisionReference `json:"artifacts,omitempty"`
	Chain                  []string                   `json:"chain,omitempty"`
	BudgetTokensTotal      int64                      `json:"budgetTokensTotal,omitempty"`
	BudgetTokensPerDay     int64                      `json:"budgetTokensPerDay,omitempty"`
	BudgetUSDCap           float64                    `json:"budgetUsdCap,omitempty"`
}

// UnifiedMission is the stable mission projection returned by the unified ledger.
// Step maps preserve additive component-specific fields from newer servers.
type UnifiedMission struct {
	CreatedAt          time.Time                  `json:"createdAt"`
	UpdatedAt          time.Time                  `json:"updatedAt"`
	ClosedAt           *time.Time                 `json:"closedAt,omitempty"`
	Data               map[string]any             `json:"data,omitempty"`
	ID                 string                     `json:"id"`
	Namespace          string                     `json:"namespace"`
	ProductID          string                     `json:"productId,omitempty"`
	Subject            string                     `json:"subject,omitempty"`
	Kind               string                     `json:"kind"`
	Goal               string                     `json:"goal"`
	Autonomy           string                     `json:"autonomy,omitempty"`
	CreatedBy          string                     `json:"createdBy,omitempty"`
	ClosedBy           string                     `json:"closedBy,omitempty"`
	ClosedReason       string                     `json:"closedReason,omitempty"`
	Origin             string                     `json:"origin"`
	OriginRef          string                     `json:"originRef,omitempty"`
	Status             string                     `json:"status"`
	Pins               []MissionRevisionReference `json:"pins,omitempty"`
	Chain              []map[string]any           `json:"chain"`
	BudgetTokensTotal  int64                      `json:"budgetTokensTotal"`
	BudgetTokensPerDay int64                      `json:"budgetTokensPerDay"`
	SpentTokens        int64                      `json:"spentTokens"`
	BudgetUSDCap       float64                    `json:"budgetUsdCap"`
	StepCount          int                        `json:"stepCount"`
}

// MissionStartResponse reports whether a physical mission was created. A false
// Created value means OriginRef idempotency returned the existing mission.
type MissionStartResponse struct {
	Mission UnifiedMission `json:"mission"`
	Created bool           `json:"created"`
}

// MissionCancelRequest is the durable operator intent accepted by Cancel.
// Reuse IdempotencyKey after a lost response to receive the original receipt.
type MissionCancelRequest struct {
	Reason         string `json:"reason,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// MissionAnswerRequest is the durable answer accepted while a mission waits
// for human input. Reuse IdempotencyKey after a lost response.
type MissionAnswerRequest struct {
	Answer         string `json:"answer"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

// MissionControlReceipt is the public, restart-stable control projection.
type MissionControlReceipt struct {
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	CommittedAt      *time.Time `json:"committedAt,omitempty"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
	ID               string     `json:"id"`
	MissionID        string     `json:"missionId"`
	IdempotencyKey   string     `json:"idempotencyKey"`
	Action           string     `json:"action"`
	Phase            string     `json:"phase"`
	Outcome          string     `json:"outcome"`
	Reason           string     `json:"reason,omitempty"`
	Resolution       string     `json:"resolution,omitempty"`
	ResolvedBy       string     `json:"resolvedBy,omitempty"`
	ResolutionReason string     `json:"resolutionReason,omitempty"`
}

// MissionExecutionBinding is the public, durable cancellation evidence for one
// exact child execution. Cluster lease and fencing internals are intentionally
// not part of this projection.
type MissionExecutionBinding struct {
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	ID            string    `json:"id"`
	NodeID        string    `json:"nodeId"`
	ExternalID    string    `json:"externalId"`
	Kind          string    `json:"kind"`
	State         string    `json:"state"`
	GraphRevision int64     `json:"graphRevision"`
	Generation    int64     `json:"generation"`
	CancelEpoch   int64     `json:"cancelEpoch,omitempty"`
	DeliveryCount int       `json:"deliveryCount,omitempty"`
}

// MissionControlResponse returns the current mission and its durable receipt.
type MissionControlResponse struct {
	Error                      map[string]any            `json:"error,omitempty"`
	ExecutionBindings          []MissionExecutionBinding `json:"executionBindings"`
	Mission                    UnifiedMission            `json:"mission"`
	Control                    MissionControlReceipt     `json:"control"`
	ExecutionBindingsAvailable bool                      `json:"executionBindingsAvailable"`
	Pending                    bool                      `json:"pending,omitempty"`
}

// MissionArchiveEnvelope is the portable digest-bound Mission, immutable
// Brief, and complete journal returned by ExportArchive.
type MissionArchiveEnvelope struct {
	Digest  string          `json:"digest"`
	Payload json.RawMessage `json:"payload"`
}

// MissionArchiveRestoreResponse distinguishes a physical restore from an
// exact idempotent replay of an archive already present in the namespace.
type MissionArchiveRestoreResponse struct {
	ID      string `json:"id"`
	Digest  string `json:"digest"`
	Created bool   `json:"created"`
}

// AutonomousMissionBudgetHint is the wire budget accepted by mission intake.
type AutonomousMissionBudgetHint struct {
	USDCap       float64 `json:"usd_cap,omitempty"`
	TokensTotal  int64   `json:"tokens_total,omitempty"`
	TokensPerDay int64   `json:"tokens_per_day,omitempty"`
}

// AutonomousMissionContextRef points mission intake at reusable product context.
type AutonomousMissionContextRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// AutonomousMissionSubmitRequest is the public intent payload accepted by mission intake.
type AutonomousMissionSubmitRequest struct {
	Goal        string                        `json:"goal"`
	ProductURL  string                        `json:"productUrl,omitempty"`
	TraceID     string                        `json:"traceId,omitempty"`
	DedupKey    string                        `json:"dedupKey,omitempty"`
	MissionID   string                        `json:"missionId,omitempty"`
	Autonomy    string                        `json:"autonomy,omitempty"`
	Options     []string                      `json:"options,omitempty"`
	ContextRefs []AutonomousMissionContextRef `json:"contextRefs,omitempty"`
	Budget      AutonomousMissionBudgetHint   `json:"budget,omitempty"`
}

// AutonomousMissionSubmitResponse identifies the accepted durable mission.
type AutonomousMissionSubmitResponse struct {
	MissionID string `json:"missionId"`
	Status    string `json:"status"`
}

// AutonomousMissionBudget is the camel-case budget returned by mission reads.
type AutonomousMissionBudget struct {
	USDCap       float64 `json:"usdCap,omitempty"`
	TokensTotal  int64   `json:"tokensTotal,omitempty"`
	TokensPerDay int64   `json:"tokensPerDay,omitempty"`
}

// AutonomousMission is the stable mission projection returned by read endpoints.
type AutonomousMission struct {
	LeaseExpiresAt    *time.Time                    `json:"leaseExpiresAt,omitempty"`
	CreatedAt         time.Time                     `json:"createdAt"`
	UpdatedAt         time.Time                     `json:"updatedAt"`
	ID                string                        `json:"id"`
	Namespace         string                        `json:"namespace"`
	UserID            string                        `json:"userId,omitempty"`
	Goal              string                        `json:"goal"`
	TraceID           string                        `json:"traceId,omitempty"`
	Status            string                        `json:"status"`
	Autonomy          string                        `json:"autonomy"`
	Source            string                        `json:"source,omitempty"`
	SourceRef         string                        `json:"sourceRef,omitempty"`
	AwaitingQuestion  string                        `json:"awaitingQuestion,omitempty"`
	AwaitingRequestID string                        `json:"awaitingRequestId,omitempty"`
	Plan              string                        `json:"plan,omitempty"`
	LeaseOwner        string                        `json:"leaseOwner,omitempty"`
	ContextRefs       []AutonomousMissionContextRef `json:"contextRefs,omitempty"`
	Options           []string                      `json:"options,omitempty"`
	Budget            AutonomousMissionBudget       `json:"budget"`
	SpentTokens       int64                         `json:"spentTokens"`
	StepCount         int                           `json:"stepCount"`
	StepInProgress    bool                          `json:"stepInProgress"`
}

// AutonomousMissionListResponse is a namespace-scoped mission page.
type AutonomousMissionListResponse struct {
	Missions []AutonomousMission `json:"missions"`
	Total    int                 `json:"total"`
}

// AutonomousMissionFlow is the aggregated supervision payload. Step, artifact,
// and source maps deliberately preserve fields added by newer servers.
type AutonomousMissionFlow struct {
	Source    map[string]any    `json:"source,omitempty"`
	Steps     []map[string]any  `json:"steps"`
	Artifacts []map[string]any  `json:"artifacts"`
	Mission   AutonomousMission `json:"mission"`
}

func (a *AutonomousMissionsAPI) Submit(ctx context.Context, req AutonomousMissionSubmitRequest) (AutonomousMissionSubmitResponse, error) {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		return AutonomousMissionSubmitResponse{}, fmt.Errorf("mockarty: autonomous mission submit: goal is required")
	}
	if req.Autonomy != "" && req.Autonomy != "recon" && req.Autonomy != "propose" && req.Autonomy != "auto" {
		return AutonomousMissionSubmitResponse{}, fmt.Errorf("mockarty: autonomous mission submit: autonomy must be recon, propose, or auto")
	}
	if req.Budget.TokensTotal < 0 || req.Budget.TokensPerDay < 0 || req.Budget.USDCap < 0 ||
		math.IsNaN(req.Budget.USDCap) || math.IsInf(req.Budget.USDCap, 0) {
		return AutonomousMissionSubmitResponse{}, fmt.Errorf("mockarty: autonomous mission submit: budget values must be finite and non-negative")
	}
	var out AutonomousMissionSubmitResponse
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/autotester/intents", req, &out); err != nil {
		return AutonomousMissionSubmitResponse{}, err
	}
	return out, nil
}

// GetEffectiveSettings previews the exact layered settings a unified mission
// would snapshot. Keep SettingsDigest and pass it to Start after user review.
func (a *AutonomousMissionsAPI) GetEffectiveSettings(ctx context.Context, opts MissionEffectiveSettingsOptions) (MissionEffectiveSettings, error) {
	if opts.RunWindowMinutes < 0 || opts.RunWindowMinutes > 20160 {
		return MissionEffectiveSettings{}, fmt.Errorf("mockarty: mission effective settings: run window must be from 1 to 20160")
	}
	q := url.Values{}
	if productID := strings.TrimSpace(opts.ProductID); productID != "" {
		q.Set("productId", productID)
	}
	if missionID := strings.TrimSpace(opts.MissionID); missionID != "" {
		q.Set("missionId", missionID)
	}
	if opts.RunWindowMinutes > 0 {
		q.Set("runWindowMinutes", strconv.Itoa(opts.RunWindowMinutes))
	}
	path := "/api/v1/missions/settings/effective"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out MissionEffectiveSettings
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return MissionEffectiveSettings{}, err
	}
	if out.Settings == nil {
		out.Settings = []MissionEffectiveSetting{}
	}
	return out, nil
}

// Start starts a mission in the unified ledger. ExpectedSettingsDigest is
// optional for compatibility, but interactive clients should always pass it.
func (a *AutonomousMissionsAPI) Start(ctx context.Context, req MissionStartRequest) (MissionStartResponse, error) {
	req.Goal = strings.TrimSpace(req.Goal)
	if req.Goal == "" {
		return MissionStartResponse{}, fmt.Errorf("mockarty: mission start: goal is required")
	}
	req.ExpectedSettingsDigest = strings.TrimSpace(req.ExpectedSettingsDigest)
	if req.ExpectedSettingsDigest != "" && !missionSettingsDigestPattern.MatchString(req.ExpectedSettingsDigest) {
		return MissionStartResponse{}, fmt.Errorf("mockarty: mission start: expected settings digest must be canonical sha256")
	}
	if req.BudgetTokensTotal < 0 || req.BudgetTokensPerDay < 0 || req.BudgetUSDCap < 0 ||
		math.IsNaN(req.BudgetUSDCap) || math.IsInf(req.BudgetUSDCap, 0) {
		return MissionStartResponse{}, fmt.Errorf("mockarty: mission start: budget values must be finite and non-negative")
	}
	var out MissionStartResponse
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/missions", req, &out); err != nil {
		return MissionStartResponse{}, err
	}
	if out.Mission.Chain == nil {
		out.Mission.Chain = []map[string]any{}
	}
	return out, nil
}

// Cancel durably stops a unified mission and every unfinished component.
func (a *AutonomousMissionsAPI) Cancel(ctx context.Context, missionID string, req MissionCancelRequest) (MissionControlResponse, error) {
	missionID = strings.TrimSpace(missionID)
	if missionID == "" {
		return MissionControlResponse{}, fmt.Errorf("mockarty: mission cancel: mission id is required")
	}
	req.Reason = strings.TrimSpace(req.Reason)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	var out MissionControlResponse
	path := "/api/v1/missions/" + url.PathEscape(missionID) + "/cancel"
	if err := a.client.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return MissionControlResponse{}, err
	}
	if out.Mission.Chain == nil {
		out.Mission.Chain = []map[string]any{}
	}
	if out.ExecutionBindings == nil {
		out.ExecutionBindings = []MissionExecutionBinding{}
	}
	return out, nil
}

// Answer durably supplies the input requested by a unified mission.
func (a *AutonomousMissionsAPI) Answer(ctx context.Context, missionID string, req MissionAnswerRequest) (MissionControlResponse, error) {
	missionID = strings.TrimSpace(missionID)
	req.Answer = strings.TrimSpace(req.Answer)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if missionID == "" {
		return MissionControlResponse{}, fmt.Errorf("mockarty: mission answer: mission id is required")
	}
	if req.Answer == "" {
		return MissionControlResponse{}, fmt.Errorf("mockarty: mission answer: answer is required")
	}
	var out MissionControlResponse
	path := "/api/v1/missions/" + url.PathEscape(missionID) + "/answer"
	if err := a.client.do(ctx, http.MethodPost, path, req, &out); err != nil {
		return MissionControlResponse{}, err
	}
	if out.Mission.Chain == nil {
		out.Mission.Chain = []map[string]any{}
	}
	if out.ExecutionBindings == nil {
		out.ExecutionBindings = []MissionExecutionBinding{}
	}
	return out, nil
}

// ExportArchive exports one at-rest mission with its immutable Brief and
// complete journal. Active missions are rejected by the server.
func (a *AutonomousMissionsAPI) ExportArchive(ctx context.Context, missionID string) (MissionArchiveEnvelope, error) {
	missionID = strings.TrimSpace(missionID)
	if missionID == "" {
		return MissionArchiveEnvelope{}, fmt.Errorf("mockarty: mission archive export: mission id is required")
	}
	var out MissionArchiveEnvelope
	path := "/api/v1/missions/" + url.PathEscape(missionID) + "/archive"
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return MissionArchiveEnvelope{}, err
	}
	if err := validateMissionArchiveEnvelope(out); err != nil {
		return MissionArchiveEnvelope{}, fmt.Errorf("mockarty: mission archive export: %w", err)
	}
	return out, nil
}

// RestoreArchive atomically restores a Mission archive into its original
// namespace. Exact replay returns Created=false.
func (a *AutonomousMissionsAPI) RestoreArchive(ctx context.Context, archive MissionArchiveEnvelope) (MissionArchiveRestoreResponse, error) {
	if err := validateMissionArchiveEnvelope(archive); err != nil {
		return MissionArchiveRestoreResponse{}, fmt.Errorf("mockarty: mission archive restore: %w", err)
	}
	var out MissionArchiveRestoreResponse
	if err := a.client.do(ctx, http.MethodPost, "/api/v1/missions/archive", archive, &out); err != nil {
		return MissionArchiveRestoreResponse{}, err
	}
	return out, nil
}

func validateMissionArchiveEnvelope(archive MissionArchiveEnvelope) error {
	archive.Digest = strings.TrimSpace(archive.Digest)
	if !missionSettingsDigestPattern.MatchString(archive.Digest) {
		return fmt.Errorf("digest must be canonical sha256")
	}
	if len(archive.Payload) == 0 || !json.Valid(archive.Payload) {
		return fmt.Errorf("payload must be valid JSON")
	}
	return nil
}

func (a *AutonomousMissionsAPI) List(ctx context.Context, status string, limit int) (AutonomousMissionListResponse, error) {
	if limit < 0 || limit > 200 {
		return AutonomousMissionListResponse{}, fmt.Errorf("mockarty: autonomous mission list: limit must be from 0 to 200")
	}
	q := url.Values{}
	if status = strings.TrimSpace(status); status != "" {
		q.Set("status", status)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	path := "/api/v1/autotester/missions"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var out AutonomousMissionListResponse
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return AutonomousMissionListResponse{}, err
	}
	if out.Missions == nil {
		out.Missions = []AutonomousMission{}
	}
	return out, nil
}

func (a *AutonomousMissionsAPI) Get(ctx context.Context, missionID string) (AutonomousMission, error) {
	path, err := autonomousMissionPath(missionID)
	if err != nil {
		return AutonomousMission{}, err
	}
	var out AutonomousMission
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return AutonomousMission{}, err
	}
	return out, nil
}

func (a *AutonomousMissionsAPI) GetFlow(ctx context.Context, missionID string) (AutonomousMissionFlow, error) {
	path, err := autonomousMissionPath(missionID)
	if err != nil {
		return AutonomousMissionFlow{}, err
	}
	var out AutonomousMissionFlow
	if err := a.client.do(ctx, http.MethodGet, path+"/flow", nil, &out); err != nil {
		return AutonomousMissionFlow{}, err
	}
	if out.Steps == nil {
		out.Steps = []map[string]any{}
	}
	if out.Artifacts == nil {
		out.Artifacts = []map[string]any{}
	}
	return out, nil
}

func autonomousMissionPath(missionID string) (string, error) {
	missionID = strings.TrimSpace(missionID)
	if missionID == "" {
		return "", fmt.Errorf("mockarty: autonomous mission: mission id is required")
	}
	return "/api/v1/autotester/missions/" + url.PathEscape(missionID), nil
}
