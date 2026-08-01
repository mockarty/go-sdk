// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"fmt"
	"net/url"
)

// GitSyncAPI binds an autotest collection (API + UI tests) to a git repository:
// pull materialises the tree into Mockarty, push writes local edits back. Git
// I/O runs server-side — the SDK just orchestrates. Ideal from CI: pull the
// team's autotests from the repo and run them, or push what you recorded.
type GitSyncAPI struct {
	client *Client
}

// GitSyncBinding is a stored binding. The token is never returned — HasToken
// reports only whether one is stored.
type GitSyncBinding struct {
	ID           string `json:"id"`
	Namespace    string `json:"namespace,omitempty"`
	RepoURL      string `json:"repoUrl"`
	Branch       string `json:"branch,omitempty"`
	Subdir       string `json:"subdir,omitempty"`
	Kind         string `json:"kind,omitempty"`
	AuthUsername string `json:"authUsername,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	LastCommit   string `json:"lastCommit,omitempty"`
	LastError    string `json:"lastError,omitempty"`
	LastSyncAt   string `json:"lastSyncAt,omitempty"`
	HasToken     bool   `json:"hasToken"`
	Enabled      bool   `json:"enabled"`
	AutoSync     bool   `json:"autoSync"`
}

// GitSyncBindingInput creates a binding. Kind is api|ui|mixed (default mixed).
// AuthToken is write-only (needed for private repos + push). AutoSync defaults
// to true when the pointer is nil.
type GitSyncBindingInput struct {
	RepoURL      string `json:"repoUrl"`
	Branch       string `json:"branch,omitempty"`
	Subdir       string `json:"subdir,omitempty"`
	Kind         string `json:"kind,omitempty"`
	AuthUsername string `json:"authUsername,omitempty"`
	AuthToken    string `json:"authToken,omitempty"`
	CollectionID string `json:"collectionId,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	AutoSync     *bool  `json:"autoSync,omitempty"`
}

// GitSyncPullResult is returned by Pull.
type GitSyncPullResult struct {
	Commit       string `json:"commit"`
	UITestsFound int    `json:"uiTestsFound"`
}

// GitSyncPushResult is returned by Push.
type GitSyncPushResult struct {
	Commit string `json:"commit"`
}

func (a *GitSyncAPI) nsQuery() string {
	if a.client.namespace != "" {
		return "?namespace=" + url.QueryEscape(a.client.namespace)
	}
	return ""
}

// CreateBinding binds a repo (POST /api/v1/git-sync/bindings).
func (a *GitSyncAPI) CreateBinding(ctx context.Context, in *GitSyncBindingInput) (*GitSyncBinding, error) {
	if in == nil || in.RepoURL == "" {
		return nil, fmt.Errorf("mockarty: GitSync.CreateBinding: repoUrl is required")
	}
	var out GitSyncBinding
	if err := a.client.do(ctx, "POST", "/api/v1/git-sync/bindings"+a.nsQuery(), in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBindings returns the namespace's bindings.
func (a *GitSyncAPI) ListBindings(ctx context.Context) ([]GitSyncBinding, error) {
	var env struct {
		Bindings []GitSyncBinding `json:"bindings"`
	}
	if err := a.client.do(ctx, "GET", "/api/v1/git-sync/bindings"+a.nsQuery(), nil, &env); err != nil {
		return nil, err
	}
	return env.Bindings, nil
}

// GetBinding retrieves a binding (with its last-sync status) by id.
func (a *GitSyncAPI) GetBinding(ctx context.Context, id string) (*GitSyncBinding, error) {
	var out GitSyncBinding
	if err := a.client.do(ctx, "GET", "/api/v1/git-sync/bindings/"+url.PathEscape(id)+a.nsQuery(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBinding removes a binding (the already-synced tests stay in Mockarty).
func (a *GitSyncAPI) DeleteBinding(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/api/v1/git-sync/bindings/"+url.PathEscape(id)+a.nsQuery(), nil, nil)
}

// Pull clones the binding's repo and materialises the tests into Mockarty.
func (a *GitSyncAPI) Pull(ctx context.Context, id string) (*GitSyncPullResult, error) {
	var out GitSyncPullResult
	if err := a.client.do(ctx, "POST", "/api/v1/git-sync/bindings/"+url.PathEscape(id)+"/pull"+a.nsQuery(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Push serialises the namespace's tests into the repo tree and commits + pushes.
// message may be empty (a default commit message is used).
func (a *GitSyncAPI) Push(ctx context.Context, id, message string) (*GitSyncPushResult, error) {
	path := "/api/v1/git-sync/bindings/" + url.PathEscape(id) + "/push"
	q := a.nsQuery()
	if message != "" {
		if q == "" {
			q = "?message=" + url.QueryEscape(message)
		} else {
			q += "&message=" + url.QueryEscape(message)
		}
	}
	var out GitSyncPushResult
	if err := a.client.do(ctx, "POST", path+q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
