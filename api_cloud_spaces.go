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

type CloudOrganizationSpace struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	Plan       string `json:"plan"`
	SeatsUsed  int    `json:"seats_used"`
	SeatsTotal int    `json:"seats_total"`
}

type CloudOrganizationMember struct {
	CooldownUntil  *time.Time `json:"cooldown_until,omitempty"`
	AffiliatedAt   time.Time  `json:"affiliated_at"`
	UserID         string     `json:"user_id"`
	Email          string     `json:"email"`
	Status         string     `json:"status"`
	Role           string     `json:"role,omitempty"`
	SpendAuthority bool       `json:"spend_authority"`
}

type CloudOrganizationLimits struct {
	People           int `json:"people"`
	SpaceMemberships int `json:"space_memberships"`
	Concurrency      int `json:"concurrency"`
}

type CloudOrganizationInvite struct {
	ExpiresAt time.Time `json:"expires_at"`
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
}

// CloudOrganizationSyncStorage is the Team's retained sync-data allowance.
// UsedBytes is a trustworthy account total only after Reconciled is true.
type CloudOrganizationSyncStorage struct {
	UsedBytes       int64 `json:"used_bytes"`
	LimitBytes      int64 `json:"limit_bytes"`
	SpaceLimitBytes int64 `json:"space_limit_bytes"`
	Reconciled      bool  `json:"reconciled"`
}

type CloudOrganization struct {
	Spaces         []CloudOrganizationSpace      `json:"spaces"`
	Members        []CloudOrganizationMember     `json:"members"`
	PendingInvites []CloudOrganizationInvite     `json:"pending_invites"`
	SyncStorage    *CloudOrganizationSyncStorage `json:"sync_storage,omitempty"`
	AccountID      string                        `json:"account_id"`
	AccountKind    string                        `json:"account_kind"`
	ViewerRole     string                        `json:"viewer_role,omitempty"`
	Status         string                        `json:"status"`
	PlanKind       string                        `json:"plan_kind"`
	Limits         CloudOrganizationLimits       `json:"limits"`
	SeatsAssigned  int                           `json:"seats_assigned"`
	SeatsReserved  int                           `json:"seats_reserved"`
	SeatsUsed      int                           `json:"seats_used"`
	SeatsTotal     int                           `json:"seats_total"`
}

// CloudTeamSpaceRequest creates a collaborative Space under an existing paid
// Team. Audience is "team" for all active Seat holders or "restricted" for an
// explicit Space ACL.
type CloudTeamSpaceRequest struct {
	TeamID   string `json:"team_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug,omitempty"`
	Audience string `json:"audience"`
}

type CloudTeamSpaceCreated struct {
	SpaceID   string `json:"space_id"`
	TeamID    string `json:"team_id"`
	RequestID string `json:"request_id"`
	Replayed  bool   `json:"replayed,omitempty"`
}

type CloudTeamMemberInviteRequest struct {
	SpaceID string `json:"space_id"`
	Email   string `json:"email"`
	Role    string `json:"role,omitempty"`
}

// CloudTeamAuditRecord is a redacted account or Space event. Audit payloads,
// session events and network identifiers are never exposed by this API.
type CloudTeamAuditRecord struct {
	OccurredAt  time.Time `json:"occurred_at"`
	ScopeID     string    `json:"scope_id"`
	ScopeKind   string    `json:"scope_kind"`
	ScopeName   string    `json:"scope_name,omitempty"`
	ActorUserID string    `json:"actor_user_id,omitempty"`
	Action      string    `json:"action"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id,omitempty"`
	ID          int64     `json:"id"`
}

type CloudTeamAuditPage struct {
	Records       []CloudTeamAuditRecord `json:"records"`
	TeamID        string                 `json:"team_id"`
	NextBeforeID  string                 `json:"next_before_id"`
	BrokenScopeID string                 `json:"broken_scope_id"`
	RequestID     string                 `json:"request_id"`
	BrokenAt      int64                  `json:"broken_at"`
	HasMore       bool                   `json:"has_more"`
	ChainOK       bool                   `json:"chain_ok"`
}

func (a *CloudSpacesAPI) requestContext(ctx context.Context, etag, idempotencyKey string) context.Context {
	headers := map[string]string{headerAPIKey: ""}
	if a.client.apiKey != "" {
		headers["Authorization"] = "Bearer " + a.client.apiKey
	}
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

// Organization reads the billing account of an explicit Space. Only that
// account's owner can see its members, Seats and other Spaces.
func (a *CloudSpacesAPI) Organization(ctx context.Context, spaceID string) (*CloudOrganization, error) {
	if _, err := cloudSpacePath(spaceID); err != nil {
		return nil, err
	}
	var out struct {
		Organization CloudOrganization `json:"organization"`
	}
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet,
		"/api/v1/cloud/organization?space_id="+url.QueryEscape(spaceID), nil, &out)
	return &out.Organization, err
}

