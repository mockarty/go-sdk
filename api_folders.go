// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// FolderAPI provides methods for managing mock folders.
type FolderAPI struct {
	client *Client
}

// MockFolder represents a folder for organizing mocks.
type MockFolder struct {
	ID        string `json:"id,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// List returns all mock folders for the client's namespace.
//
// Wire shape: server replies with `{"folders":[...], "mockCounts":{...}}`.
// The SDK unwraps the envelope and returns the folders slice.
func (a *FolderAPI) List(ctx context.Context) ([]MockFolder, error) {
	params := url.Values{}
	if a.client.namespace != "" {
		params.Set("namespace", a.client.namespace)
	}

	path := "/api/v1/mock-folders"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var env struct {
		Folders []MockFolder `json:"folders"`
	}
	if err := a.client.do(ctx, "GET", path, nil, &env); err != nil {
		return nil, err
	}
	return env.Folders, nil
}

// Create creates a new folder.
func (a *FolderAPI) Create(ctx context.Context, folder *MockFolder) (*MockFolder, error) {
	if folder.Namespace == "" && a.client.namespace != "" {
		folder.Namespace = a.client.namespace
	}
	var result MockFolder
	if err := a.client.do(ctx, "POST", "/api/v1/mock-folders", folder, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update updates a folder by ID.
func (a *FolderAPI) Update(ctx context.Context, id string, folder *MockFolder) (*MockFolder, error) {
	var result MockFolder
	if err := a.client.do(ctx, "PUT", "/api/v1/mock-folders/"+url.PathEscape(id), folder, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Delete deletes a folder by ID.
func (a *FolderAPI) Delete(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/mock-folders/"+url.PathEscape(id), nil, nil)
}

// Move moves a folder to a new parent.
func (a *FolderAPI) Move(ctx context.Context, id string, parentID string) error {
	body := struct {
		ParentID string `json:"parentId"`
	}{ParentID: parentID}
	return a.client.do(ctx, "PATCH", "/api/v1/mock-folders/"+url.PathEscape(id)+"/move", body, nil)
}
