// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// UndefinedAPI provides methods for managing undefined (unmatched) requests.
type UndefinedAPI struct {
	client *Client
}

// UndefinedRequest represents an unmatched request recorded by the system.
type UndefinedRequest struct {
	ID        string         `json:"id"`
	Method    string         `json:"method"`
	Path      string         `json:"path"`
	Protocol  string         `json:"protocol,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
	Count     int            `json:"count,omitempty"`
	Body      map[string]any `json:"body,omitempty"`
	Headers   map[string]any `json:"headers,omitempty"`
}

// List returns all unmatched requests for the client's namespace.
//
// Wire shape: `{"requests": [...], "total": N, "offset": N, "limit": N}`.
// The SDK unwraps the envelope and threads ?namespace= (server reads
// it from query only).
func (a *UndefinedAPI) List(ctx context.Context) ([]UndefinedRequest, error) {
	q := ""
	if a.client.namespace != "" {
		q = "?namespace=" + url.QueryEscape(a.client.namespace)
	}
	var env struct {
		Requests []UndefinedRequest `json:"requests"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/undefined-requests"+q, nil, &env); err != nil {
		return nil, err
	}
	return env.Requests, nil
}

// Ignore marks an undefined request as ignored.
func (a *UndefinedAPI) Ignore(ctx context.Context, id string) error {
	return a.client.do(ctx, "PATCH", "/api/v1/undefined-requests/"+url.PathEscape(id)+"/ignore", nil, nil)
}

// Delete deletes specific undefined requests by their IDs.
func (a *UndefinedAPI) Delete(ctx context.Context, ids []string) error {
	body := struct {
		IDs []string `json:"ids"`
	}{IDs: ids}
	return a.client.do(ctx, "DELETE", "/api/v1/undefined-requests", body, nil)
}

// ClearAll clears all undefined requests.
func (a *UndefinedAPI) ClearAll(ctx context.Context) error {
	return a.client.do(ctx, "DELETE", "/api/v1/undefined-requests/all", nil, nil)
}

// CreateMock auto-generates a mock from a recorded undefined request.
//
// Wire shape: this targets the `/convert` endpoint, which derives the
// mock from the stored undefined-request row (protocol auto-detected
// server-side) and returns a `{mock, mockId, protocol, isNew}`
// envelope. The older SDK build pointed at `/create-mock`, which
// instead expects a caller-supplied `{mockData}` body and 400'd with
// 'invalid request payload' on the no-body call. The unwrapped `mock`
// is returned so callers get the persisted Mock directly.
func (a *UndefinedAPI) CreateMock(ctx context.Context, id string) (*Mock, error) {
	var env struct {
		Mock Mock `json:"mock"`
	}
	if err := a.client.do(ctx, "POST", "/api/v1/undefined-requests/"+url.PathEscape(id)+"/convert", nil, &env); err != nil {
		return nil, err
	}
	return &env.Mock, nil
}
