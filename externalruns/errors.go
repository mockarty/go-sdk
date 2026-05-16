package externalruns

import (
	"errors"
	"fmt"
)

// APIError is returned for any non-2xx HTTP response. It exposes the
// status code so callers can branch on 4xx vs 5xx without string-matching.
//
// The server-side error envelope is best-effort: when the body is JSON
// matching {"error": "..."} the message is captured, otherwise RawBody
// holds the (possibly truncated) raw response.
type APIError struct {
	Code       string
	Message    string
	RawBody    string
	StatusCode int
}

// Error implements error.
func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" {
		return fmt.Sprintf("mockarty externalruns: %d %s", e.StatusCode, e.Message)
	}
	if e.RawBody != "" {
		return fmt.Sprintf("mockarty externalruns: %d (raw=%q)", e.StatusCode, truncate(e.RawBody, 256))
	}
	return fmt.Sprintf("mockarty externalruns: %d", e.StatusCode)
}

// Sentinel errors returned by validation before any HTTP traffic.
var (
	// ErrInvalidConfig is returned when NewClient is called without the
	// required fields (server URL, token).
	ErrInvalidConfig = errors.New("mockarty externalruns: invalid client config")

	// ErrInvalidRequest is returned when a request struct fails local
	// validation (e.g. empty Name, blank run ID, malformed status).
	ErrInvalidRequest = errors.New("mockarty externalruns: invalid request")
)

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
