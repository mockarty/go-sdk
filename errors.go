// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Sentinel errors for common error categories returned by the Mockarty API.
// These match the "code" field in the server's JSON error envelope
// (see internal/errors.Kind in the server source) and let callers use
// errors.Is to branch without string-matching on messages.
var (
	ErrValidation   = errors.New("mockarty: validation")
	ErrUnauthorized = errors.New("mockarty: unauthorized")
	ErrForbidden    = errors.New("mockarty: forbidden")
	ErrNotFound     = errors.New("mockarty: not found")
	ErrConflict     = errors.New("mockarty: conflict")
	ErrRateLimited  = errors.New("mockarty: rate limited")
	ErrUnavailable  = errors.New("mockarty: service unavailable")
	ErrExternal     = errors.New("mockarty: external dependency error")
	ErrServerError  = errors.New("mockarty: server error")
)

// APIError represents a structured error returned by the Mockarty API.
//
// The server emits a uniform JSON envelope for every error:
//
//	{"error": "human message", "code": "not_found", "request_id": "..."}
//
// Both RequestID and Code are guaranteed to be populated on 4xx/5xx responses
// from a current Mockarty server. When talking to an older server that only
// sent {"error": "..."} the Code and RequestID may be empty.
type APIError struct {
	// StatusCode is the HTTP status code returned by the server.
	StatusCode int `json:"statusCode"`

	// Message is the human-readable, sanitized error text. Never contains
	// SQL errors, stack traces, or internal paths — safe to surface in UIs.
	Message string `json:"message"`

	// Code is the stable machine-readable identifier of the error category
	// (e.g. "not_found", "validation", "rate_limit"). Branch on this field
	// instead of parsing Message.
	Code string `json:"code,omitempty"`

	// RequestID is the server-side correlation ID for this request. Include
	// it in support tickets so operators can find the matching log entry.
	RequestID string `json:"requestId,omitempty"`

	// Reason identifies the specific operation failure, when supplied.
	Reason string `json:"reason,omitempty"`

	// RunID identifies an already saved launch. Inspect it before launching again.
	RunID string `json:"runId,omitempty"`

	// OperationID and CaptureID are opaque handles from an exact unknown
	// comment capture error. Neither handle proves the comment was applied.
	OperationID string `json:"operationId,omitempty"`
	CaptureID   string `json:"captureId,omitempty"`
	State       string `json:"state,omitempty"`
}

// commentCaptureMetadata leaves legacy error parsing untouched. Metadata is
// retained only from the bounded exact shape, not from partial or mixed claims.
func commentCaptureMetadata(body []byte, status int) (operation, capture, state string) {
	if status != 503 || len(body) > 2048 {
		return "", "", ""
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return "", "", ""
	}
	fields := make(map[string]string, 5)
	for d.More() {
		token, err = d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return "", "", ""
		}
		switch key {
		case "error", "state", "operation_id", "capture_id", "code":
		default:
			return "", "", ""
		}
		if _, exists := fields[key]; exists {
			return "", "", ""
		}
		token, err = d.Token()
		value, ok := token.(string)
		if err != nil || !ok {
			return "", "", ""
		}
		fields[key] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') {
		return "", "", ""
	}
	if _, err = d.Token(); err != io.EOF {
		return "", "", ""
	}
	if fields["error"] != "collaboration_capture_outcome_unknown" || fields["state"] != "unknown" ||
		(fields["code"] != "" && fields["code"] != "unavailable") ||
		!commentCaptureUUID(fields["operation_id"]) || !commentCaptureUUID(fields["capture_id"]) {
		return "", "", ""
	}
	return fields["operation_id"], fields["capture_id"], "unknown"
}

func commentCaptureUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	nonzero := false
	for i := 0; i < len(value); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if value[i] != '-' {
				return false
			}
			continue
		}
		if (value[i] < '0' || value[i] > '9') && (value[i] < 'a' || value[i] > 'f') {
			return false
		}
		nonzero = nonzero || value[i] != '0'
	}
	return nonzero
}

// Error implements the error interface.
func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.RequestID != "":
		return fmt.Sprintf("mockarty: HTTP %d %s: %s (request_id=%s)", e.StatusCode, e.Code, e.Message, e.RequestID)
	case e.Code != "":
		return fmt.Sprintf("mockarty: HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
	case e.RequestID != "":
		return fmt.Sprintf("mockarty: HTTP %d: %s (request_id=%s)", e.StatusCode, e.Message, e.RequestID)
	default:
		return fmt.Sprintf("mockarty: HTTP %d: %s", e.StatusCode, e.Message)
	}
}

// Unwrap returns the sentinel error corresponding to the error Code (preferred)
// or the StatusCode (fallback), allowing callers to use errors.Is for matching.
func (e *APIError) Unwrap() error {
	// Prefer the stable server-side Code — it is transport-agnostic and
	// won't shift if the HTTP status mapping changes.
	switch e.Code {
	case "validation":
		return ErrValidation
	case "unauthorized":
		return ErrUnauthorized
	case "forbidden":
		return ErrForbidden
	case "not_found":
		return ErrNotFound
	case "conflict":
		return ErrConflict
	case "rate_limit":
		return ErrRateLimited
	case "unavailable":
		return ErrUnavailable
	case "external":
		return ErrExternal
	case "internal":
		return ErrServerError
	}
	// Fallback: old server without "code" field — map by HTTP status.
	switch e.StatusCode {
	case 400:
		return ErrValidation
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
	case 502:
		return ErrExternal
	case 503:
		return ErrUnavailable
	default:
		if e.StatusCode >= 500 {
			return ErrServerError
		}
		return nil
	}
}
