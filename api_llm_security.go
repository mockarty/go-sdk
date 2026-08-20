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

type LLMSecurityAPI struct{ client *Client }

type LLMSecurityPolicy struct {
	SurfaceActions      map[string]string `json:"surfaceActions,omitempty"`
	RuleIDs             []string          `json:"ruleIds,omitempty"`
	BlockedCapabilities []string          `json:"blockedCapabilities,omitempty"`
	Mode                string            `json:"mode,omitempty"`
	Enabled             *bool             `json:"enabled,omitempty"`
	FailClosed          *bool             `json:"failClosed,omitempty"`
	MaxInputBytes       int64             `json:"maxInputBytes,omitempty"`
	MaxOutputBytes      int64             `json:"maxOutputBytes,omitempty"`
	MaxDecodedBytes     int64             `json:"maxDecodedBytes,omitempty"`
	BlockThreshold      uint16            `json:"blockThreshold,omitempty"`
	RedactThreshold     uint16            `json:"redactThreshold,omitempty"`
	MaxFindings         uint16            `json:"maxFindings,omitempty"`
	MaxDecodeCandidates uint8             `json:"maxDecodeCandidates,omitempty"`
	MaxDecodeDepth      uint8             `json:"maxDecodeDepth,omitempty"`
}

type LLMSecurityDelegation struct {
	Layers []string `json:"layers"`
	Kind   string   `json:"kind"`
	Key    string   `json:"key"`
	Item   string   `json:"item,omitempty"`
}

type LLMSecurityPolicyDocument struct {
	Value       *LLMSecurityPolicy      `json:"value,omitempty"`
	Additions   map[string][]string     `json:"additions,omitempty"`
	Denies      map[string][]string     `json:"denies,omitempty"`
	Allows      map[string][]string     `json:"allows,omitempty"`
	Caps        map[string]float64      `json:"caps,omitempty"`
	Delegations []LLMSecurityDelegation `json:"delegations,omitempty"`
}

type LLMSecuritySource struct {
	Layer    string `json:"layer"`
	ScopeID  string `json:"scopeId"`
	ActorID  string `json:"actorId,omitempty"`
	Revision int64  `json:"revision,omitempty"`
}

type LLMSecurityDeny struct {
	Authorities []LLMSecuritySource `json:"authorities"`
	Origin      LLMSecuritySource   `json:"origin"`
}

type LLMSecurityCapConstraint struct {
	Origin LLMSecuritySource `json:"origin"`
	Value  float64           `json:"value"`
}

type LLMSecurityCap struct {
	Constraints []LLMSecurityCapConstraint `json:"constraints"`
	Origin      LLMSecuritySource          `json:"origin"`
	Value       float64                    `json:"value"`
}

type LLMSecurityCapRelaxation struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
}

type LLMSecurityRelaxation struct {
	Cap       *LLMSecurityCapRelaxation `json:"cap,omitempty"`
	Authority LLMSecuritySource         `json:"authority"`
	Source    LLMSecuritySource         `json:"source"`
	Kind      string                    `json:"kind"`
	Key       string                    `json:"key"`
	Item      string                    `json:"item,omitempty"`
}

type LLMSecurityRestrictions struct {
	Denies      map[string]map[string]LLMSecurityDeny `json:"denies"`
	Caps        map[string]LLMSecurityCap             `json:"caps"`
	Relaxations []LLMSecurityRelaxation               `json:"relaxations"`
}

type LLMSecurityPolicyResponse struct {
	Effective        LLMSecurityPolicy         `json:"effective"`
	Document         LLMSecurityPolicyDocument `json:"document"`
	Restrictions     LLMSecurityRestrictions   `json:"restrictions"`
	Applied          []LLMSecuritySource       `json:"applied"`
	Mode             string                    `json:"mode"`
	Layer            string                    `json:"layer"`
	Namespace        string                    `json:"namespace,omitempty"`
	Revision         int64                     `json:"revision"`
	Active           bool                      `json:"active"`
	Local            bool                      `json:"local"`
	DeliveryDeferred bool                      `json:"deliveryDeferred,omitempty"`
}

type LLMSecurityPolicyRequest struct {
	Document         LLMSecurityPolicyDocument `json:"document"`
	Mode             string                    `json:"mode"`
	Active           *bool                     `json:"active,omitempty"`
	ExpectedRevision int64                     `json:"expectedRevision"`
}

type LLMSecuritySandboxRequest struct {
	Document         *LLMSecurityPolicyDocument `json:"document,omitempty"`
	Text             string                     `json:"text"`
	Mode             string                     `json:"mode,omitempty"`
	Surface          string                     `json:"surface,omitempty"`
	TrustClass       string                     `json:"trustClass,omitempty"`
	Active           *bool                      `json:"active,omitempty"`
	ExpectedRevision int64                      `json:"expectedRevision,omitempty"`
}

type LLMSecurityFinding struct {
	RuleID       string `json:"ruleId"`
	Category     string `json:"category"`
	Path         string `json:"path"`
	Fingerprint  string `json:"fingerprint"`
	Start        int    `json:"start"`
	End          int    `json:"end"`
	Score        uint16 `json:"score"`
	DecodedDepth uint8  `json:"decodedDepth"`
	Normalized   bool   `json:"normalized"`
}

type LLMSecuritySandboxResponse struct {
	Findings  []LLMSecurityFinding `json:"findings"`
	Decision  string               `json:"decision"`
	Mode      string               `json:"mode"`
	Score     uint16               `json:"score"`
	Truncated bool                 `json:"truncated"`
}

