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

// CloudEntitlementsAPI reads committed, unsigned Cloud entitlement
// projections. The projection is inspection data; it is not an offline grant.
type CloudEntitlementsAPI struct{ client *Client }

type CloudEntitlementModuleGrant struct {
	Key     string `json:"key"`
	Limit   int64  `json:"limit"`
	Enabled bool   `json:"enabled"`
}

type CloudEntitlementCapacityGrant struct {
	Kind     string `json:"kind"`
	Unit     string `json:"unit"`
	Currency string `json:"currency,omitempty"`
	Included int64  `json:"included"`
	Reserved int64  `json:"reserved"`
}

type CloudEntitlementSnapshot struct {
	IssuedAt           time.Time                       `json:"issued_at"`
	NotBefore          time.Time                       `json:"not_before"`
	EffectiveAt        time.Time                       `json:"effective_at"`
	ExpiresAt          time.Time                       `json:"expires_at"`
	GraceUntil         time.Time                       `json:"grace_until"`
	RevokedAt          *time.Time                      `json:"revoked_at,omitempty"`
	SchemaVersion      string                          `json:"schema_version"`
	SubjectID          string                          `json:"subject_id"`
	BillingAccountID   string                          `json:"billing_account_id"`
	BenefitGroupID     string                          `json:"benefit_group_id"`
	SpaceID            string                          `json:"space_id"`
	Product            string                          `json:"product"`
	Plan               string                          `json:"plan"`
	PolicyVersion      string                          `json:"policy_version"`
	IssuanceID         string                          `json:"issuance_id"`
	AuthorityDomain    string                          `json:"authority_domain"`
	KeyID              string                          `json:"key_id"`
	KeyDomain          string                          `json:"key_domain"`
	RevocationReason   string                          `json:"revocation_reason,omitempty"`
	Modules            []CloudEntitlementModuleGrant   `json:"modules"`
	Capacity           []CloudEntitlementCapacityGrant `json:"capacity"`
	RequiredExtensions []string                        `json:"required_extensions,omitempty"`
	Extensions         map[string]any                  `json:"extensions,omitempty"`
	Revision           int64                           `json:"revision"`
	HumanSeats         int32                           `json:"human_seats"`
	ServiceAccounts    int32                           `json:"service_accounts"`
}

type CloudEntitlementProjection struct {
	Snapshot CloudEntitlementSnapshot `json:"snapshot"`
	Digest   string                   `json:"digest"`
	Revision int64                    `json:"revision"`
}

// Get returns the caller's committed entitlement projection for one explicit
// Space. A 409 from Cloud means the commercial revision is not reconciled yet.
func (a *CloudEntitlementsAPI) Get(ctx context.Context, spaceID string) (*CloudEntitlementProjection, error) {
	if strings.TrimSpace(spaceID) == "" {
		return nil, fmt.Errorf("mockarty: Space id is required")
	}
	values := url.Values{"space_id": []string{spaceID}}
	var out CloudEntitlementProjection
	err := a.client.do(ctx, http.MethodGet,
		"/api/v1/cloud/entitlements?"+values.Encode(), nil, &out)
	return &out, err
}
