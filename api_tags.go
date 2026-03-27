// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
)

// TagAPI provides methods for managing tags.
type TagAPI struct {
	client *Client
}

// Tag represents a mock tag with its usage count.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count,omitempty"`
}

// List returns all tags.
func (a *TagAPI) List(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	if err := a.client.do(ctx, "GET", "/api/v1/tags", nil, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// Create creates a new tag.
func (a *TagAPI) Create(ctx context.Context, name string) (*Tag, error) {
	body := struct {
		Name string `json:"name"`
	}{Name: name}
	var result Tag
	if err := a.client.do(ctx, "POST", "/api/v1/tags", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
