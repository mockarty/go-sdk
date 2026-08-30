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
