package externalruns

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// userAgent is the default User-Agent header value. It is overridable via
// WithUserAgent — useful when this client is embedded inside another tool
// that wants to identify itself (e.g. mockarty-cli/v1.2.3).
const userAgent = "mockarty-go-sdk/externalruns"

// defaultTimeout caps a single HTTP round-trip. It is intentionally
// generous — Mockarty deployments behind slow CI proxies do exist — and
// can be overridden via WithTimeout or by supplying a custom *http.Client.
const defaultTimeout = 30 * time.Second

// Client is a thin REST wrapper around Mockarty's external-runs endpoint.
// A *Client is safe for concurrent use by multiple goroutines.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
	token      string
	namespace  string
	userAgent  string
}

// Option mutates a *Client during construction. Use the With* helpers
// rather than touching unexported fields directly — they apply consistent
// defaults and validation.
type Option func(*Client)

// WithHTTPClient injects a caller-owned *http.Client. The caller is
// responsible for its lifecycle and for setting any TLS / proxy / retry
// transport. When unset, a fresh client with defaultTimeout is used.
func WithHTTPClient(c *http.Client) Option {
	return func(cl *Client) {
		if c != nil {
			cl.httpClient = c
		}
	}
}

// WithTimeout overrides the default per-request timeout. Ignored when a
// custom *http.Client is also supplied — set the timeout on the client
// instead.
func WithTimeout(d time.Duration) Option {
	return func(cl *Client) {
		if d > 0 {
			cl.httpClient.Timeout = d
		}
	}
}

// WithUserAgent sets the User-Agent string sent on every request.
func WithUserAgent(ua string) Option {
	return func(cl *Client) {
		if strings.TrimSpace(ua) != "" {
			cl.userAgent = ua
		}
	}
}

// NewClient constructs an ExternalRuns client.
//
// baseURL is the Mockarty admin server's root (e.g. "https://mockarty.example.com"),
// namespace selects the target Mockarty namespace, token is an API key
// minted via the Admin UI or POST /api/v1/auth/tokens.
//
// Returns ErrInvalidConfig if any required field is empty or baseURL is
// not a syntactically valid HTTP(S) URL.
func NewClient(baseURL, namespace, token string, opts ...Option) (*Client, error) {
	baseURL = strings.TrimSpace(baseURL)
	namespace = strings.TrimSpace(namespace)
	token = strings.TrimSpace(token)
	if baseURL == "" || namespace == "" || token == "" {
		return nil, fmt.Errorf("%w: baseURL, namespace, and token are required", ErrInvalidConfig)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: parse baseURL: %v", ErrInvalidConfig, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: baseURL scheme must be http or https, got %q", ErrInvalidConfig, parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%w: baseURL missing host", ErrInvalidConfig)
	}
	// Strip trailing slash so url.JoinPath produces a clean concatenation.
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    parsed,
		token:      token,
		namespace:  namespace,
		userAgent:  userAgent,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c, nil
}

// Namespace returns the namespace the client is bound to.
func (c *Client) Namespace() string { return c.namespace }

// BaseURL returns the configured admin URL (without trailing slash).
func (c *Client) BaseURL() string { return c.baseURL.String() }

// runSegments returns the path segments leading to the external-runs
// endpoint. Each element is an unescaped segment; endpointURL handles
// escaping. Returning a slice (rather than a "/"-joined string) keeps a
// caller-supplied namespace or run ID containing literal "/" from being
// silently split.
func (c *Client) runSegments(suffix ...string) []string {
	segs := []string{"api", "v1", "namespaces", c.namespace, "tcm", "external-runs"}
	for _, s := range suffix {
		if s == "" {
			continue
		}
		segs = append(segs, s)
	}
	return segs
}

