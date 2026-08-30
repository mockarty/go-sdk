// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type CloudRiskAPI struct{ client *Client }

type CloudRiskCase struct {
	OpenedAt        time.Time  `json:"opened_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	ID              string     `json:"id"`
	SignalType      string     `json:"signal_type"`
	ScopeType       string     `json:"scope_type"`
	EnforcementKind string     `json:"enforcement_kind"`
	Status          string     `json:"status"`
	Severity        string     `json:"severity"`
	ReasonCode      string     `json:"reason_code"`
	Decision        string     `json:"decision"`
	Revision        int64      `json:"revision"`
}

type CloudRiskEvent struct {
	OccurredAt  time.Time         `json:"occurred_at"`
	ID          string            `json:"id"`
	Evidence    map[string]string `json:"evidence"`
	SignalType  string            `json:"signal_type"`
	Provider    string            `json:"provider,omitempty"`
	Currency    string            `json:"currency"`
	Decision    string            `json:"decision"`
	ReasonCode  string            `json:"reason_code"`
	AmountMinor int64             `json:"amount_minor"`
}

type CloudRiskEnforcement struct {
	StartsAt      time.Time  `json:"starts_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ID            string     `json:"id"`
	CaseID        string     `json:"case_id"`
	ScopeType     string     `json:"scope_type"`
	SignalType    string     `json:"signal_type"`
	Kind          string     `json:"kind"`
	Status        string     `json:"status"`
	ReasonCode    string     `json:"reason_code"`
	ReleaseReason string     `json:"release_reason,omitempty"`
	Revision      int64      `json:"revision"`
}

type CloudRiskCaseDetail struct {
	Case         CloudRiskCase          `json:"case"`
	Events       []CloudRiskEvent       `json:"events"`
	Enforcements []CloudRiskEnforcement `json:"enforcements"`
}

func (a *CloudRiskAPI) ListCases(ctx context.Context, status string, limit int) ([]CloudRiskCase, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("mockarty: limit must be between 1 and 100")
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}}
	if status != "" {
		query.Set("status", status)
	}
	var out struct {
		Cases []CloudRiskCase `json:"cases"`
	}
	err := a.client.do(a.client.cloudContext(ctx), "GET", "/api/v1/cloud/operator/risk/cases?"+query.Encode(), nil, &out)
	return out.Cases, err
}

func (a *CloudRiskAPI) GetCase(ctx context.Context, caseID string) (*CloudRiskCaseDetail, error) {
	if caseID == "" {
		return nil, fmt.Errorf("mockarty: case id is required")
	}
	var out CloudRiskCaseDetail
	err := a.client.do(a.client.cloudContext(ctx), "GET", "/api/v1/cloud/operator/risk/cases/"+url.PathEscape(caseID), nil, &out)
	return &out, err
}

func (a *CloudRiskAPI) ReleaseEnforcement(ctx context.Context, caseID, enforcementID string, revision int64, reason string) (*CloudRiskEnforcement, error) {
	reason = strings.TrimSpace(reason)
	if caseID == "" || enforcementID == "" || revision < 1 || !utf8.ValidString(reason) ||
		utf8.RuneCountInString(reason) < 3 || utf8.RuneCountInString(reason) > 512 {
		return nil, fmt.Errorf("mockarty: case id, enforcement id, positive revision, and release reason are required")
	}
	var out struct {
		Enforcement CloudRiskEnforcement `json:"enforcement"`
	}
	digest := sha256.Sum256([]byte(caseID + "\x00" + enforcementID + "\x00" + strconv.FormatInt(revision, 10) + "\x00" + reason))
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": "risk-release:" + hex.EncodeToString(digest[:])})
	err := a.client.do(ctx, "POST", "/api/v1/cloud/operator/risk/cases/"+url.PathEscape(caseID)+
		"/enforcements/"+url.PathEscape(enforcementID)+"/release", map[string]any{"revision": revision, "reason": reason}, &out)
	return &out.Enforcement, err
}
