// Copyright (c) 2026 Mockarty. All rights reserved.
// SPDX-License-Identifier: MIT

package mockarty

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// CloudTeamPackageQuote is a published, expiring offer for the owner of a
// Personal Space. Reading it neither reserves its price nor creates a Team.
type CloudTeamPackageQuote struct {
	ExpiresAt             time.Time `json:"expires_at"`
	PriceVersionID        string    `json:"price_version_id"`
	BaseProPriceVersionID string    `json:"base_pro_price_version_id"`
	PolicyBundleID        string    `json:"policy_bundle_id"`
	SellerCode            string    `json:"seller_code"`
	MarketCode            string    `json:"market_code"`
	Currency              string    `json:"currency"`
	BillingPeriod         string    `json:"billing_period"`
	AmountMinor           int64     `json:"amount_minor"`
	TaxMinor              int64     `json:"tax_minor"`
	TotalMinor            int64     `json:"total_minor"`
	Seats                 int       `json:"seats"`
	DiscountBPS           int       `json:"discount_bps"`
}

type CloudTeamBillingInvoice struct {
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	PeriodStart   time.Time  `json:"period_start"`
	PeriodEnd     time.Time  `json:"period_end"`
	ID            string     `json:"id"`
	Number        string     `json:"number"`
	Status        string     `json:"status"`
	Currency      string     `json:"currency"`
	TotalMinor    int64      `json:"total_minor"`
	RefundedMinor int64      `json:"refunded_minor"`
}

type CloudTeamBillingWallet struct {
	Kind           string `json:"kind"`
	UnitCode       string `json:"unit_code"`
	AvailableUnits int64  `json:"available_units"`
	HeldUnits      int64  `json:"held_units"`
	ConsumedUnits  int64  `json:"consumed_units"`
}

// CloudTeamBilling is available only to a current Team owner or billing
// manager. Monetary values use the currency's minor unit.
type CloudTeamBilling struct {
	PeriodStart    time.Time                 `json:"period_start"`
	PeriodEnd      time.Time                 `json:"period_end"`
	TeamID         string                    `json:"team_id"`
	SubscriptionID string                    `json:"subscription_id"`
	DefaultSpaceID string                    `json:"default_space_id"`
	Role           string                    `json:"role"`
	Status         string                    `json:"status"`
	PayerKind      string                    `json:"payer_kind"`
	PayerName      string                    `json:"payer_name,omitempty"`
	Currency       string                    `json:"currency"`
	BillingPeriod  string                    `json:"billing_period"`
	Invoices       []CloudTeamBillingInvoice `json:"invoices"`
	Wallets        []CloudTeamBillingWallet  `json:"wallets"`
	PendingChange  map[string]any            `json:"pending_change,omitempty"`
	PackageSeats   int                       `json:"package_seats"`
	AssignedSeats  int                       `json:"assigned_seats"`
	ReservedSeats  int                       `json:"reserved_seats"`
	DiscountBPS    int                       `json:"discount_bps"`
	NetPriceMinor  int64                     `json:"net_price_minor"`
	AutoRenew      bool                      `json:"auto_renew"`
}

// ListTeamPackages reads published package quotes for the owner of an explicit
// Personal Space. A quote can expire before checkout and grants no price lock.
func (a *CloudSpacesAPI) ListTeamPackages(ctx context.Context, personalSpaceID string) ([]CloudTeamPackageQuote, error) {
	if _, err := cloudSpacePath(personalSpaceID); err != nil {
		return nil, err
	}
	var out struct {
		Packages []CloudTeamPackageQuote `json:"packages"`
	}
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet,
		"/api/v1/cloud/team/packages?"+url.Values{"space_id": {personalSpaceID}}.Encode(), nil, &out)
	return out.Packages, err
}

// GetTeamBilling reads the selected paid Team account's billing summary. The
// caller must hold a current owner or billing-manager role on that Team.
func (a *CloudSpacesAPI) GetTeamBilling(ctx context.Context, teamSpaceID string) (*CloudTeamBilling, error) {
	if _, err := cloudSpacePath(teamSpaceID); err != nil {
		return nil, err
	}
	var out struct {
		Billing CloudTeamBilling `json:"billing"`
	}
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet,
		"/api/v1/cloud/team/billing?"+url.Values{"space_id": {teamSpaceID}}.Encode(), nil, &out)
	return &out.Billing, err
}
