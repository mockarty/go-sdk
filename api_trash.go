// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TrashAPI covers the Recycle Bin feature surface:
//
//   - List / summary / settings for namespace and admin scopes.
//   - Single cascade restore and bulk restore (up to 100 groups per call).
//   - Bulk purge — irreversible hard-delete guarded by the confirmation
//     phrase "I understand this is permanent".
//   - Manual retention-scheduler tick (admin only).
//
// See internal/webui/integration_routes_trash*.go on the server side and
// docs/research/SOFT_DELETE_RESTORE.md for the full design.
type TrashAPI struct {
	client *Client
}

// ---------------------------------------------------------------------------
// Sentinels
// ---------------------------------------------------------------------------

// ErrTrashConfirmationMissing is returned by BulkPurge / AdminBulkPurge when
// the caller forgets to set Confirmation. This is a client-side guard so we
// never send a request that the server will reject with 400.
var ErrTrashConfirmationMissing = errors.New("mockarty: trash purge confirmation phrase is required")

// TrashPurgeConfirmationPhrase mirrors the server-side constant
// (internal/webui.TrashPurgeConfirmationPhrase). Exported so callers can
// set BulkPurgeRequest.Confirmation = mockarty.TrashPurgeConfirmationPhrase
// without magic literals.
const TrashPurgeConfirmationPhrase = "I understand this is permanent"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// TrashItem is the JSON projection of a single soft-deleted row.
//
// Fields ordered by descending alignment (24B time, then 16B strings, 8B int,
// 1B bool) per project convention.
type TrashItem struct {
	ClosedAt         time.Time `json:"closed_at"`
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Namespace        string    `json:"namespace"`
	EntityType       string    `json:"entity_type"`
	ClosedBy         string    `json:"closed_by,omitempty"`
	ClosedReason     string    `json:"closed_reason,omitempty"`
	CascadeGroupID   string    `json:"cascade_group_id,omitempty"`
	NumericID        int64     `json:"numeric_id,omitempty"`
	RestoreAvailable bool      `json:"restore_available"`
}

// TrashListOptions narrows a ListTrash / AdminListTrash call.
//
// All fields are optional. Zero values are omitted from the query string so
// the server applies its defaults (limit=50, offset=0, no filter).
type TrashListOptions struct {
	FromTime       time.Time
	ToTime         time.Time
	ClosedBy       string
	SearchQuery    string
	CascadeGroupID string
	EntityTypes    []string
	Limit          int
	Offset         int
}

// TrashListResult is the server envelope for list endpoints.
type TrashListResult struct {
	Items  []TrashItem `json:"items"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// TrashSummaryCount is one row in a TrashSummary response.
type TrashSummaryCount struct {
	EntityType string `json:"entity_type"`
	Count      int64  `json:"count"`
}

// TrashSummary is the per-entity-type aggregate used for badge rendering.
type TrashSummary struct {
	Counts []TrashSummaryCount `json:"counts"`
	Total  int64               `json:"total"`
}

// TrashSettings is the GET response body for retention settings.
type TrashSettings struct {
	UpdatedAt     string `json:"updated_at,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	UpdatedBy     string `json:"updated_by,omitempty"`
	Scope         string `json:"scope"`
	RetentionDays int    `json:"retention_days"`
	Enabled       bool   `json:"enabled"`
	Inherited     bool   `json:"inherited,omitempty"`
}

// TrashSettingsUpdate is the PUT request body.
type TrashSettingsUpdate struct {
	RetentionDays int  `json:"retention_days"`
	Enabled       bool `json:"enabled"`
}

// RestoreResult mirrors internal/model.RestoreResult.
type RestoreResult struct {
	CascadeGroupID string `json:"cascadeGroupId"`
	RestoredCount  int    `json:"restoredCount"`
	MissingCount   int    `json:"missingCount,omitempty"`
	ParentDeleted  bool   `json:"parentDeleted,omitempty"`
}