// endpointURL composes baseURL + path segments + optional query, escaping
// each segment exactly once. Required because the namespace (and run IDs)
// may legitimately contain characters that need encoding.
//
// net/url contract: Path is the decoded form, RawPath the encoded hint —
// when they correspond (RawPath unescapes to Path) URL.String() uses
// RawPath verbatim, otherwise it re-escapes Path. We set both.
func (c *Client) endpointURL(segments []string, query url.Values) string {
	u := *c.baseURL
	encoded := make([]string, 0, len(segments)+1)
	decoded := make([]string, 0, len(segments)+1)
	// Preserve any prefix path baked into baseURL (e.g. server mounted at /mock/).
	if base := strings.Trim(c.baseURL.Path, "/"); base != "" {
		for _, p := range strings.Split(base, "/") {
			encoded = append(encoded, url.PathEscape(p))
			decoded = append(decoded, p)
		}
	}
	for _, s := range segments {
		encoded = append(encoded, url.PathEscape(s))
		decoded = append(decoded, s)
	}
	u.Path = "/" + strings.Join(decoded, "/")
	u.RawPath = "/" + strings.Join(encoded, "/")
	if query != nil {
		u.RawQuery = query.Encode()
	} else {
		u.RawQuery = ""
	}
	return u.String()
}

// doJSON executes a JSON-in/JSON-out call. respOut may be nil when the
// caller does not need the response body parsed.
func (c *Client) doJSON(ctx context.Context, method string, segments []string, query url.Values, body, respOut any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", method, err)
		}
		reader = bytes.NewReader(buf)
	}

	endpoint := c.endpointURL(segments, query)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build %s %s: %w", method, endpoint, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.applyAuthHeaders(req)

	return c.execute(req, respOut)
}

// applyAuthHeaders writes the auth / schema / UA headers onto req. Called
// from both JSON and multipart code paths so they stay consistent.
func (c *Client) applyAuthHeaders(req *http.Request) {
	req.Header.Set(AuthHeader, c.token)
	req.Header.Set(SchemaVersionHeader, strconv.Itoa(SchemaVersion))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
}

// execute performs the round-trip and decodes the body. The response
// body is always drained-and-closed so connections return to the pool.
func (c *Client) execute(req *http.Request, respOut any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}

	if respOut == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(respOut); err != nil {
		return fmt.Errorf("decode %s %s: %w", req.Method, req.URL.Path, err)
	}
	return nil
}

// decodeAPIError reads a non-2xx response body and synthesises an *APIError.
// It tolerates non-JSON server replies (proxies, html error pages).
func decodeAPIError(resp *http.Response) error {
	const maxBody = 16 << 10 // 16 KiB — enough for any reasonable error
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	apiErr := &APIError{StatusCode: resp.StatusCode, RawBody: string(raw)}
	if len(raw) > 0 && bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		var env struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    string `json:"code"`
		}
		if err := json.Unmarshal(raw, &env); err == nil {
			apiErr.Message = firstNonEmpty(env.Error, env.Message)
			apiErr.Code = env.Code
		}
	}
	return apiErr
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// CreateRun registers a new external run with the admin server.
//
// On success the returned *Run has its server-assigned ID populated; pass
// it to AddSteps / AttachReport / FinishRun.
func (c *Client) CreateRun(ctx context.Context, req CreateRunRequest) (*Run, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("%w: CreateRunRequest.Name is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(req.Framework) == "" {
		return nil, fmt.Errorf("%w: CreateRunRequest.Framework is required", ErrInvalidRequest)
	}
	var out Run
	if err := c.doJSON(ctx, http.MethodPost, c.runSegments(), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddSteps appends (or upserts on StepKey) one or more steps to an
// existing run. The call is idempotent on (run_id, step_key).
//
// Passing an empty slice is a no-op and returns nil without contacting
// the server — convenient for streaming reporters that flush periodically.
func (c *Client) AddSteps(ctx context.Context, runID string, steps []Step) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("%w: runID is required", ErrInvalidRequest)
	}
	if len(steps) == 0 {
		return nil
	}
	for i := range steps {
		if strings.TrimSpace(steps[i].StepKey) == "" {
			return fmt.Errorf("%w: steps[%d].StepKey is required", ErrInvalidRequest, i)
		}
		if steps[i].Status != "" && !steps[i].Status.Valid() {
			return fmt.Errorf("%w: steps[%d].Status %q is not a known value", ErrInvalidRequest, i, steps[i].Status)
		}
	}
	body := struct {
		Steps []Step `json:"steps"`
	}{Steps: steps}
	return c.doJSON(ctx, http.MethodPost, c.runSegments(runID, "steps"), nil, body, nil)
}

