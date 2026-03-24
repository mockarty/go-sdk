// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
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

// List returns all uploaded template files.
func (a *TemplateAPI) List(ctx context.Context) ([]TemplateFile, error) {
	var files []TemplateFile
	if err := a.client.do(ctx, "GET", "/ui/api/templates", nil, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// Get retrieves the contents of a template file by name.
func (a *TemplateAPI) Get(ctx context.Context, fileName string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "GET", "/ui/api/templates/"+url.PathEscape(fileName), nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Upload uploads or replaces a template file.
func (a *TemplateAPI) Upload(ctx context.Context, fileName string, content []byte) error {
	body := struct {
		Content string `json:"content"`
	}{Content: string(content)}
	return a.client.do(ctx, "POST", "/ui/api/templates/"+url.PathEscape(fileName), body, nil)
}

// Delete deletes a template file by name.
func (a *TemplateAPI) Delete(ctx context.Context, fileName string) error {
	return a.client.do(ctx, "DELETE", "/ui/api/templates/"+url.PathEscape(fileName), nil, nil)
}
