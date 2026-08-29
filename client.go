// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

const (
	defaultNamespace = "sandbox"
	defaultTimeout   = 30 * time.Second
	headerAPIKey     = "X-API-Key"
	headerRequestID  = "X-Request-Id"
)

type requestHeadersContextKey struct{}

func withRequestHeaders(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, requestHeadersContextKey{}, headers)
}

func (c *Client) cloudContext(ctx context.Context) context.Context {
	headers := map[string]string{headerAPIKey: ""}
	if c.apiKey != "" {
		headers["Authorization"] = "Bearer " + c.apiKey
	}
	return withRequestHeaders(ctx, headers)
}

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

	// Sub-API singletons, all eagerly initialized in NewClient.
	mockAPI                *MockAPI
	namespaceAPI           *NamespaceAPI
	storeAPI               *StoreAPI
	collectionAPI          *CollectionAPI
	perfAPI                *PerfAPI
	healthAPI              *HealthAPI
	generatorAPI           *GeneratorAPI
	fuzzingAPI             *FuzzingAPI
	contractAPI            *ContractAPI
	recorderAPI            *RecorderAPI
	templateAPI            *TemplateAPI
	importAPI              *ImportAPI
	testRunAPI             *TestRunAPI
	tagAPI                 *TagAPI
	uiTestAPI              *UITestAPI
	gitSyncAPI             *GitSyncAPI
	folderAPI              *FolderAPI
	undefinedAPI           *UndefinedAPI
	statsAPI               *StatsAPI
	agentTaskAPI           *AgentTaskAPI
	issueTrackerAPI        *IssueTrackerAPI
	mcpAPI                 *MCPAPI
	namespaceSettingsAPI   *NamespaceSettingsAPI
	proxyAPI               *ProxyAPI
	environmentAPI         *EnvironmentAPI
	chaosAPI               *ChaosAPI
	testPlansAPI           *TestPlansAPI
	entitySearchAPI        *EntitySearchAPI
	secretsAPI             *SecretsAPI
	promptsAPI             *PromptsAPI
	meAPI                  *MeAPI
	externalRunsAPI        *ExternalRunsAPI
	flowRunsAPI            *FlowRunsAPI
	discoveryAPI           *DiscoveryAPI
	experienceAPI          *ExperienceAPI
	economicsAPI           *EconomicsAPI
	cloudWebhooksAPI       *CloudWebhooksAPI
	cloudInstancesAPI      *CloudInstancesAPI
	cloudSpacesAPI         *CloudSpacesAPI
	cloudEntitlementsAPI   *CloudEntitlementsAPI
	cloudSharedProjectsAPI *CloudSharedProjectsAPI
	cloudOAuthProvidersAPI *CloudOAuthProvidersAPI
	cloudRiskAPI           *CloudRiskAPI
	cloudIdentityAPI       *CloudIdentityAPI
	autonomousMissionsAPI  *AutonomousMissionsAPI
	workflowDefinitionsAPI *WorkflowDefinitionsAPI
	coderDeliveryAPI       *CoderDeliveryAPI
	deliveryPolicyAPI      *DeliveryPolicyAPI
	llmSecurityAPI         *LLMSecurityAPI
	pageAnalyzerAPI        *PageAnalyzerAPI
}

