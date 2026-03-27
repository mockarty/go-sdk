// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

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

// List returns all unmatched requests.
func (a *UndefinedAPI) List(ctx context.Context) ([]UndefinedRequest, error) {
	var requests []UndefinedRequest
	if err := a.client.do(ctx, "GET", "/api/v1/undefined-requests", nil, &requests); err != nil {
		return nil, err
	}
	return requests, nil
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

// CreateMock creates a mock from an undefined request.
func (a *UndefinedAPI) CreateMock(ctx context.Context, id string) (*Mock, error) {
	var mock Mock
	if err := a.client.do(ctx, "POST", "/api/v1/undefined-requests/"+url.PathEscape(id)+"/create-mock", nil, &mock); err != nil {
		return nil, err
	}
	return &mock, nil
}