// ListTeamAudit reads a Team-wide, newest-first audit page through an explicit
// Team Space. The caller needs an active Seat and owner or billing-manager role.
// Pass the returned NextBeforeID unchanged to fetch the next page; limit zero
// uses the server default of 50.
func (a *CloudSpacesAPI) ListTeamAudit(ctx context.Context, spaceID, beforeID string, limit int) (*CloudTeamAuditPage, error) {
	if _, err := cloudSpacePath(spaceID); err != nil {
		return nil, err
	}
	if limit < 0 || limit > 200 {
		return nil, fmt.Errorf("mockarty: Team audit limit must be between 1 and 200, or zero for the default")
	}
	values := url.Values{"space_id": {spaceID}}
	if beforeID != "" {
		parsed, err := strconv.ParseInt(beforeID, 10, 64)
		if err != nil || parsed < 1 {
			return nil, fmt.Errorf("mockarty: Team audit before id must be a positive decimal event id")
		}
		values.Set("before_id", beforeID)
	}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	var out CloudTeamAuditPage
	err := a.client.do(a.requestContext(ctx, "", ""), http.MethodGet,
		"/api/v1/cloud/team/audit?"+values.Encode(), nil, &out)
	return &out, err
}

// CreateTeamSpace adds a Space to an already active paid Team. Reuse the same
// idempotency key when retrying an uncertain response.
func (a *CloudSpacesAPI) CreateTeamSpace(ctx context.Context, request CloudTeamSpaceRequest, key string) (*CloudTeamSpaceCreated, error) {
	if strings.TrimSpace(request.TeamID) == "" || strings.TrimSpace(request.Name) == "" ||
		(request.Audience != "team" && request.Audience != "restricted") || strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("mockarty: Team id, Space name, audience and idempotency key are required")
	}
	var out CloudTeamSpaceCreated
	err := a.client.do(a.requestContext(ctx, "", key), http.MethodPost,
		"/api/v1/cloud/team/spaces", request, &out)
	return &out, err
}

// InviteTeamMember reserves a paid Seat and sends the invitation. The server
// acknowledges without exposing the invitation token to the caller.
func (a *CloudSpacesAPI) InviteTeamMember(ctx context.Context, request CloudTeamMemberInviteRequest, key string) error {
	if strings.TrimSpace(request.SpaceID) == "" || strings.TrimSpace(request.Email) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("mockarty: Team Space id, recipient email and idempotency key are required")
	}
	if request.Role != "member" && request.Role != "admin" && request.Role != "billing_manager" {
		return fmt.Errorf("mockarty: Team role must be member, admin or billing_manager")
	}
	return a.client.do(a.requestContext(ctx, "", key), http.MethodPost,
		"/api/v1/cloud/team/invites", request, nil)
}

// RevokeTeamInvite releases one pending Team Seat reservation. The idempotency
// key must be reused when retrying an uncertain response.
func (a *CloudSpacesAPI) RevokeTeamInvite(ctx context.Context, spaceID, inviteID, key string) error {
	if _, err := cloudSpacePath(spaceID); err != nil {
		return err
	}
	if strings.TrimSpace(inviteID) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("mockarty: Team invite id and idempotency key are required")
	}
	path := "/api/v1/cloud/team/invites/" + url.PathEscape(inviteID) + "?space_id=" + url.QueryEscape(spaceID)
	return a.client.do(a.requestContext(ctx, "", key), http.MethodDelete, path, nil, nil)
}

// RemoveTeamMember releases the person's Team Seat, Team Space access and Team
// Desktop authority. A member can pass their own ID to leave. Personal Spaces
// remain theirs; the current owner must transfer ownership before leaving.
func (a *CloudSpacesAPI) RemoveTeamMember(ctx context.Context, spaceID, memberID, key string) error {
	if _, err := cloudSpacePath(spaceID); err != nil {
		return err
	}
	if strings.TrimSpace(memberID) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("mockarty: Team member id and idempotency key are required")
	}
	path := "/api/v1/cloud/team/members/" + url.PathEscape(memberID) + "?space_id=" + url.QueryEscape(spaceID)
	return a.client.do(a.requestContext(ctx, "", key), http.MethodDelete, path, nil, nil)
}

// TransferTeamOwner appoints an active Team member to manage the Team account.
// The payer and owners of individual Spaces do not change.
func (a *CloudSpacesAPI) TransferTeamOwner(ctx context.Context, spaceID, successorUserID, key string) error {
	if _, err := cloudSpacePath(spaceID); err != nil {
		return err
	}
	if strings.TrimSpace(successorUserID) == "" || strings.TrimSpace(key) == "" {
		return fmt.Errorf("mockarty: successor user id and idempotency key are required")
	}
	return a.client.do(a.requestContext(ctx, "", key), http.MethodPost,
		"/api/v1/cloud/team/owner-transfer",
		map[string]string{"space_id": spaceID, "successor_user_id": successorUserID}, nil)
}

// Rename changes only the user-visible Space name and requires both revision
// and idempotency fencing for exact retries.
func (a *CloudSpacesAPI) Rename(ctx context.Context, spaceID, name, etag, key string) (*CloudSpace, error) {
	if err := requireCloudSpaceMutation(etag, key); err != nil {
		return nil, err
	}
	path, err := cloudSpacePath(spaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("mockarty: Space name is required")
	}
	var out struct {
		Space CloudSpace `json:"space"`
	}
	err = a.client.do(a.requestContext(ctx, etag, key), http.MethodPatch, path, map[string]string{"name": name}, &out)
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
