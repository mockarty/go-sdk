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
	"time"
)

// CloudSpacesAPI is the curated Cloud collaboration surface. Every scoped
// method requires an explicit Space id; no client-side default is inferred.
type CloudSpacesAPI struct{ client *Client }

type CloudSpaceUsage struct {
	SeatsTotal               int  `json:"seats_total"`
	SeatsUsed                int  `json:"seats_used"`
	AcceptedHumans           int  `json:"accepted_humans"`
	PendingInvites           int  `json:"pending_invites"`
	FreeHumanLimit           int  `json:"free_human_limit"`
	FreeSpaceMembershipLimit int  `json:"free_space_membership_limit"`
	CanSpendFreeBenefit      bool `json:"can_spend_free_benefit"`
}

type CloudSpace struct {
	CreatedAt        time.Time       `json:"created_at"`
	ID               string          `json:"id"`
	BillingAccountID string          `json:"billing_account_id"`
	BenefitGroupID   string          `json:"benefit_group_id"`
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	Plan             string          `json:"plan"`
	Kind             string          `json:"kind"`
	Role             string          `json:"role"`
	Permissions      []string        `json:"permissions"`
	Usage            CloudSpaceUsage `json:"usage"`
	Revision         int64           `json:"revision"`
	Owned            bool            `json:"owned"`
}

type CloudSpaceMember struct {
	AddedAt  time.Time `json:"added_at"`
	ID       string    `json:"id"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
}

type CloudSpaceInvite struct {
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	Token       string    `json:"token,omitempty"`
}

type CloudSpacePage struct {
	Items              []CloudSpace `json:"items"`
	NextCursor         string       `json:"next_cursor"`
	CollectionRevision int64        `json:"collection_revision"`
	HasMore            bool         `json:"has_more"`
}

type CloudSpaceMemberPage struct {
	Items      []CloudSpaceMember `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type CloudSpaceInvitePage struct {
	Items      []CloudSpaceInvite `json:"items"`
	NextCursor string             `json:"next_cursor"`
	HasMore    bool               `json:"has_more"`
}

type CloudSpaceInviteRequest struct {
	Email          string `json:"email"`
	Role           string `json:"role,omitempty"`
	ExpiresInHours int    `json:"expires_in_hours,omitempty"`
}

type CloudSpaceMutation struct {
	SpaceID   string            `json:"space_id,omitempty"`
	Status    string            `json:"status"`
	Role      string            `json:"role,omitempty"`
	AcceptURL string            `json:"accept_url,omitempty"`
	Invite    *CloudSpaceInvite `json:"invite,omitempty"`
	Revision  int64             `json:"revision"`
}

type CloudSpaceInvitePreview struct {
	Invite CloudSpaceInvite `json:"invite"`
	ETag   string           `json:"etag"`
}

func (a *CloudSpacesAPI) requestContext(ctx context.Context, etag, idempotencyKey string) context.Context {
	headers := map[string]string{"Authorization": "Bearer " + a.client.apiKey}
	if etag != "" {
		headers["If-Match"] = etag
	}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	return withRequestHeaders(ctx, headers)
}

func cloudSpacePageQuery(cursor string, limit int) string {
	values := url.Values{}
	if cursor != "" {
		values.Set("cursor", cursor)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if encoded := values.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

func requireCloudSpaceMutation(etag, key string) error {
	if strings.TrimSpace(etag) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("mockarty: Space ETag and idempotency key are required")
	}
	return nil
}

func cloudSpacePath(spaceID string) (string, error) {
	if strings.TrimSpace(spaceID) == "" {
		return "", fmt.Errorf("mockarty: Space id is required")
	}
	return "/api/v1/cloud/spaces/" + url.PathEscape(spaceID), nil
}

func (a *CloudSpacesAPI) List(ctx context.Context, cursor string, limit int) (*CloudSpacePage, error) {
	var out CloudSpacePage
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet, "/api/v1/cloud/spaces"+cloudSpacePageQuery(cursor, limit), nil, &out)
	return &out, err
}

func (a *CloudSpacesAPI) Get(ctx context.Context, spaceID string) (*CloudSpace, error) {
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out struct {
		Space CloudSpace `json:"space"`
	}
	err = a.client.do(a.requestContext(ctx, "", ""), http.MethodGet, path, nil, &out)
	return &out.Space, err
}

func (a *CloudSpacesAPI) ListMembers(ctx context.Context, spaceID, cursor string, limit int) (*CloudSpaceMemberPage, error) {
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out CloudSpaceMemberPage
	err = a.client.do(a.requestContext(ctx, "", ""), http.MethodGet, path+"/members"+cloudSpacePageQuery(cursor, limit), nil, &out)
	return &out, err
}

func (a *CloudSpacesAPI) ListInvites(ctx context.Context, spaceID, cursor string, limit int) (*CloudSpaceInvitePage, error) {
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out CloudSpaceInvitePage
	err = a.client.do(a.requestContext(ctx, "", ""), http.MethodGet, path+"/invites"+cloudSpacePageQuery(cursor, limit), nil, &out)
	return &out, err
}

func (a *CloudSpacesAPI) PreviewInvite(ctx context.Context, token string) (*CloudSpaceInvitePreview, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("mockarty: invite token is required")
	}
	var out CloudSpaceInvitePreview
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet, "/api/v1/cloud/invites/"+url.PathEscape(token), nil, &out)
	return &out, err
}

func (a *CloudSpacesAPI) CreateInvite(ctx context.Context, spaceID string, request CloudSpaceInviteRequest, etag, key string) (*CloudSpaceMutation, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	var out CloudSpaceMutation
	err = a.client.do(a.requestContext(ctx, etag, key), http.MethodPost, path+"/invites", request, &out)
	return &out, err
}

func (a *CloudSpacesAPI) RevokeInvite(ctx context.Context, spaceID, inviteID, etag, key string) (*CloudSpaceMutation, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if inviteID == "" {
		return nil, fmt.Errorf("mockarty: invite id is required")
	}
	var out CloudSpaceMutation
	err = a.client.do(a.requestContext(ctx, etag, key), http.MethodDelete, path+"/invites/"+url.PathEscape(inviteID), nil, &out)
	return &out, err
}

func (a *CloudSpacesAPI) AcceptInvite(ctx context.Context, token, etag, key string) (*CloudSpaceMutation, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("mockarty: invite token is required")
	}
	var out CloudSpaceMutation
	err := a.client.do(a.requestContext(ctx, etag, key), http.MethodPost, "/api/v1/cloud/invites/"+url.PathEscape(token)+"/accept", map[string]any{}, &out)
	return &out, err
}

func (a *CloudSpacesAPI) UpdateMemberRole(ctx context.Context, spaceID, memberID, role, etag, key string) (*CloudSpaceMutation, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if memberID == "" {
		return nil, fmt.Errorf("mockarty: member id is required")
	}
	var out CloudSpaceMutation
	err = a.client.do(a.requestContext(ctx, etag, key), http.MethodPatch, path+"/members/"+url.PathEscape(memberID), map[string]any{"role": role}, &out)
	return &out, err
}

func (a *CloudSpacesAPI) RemoveMember(ctx context.Context, spaceID, memberID, etag, key string) (*CloudSpaceMutation, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if memberID == "" {
		return nil, fmt.Errorf("mockarty: member id is required")
	}
	var out CloudSpaceMutation
	err = a.client.do(a.requestContext(ctx, etag, key), http.MethodDelete, path+"/members/"+url.PathEscape(memberID), nil, &out)
	return &out, err
}
