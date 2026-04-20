// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	defaultNamespace = "sandbox"
	defaultTimeout   = 30 * time.Second
	headerAPIKey     = "X-API-Key"
	headerRequestID  = "X-Request-Id"
)

// Client is the Mockarty API client.
// Create one using NewClient and reuse it across goroutines.
type Client struct {
	baseURL    string
	apiKey     string
	namespace  string
	httpClient *http.Client
	logger     *slog.Logger
	maxRetries int
	retryDelay time.Duration

	// Sub-API singletons (lazy-initialized through accessor methods)
	mockAPI              *MockAPI
	namespaceAPI         *NamespaceAPI
	storeAPI             *StoreAPI
	collectionAPI        *CollectionAPI
	perfAPI              *PerfAPI
	healthAPI            *HealthAPI
	generatorAPI         *GeneratorAPI
	fuzzingAPI           *FuzzingAPI
	contractAPI          *ContractAPI
	recorderAPI          *RecorderAPI
	templateAPI          *TemplateAPI
	importAPI            *ImportAPI
	testRunAPI           *TestRunAPI
	tagAPI               *TagAPI
	folderAPI            *FolderAPI
	undefinedAPI         *UndefinedAPI
	statsAPI             *StatsAPI
	agentTaskAPI         *AgentTaskAPI
	namespaceSettingsAPI *NamespaceSettingsAPI
	proxyAPI             *ProxyAPI
	environmentAPI       *EnvironmentAPI
	chaosAPI             *ChaosAPI
	testPlansAPI         *TestPlansAPI
	trashAPI             *TrashAPI
	entitySearchAPI      *EntitySearchAPI
}