// AttachReport uploads an artefact (Allure JSON, JUnit XML, screenshot,
// log file, ...) to a run via multipart/form-data.
//
// mime may be empty — the server then derives it from the file name. To
// stream a large file without buffering, supply a *bytes.Buffer or any
// other io.Reader via an internal helper (kept private until a use case
// arrives — see Phase 2 follow-ups).
func (c *Client) AttachReport(ctx context.Context, runID, name string, content []byte, mime string) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("%w: runID is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%w: attachment name is required", ErrInvalidRequest)
	}
	if len(content) == 0 {
		return fmt.Errorf("%w: attachment content is empty", ErrInvalidRequest)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Custom part headers — multipart.CreateFormFile hard-codes octet-stream.
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, name))
	if strings.TrimSpace(mime) != "" {
		hdr.Set("Content-Type", mime)
	} else {
		hdr.Set("Content-Type", "application/octet-stream")
	}
	part, err := writer.CreatePart(hdr)
	if err != nil {
		return fmt.Errorf("multipart part: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("multipart write: %w", err)
	}
	if err := writer.WriteField("name", name); err != nil {
		return fmt.Errorf("multipart field name: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("multipart close: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL(c.runSegments(runID, "attachments"), nil), &buf)
	if err != nil {
		return fmt.Errorf("build attachment request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.applyAuthHeaders(req)
	return c.execute(req, nil)
}

// FinishRun closes the run and sets its final status. After this call,
// AddSteps and AttachReport on the same runID are rejected by the server.
func (c *Client) FinishRun(ctx context.Context, runID string, req FinishRunRequest) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("%w: runID is required", ErrInvalidRequest)
	}
	if req.Status != "" && !req.Status.Valid() {
		return fmt.Errorf("%w: FinishRunRequest.Status %q is not a known value", ErrInvalidRequest, req.Status)
	}
	return c.doJSON(ctx, http.MethodPost, c.runSegments(runID, "finish"), nil, req, nil)
}

// GetRun fetches a single run by its server-assigned ID.
func (c *Client) GetRun(ctx context.Context, runID string) (*Run, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("%w: runID is required", ErrInvalidRequest)
	}
	var out Run
	if err := c.doJSON(ctx, http.MethodGet, c.runSegments(runID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListRuns returns runs in the bound namespace filtered by the supplied
// options. Iterate over RunList.NextCursor (when non-empty) to walk
// further pages.
func (c *Client) ListRuns(ctx context.Context, opts ListRunsOptions) (*RunList, error) {
	if opts.Status != "" && !opts.Status.Valid() {
		return nil, fmt.Errorf("%w: ListRunsOptions.Status %q is not a known value", ErrInvalidRequest, opts.Status)
	}
	q := url.Values{}
	if opts.SuiteID != "" {
		q.Set("suite_id", opts.SuiteID)
	}
	if opts.Framework != "" {
		q.Set("framework", opts.Framework)
	}
	if opts.Status != "" {
		q.Set("status", string(opts.Status))
	}
	if opts.ExternalID != "" {
		q.Set("external_id", opts.ExternalID)
	}
	if opts.Cursor != "" {
		q.Set("cursor", opts.Cursor)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	var out RunList
	if err := c.doJSON(ctx, http.MethodGet, c.runSegments(), q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AsAPIError unwraps err and returns the underlying *APIError, or nil if
// err is not an API-level error (e.g. it is a network or validation
// error). Helper to make caller branching readable:
//
//	if api := externalruns.AsAPIError(err); api != nil && api.StatusCode == 404 { ... }
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}
