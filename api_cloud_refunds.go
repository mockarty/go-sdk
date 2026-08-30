// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// CloudRefundsAPI resolves provider-backed refunds held for operator action.
// The browser-session /api/v1/cloud/billing/refunds endpoint is deliberately
// absent: creating a refund is an interactive, action-bound step-up operation.
type CloudRefundsAPI struct{ client *Client }

type CloudRefundResolutionAction string

const (
	CloudRefundReject CloudRefundResolutionAction = "reject"
	CloudRefundRetry  CloudRefundResolutionAction = "retry"
)

type CloudRefundIncident struct {
	DeadlineAt               time.Time `json:"deadline_at"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	Provider                 string    `json:"provider"`
	ProviderAccountID        string    `json:"provider_account_id"`
	ProviderOperationID      string    `json:"provider_operation_id,omitempty"`
	Currency                 string    `json:"currency"`
	Status                   string    `json:"status"`
	OperationKind            string    `json:"operation_kind"`
	OperationID              string    `json:"operation_id"`
	OrderID                  string    `json:"order_id"`
	BillingAccountID         string    `json:"billing_account_id"`
	SpaceID                  string    `json:"space_id"`
	SubscriptionID           string    `json:"subscription_id"`
	InvoiceID                string    `json:"invoice_id"`
	ParentPaymentOperationID string    `json:"parent_payment_operation_id"`
	ConnectorVersionID       string    `json:"connector_version_id"`
	AmountMinor              int64     `json:"amount_minor"`
	Generation               int64     `json:"generation"`
	AttemptCount             int32     `json:"attempt_count"`
}

type CloudRefundResolution struct {
	Refund    CloudRefundIncident `json:"refund"`
	RequestID string              `json:"request_id"`
	Replayed  bool                `json:"replayed,omitempty"`
}

func validCloudRefundIncident(incident CloudRefundIncident) bool {
	return incident.OperationID != "" && incident.Generation >= 0 && incident.Status != "" &&
		incident.AmountMinor >= 0 && incident.Currency != "" && incident.Provider != ""
}

// ListRefunds returns the redacted, actionable refund projection from the
// operator commerce ledger. The caller needs the exact operator:commerce:write
// token scope (or an interactive operator session). Payment records sharing the
// transport envelope are deliberately ignored.
func (a *CloudRefundsAPI) ListRefunds(ctx context.Context) ([]CloudRefundIncident, error) {
	var out struct {
		Refunds []CloudRefundIncident `json:"refunds"`
	}
	if err := a.client.do(a.client.cloudContext(ctx), "GET", "/api/v1/cloud/operator/refunds", nil, &out); err != nil {
		return nil, err
	}
	if out.Refunds == nil {
		return nil, fmt.Errorf("mockarty: operator refunds response is missing refunds")
	}
	for _, refund := range out.Refunds {
		if !validCloudRefundIncident(refund) {
			return nil, fmt.Errorf("mockarty: operator refunds response contains an invalid refund projection")
		}
	}
	return out.Refunds, nil
}

var (
	cloudRefundReasonPattern      = regexp.MustCompile(`^[a-z0-9._:-]{2,64}$`)
	cloudRefundIdempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9._:/@-]{1,128}$`)
)

// ResolveRefund rejects an operator-required refund or reopens its durable
// provider recovery. It cannot manufacture a successful monetary outcome.
func (a *CloudRefundsAPI) ResolveRefund(
	ctx context.Context,
	operationID string,
	action CloudRefundResolutionAction,
	reasonCode string,
	generation int64,
	idempotencyKey string,
) (*CloudRefundResolution, error) {
	if strings.TrimSpace(operationID) == "" || (action != CloudRefundReject && action != CloudRefundRetry) ||
		generation < 0 || !cloudRefundReasonPattern.MatchString(reasonCode) ||
		!cloudRefundIdempotencyPattern.MatchString(idempotencyKey) {
		return nil, fmt.Errorf("mockarty: operation id, reject or retry action, safe reason code, non-negative generation, and safe idempotency key are required")
	}
	var out CloudRefundResolution
	ctx = a.client.cloudContextWithHeaders(ctx, map[string]string{"Idempotency-Key": idempotencyKey})
	err := a.client.do(ctx, "POST", "/api/v1/cloud/operator/refunds/"+url.PathEscape(operationID)+"/resolve",
		map[string]any{"action": action, "reason_code": reasonCode, "generation": generation}, &out)
	return &out, err
}
