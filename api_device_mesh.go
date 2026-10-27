// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceMeshAPI lends machines to a workspace and places work on them. It is the
// SDK face of the device mesh: what a CI job or a script needs in order to put a
// command on somebody's computer and read the receipt back, without knowing the
// REST route shapes.
//
// Two operational rules the server enforces and this API does not soften:
//   - only a device's owner can lend it (the grant), so an agent cannot grant
//     itself access;
//   - claim/report are owner-only, while submit needs a grant covering the
//     command's capabilities.
type DeviceMeshAPI struct{ client *Client }

// MeshDevice is one machine, phone or runner registered in the mesh. Presence is
// derived from the heartbeat, so State reports "offline" for a device that
// stopped reporting without anything being deleted.
type MeshDevice struct {
	HeartbeatAt  time.Time         `json:"heartbeatAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	CreatedAt    time.Time         `json:"createdAt"`
	Labels       map[string]string `json:"labels,omitempty"`
	DeviceID     string            `json:"deviceId"`
	Namespace    string            `json:"namespace"`
	Kind         string            `json:"kind"`
	State        string            `json:"state"`
	OwnerUserID  string            `json:"ownerUserId"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

// MeshGrant is one owner's decision to lend a device to a colleague for a set
// of capabilities. An empty capability list keeps the original meaning: every
// kind of work the device advertises.
type MeshGrant struct {
	CreatedAt     time.Time `json:"createdAt"`
	DeviceID      string    `json:"deviceId"`
	Namespace     string    `json:"namespace"`
	GranteeUserID string    `json:"granteeUserId"`
	Capabilities  []string  `json:"capabilities,omitempty"`
}

// MeshCommand is one piece of work on one device (or unbound, when placement
// found no candidate — see PlacementReason).
type MeshCommand struct {
	QueuedAt             time.Time       `json:"queuedAt"`
	ClaimedAt            *time.Time      `json:"claimedAt,omitempty"`
	StartedAt            *time.Time      `json:"startedAt,omitempty"`
	FinishedAt           *time.Time      `json:"finishedAt,omitempty"`
	LeaseExpiresAt       *time.Time      `json:"leaseExpiresAt,omitempty"`
	Payload              json.RawMessage `json:"payload,omitempty"`
	ID                   string          `json:"id"`
	Namespace            string          `json:"namespace"`
	Kind                 string          `json:"kind"`
	State                string          `json:"state"`
	TargetDeviceID       string          `json:"targetDeviceId,omitempty"`
	LeaseOwner           string          `json:"leaseOwner,omitempty"`
	PlacementReason      string          `json:"placementReason,omitempty"`
	ResultSummary        string          `json:"resultSummary,omitempty"`
	ResultRef            string          `json:"resultRef,omitempty"`
	FailureReason        string          `json:"failureReason,omitempty"`
	RequiredCapabilities []string        `json:"requiredCapabilities,omitempty"`
}

// MeshExecutionReport is the durable receipt of a finished command. It is what a
// CI job cites: who ran what, where, and how it ended.
type MeshExecutionReport struct {
	RecordedAt   time.Time  `json:"recordedAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
	ID           string     `json:"id"`
	Ref          string     `json:"ref"`
	Namespace    string     `json:"namespace"`
	DeviceID     string     `json:"deviceId"`
	SubmitterID  string     `json:"submitterUserId"`
	State        string     `json:"state"`
	Summary      string     `json:"summary"`
	FailureCode  string     `json:"failureCode,omitempty"`
	ReportKind   string     `json:"reportKind,omitempty"`
	SubjectID    string     `json:"subjectId,omitempty"`
}

// MeshSubmitInput describes work to place. RequiredCapabilities is the closed
// capability vocabulary (api_test, ui_test_browser, ui_test_mobile, perf, fuzz,
// chaos, coder, computer_use); an empty TargetDeviceID lets the server choose a
// candidate, and the returned command then carries a PlacementReason instead.
type MeshSubmitInput struct {
	Payload              json.RawMessage
	Labels               map[string]string
	Kind                 string
	TargetDeviceID       string
	RequiredCapabilities []string
	IdempotencyKey       string
}

func (a *DeviceMeshAPI) base(namespace string) (string, error) {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return "", fmt.Errorf("mockarty: device mesh: namespace required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/device-mesh", nil
}

// ListDevices returns the devices this caller may place work on.
func (a *DeviceMeshAPI) ListDevices(ctx context.Context, namespace string) ([]MeshDevice, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	var out struct {
		Devices []MeshDevice `json:"devices"`
	}
	if err := a.client.do(ctx, http.MethodGet, base+"/devices", nil, &out); err != nil {
		return nil, err
	}
	return out.Devices, nil
}

// CreateGrant lends the device to granteeUserID for the given capabilities. Only
// the device's owner may do this.
func (a *DeviceMeshAPI) CreateGrant(ctx context.Context, namespace, deviceID, granteeUserID string, capabilities []string) (MeshGrant, error) {
	base, err := a.base(namespace)
	if err != nil {
		return MeshGrant{}, err
	}
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(granteeUserID) == "" {
		return MeshGrant{}, fmt.Errorf("mockarty: device mesh: device id and grantee are required")
	}
	body := map[string]any{"granteeUserId": granteeUserID}
	if len(capabilities) > 0 {
		body["capabilities"] = capabilities
	}
	var out MeshGrant
	path := base + "/devices/" + url.PathEscape(deviceID) + "/grants"
	if err := a.client.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return MeshGrant{}, err
	}
	return out, nil
}

// RevokeGrant withdraws a lend. The grantee keeps nothing: the next submit for
// that device and capability is refused.
func (a *DeviceMeshAPI) RevokeGrant(ctx context.Context, namespace, deviceID, granteeUserID string) error {
	base, err := a.base(namespace)
	if err != nil {
		return err
	}
	path := base + "/devices/" + url.PathEscape(deviceID) + "/grants/" + url.PathEscape(granteeUserID)
	return a.client.do(ctx, http.MethodDelete, path, nil, nil)
}

// SubmitCommand queues work. The boolean reports whether the idempotency key
// replayed an existing command — a retry after a lost response must not place a
// second piece of work.
func (a *DeviceMeshAPI) SubmitCommand(ctx context.Context, namespace string, in MeshSubmitInput) (MeshCommand, bool, error) {
	base, err := a.base(namespace)
	if err != nil {
		return MeshCommand{}, false, err
	}
	if strings.TrimSpace(in.Kind) == "" || strings.TrimSpace(in.IdempotencyKey) == "" {
		return MeshCommand{}, false, fmt.Errorf("mockarty: device mesh: kind and idempotency key are required")
	}
	body := map[string]any{
		"kind":           in.Kind,
		"idempotencyKey": in.IdempotencyKey,
		"payload":        in.Payload,
	}
	if len(in.RequiredCapabilities) > 0 {
		body["requiredCapabilities"] = in.RequiredCapabilities
	}
	if in.TargetDeviceID != "" {
		body["targetDeviceId"] = in.TargetDeviceID
	}
	if len(in.Labels) > 0 {
		body["targetLabels"] = in.Labels
	}
	var out struct {
		Command  MeshCommand `json:"command"`
		Replayed bool        `json:"replayed"`
	}
	if err := a.client.do(ctx, http.MethodPost, base+"/commands", body, &out); err != nil {
		return MeshCommand{}, false, err
	}
	return out.Command, out.Replayed, nil
}

// GetCommand reads one command's current state, including the terminal state and
// the failure reason when it ended badly.
func (a *DeviceMeshAPI) GetCommand(ctx context.Context, namespace, commandID string) (MeshCommand, error) {
	base, err := a.base(namespace)
	if err != nil {
		return MeshCommand{}, err
	}
	if strings.TrimSpace(commandID) == "" {
		return MeshCommand{}, fmt.Errorf("mockarty: device mesh: command id required")
	}
	var out MeshCommand
	path := base + "/commands/" + url.PathEscape(commandID)
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return MeshCommand{}, err
	}
	return out, nil
}

// ReportCommand records the outcome as the executing device. Only the device
// that holds the lease may report; a terminal report is final.
func (a *DeviceMeshAPI) ReportCommand(ctx context.Context, namespace, deviceID, commandID string, report MeshCommandReport) (MeshCommand, error) {
	base, err := a.base(namespace)
	if err != nil {
		return MeshCommand{}, err
	}
	if strings.TrimSpace(deviceID) == "" || strings.TrimSpace(commandID) == "" {
		return MeshCommand{}, fmt.Errorf("mockarty: device mesh: device id and command id are required")
	}
	body := map[string]any{"state": report.State}
	if report.ResultSummary != "" {
		body["resultSummary"] = report.ResultSummary
	}
	if report.ResultRef != "" {
		body["resultRef"] = report.ResultRef
	}
	if report.FailureReason != "" {
		body["failureReason"] = report.FailureReason
	}
	path := base + "/commands/" + url.PathEscape(commandID) + "/report?deviceId=" + url.QueryEscape(deviceID)
	var out MeshCommand
	if err := a.client.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return MeshCommand{}, err
	}
	return out, nil
}

// MeshCommandReport is the body a device sends when it reports progress or the
// terminal outcome.
type MeshCommandReport struct {
	State         string
	ResultSummary string
	ResultRef     string
	FailureReason string
}

// ListReports returns the durable receipts for one command.
func (a *DeviceMeshAPI) ListReports(ctx context.Context, namespace, commandID string) ([]MeshExecutionReport, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(commandID) == "" {
		return nil, fmt.Errorf("mockarty: device mesh: command id required")
	}
	var out struct {
		Reports []MeshExecutionReport `json:"reports"`
	}
	path := base + "/reports?commandId=" + url.QueryEscape(commandID)
	if err := a.client.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.Reports, nil
}

// SetDrain records an operator decision that survives the device's own
// heartbeat: true stops new work reaching it, false releases an earlier drain.
// A device that chose to drain itself is a different thing and needs no operator.
func (a *DeviceMeshAPI) SetDrain(ctx context.Context, namespace, deviceID string, draining bool, reason string) (MeshDevice, error) {
	base, err := a.base(namespace)
	if err != nil {
		return MeshDevice{}, err
	}
	if strings.TrimSpace(deviceID) == "" {
		return MeshDevice{}, fmt.Errorf("mockarty: device mesh: device id required")
	}
	body := map[string]any{"draining": draining}
	if reason != "" {
		body["reason"] = reason
	}
	var out MeshDevice
	path := base + "/devices/" + url.PathEscape(deviceID) + "/drain"
	if err := a.client.do(ctx, http.MethodPost, path, body, &out); err != nil {
		return MeshDevice{}, err
	}
	return out, nil
}
