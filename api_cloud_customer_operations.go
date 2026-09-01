// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CloudCustomerAPI exposes customer-authorized Cloud product operations.
type CloudCustomerAPI struct{ client *Client }

// CloudOperationsAPI exposes least-privilege Cloud operator operations.
type CloudOperationsAPI struct{ client *Client }

type CloudSupportOpenRequest struct {
	Subject        string `json:"subject"`
	Category       string `json:"category"`
	Priority       string `json:"priority"`
	IdempotencyKey string `json:"idempotency_key"`
	Message        string `json:"message"`
}

func cloudProductPageQuery(status, cursor string, limit int) string {
	values := url.Values{}
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if status != "" {
		values.Set("status", status)
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func cloudEntityPath(prefix, id, label string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("mockarty: %s is required", label)
	}
	return prefix + url.PathEscape(id), nil
}

func (a *CloudCustomerAPI) ListLoyaltyRedemptions(ctx context.Context, spaceID, cursor string, limit int) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, base+"/loyalty/redemptions"+cloudProductPageQuery("", cursor, limit), nil, &out)
	return out, err
}

func (a *CloudCustomerAPI) RedeemLoyalty(ctx context.Context, spaceID, code, region, idempotencyKey string) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, base+"/loyalty/redemptions", map[string]any{
		"code": code, "region": region, "idempotency_key": idempotencyKey,
	}, &out)
	return out, err
}

func (a *CloudCustomerAPI) ListSupportCases(ctx context.Context, spaceID, status, cursor string, limit int) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, base+"/support/cases"+cloudProductPageQuery(status, cursor, limit), nil, &out)
	return out, err
}

func (a *CloudCustomerAPI) OpenSupportCase(ctx context.Context, spaceID string, request CloudSupportOpenRequest) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.IdempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, base+"/support/cases", request, &out)
	return out, err
}

func (a *CloudCustomerAPI) GetSupportCase(ctx context.Context, spaceID, caseID string) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	path, err := cloudEntityPath(base+"/support/cases/", caseID, "case id")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return out, err
}

func (a *CloudCustomerAPI) ReplySupportCase(ctx context.Context, spaceID, caseID, body, idempotencyKey string) (map[string]any, error) {
	base, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	path, err := cloudEntityPath(base+"/support/cases/", caseID, "case id")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, path+"/messages", map[string]any{
		"body": body, "visibility": "customer", "idempotency_key": idempotencyKey,
	}, &out)
	return out, err
}

func (a *CloudCustomerAPI) GetRiskAppeal(ctx context.Context, caseID string) (map[string]any, error) {
	path, err := cloudEntityPath("/api/v1/cloud/risk/cases/", caseID, "case id")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path+"/appeal", nil, &out)
	return out, err
}

func (a *CloudCustomerAPI) SubmitRiskAppeal(ctx context.Context, caseID, reason, idempotencyKey string) (map[string]any, error) {
	path, err := cloudEntityPath("/api/v1/cloud/risk/cases/", caseID, "case id")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": idempotencyKey})
	var out map[string]any
	err = a.client.do(ctx, http.MethodPost, path+"/appeal", map[string]any{"reason": reason}, &out)
	return out, err
}

func (a *CloudOperationsAPI) ListSupportCases(ctx context.Context, status, cursor string, limit int) (map[string]any, error) {
	var out map[string]any
	err := a.client.do(a.client.cloudContext(ctx), http.MethodGet, "/api/v1/cloud/operator/support/cases"+cloudProductPageQuery(status, cursor, limit), nil, &out)
	return out, err
}

func (a *CloudOperationsAPI) supportCasePath(caseID string) (string, error) {
	return cloudEntityPath("/api/v1/cloud/operator/support/cases/", caseID, "case id")
}

func (a *CloudOperationsAPI) GetSupportCase(ctx context.Context, caseID string) (map[string]any, error) {
	path, err := a.supportCasePath(caseID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodGet, path, nil, &out)
	return out, err
}

func (a *CloudOperationsAPI) ReplySupportCase(ctx context.Context, caseID, body, visibility, idempotencyKey string) (map[string]any, error) {
	path, err := a.supportCasePath(caseID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, fmt.Errorf("mockarty: idempotency key is required")
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, path+"/messages", map[string]any{
		"body": body, "visibility": visibility, "idempotency_key": idempotencyKey,
	}, &out)
	return out, err
}

func (a *CloudOperationsAPI) AssignSupportCase(ctx context.Context, caseID, assigneeUserID string, expectedGeneration int64) (map[string]any, error) {
	path, err := a.supportCasePath(caseID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, path+"/assign", map[string]any{
		"assignee_user_id": assigneeUserID, "expected_generation": expectedGeneration,
	}, &out)
	return out, err
}

func (a *CloudOperationsAPI) TransitionSupportCase(ctx context.Context, caseID, status string, expectedGeneration int64) (map[string]any, error) {
	path, err := a.supportCasePath(caseID)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = a.client.do(a.client.cloudContext(ctx), http.MethodPost, path+"/transition", map[string]any{
		"status": status, "expected_generation": expectedGeneration,
	}, &out)
	return out, err
}

func (a *CloudOperationsAPI) ProductAnalytics(ctx context.Context, days int) (map[string]any, error) {
	if days < 1 || days > 90 {
		return nil, fmt.Errorf("mockarty: days must be between 1 and 90")
	}
	var out map[string]any
	err := a.client.do(a.client.cloudContext(ctx), http.MethodGet, "/api/v1/cloud/operator/analytics/product?days="+strconv.Itoa(days), nil, &out)
	return out, err
}
