// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

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

// List returns all tags in the client's namespace.
//
// Wire shape: server returns `{tags: ["tag-a","tag-b",...], namespace}`
// — bare string list, NOT {name,count} objects. Older SDK builds had
// Tag{Name,Count} which assumed objects; we project the string list
// onto Tag{Name:name} so call-sites that iterate .Name keep working.
func (a *TagAPI) List(ctx context.Context) ([]Tag, error) {
	q := ""
	if a.client.namespace != "" {
		q = "?namespace=" + a.client.namespace
	}
	var env struct {
		Tags []string `json:"tags"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/tags"+q, nil, &env); err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(env.Tags))
	for _, name := range env.Tags {
		out = append(out, Tag{Name: name})
	}
	return out, nil
}

// Create creates a new tag.
//
// Wire shape: server expects `{tag: "...", namespace: "..."}`, NOT
// `{name: "..."}` — the SDK previously sent the wrong field and every
// Create returned 400 'tag name is required'.
func (a *TagAPI) Create(ctx context.Context, name string) (*Tag, error) {
	body := map[string]any{"tag": name}
	if a.client.namespace != "" {
		body["namespace"] = a.client.namespace
	}
	var result Tag
	if err := a.client.do(ctx, "POST", "/api/v1/tags", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