// NewClient creates a new Mockarty API client.
//
//	client := mockarty.NewClient("http://localhost:5770",
//	    mockarty.WithAPIKey("mk_..."),
//	    mockarty.WithNamespace("production"),
//	)
//
// The returned *Client is safe for concurrent use by multiple goroutines:
// every sub-API singleton (Mocks(), Namespaces(), ...) is initialised
// eagerly here so the accessor methods do not race on lazy assignment.
func NewClient(baseURL string, opts ...Option) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	jar, _ := cookiejar.New(nil)

	c := &Client{
		baseURL:   baseURL,
		namespace: defaultNamespace,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
			Jar:     jar,
		},
		logger: slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Eagerly construct every sub-API so accessor methods are race-free.
	// Each binding is a single-field struct (8 bytes), so the up-front cost
	// is negligible — far cheaper than guarding 28 lazy slots with a mutex
	// or sync.Once each.
	c.mockAPI = &MockAPI{client: c}
	c.namespaceAPI = &NamespaceAPI{client: c}
	c.storeAPI = &StoreAPI{client: c}
	c.collectionAPI = &CollectionAPI{client: c}
	c.perfAPI = &PerfAPI{client: c}
	c.healthAPI = &HealthAPI{client: c}
	c.generatorAPI = &GeneratorAPI{client: c}
	c.fuzzingAPI = &FuzzingAPI{client: c}
	c.contractAPI = &ContractAPI{client: c}
	c.recorderAPI = &RecorderAPI{client: c}
	c.templateAPI = &TemplateAPI{client: c}
	c.importAPI = &ImportAPI{client: c}
	c.testRunAPI = &TestRunAPI{client: c}
	c.tagAPI = &TagAPI{client: c}
	c.uiTestAPI = &UITestAPI{client: c}
	c.gitSyncAPI = &GitSyncAPI{client: c}
	c.folderAPI = &FolderAPI{client: c}
	c.undefinedAPI = &UndefinedAPI{client: c}
	c.statsAPI = &StatsAPI{client: c}
	c.agentTaskAPI = &AgentTaskAPI{client: c}
	c.issueTrackerAPI = &IssueTrackerAPI{client: c}
	c.mcpAPI = &MCPAPI{client: c, endpoint: c.baseURL + "/mcp"}
	c.namespaceSettingsAPI = &NamespaceSettingsAPI{client: c}
	c.proxyAPI = &ProxyAPI{client: c}
	c.environmentAPI = &EnvironmentAPI{client: c}
	c.chaosAPI = &ChaosAPI{client: c}
	c.testPlansAPI = &TestPlansAPI{client: c}
	c.entitySearchAPI = &EntitySearchAPI{client: c}
	c.secretsAPI = &SecretsAPI{client: c}
	c.promptsAPI = &PromptsAPI{client: c}
	c.meAPI = &MeAPI{client: c}
	c.externalRunsAPI = &ExternalRunsAPI{client: c}
	c.flowRunsAPI = &FlowRunsAPI{client: c}
	c.discoveryAPI = &DiscoveryAPI{client: c}
	c.experienceAPI = &ExperienceAPI{client: c}
	c.economicsAPI = &EconomicsAPI{client: c}
	c.cloudWebhooksAPI = &CloudWebhooksAPI{client: c}
	c.cloudInstancesAPI = &CloudInstancesAPI{client: c}
	c.cloudSpacesAPI = &CloudSpacesAPI{client: c}
	c.cloudEntitlementsAPI = &CloudEntitlementsAPI{client: c}
	c.cloudSharedProjectsAPI = &CloudSharedProjectsAPI{client: c}
	c.cloudOAuthProvidersAPI = &CloudOAuthProvidersAPI{client: c}
	c.cloudRiskAPI = &CloudRiskAPI{client: c}
	c.cloudIdentityAPI = &CloudIdentityAPI{client: c}
	c.autonomousMissionsAPI = &AutonomousMissionsAPI{client: c}
	c.workflowDefinitionsAPI = &WorkflowDefinitionsAPI{client: c}
	c.coderDeliveryAPI = &CoderDeliveryAPI{client: c}
	c.deliveryPolicyAPI = &DeliveryPolicyAPI{client: c}
	c.llmSecurityAPI = &LLMSecurityAPI{client: c}
	c.pageAnalyzerAPI = &PageAnalyzerAPI{client: c}

	return c
}

// ExternalRuns returns the external-runs upload API — used to ship
// arbitrary test results (e.g. fluent Tester DSL output) into TCM.
func (c *Client) ExternalRuns() *ExternalRunsAPI { return c.externalRunsAPI }

// PageAnalyzer returns the HTTP-level page analysis API.
func (c *Client) PageAnalyzer() *PageAnalyzerAPI { return c.pageAnalyzerAPI }

// FlowRuns returns the client for POST /api/v1/api-tester/flow-runs —
// the server-side IR runner. See sdk/go-sdk/api_flow_runs.go for usage.
func (c *Client) FlowRuns() *FlowRunsAPI { return c.flowRunsAPI }

// Discovery returns the TCM test-discovery API — used to sync a test
// framework's collected case list (pytest --collect-only, `go test -list`,
// etc.) into the Mockarty catalogue. See sdk/go-sdk/api_discovery.go.
func (c *Client) Discovery() *DiscoveryAPI { return c.discoveryAPI }

// Experience returns the reusable run-experience search and record API.
func (c *Client) Experience() *ExperienceAPI { return c.experienceAPI }

// Economics returns the administrator LLM usage and immutable price-book API.
func (c *Client) Economics() *EconomicsAPI { return c.economicsAPI }

// WorkflowDefinitions returns the versioned workflow authoring API.
func (c *Client) WorkflowDefinitions() *WorkflowDefinitionsAPI { return c.workflowDefinitionsAPI }

