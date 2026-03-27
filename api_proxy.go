// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
)

// ProxyAPI provides methods for proxying requests through Mockarty.
type ProxyAPI struct {
	client *Client
}

// HTTP sends an HTTP proxy request through Mockarty.
func (a *ProxyAPI) HTTP(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/proxy/http", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SOAP sends a SOAP proxy request through Mockarty.
func (a *ProxyAPI) SOAP(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/proxy/soap", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GRPC sends a gRPC proxy request through Mockarty.
func (a *ProxyAPI) GRPC(ctx context.Context, req any) (any, error) {
	var result any
	if err := a.client.do(ctx, "POST", "/api/v1/proxy/grpc", req, &result); err != nil {
		return nil, err
	}
	return result, nil
}
