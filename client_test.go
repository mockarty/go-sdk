// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("http://localhost:5770")
	if c.BaseURL() != "http://localhost:5770" {
		t.Errorf("expected base URL http://localhost:5770, got %s", c.BaseURL())
	}
	if c.Namespace() != "sandbox" {
		t.Errorf("expected namespace sandbox, got %s", c.Namespace())
	}
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_TrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:5770/")
	if c.BaseURL() != "http://localhost:5770" {
		t.Errorf("expected trailing slash trimmed, got %s", c.BaseURL())
	}
}

func TestNewClient_Options(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	customHTTP := &http.Client{Timeout: 60 * time.Second}

	c := NewClient("http://example.com",
		WithAPIKey("test-key"),
		WithNamespace("production"),
		WithTimeout(10*time.Second),
		WithLogger(logger),
		WithRetry(3, 100*time.Millisecond),
	)

	if c.apiKey != "test-key" {
		t.Error("expected API key to be set")
	}
	if c.namespace != "production" {
		t.Error("expected namespace production")
	}
	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", c.httpClient.Timeout)
	}
	if c.maxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", c.maxRetries)
	}

	c2 := NewClient("http://example.com", WithHTTPClient(customHTTP))
	if c2.httpClient != customHTTP {
		t.Error("expected custom HTTP client")
	}
}

func TestClient_APIKeyHeader(t *testing.T) {
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("my-secret-key"))
	_ = c.do(context.Background(), "GET", "/test", nil, nil)

	if gotKey != "my-secret-key" {
		t.Errorf("expected X-API-Key=my-secret-key, got %q", gotKey)
	}
}

func TestClient_NoAPIKey(t *testing.T) {
	var hasKey bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasKey = r.Header.Get("X-API-Key") != ""
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_ = c.do(context.Background(), "GET", "/test", nil, nil)

	if hasKey {
		t.Error("expected no X-API-Key header when key not configured")
	}
}

func TestClient_ContentTypeJSON(t *testing.T) {
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_ = c.do(context.Background(), "POST", "/test", map[string]string{"key": "val"}, nil)

	if gotContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotContentType)
	}
}

func TestClient_ErrorParsing(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    error
		wantMsg    string
	}{
		{
			name:       "not found with error field",
			statusCode: 404,
			body:       `{"error": "mock not found"}`,
			wantErr:    ErrNotFound,
			wantMsg:    "mock not found",
		},
		{
			name:       "unauthorized with message field",
			statusCode: 401,
			body:       `{"message": "invalid token"}`,
			wantErr:    ErrUnauthorized,
			wantMsg:    "invalid token",
		},
		{
			name:       "forbidden",
			statusCode: 403,
			body:       `{"error": "access denied"}`,
			wantErr:    ErrForbidden,
		},
		{
			name:       "conflict",
			statusCode: 409,
			body:       `{"error": "already exists"}`,
			wantErr:    ErrConflict,
		},
		{
			name:       "rate limited",
			statusCode: 429,
			body:       `{"error": "too many requests"}`,
			wantErr:    ErrRateLimited,
		},
		{
			name:       "server error",
			statusCode: 500,
			body:       `{"error": "internal server error"}`,
			wantErr:    ErrServerError,
		},
		{
			name:       "plain text error",
			statusCode: 503,
			body:       `service unavailable`,
			wantErr:    ErrServerError,
			wantMsg:    "service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			err := c.do(context.Background(), "GET", "/test", nil, nil)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected errors.Is(%v, %v) to be true", err, tt.wantErr)
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatal("expected error to be *APIError")
			}

			if apiErr.StatusCode != tt.statusCode {
				t.Errorf("expected status %d, got %d", tt.statusCode, apiErr.StatusCode)
			}

			if tt.wantMsg != "" && apiErr.Message != tt.wantMsg {
				t.Errorf("expected message %q, got %q", tt.wantMsg, apiErr.Message)
			}
		})
	}
}

func TestClient_Retry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(3, 1*time.Millisecond))

	var result map[string]string
	err := c.do(context.Background(), "GET", "/test", nil, &result)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", result)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_RetryExhausted(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"always failing"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(2, 1*time.Millisecond))
	err := c.do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if !errors.Is(err, ErrServerError) {
		t.Errorf("expected ErrServerError, got %v", err)
	}
	// 1 initial + 2 retries = 3 total attempts
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_NoRetryOn4xx(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(3, 1*time.Millisecond))
	err := c.do(context.Background(), "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("expected 1 attempt (no retry on 404), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_RetryOn429(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, WithRetry(2, 1*time.Millisecond))
	err := c.do(context.Background(), "GET", "/test", nil, nil)
	if err != nil {
		t.Fatalf("expected success after retry on 429, got %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.do(ctx, "GET", "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error due to context timeout")
	}
}

func TestClient_RequestIDInError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-12345")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.do(context.Background(), "GET", "/test", nil, nil)

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.RequestID != "req-12345" {
			t.Errorf("expected request ID req-12345, got %q", apiErr.RequestID)
		}
	} else {
		t.Error("expected *APIError")
	}
}

func TestClient_DoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"key":"value"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	data, err := c.doJSON(context.Background(), "GET", "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result)
	}
}

func TestClient_SubAPISingletons(t *testing.T) {
	c := NewClient("http://localhost")

	// Each accessor should return the same pointer
	if c.Mocks() != c.Mocks() {
		t.Error("Mocks() should return the same instance")
	}
	if c.Namespaces() != c.Namespaces() {
		t.Error("Namespaces() should return the same instance")
	}
	if c.Stores() != c.Stores() {
		t.Error("Stores() should return the same instance")
	}
	if c.Collections() != c.Collections() {
		t.Error("Collections() should return the same instance")
	}
	if c.Perf() != c.Perf() {
		t.Error("Perf() should return the same instance")
	}
	if c.Health() != c.Health() {
		t.Error("Health() should return the same instance")
	}
}

func TestClient_RequestBodySerialization(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	body := map[string]any{
		"name":  "test",
		"count": float64(42),
	}
	_ = c.do(context.Background(), "POST", "/test", body, nil)

	if gotBody["name"] != "test" {
		t.Errorf("expected name=test, got %v", gotBody["name"])
	}
	if gotBody["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", gotBody["count"])
	}
}

func TestClient_AcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_ = c.do(context.Background(), "GET", "/test", nil, nil)

	if gotAccept != "application/json" {
		t.Errorf("expected Accept=application/json, got %q", gotAccept)
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      APIError
		expected string
	}{
		{
			name:     "without request ID",
			err:      APIError{StatusCode: 404, Message: "not found"},
			expected: "mockarty: HTTP 404: not found",
		},
		{
			name:     "with request ID",
			err:      APIError{StatusCode: 500, Message: "internal error", RequestID: "abc-123"},
			expected: "mockarty: HTTP 500: internal error (request_id=abc-123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	tests := []struct {
		statusCode int
		want       error
	}{
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{409, ErrConflict},
		{429, ErrRateLimited},
		{500, ErrServerError},
		{502, ErrServerError},
		{503, ErrServerError},
		{400, nil},
		{422, nil},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.statusCode), func(t *testing.T) {
			apiErr := &APIError{StatusCode: tt.statusCode, Message: "test"}
			got := apiErr.Unwrap()
			if got != tt.want {
				t.Errorf("Unwrap() = %v, want %v", got, tt.want)
			}
		})
	}
}