// CloudWebhooks returns the curated Cloud webhook lifecycle API.
func (c *Client) CloudWebhooks() *CloudWebhooksAPI { return c.cloudWebhooksAPI }

// CloudInstances returns the dedicated Cloud contour lifecycle API.
func (c *Client) CloudInstances() *CloudInstancesAPI { return c.cloudInstancesAPI }

// CloudSpaces returns the explicit Space collaboration API.
func (c *Client) CloudSpaces() *CloudSpacesAPI { return c.cloudSpacesAPI }

// CloudEntitlements returns the committed Cloud entitlement projection API.
func (c *Client) CloudEntitlements() *CloudEntitlementsAPI { return c.cloudEntitlementsAPI }

// CloudSharedProjects returns the public Shared SaaS project CRUD API.
func (c *Client) CloudSharedProjects() *CloudSharedProjectsAPI { return c.cloudSharedProjectsAPI }

// CloudOAuthProviders returns the operator-only cabinet sign-in provider API.
func (c *Client) CloudOAuthProviders() *CloudOAuthProvidersAPI { return c.cloudOAuthProvidersAPI }

// CloudRisk returns the operator-only risk case and enforcement API.
func (c *Client) CloudRisk() *CloudRiskAPI { return c.cloudRiskAPI }

// CloudIdentity returns the Cloud account sign-in-method and step-up API.
func (c *Client) CloudIdentity() *CloudIdentityAPI { return c.cloudIdentityAPI }

// AutonomousMissions returns the API used to submit and supervise autonomous
// testing missions.
func (c *Client) AutonomousMissions() *AutonomousMissionsAPI { return c.autonomousMissionsAPI }

// CoderDelivery returns the admitted-repository and deployment mission API.
func (c *Client) CoderDelivery() *CoderDeliveryAPI { return c.coderDeliveryAPI }

// DeliveryPolicy returns the administrator environment-policy API.
func (c *Client) DeliveryPolicy() *DeliveryPolicyAPI { return c.deliveryPolicyAPI }

// LLMSecurity returns the layered prompt-security management API.
func (c *Client) LLMSecurity() *LLMSecurityAPI { return c.llmSecurityAPI }

// BaseURL returns the configured base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// Namespace returns the configured default namespace.
func (c *Client) Namespace() string { return c.namespace }

// ---------------------------------------------------------------------------
// Sub-API accessors (each returns the eagerly-initialised singleton; safe
// for concurrent use — see NewClient).
// ---------------------------------------------------------------------------

// Mocks returns the Mock CRUD API.
func (c *Client) Mocks() *MockAPI { return c.mockAPI }

// Namespaces returns the Namespace API.
func (c *Client) Namespaces() *NamespaceAPI { return c.namespaceAPI }

// Stores returns the Store API.
func (c *Client) Stores() *StoreAPI { return c.storeAPI }

// Collections returns the Collection API.
func (c *Client) Collections() *CollectionAPI { return c.collectionAPI }

// Perf returns the Performance Testing API.
func (c *Client) Perf() *PerfAPI { return c.perfAPI }

// Health returns the Health API.
func (c *Client) Health() *HealthAPI { return c.healthAPI }

// Generator returns the Generator API for creating mocks from specifications.
func (c *Client) Generator() *GeneratorAPI { return c.generatorAPI }

// Fuzzing returns the Fuzzing API for security and fuzz testing.
func (c *Client) Fuzzing() *FuzzingAPI { return c.fuzzingAPI }

// Contracts returns the Contract Testing API.
func (c *Client) Contracts() *ContractAPI { return c.contractAPI }

// Recorder returns the Recorder API for traffic recording.
func (c *Client) Recorder() *RecorderAPI { return c.recorderAPI }

// Templates returns the Template API for managing response templates.
func (c *Client) Templates() *TemplateAPI { return c.templateAPI }

// Import returns the Import API for importing API definitions.
func (c *Client) Import() *ImportAPI { return c.importAPI }

// TestRuns returns the Test Run API for managing test executions.
func (c *Client) TestRuns() *TestRunAPI { return c.testRunAPI }

// Tags returns the Tag API for managing mock tags.
func (c *Client) Tags() *TagAPI { return c.tagAPI }

// UITests returns the recorded-UI-test API (save / run / poll / export).
func (c *Client) UITests() *UITestAPI { return c.uiTestAPI }

// GitSync returns the git-sync API (bind a repo, pull/push autotest collections).
func (c *Client) GitSync() *GitSyncAPI { return c.gitSyncAPI }

