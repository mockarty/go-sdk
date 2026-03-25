// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"log/slog"
	"net/http"
	"time"
)

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key used for authentication.
// The key is sent via the X-API-Key header on every request.
func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

// WithNamespace sets the default namespace for API operations.
// If not set, "sandbox" is used.
func WithNamespace(ns string) Option {
	return func(c *Client) {
		c.namespace = ns
	}
}

// WithTimeout sets the HTTP client timeout. Default is 30 seconds.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient replaces the default HTTP client entirely.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithLogger sets a structured logger. Default is slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// WithRetry configures retry behaviour for transient failures (5xx, timeouts).
// maxRetries is the total number of additional attempts after the first failure.
// initialDelay is the delay before the first retry; subsequent retries use
// exponential back-off (delay * 2^attempt).
func WithRetry(maxRetries int, initialDelay time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryDelay = initialDelay
	}
}