type LLMSecurityEvent struct {
	CreatedAt      time.Time `json:"createdAt"`
	Mode           string    `json:"mode"`
	Source         string    `json:"source"`
	Namespace      string    `json:"namespace,omitempty"`
	RuleID         string    `json:"ruleId"`
	ProfileID      string    `json:"profileId,omitempty"`
	Category       string    `json:"category"`
	Decision       string    `json:"decision"`
	Surface        string    `json:"surface"`
	TrustClass     string    `json:"trustClass"`
	Fingerprint    string    `json:"fingerprint,omitempty"`
	ID             int64     `json:"id"`
	LatencyUS      int64     `json:"latencyUs"`
	PolicyRevision int64     `json:"policyRevision"`
	Matches        int       `json:"matches"`
	Score          uint16    `json:"score"`
	Truncated      bool      `json:"truncated"`
}

type LLMSecurityEventsResponse struct {
	Events []LLMSecurityEvent `json:"events"`
}

func (a *LLMSecurityAPI) GetNamespacePolicy(ctx context.Context, namespace string) (LLMSecurityPolicyResponse, error) {
	return a.get(ctx, a.namespacePath(namespace)+"/policy")
}

func (a *LLMSecurityAPI) SaveNamespacePolicy(ctx context.Context, namespace string, request LLMSecurityPolicyRequest) (LLMSecurityPolicyResponse, error) {
	return a.put(ctx, a.namespacePath(namespace)+"/policy", request)
}

func (a *LLMSecurityAPI) PreviewNamespacePolicy(ctx context.Context, namespace string, request LLMSecurityPolicyRequest) (LLMSecurityPolicyResponse, error) {
	if request.Mode == "" {
		request.Mode = "merge"
	}
	var out LLMSecurityPolicyResponse
	if err := a.client.do(ctx, http.MethodPost, a.namespacePath(namespace)+"/preview", request, &out); err != nil {
		return LLMSecurityPolicyResponse{}, err
	}
	normalizeLLMSecurityPolicyResponse(&out)
	return out, nil
}

func (a *LLMSecurityAPI) TestNamespaceText(ctx context.Context, namespace string, request LLMSecuritySandboxRequest) (LLMSecuritySandboxResponse, error) {
	if strings.TrimSpace(request.Text) == "" {
		return LLMSecuritySandboxResponse{}, fmt.Errorf("mockarty: LLM security test: text is required")
	}
	var out LLMSecuritySandboxResponse
	if err := a.client.do(ctx, http.MethodPost, a.namespacePath(namespace)+"/sandbox", request, &out); err != nil {
		return LLMSecuritySandboxResponse{}, err
	}
	if out.Findings == nil {
		out.Findings = []LLMSecurityFinding{}
	}
	return out, nil
}

func (a *LLMSecurityAPI) ListNamespaceEvents(ctx context.Context, namespace string, limit int) (LLMSecurityEventsResponse, error) {
	return a.listEvents(ctx, a.namespacePath(namespace)+"/events", limit)
}

func (a *LLMSecurityAPI) GetInstallationPolicy(ctx context.Context) (LLMSecurityPolicyResponse, error) {
	return a.get(ctx, "/api/v1/admin/llm-security/policy")
}

func (a *LLMSecurityAPI) SaveInstallationPolicy(ctx context.Context, request LLMSecurityPolicyRequest) (LLMSecurityPolicyResponse, error) {
	return a.put(ctx, "/api/v1/admin/llm-security/policy", request)
}

func (a *LLMSecurityAPI) ListInstallationEvents(ctx context.Context, limit int) (LLMSecurityEventsResponse, error) {
	return a.listEvents(ctx, "/api/v1/admin/llm-security/events", limit)
}

func (a *LLMSecurityAPI) listEvents(ctx context.Context, path string, limit int) (LLMSecurityEventsResponse, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return LLMSecurityEventsResponse{}, fmt.Errorf("mockarty: LLM security events: limit must be between 1 and 500")
	}
	var out LLMSecurityEventsResponse
	if err := a.client.do(ctx, http.MethodGet, fmt.Sprintf("%s?limit=%d", path, limit), nil, &out); err != nil {
		return LLMSecurityEventsResponse{}, err
	}
	if out.Events == nil {
		out.Events = []LLMSecurityEvent{}
	}
	return out, nil
}

func (a *LLMSecurityAPI) namespacePath(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = a.client.Namespace()
	}
	return "/api/v1/namespaces/" + url.PathEscape(namespace) + "/llm-security"
}

func (a *LLMSecurityAPI) get(ctx context.Context, path string) (LLMSecurityPolicyResponse, error) {
	var out LLMSecurityPolicyResponse
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return LLMSecurityPolicyResponse{}, err
	}
	normalizeLLMSecurityPolicyResponse(&out)
	return out, nil
}

func (a *LLMSecurityAPI) put(ctx context.Context, path string, request LLMSecurityPolicyRequest) (LLMSecurityPolicyResponse, error) {
	if request.Mode == "" {
		request.Mode = "merge"
	}
	var out LLMSecurityPolicyResponse
	if err := a.client.do(ctx, http.MethodPut, path, request, &out); err != nil {
		return LLMSecurityPolicyResponse{}, err
	}
	normalizeLLMSecurityPolicyResponse(&out)
	return out, nil
}

func normalizeLLMSecurityPolicyResponse(response *LLMSecurityPolicyResponse) {
	if response.Applied == nil {
		response.Applied = []LLMSecuritySource{}
	}
	if response.Restrictions.Relaxations == nil {
		response.Restrictions.Relaxations = []LLMSecurityRelaxation{}
	}
}
