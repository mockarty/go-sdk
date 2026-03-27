// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"errors"
	"fmt"
)

// Sentinel errors for common HTTP status codes.
var (
	ErrNotFound     = errors.New("mockarty: not found")
	ErrUnauthorized = errors.New("mockarty: unauthorized")
	ErrForbidden    = errors.New("mockarty: forbidden")
	ErrConflict     = errors.New("mockarty: conflict")
	ErrRateLimited  = errors.New("mockarty: rate limited")
	ErrServerError  = errors.New("mockarty: server error")
)

// APIError represents a structured error returned by the Mockarty API.
type APIError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	RequestID  string `json:"requestId,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("mockarty: HTTP %d: %s (request_id=%s)", e.StatusCode, e.Message, e.RequestID)
	}
	return fmt.Sprintf("mockarty: HTTP %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the sentinel error corresponding to the status code,
// allowing callers to use errors.Is for matching.
func (e *APIError) Unwrap() error {
	switch e.StatusCode {
	case 401:
		return ErrUnauthorized
	case 403:
		return ErrForbidden
	case 404:
		return ErrNotFound
	case 409:
		return ErrConflict
	case 429:
		return ErrRateLimited
	default:
		if e.StatusCode >= 500 {
			return ErrServerError
		}
		return nil
	}
}
