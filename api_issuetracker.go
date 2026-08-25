// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// IssueTrackerAPI is the task-automation surface over Mockarty's built-in issue
// tracker — create/read/update/transition issues, comment, search, claim the
// next issue, and manage projects/sprints. Issue payloads are rich and evolve,
// so this API uses loosely-typed map I/O (mirrored by the Python dict and Java
// JsonNode SDKs) instead of pinning a large struct.
//
// Every method takes a namespace argument; pass "" to use the client default.
// Server contract: /api/v1/namespaces/:ns/issuetracker/...
type IssueTrackerAPI struct {
	client *Client
}

// Issue is a loosely-typed issue record. Use the map directly (e.g.
// issue["issueKey"], issue["status"]) — field names match the server JSON.
type Issue = map[string]any

func (a *IssueTrackerAPI) base(namespace string) (string, error) {
	ns := namespace
	if ns == "" {
		ns = a.client.namespace
	}
	if ns == "" {
		return "", fmt.Errorf("issuetracker: namespace required")
	}
	return "/api/v1/namespaces/" + url.PathEscape(ns) + "/issuetracker", nil
}

func (a *IssueTrackerAPI) getObject(ctx context.Context, path string) (Issue, error) {
	data, err := a.client.doJSON(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var out Issue
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("issuetracker: decode: %w", err)
	}
	return out, nil
}

func (a *IssueTrackerAPI) sendObject(ctx context.Context, method, path string, body any) (Issue, error) {
	data, err := a.client.doJSON(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return Issue{}, nil
	}
	var out Issue
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("issuetracker: decode: %w", err)
	}
	return out, nil
}

// listUnder GETs path and unwraps the `{<key>: [...]}` envelope the tracker
// list endpoints use (issues / projects / sprints).
func (a *IssueTrackerAPI) listUnder(ctx context.Context, path, key string) ([]Issue, error) {
	data, err := a.client.doJSON(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("issuetracker: decode list: %w", err)
	}
	raw, ok := env[key]
	if !ok {
		return []Issue{}, nil
	}
	var out []Issue
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("issuetracker: decode %s: %w", key, err)
	}
	return out, nil
}

// CreateIssue creates an issue. The map accepts the server issue fields
// (projectId, type, title, description, priority, assigneeId, …).
func (a *IssueTrackerAPI) CreateIssue(ctx context.Context, namespace string, issue Issue) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.sendObject(ctx, "POST", base+"/issues", issue)
}

// GetIssue fetches an issue by its UUID.
func (a *IssueTrackerAPI) GetIssue(ctx context.Context, namespace, issueID string) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.getObject(ctx, base+"/issues/"+url.PathEscape(issueID))
}

// GetIssueByKey fetches an issue by its human key (e.g. "MK-42").
func (a *IssueTrackerAPI) GetIssueByKey(ctx context.Context, namespace, key string) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.getObject(ctx, base+"/issues/by-key/"+url.PathEscape(key))
}

// ListIssues lists issues, optionally filtered via query params (e.g.
// {"projectId": "...", "status": "open"}).
func (a *IssueTrackerAPI) ListIssues(ctx context.Context, namespace string, filters map[string]string) ([]Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.listUnder(ctx, base+"/issues"+encodeQuery(filters), "issues")
}

// SearchIssues runs a text search over issues.
func (a *IssueTrackerAPI) SearchIssues(ctx context.Context, namespace, query string) ([]Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.listUnder(ctx, base+"/issues/search"+encodeQuery(map[string]string{"q": query}), "issues")
}

// NextIssue returns the next issue available to work on (agent claim flow).
// params may carry an assigneeId to scope the claim.
func (a *IssueTrackerAPI) NextIssue(ctx context.Context, namespace string, params map[string]string) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.getObject(ctx, base+"/issues/next"+encodeQuery(params))
}

// UpdateIssue applies a partial update to an issue.
func (a *IssueTrackerAPI) UpdateIssue(ctx context.Context, namespace, issueID string, fields Issue) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.sendObject(ctx, "PUT", base+"/issues/"+url.PathEscape(issueID), fields)
}

// MoveIssue transitions an issue to a new workflow status. resolution is
// required when moving into a terminal (closed) status; pass "" otherwise.
func (a *IssueTrackerAPI) MoveIssue(ctx context.Context, namespace, issueID, status, resolution string) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"status": status}
	if resolution != "" {
		body["resolution"] = resolution
	}
	return a.sendObject(ctx, "POST", base+"/issues/"+url.PathEscape(issueID)+"/move", body)
}

// DeleteIssue soft-deletes an issue.
func (a *IssueTrackerAPI) DeleteIssue(ctx context.Context, namespace, issueID string) error {
	base, err := a.base(namespace)
	if err != nil {
		return err
	}
	return a.client.do(ctx, "DELETE", base+"/issues/"+url.PathEscape(issueID), nil, nil)
}

// AddComment posts a comment to an issue.
func (a *IssueTrackerAPI) AddComment(ctx context.Context, namespace, issueID, body string) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.sendObject(ctx, "POST", base+"/issues/"+url.PathEscape(issueID)+"/comments",
		map[string]any{"body": body})
}

// ListComments lists an issue's comments.
func (a *IssueTrackerAPI) ListComments(ctx context.Context, namespace, issueID string) ([]Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.listUnder(ctx, base+"/issues/"+url.PathEscape(issueID)+"/comments", "comments")
}

// BulkAssign assigns many issues to one assignee in a single call.
func (a *IssueTrackerAPI) BulkAssign(ctx context.Context, namespace string, issueIDs []string, assigneeID string) error {
	base, err := a.base(namespace)
	if err != nil {
		return err
	}
	body := map[string]any{"ids": issueIDs, "assigneeId": assigneeID}
	return a.client.do(ctx, "POST", base+"/issues/bulk/assign", body, nil)
}

// ListProjects lists the tracker's projects.
func (a *IssueTrackerAPI) ListProjects(ctx context.Context, namespace string) ([]Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.listUnder(ctx, base+"/projects", "projects")
}

// CreateProject creates a project.
func (a *IssueTrackerAPI) CreateProject(ctx context.Context, namespace string, project Issue) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.sendObject(ctx, "POST", base+"/projects", project)
}

// ListSprints lists sprints, optionally filtered (e.g. {"projectId": "..."}).
func (a *IssueTrackerAPI) ListSprints(ctx context.Context, namespace string, filters map[string]string) ([]Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.listUnder(ctx, base+"/sprints"+encodeQuery(filters), "sprints")
}

// CreateSprint creates a sprint.
func (a *IssueTrackerAPI) CreateSprint(ctx context.Context, namespace string, sprint Issue) (Issue, error) {
	base, err := a.base(namespace)
	if err != nil {
		return nil, err
	}
	return a.sendObject(ctx, "POST", base+"/sprints", sprint)
}

// encodeQuery renders a "?k=v&..." string (sorted for stable output); "" when empty.
func encodeQuery(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	vals := url.Values{}
	for _, k := range sortedKeys(params) {
		if params[k] != "" {
			vals.Set(k, params[k])
		}
	}
	q := vals.Encode()
	if q == "" {
		return ""
	}
	return "?" + q
}