// BulkRestoreRequest is the body for POST /trash/restore.
type BulkRestoreRequest struct {
	CascadeGroupIDs []string `json:"cascade_group_ids"`
	Reason          string   `json:"reason,omitempty"`
}

// BulkRestoreOutcome documents a single group outcome in a bulk restore.
type BulkRestoreOutcome struct {
	CascadeGroupID string `json:"cascade_group_id"`
	EntityType     string `json:"entity_type,omitempty"`
	Error          string `json:"error,omitempty"`
	RestoredCount  int    `json:"restored_count"`
}

// BulkRestoreResult is the server envelope for bulk restore responses.
type BulkRestoreResult struct {
	Restored []BulkRestoreOutcome `json:"restored"`
	Failed   []BulkRestoreOutcome `json:"failed"`
	NotFound []string             `json:"not_found"`
}

// BulkPurgeRequest is the body for POST /trash/purge. The Confirmation
// field MUST equal TrashPurgeConfirmationPhrase exactly.
type BulkPurgeRequest struct {
	CascadeGroupIDs []string `json:"cascade_group_ids"`
	Confirmation    string   `json:"confirmation"`
	Reason          string   `json:"reason,omitempty"`
}

// BulkPurgeOutcome documents a single group outcome in a bulk purge.
type BulkPurgeOutcome struct {
	CascadeGroupID string `json:"cascade_group_id"`
	EntityType     string `json:"entity_type,omitempty"`
	Error          string `json:"error,omitempty"`
	RowsDeleted    int64  `json:"rows_deleted"`
}

// BulkPurgeResult is the server envelope for bulk purge responses.
type BulkPurgeResult struct {
	Purged   []BulkPurgeOutcome `json:"purged"`
	Failed   []BulkPurgeOutcome `json:"failed"`
	NotFound []string           `json:"not_found"`
}

// PurgeNowResult is the response of the manual retention-scheduler tick.
type PurgeNowResult struct {
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	PurgedTotal int64  `json:"purged_total"`
	Namespaces  int    `json:"namespaces_scanned"`
}

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

const trashAdminBase = "/api/v1/admin/trash"

