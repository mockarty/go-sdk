// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"net/url"
)

// RecorderAPI provides methods for managing traffic recording sessions.
type RecorderAPI struct {
	client *Client
}

// RecorderSession represents a recording session configuration and status.
type RecorderSession struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	TargetURL  string `json:"targetUrl,omitempty"`
	Status     string `json:"status,omitempty"` // idle, recording, stopped
	Namespace  string `json:"namespace,omitempty"`
	CreatedAt  int64  `json:"createdAt,omitempty"`
	EntryCount int    `json:"entryCount,omitempty"`
}

// RecorderEntry represents a single recorded request/response pair.
type RecorderEntry struct {
	ID         string `json:"id,omitempty"`
	Method     string `json:"method,omitempty"`
	Path       string `json:"path,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	Duration   int64  `json:"duration,omitempty"`
	Timestamp  int64  `json:"timestamp,omitempty"`
}

// CreateSession creates a new recording session.
func (a *RecorderAPI) CreateSession(ctx context.Context, session *RecorderSession) (*RecorderSession, error) {
	if session.Namespace == "" && a.client.namespace != "" {
		session.Namespace = a.client.namespace
	}
	var result RecorderSession
	if err := a.client.do(ctx, "POST", "/ui/api/recorder/sessions", session, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSession retrieves a recording session by ID.
func (a *RecorderAPI) GetSession(ctx context.Context, id string) (*RecorderSession, error) {
	var session RecorderSession
	if err := a.client.do(ctx, "GET", "/ui/api/recorder/sessions/"+url.PathEscape(id), nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// ListSessions returns all recording sessions.
func (a *RecorderAPI) ListSessions(ctx context.Context) ([]RecorderSession, error) {
	var sessions []RecorderSession
	if err := a.client.do(ctx, "GET", "/ui/api/recorder/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// StartRecording starts recording on a session.
func (a *RecorderAPI) StartRecording(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/ui/api/recorder/sessions/"+url.PathEscape(id)+"/start", nil, nil)
}

// StopRecording stops recording on a session.
func (a *RecorderAPI) StopRecording(ctx context.Context, id string) error {
	return a.client.do(ctx, "POST", "/ui/api/recorder/sessions/"+url.PathEscape(id)+"/stop", nil, nil)
}

// DeleteSession deletes a recording session by ID.
func (a *RecorderAPI) DeleteSession(ctx context.Context, id string) error {
	return a.client.do(ctx, "DELETE", "/ui/api/recorder/sessions/"+url.PathEscape(id), nil, nil)
}

// GetEntries retrieves all recorded entries for a session.
func (a *RecorderAPI) GetEntries(ctx context.Context, sessionID string) ([]RecorderEntry, error) {
	var entries []RecorderEntry
	if err := a.client.do(ctx, "GET", "/ui/api/recorder/sessions/"+url.PathEscape(sessionID)+"/entries", nil, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// CreateMockFromEntry creates a mock from a recorded entry.
func (a *RecorderAPI) CreateMockFromEntry(ctx context.Context, sessionID, entryID string) (*Mock, error) {
	var mock Mock
	path := "/ui/api/recorder/sessions/" + url.PathEscape(sessionID) + "/entries/" + url.PathEscape(entryID) + "/create-mock"
	if err := a.client.do(ctx, "POST", path, nil, &mock); err != nil {
		return nil, err
	}
	return &mock, nil
}

// ExportSession exports a recording session as raw bytes (HAR format).
func (a *RecorderAPI) ExportSession(ctx context.Context, id string) ([]byte, error) {
	data, err := a.client.doJSON(ctx, "POST", "/ui/api/recorder/sessions/"+url.PathEscape(id)+"/export", nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}