// Folders returns the Folder API for managing mock folders.
func (c *Client) Folders() *FolderAPI { return c.folderAPI }

// Undefined returns the Undefined API for managing unmatched requests.
func (c *Client) Undefined() *UndefinedAPI { return c.undefinedAPI }

// Stats returns the Stats API for retrieving platform statistics.
func (c *Client) Stats() *StatsAPI { return c.statsAPI }

// AgentTasks returns the Agent Task API for managing AI agent tasks.
func (c *Client) AgentTasks() *AgentTaskAPI { return c.agentTaskAPI }

// IssueTracker returns the issue-tracker task-automation API.
func (c *Client) IssueTracker() *IssueTrackerAPI { return c.issueTrackerAPI }

// NamespaceSettings returns the Namespace Settings API.
func (c *Client) NamespaceSettings() *NamespaceSettingsAPI { return c.namespaceSettingsAPI }

// Proxy returns the Proxy API for proxying requests.
func (c *Client) Proxy() *ProxyAPI { return c.proxyAPI }

// Environments returns the Environment API for managing API Tester environments.
func (c *Client) Environments() *EnvironmentAPI { return c.environmentAPI }

// Chaos returns the Chaos Engineering API for managing chaos experiments.
func (c *Client) Chaos() *ChaosAPI { return c.chaosAPI }

// TestPlans returns the Test Plans API — the master orchestrator for
// functional / fuzz / chaos / load / contract runs under a single plan.
func (c *Client) TestPlans() *TestPlansAPI { return c.testPlansAPI }

// Secrets returns the centralised Secrets Storage API (Phase A0 —
// namespace-scoped encrypted key/value stores with optional Vault backend).
func (c *Client) Secrets() *SecretsAPI { return c.secretsAPI }

// Prompts returns the Prompts Storage API — managed AI prompts with
// FIFO-20 version history and rollback.
func (c *Client) Prompts() *PromptsAPI { return c.promptsAPI }

// EntitySearch returns the unified entity-picker API. Use it to look up
// mocks / Test Plans / perf configs / fuzz configs / chaos experiments /
// contract pacts by case-insensitive name match — the same backend the
// admin UI pickers consume.
func (c *Client) EntitySearch() *EntitySearchAPI { return c.entitySearchAPI }

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
//
// Body marshalling rules:
//   - body == nil           → no body
//   - body is []byte        → sent as-is (binary upload path — used by
//     Templates().Upload, Recorder export
//     import, etc.)
//   - body is io.Reader     → sent as-is
//   - anything else         → JSON-marshalled (the common case)
func (c *Client) doRaw(ctx context.Context, method, path string, body any) (io.ReadCloser, error) {
	return c.doRawCT(ctx, method, path, body, "application/json")
}

// doRawCT is doRaw with an explicit request Content-Type. Multipart uploads
// (TCM attachments) need "multipart/form-data; boundary=..." rather than JSON.
//
// The body is buffered into a []byte ONCE up front so every retry replays the
// exact same bytes: a []byte / io.Reader / multipart body must NOT be
// re-JSON-marshalled on retry (the previous code double-encoded []byte uploads
// and would have JSON-marshalled an already-consumed io.Reader).
func (c *Client) doRawCT(ctx context.Context, method, path string, body any, contentType string) (io.ReadCloser, error) {
	url := c.baseURL + path

	hasBody := body != nil
	var rawBody []byte
	if hasBody {
		switch b := body.(type) {
		case []byte:
			rawBody = b
		case io.Reader:
			data, err := io.ReadAll(b)
			if err != nil {
				return nil, fmt.Errorf("mockarty: read request body: %w", err)
			}
			rawBody = data
		default:
			data, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("mockarty: marshal request: %w", err)
			}
			rawBody = data
		}
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
		}

		// Fresh reader over the buffered bytes for every attempt (replayable).
		var bodyReader io.Reader
		if hasBody {
			bodyReader = bytes.NewReader(rawBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("mockarty: create request: %w", err)
		}

		if hasBody {
			req.Header.Set("Content-Type", contentType)
		}
		req.Header.Set("Accept", "application/json")
		suppressAPIKey := false
		if headers, ok := ctx.Value(requestHeadersContextKey{}).(map[string]string); ok {
			for name, value := range headers {
				if strings.EqualFold(name, headerAPIKey) && value == "" {
					suppressAPIKey = true
					continue
				}
				if value != "" {
					req.Header.Set(name, value)
				}
			}
		}

		if c.apiKey != "" && !suppressAPIKey {
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