func trashNamespaceBase(namespace string) (string, error) {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return "", fmt.Errorf("mockarty: trash: namespace is required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/trash", nil
}

// ---------------------------------------------------------------------------
// Shared query-builder
// ---------------------------------------------------------------------------

// buildQuery serialises the filter options into the wire format used by the
// list endpoints (?type=,&q=,&from=,...). Empty / zero values are omitted so
// server defaults kick in.
func (o TrashListOptions) buildQuery() string {
	q := url.Values{}
	if len(o.EntityTypes) > 0 {
		q.Set("type", strings.Join(o.EntityTypes, ","))
	}
	if o.SearchQuery != "" {
		q.Set("q", o.SearchQuery)
	}
	if !o.FromTime.IsZero() {
		q.Set("from", o.FromTime.UTC().Format(time.RFC3339))
	}
	if !o.ToTime.IsZero() {
		q.Set("to", o.ToTime.UTC().Format(time.RFC3339))
	}
	if o.ClosedBy != "" {
		q.Set("closed_by", o.ClosedBy)
	}
	if o.CascadeGroupID != "" {
		q.Set("cascade", o.CascadeGroupID)
	}
	if o.Limit > 0 {
		q.Set("limit", intToString(o.Limit))
	}
	if o.Offset > 0 {
		q.Set("offset", intToString(o.Offset))
	}
	if encoded := q.Encode(); encoded != "" {
		return "?" + encoded
	}
	return ""
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// ListTrash returns soft-deleted items in the given namespace.
func (a *TrashAPI) ListTrash(ctx context.Context, namespace string, opts TrashListOptions) (TrashListResult, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return TrashListResult{}, err
	}
	var out TrashListResult
	if err := a.client.do(ctx, http.MethodGet, base+opts.buildQuery(), nil, &out); err != nil {
		return TrashListResult{}, err
	}
	return out, nil
}

// AdminListTrash returns soft-deleted items across all namespaces.
// Requires platform admin or support role.
func (a *TrashAPI) AdminListTrash(ctx context.Context, opts TrashListOptions) (TrashListResult, error) {
	var out TrashListResult
	if err := a.client.do(ctx, http.MethodGet, trashAdminBase+opts.buildQuery(), nil, &out); err != nil {
		return TrashListResult{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

// TrashSummary returns per-entity-type counts for the given namespace.
func (a *TrashAPI) TrashSummary(ctx context.Context, namespace string) (TrashSummary, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return TrashSummary{}, err
	}
	var out TrashSummary
	if err := a.client.do(ctx, http.MethodGet, base+"/summary", nil, &out); err != nil {
		return TrashSummary{}, err
	}
	return out, nil
}

// AdminTrashSummary returns platform-wide per-entity-type counts.
func (a *TrashAPI) AdminTrashSummary(ctx context.Context) (TrashSummary, error) {
	var out TrashSummary
	if err := a.client.do(ctx, http.MethodGet, trashAdminBase+"/summary", nil, &out); err != nil {
		return TrashSummary{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// GetTrashSettings fetches the namespace retention settings. When no override
// exists the response carries Inherited=true and the global defaults.
func (a *TrashAPI) GetTrashSettings(ctx context.Context, namespace string) (TrashSettings, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return TrashSettings{}, err
	}
	var out TrashSettings
	if err := a.client.do(ctx, http.MethodGet, base+"/settings", nil, &out); err != nil {
		return TrashSettings{}, err
	}
	return out, nil
}

// UpdateTrashSettings upserts the namespace retention settings. Retention
// days must be 1..365.
func (a *TrashAPI) UpdateTrashSettings(ctx context.Context, namespace string, req TrashSettingsUpdate) (TrashSettings, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return TrashSettings{}, err
	}
	var out TrashSettings
	if err := a.client.do(ctx, http.MethodPut, base+"/settings", req, &out); err != nil {
		return TrashSettings{}, err
	}
	return out, nil
}

// GetGlobalTrashSettings fetches the platform-wide defaults.
func (a *TrashAPI) GetGlobalTrashSettings(ctx context.Context) (TrashSettings, error) {
	var out TrashSettings
	if err := a.client.do(ctx, http.MethodGet, trashAdminBase+"/settings/global", nil, &out); err != nil {
		return TrashSettings{}, err
	}
	return out, nil
}

// UpdateGlobalTrashSettings updates the platform-wide defaults (platform
// admin only — support role receives 403).
func (a *TrashAPI) UpdateGlobalTrashSettings(ctx context.Context, req TrashSettingsUpdate) (TrashSettings, error) {
	var out TrashSettings
	if err := a.client.do(ctx, http.MethodPut, trashAdminBase+"/settings/global", req, &out); err != nil {
		return TrashSettings{}, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Restore — single cascade
// ---------------------------------------------------------------------------

// RestoreCascade restores every row sharing the given cascade group id in the
// given namespace. Idempotent — a second call reports RestoredCount=0.
func (a *TrashAPI) RestoreCascade(ctx context.Context, namespace, cascadeGroupID string) (RestoreResult, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return RestoreResult{}, err
	}
	if strings.TrimSpace(cascadeGroupID) == "" {
		return RestoreResult{}, fmt.Errorf("mockarty: trash: cascade_group_id is required")
	}
	var out RestoreResult
	err = a.client.do(ctx, http.MethodPost,
		base+"/restore-cascade/"+url.PathEscape(cascadeGroupID),
		nil, &out)
	return out, err
}

// AdminRestoreCascade restores a cascade group regardless of namespace.
func (a *TrashAPI) AdminRestoreCascade(ctx context.Context, cascadeGroupID string) (RestoreResult, error) {
	if strings.TrimSpace(cascadeGroupID) == "" {
		return RestoreResult{}, fmt.Errorf("mockarty: trash: cascade_group_id is required")
	}
	var out RestoreResult
	err := a.client.do(ctx, http.MethodPost,
		trashAdminBase+"/restore-cascade/"+url.PathEscape(cascadeGroupID),
		nil, &out)
	return out, err
}

// ---------------------------------------------------------------------------
// Restore — bulk
// ---------------------------------------------------------------------------

// BulkRestore restores multiple cascade groups in a single call. The server
// accepts up to 100 groups per request. The response surfaces partial
// successes / failures — do not expect an all-or-nothing transaction.
func (a *TrashAPI) BulkRestore(ctx context.Context, namespace string, req BulkRestoreRequest) (BulkRestoreResult, error) {
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return BulkRestoreResult{}, err
	}
	if len(req.CascadeGroupIDs) == 0 {
		return BulkRestoreResult{}, fmt.Errorf("mockarty: trash: cascade_group_ids is required")
	}
	var out BulkRestoreResult
	err = a.client.do(ctx, http.MethodPost, base+"/restore", req, &out)
	return out, err
}

// AdminBulkRestore is the admin-scope counterpart of BulkRestore.
func (a *TrashAPI) AdminBulkRestore(ctx context.Context, req BulkRestoreRequest) (BulkRestoreResult, error) {
	if len(req.CascadeGroupIDs) == 0 {
		return BulkRestoreResult{}, fmt.Errorf("mockarty: trash: cascade_group_ids is required")
	}
	var out BulkRestoreResult
	err := a.client.do(ctx, http.MethodPost, trashAdminBase+"/restore", req, &out)
	return out, err
}

// ---------------------------------------------------------------------------
// Purge — bulk
// ---------------------------------------------------------------------------

// BulkPurge hard-deletes each listed cascade group. IRREVERSIBLE. The
// Confirmation field MUST be set to TrashPurgeConfirmationPhrase — the SDK
// enforces this client-side to avoid an ambiguous 400 response from the
// server.
func (a *TrashAPI) BulkPurge(ctx context.Context, namespace string, req BulkPurgeRequest) (BulkPurgeResult, error) {
	if req.Confirmation != TrashPurgeConfirmationPhrase {
		return BulkPurgeResult{}, ErrTrashConfirmationMissing
	}
	base, err := trashNamespaceBase(namespace)
	if err != nil {
		return BulkPurgeResult{}, err
	}
	if len(req.CascadeGroupIDs) == 0 {
		return BulkPurgeResult{}, fmt.Errorf("mockarty: trash: cascade_group_ids is required")
	}
	var out BulkPurgeResult
	err = a.client.do(ctx, http.MethodPost, base+"/purge", req, &out)
	return out, err
}

// AdminBulkPurge is the admin-scope counterpart of BulkPurge. Support role
// cannot purge — the server returns 403 if invoked by a support user.
func (a *TrashAPI) AdminBulkPurge(ctx context.Context, req BulkPurgeRequest) (BulkPurgeResult, error) {
	if req.Confirmation != TrashPurgeConfirmationPhrase {
		return BulkPurgeResult{}, ErrTrashConfirmationMissing
	}
	if len(req.CascadeGroupIDs) == 0 {
		return BulkPurgeResult{}, fmt.Errorf("mockarty: trash: cascade_group_ids is required")
	}
	var out BulkPurgeResult
	err := a.client.do(ctx, http.MethodPost, trashAdminBase+"/purge", req, &out)
	return out, err
}

// AdminPurgeNow triggers a synchronous tick of the retention scheduler.
// Platform admin only. Follower nodes respond with 503.
func (a *TrashAPI) AdminPurgeNow(ctx context.Context) (PurgeNowResult, error) {
	var out PurgeNowResult
	err := a.client.do(ctx, http.MethodPost, trashAdminBase+"/purge-now", nil, &out)
	return out, err
}
