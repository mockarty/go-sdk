// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"io"
	"net/url"
)

// TemplateAPI provides methods for managing response template files.
type TemplateAPI struct {
	client *Client
}

// TemplateFile represents metadata about an uploaded template file.
type TemplateFile struct {
	Name      string `json:"name,omitempty"`
	Size      int64  `json:"size,omitempty"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// nsQuery returns "?namespace=<X>" using the client's default NS or
// "sandbox" as the last-resort fallback. Templates are namespace-
// scoped on the server side via the query string.
func (a *TemplateAPI) nsQuery() string {
	ns := a.client.namespace
	if ns == "" {
		ns = "sandbox"
	}
	return "?namespace=" + url.QueryEscape(ns)
}

// List returns all uploaded template files in the client's namespace.
//
// Wire shape: `{"templates":["a.json","b.xml",...], "total":N,
// "limit":N, "offset":N, "namespace":"..."}`. The server stores only
// filenames at the list level — full metadata (size, updatedAt)
// requires a per-file follow-up. The SDK projects the filename onto
// TemplateFile.Name; Size/UpdatedAt stay zero until those become
// available server-side.
func (a *TemplateAPI) List(ctx context.Context) ([]TemplateFile, error) {
	var env struct {
		Templates []string `json:"templates"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/templates"+a.nsQuery(), nil, &env); err != nil {
		return nil, err
	}
	out := make([]TemplateFile, 0, len(env.Templates))
	for _, name := range env.Templates {
		out = append(out, TemplateFile{Name: name})
	}
	return out, nil
}

// Get retrieves the raw contents of a template file by name.
//
// Wire shape: the server emits the file bytes directly via gin.Data
// (NOT a JSON envelope). The SDK reads them as-is so callers see
// exactly what was uploaded.
func (a *TemplateAPI) Get(ctx context.Context, fileName string) ([]byte, error) {
	rc, err := a.client.doRaw(ctx, "GET", "/api/v1/templates/"+url.PathEscape(fileName)+a.nsQuery(), nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Upload uploads or replaces a template file.
//
// Wire shape: the server reads the request body as raw bytes — NOT
// {"content": "..."} JSON. The SDK now sends bytes directly so a
// subsequent Get() returns exactly what was uploaded (no escape /
// double-encoding round-trip).
func (a *TemplateAPI) Upload(ctx context.Context, fileName string, content []byte) error {
	return a.client.do(ctx, "POST",
		"/api/v1/templates/"+url.PathEscape(fileName)+a.nsQuery(),
		content, nil)
}

// Delete deletes a template file by name.
func (a *TemplateAPI) Delete(ctx context.Context, fileName string) error {
	return a.client.do(ctx, "DELETE",
		"/api/v1/templates/"+url.PathEscape(fileName)+a.nsQuery(),
		nil, nil)
}