// NewClient creates a new Mockarty API client.
//
//	client := mockarty.NewClient("http://localhost:5770",
//	    mockarty.WithAPIKey("mk_..."),
//	    mockarty.WithNamespace("production"),
//	)
func NewClient(baseURL string, opts ...Option) *Client {
	baseURL = strings.TrimRight(baseURL, "/")

	c := &Client{
		baseURL:   baseURL,
		namespace: defaultNamespace,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// Namespace returns the configured default namespace.
func (c *Client) Namespace() string { return c.namespace }

// ---------------------------------------------------------------------------
// Sub-API accessors
// ---------------------------------------------------------------------------

// Mocks returns the Mock CRUD API.
func (c *Client) Mocks() *MockAPI {
	if c.mockAPI == nil {
		c.mockAPI = &MockAPI{client: c}
	}
	return c.mockAPI
}

// Namespaces returns the Namespace API.
func (c *Client) Namespaces() *NamespaceAPI {
	if c.namespaceAPI == nil {
		c.namespaceAPI = &NamespaceAPI{client: c}
	}
	return c.namespaceAPI
}

// Stores returns the Store API.
func (c *Client) Stores() *StoreAPI {
	if c.storeAPI == nil {
		c.storeAPI = &StoreAPI{client: c}
	}
	return c.storeAPI
}

// Collections returns the Collection API.
func (c *Client) Collections() *CollectionAPI {
	if c.collectionAPI == nil {
		c.collectionAPI = &CollectionAPI{client: c}
	}
	return c.collectionAPI
}

// Perf returns the Performance Testing API.
func (c *Client) Perf() *PerfAPI {
	if c.perfAPI == nil {
		c.perfAPI = &PerfAPI{client: c}
	}
	return c.perfAPI
}

// Health returns the Health API.
func (c *Client) Health() *HealthAPI {
	if c.healthAPI == nil {
		c.healthAPI = &HealthAPI{client: c}
	}
	return c.healthAPI
}

// Generator returns the Generator API for creating mocks from specifications.
func (c *Client) Generator() *GeneratorAPI {
	if c.generatorAPI == nil {
		c.generatorAPI = &GeneratorAPI{client: c}
	}
	return c.generatorAPI
}

// Fuzzing returns the Fuzzing API for security and fuzz testing.
func (c *Client) Fuzzing() *FuzzingAPI {
	if c.fuzzingAPI == nil {
		c.fuzzingAPI = &FuzzingAPI{client: c}
	}
	return c.fuzzingAPI
}

// Contracts returns the Contract Testing API.
func (c *Client) Contracts() *ContractAPI {
	if c.contractAPI == nil {
		c.contractAPI = &ContractAPI{client: c}
	}
	return c.contractAPI
}

// Recorder returns the Recorder API for traffic recording.
func (c *Client) Recorder() *RecorderAPI {
	if c.recorderAPI == nil {
		c.recorderAPI = &RecorderAPI{client: c}
	}
	return c.recorderAPI
}

// Templates returns the Template API for managing response templates.
func (c *Client) Templates() *TemplateAPI {
	if c.templateAPI == nil {
		c.templateAPI = &TemplateAPI{client: c}
	}
	return c.templateAPI
}

// Import returns the Import API for importing API definitions.
func (c *Client) Import() *ImportAPI {
	if c.importAPI == nil {
		c.importAPI = &ImportAPI{client: c}
	}
	return c.importAPI
}

// TestRuns returns the Test Run API for managing test executions.
func (c *Client) TestRuns() *TestRunAPI {
	if c.testRunAPI == nil {
		c.testRunAPI = &TestRunAPI{client: c}
	}
	return c.testRunAPI
}

// Tags returns the Tag API for managing mock tags.
func (c *Client) Tags() *TagAPI {
	if c.tagAPI == nil {
		c.tagAPI = &TagAPI{client: c}
	}
	return c.tagAPI
}

// Folders returns the Folder API for managing mock folders.
func (c *Client) Folders() *FolderAPI {
	if c.folderAPI == nil {
		c.folderAPI = &FolderAPI{client: c}
	}
	return c.folderAPI
}

// Undefined returns the Undefined API for managing unmatched requests.
func (c *Client) Undefined() *UndefinedAPI {
	if c.undefinedAPI == nil {
		c.undefinedAPI = &UndefinedAPI{client: c}
	}
	return c.undefinedAPI
}

// Stats returns the Stats API for retrieving platform statistics.
func (c *Client) Stats() *StatsAPI {
	if c.statsAPI == nil {
		c.statsAPI = &StatsAPI{client: c}
	}
	return c.statsAPI
}

// AgentTasks returns the Agent Task API for managing AI agent tasks.
func (c *Client) AgentTasks() *AgentTaskAPI {
	if c.agentTaskAPI == nil {
		c.agentTaskAPI = &AgentTaskAPI{client: c}
	}
	return c.agentTaskAPI
}

// NamespaceSettings returns the Namespace Settings API.
func (c *Client) NamespaceSettings() *NamespaceSettingsAPI {
	if c.namespaceSettingsAPI == nil {
		c.namespaceSettingsAPI = &NamespaceSettingsAPI{client: c}
	}
	return c.namespaceSettingsAPI
}

// Proxy returns the Proxy API for proxying requests.
func (c *Client) Proxy() *ProxyAPI {
	if c.proxyAPI == nil {
		c.proxyAPI = &ProxyAPI{client: c}
	}
	return c.proxyAPI
}

// Environments returns the Environment API for managing API Tester environments.
func (c *Client) Environments() *EnvironmentAPI {
	if c.environmentAPI == nil {
		c.environmentAPI = &EnvironmentAPI{client: c}
	}
	return c.environmentAPI
}

// Chaos returns the Chaos Engineering API for managing chaos experiments.
func (c *Client) Chaos() *ChaosAPI {
	if c.chaosAPI == nil {
		c.chaosAPI = &ChaosAPI{client: c}
	}
	return c.chaosAPI
}

// TestPlans returns the Test Plans API — the master orchestrator for
// functional / fuzz / chaos / load / contract runs under a single plan.
func (c *Client) TestPlans() *TestPlansAPI {
	if c.testPlansAPI == nil {
		c.testPlansAPI = &TestPlansAPI{client: c}
	}
	return c.testPlansAPI
}

// Trash returns the Recycle Bin / Soft-Delete API for listing, restoring,
// and purging soft-deleted entities (cascade-aware).
func (c *Client) Trash() *TrashAPI {
	if c.trashAPI == nil {
		c.trashAPI = &TrashAPI{client: c}
	}
	return c.trashAPI
}

// EntitySearch returns the unified entity-picker API. Use it to look up
// mocks / Test Plans / perf configs / fuzz configs / chaos experiments /
// contract pacts by case-insensitive name match — the same backend the
// admin UI pickers consume.
func (c *Client) EntitySearch() *EntitySearchAPI {
	if c.entitySearchAPI == nil {
		c.entitySearchAPI = &EntitySearchAPI{client: c}
	}
	return c.entitySearchAPI
}

// ---------------------------------------------------------------------------
// Internal HTTP helpers
// ---------------------------------------------------------------------------

// do performs an HTTP request with auth, retry, and error handling.
// If body is non-nil it is marshalled to JSON.
// If result is non-nil the response body is decoded into it.
func (c *Client) do(ctx context.Context, method, path string, body any, result any) error {
	respBody, err := c.doRaw(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer respBody.Close()

	if result != nil {
		if err := json.NewDecoder(respBody).Decode(result); err != nil {
			return fmt.Errorf("mockarty: decode response: %w", err)
		}
	}
	return nil
}

// doJSON performs an HTTP request and returns the raw response bytes.
func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	respBody, err := c.doRaw(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer respBody.Close()

	data, err := io.ReadAll(respBody)
	if err != nil {
		return nil, fmt.Errorf("mockarty: read response: %w", err)
	}
	return data, nil
}

// doRaw executes the request with retries and returns the response body reader.
// The caller must close the returned reader.
func (c *Client) doRaw(ctx context.Context, method, path string, body any) (io.ReadCloser, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("mockarty: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	attempts := 1 + c.maxRetries
	delay := c.retryDelay

	var lastErr error

	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying request",
				slog.String("method", method),
				slog.String("url", url),
				slog.Int("attempt", attempt+1),
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2 // exponential back-off

			// Re-create body reader for retry
			if body != nil {
				data, err := json.Marshal(body)
				if err != nil {
					return nil, fmt.Errorf("mockarty: marshal request: %w", err)
				}
				bodyReader = bytes.NewReader(data)
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("mockarty: create request: %w", err)
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		if c.apiKey != "" {
			req.Header.Set(headerAPIKey, c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("mockarty: http request: %w", err)
			// Retry on transport errors
			if attempt < attempts-1 {
				continue
			}
			return nil, lastErr
		}

		// Success range — return body
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.Body, nil
		}

		// Read error body
		errBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			RequestID:  resp.Header.Get(headerRequestID),
		}

		// Parse the server's uniform error envelope:
		//   {"error": "...", "code": "...", "request_id": "..."}
		// `message` is accepted as a legacy fallback for very old servers.
		var errResp struct {
			Error     string `json:"error"`
			Message   string `json:"message"`
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		}
		if json.Unmarshal(errBody, &errResp) == nil {
			if errResp.Error != "" {
				apiErr.Message = errResp.Error
			} else if errResp.Message != "" {
				apiErr.Message = errResp.Message
			}
			if errResp.Code != "" {
				apiErr.Code = errResp.Code
			}
			// Body request_id wins over the X-Request-Id header: if both
			// are present they should match, but the body is the canonical
			// source in the new envelope.
			if errResp.RequestID != "" {
				apiErr.RequestID = errResp.RequestID
			}
		}
		if apiErr.Message == "" {
			apiErr.Message = strings.TrimSpace(string(errBody))
		}

		lastErr = apiErr

		// Only retry on 5xx or 429
		if resp.StatusCode >= 500 || resp.StatusCode == 429 {
			if attempt < attempts-1 {
				continue
			}
		}

		// Non-retryable error
		return nil, apiErr
	}

	return nil, lastErr
}
