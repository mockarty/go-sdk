// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package pact

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Verifier replays consumer-published pact interactions against a
// real provider HTTP service and reports verification results.
//
// Typical flow (from a provider's CI job):
//
//	v, _ := pact.NewVerifier(
//	    pact.WithProviderURL("http://localhost:8080"),
//	    pact.WithProviderName("OrderAPI"),
//	    pact.WithProviderVersion(os.Getenv("GIT_COMMIT")),
//	    pact.WithStateHandler("user 42 exists", setUpUser42),
//	    pact.WithBrokerClient(brokerClient),
//	)
//	res, err := v.VerifyFromBroker(ctx, "OrderClient", "OrderAPI", "latest")
//	if !res.OK() { ... }
//	_ = v.PublishResults(ctx, "OrderClient", "OrderAPI", "1.2.3", res)
//
// The verifier is parity-aligned with the Python + Java SDK verifiers
// landing in the same change. The matcher engine is reused from the
// consumer-side mock server, so the same `Like`/`Regex`/`EachLike`
// rules that govern publish-time recording govern verify-time
// matching.
type Verifier struct {
	http             *http.Client
	broker           *BrokerClient
	stateHandlers    map[string]StateHandler
	stateSetupURL    string
	providerURL      string
	providerName     string
	providerVersion  string
	providerBranch   string
	requestFilter    RequestFilter
	requestTimeout   time.Duration
	messageProducers map[string]MessageProducer
}

// StateHandler is invoked once per provider-state declared on an
// interaction, BEFORE the request replay. The handler MUST mutate the
// real provider (e.g. seed a database row, flip a feature flag) so the
// interaction's expected response is reachable.
type StateHandler func(ctx context.Context, state string, params map[string]any) error

// RequestFilter rewrites an outgoing HTTP request before it hits the
// provider — useful for stamping a fresh JWT, signing the request, or
// adding a tenant header that the consumer's recorded test fixtures
// don't carry. Returning an error fails the interaction.
type RequestFilter func(ctx context.Context, req *http.Request) error

// VerifierOption configures a Verifier (functional options pattern).
type VerifierOption func(*Verifier)

// WithProviderURL sets the base URL of the provider under test.
// REQUIRED.
func WithProviderURL(u string) VerifierOption {
	return func(v *Verifier) { v.providerURL = strings.TrimRight(u, "/") }
}

// WithProviderName tags the verification result. REQUIRED when
// publishing results back to the broker.
func WithProviderName(name string) VerifierOption {
	return func(v *Verifier) { v.providerName = name }
}

// WithProviderVersion stamps the verification result with the
// provider's build version (semver / git-sha). REQUIRED when
// publishing.
func WithProviderVersion(version string) VerifierOption {
	return func(v *Verifier) { v.providerVersion = version }
}

// WithProviderBranch tags published results with a branch name.
func WithProviderBranch(branch string) VerifierOption {
	return func(v *Verifier) { v.providerBranch = branch }
}

// WithBrokerClient wires a broker for VerifyFromBroker / PublishResults.
func WithBrokerClient(c *BrokerClient) VerifierOption {
	return func(v *Verifier) { v.broker = c }
}

// WithStateHandler registers a per-state setup callback. The state
// name matches the pact's `providerStates[].name` (V4) or
// `providerState` (V3) verbatim.
func WithStateHandler(state string, fn StateHandler) VerifierOption {
	return func(v *Verifier) {
		if v.stateHandlers == nil {
			v.stateHandlers = make(map[string]StateHandler)
		}
		v.stateHandlers[state] = fn
	}
}

// WithStateSetupURL points the verifier at the provider's
// state-setup endpoint (pact-foundation convention: provider exposes
// `POST /_pact/provider_states` that accepts {"state":"...","params":{}}).
// Used when registering per-state handlers in code is impractical.
func WithStateSetupURL(u string) VerifierOption {
	return func(v *Verifier) { v.stateSetupURL = u }
}

// WithRequestFilter installs a request-rewrite hook (auth, tenant
// headers, etc.) applied to every replayed request.
func WithRequestFilter(fn RequestFilter) VerifierOption {
	return func(v *Verifier) { v.requestFilter = fn }
}

// WithRequestTimeout caps the per-interaction HTTP timeout. Applied
// via a per-request context.WithTimeout so it works regardless of
// whether a custom http.Client is also injected via WithHTTPClient
// (a Client.Timeout would be overridden by the custom client).
func WithRequestTimeout(d time.Duration) VerifierOption {
	return func(v *Verifier) { v.requestTimeout = d }
}

// WithHTTPClient injects a custom http.Client (mTLS, custom
// transport). The per-request timeout from WithRequestTimeout still
// applies via context — set Client.Timeout = 0 on your custom client
// if you want the verifier's timeout to be the only deadline.
func WithHTTPClient(c *http.Client) VerifierOption {
	return func(v *Verifier) { v.http = c }
}

// NewVerifier constructs a verifier. ProviderURL is required.
func NewVerifier(opts ...VerifierOption) (*Verifier, error) {
	v := &Verifier{
		requestTimeout: 30 * time.Second,
		stateHandlers:  map[string]StateHandler{},
	}
	for _, opt := range opts {
		opt(v)
	}
	if v.providerURL == "" {
		return nil, errors.New("pact: NewVerifier requires WithProviderURL")
	}
	if v.http == nil {
		// Default client has no Client.Timeout — the per-request
		// context deadline (see verifyOne) is the single source of
		// truth so the behaviour is consistent with the custom-
		// client path.
		v.http = &http.Client{}
	}
	return v, nil
}

// VerificationResult collects per-interaction outcomes.
type VerificationResult struct {
	Provider     string
	StartedAt    time.Time
	FinishedAt   time.Time
	Interactions []InteractionResult
}

// InteractionResult is a single interaction's verification outcome.
type InteractionResult struct {
	Description string
	State       string
	Mismatches  []MatchMismatch
	Error       string
	StatusCode  int
	Passed      bool
}

// OK reports whether every interaction passed.
func (r *VerificationResult) OK() bool {
	if r == nil {
		return false
	}
	for _, ir := range r.Interactions {
		if !ir.Passed {
			return false
		}
	}
	return true
}

// VerifyPactBytes parses a pact JSON document and replays every
// interaction against the configured provider.
func (v *Verifier) VerifyPactBytes(ctx context.Context, raw []byte) (*VerificationResult, error) {
	doc, err := parsePactDoc(raw)
	if err != nil {
		return nil, err
	}
	return v.verifyInteractions(ctx, doc.Interactions)
}

// VerifyPactFile is VerifyPactBytes with a filesystem load.
func (v *Verifier) VerifyPactFile(ctx context.Context, path string) (*VerificationResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("pact: read %s: %w", path, err)
	}
	return v.VerifyPactBytes(ctx, b)
}

// VerifyFromBroker pulls a published pact from the broker and
// verifies it. Requires WithBrokerClient.
func (v *Verifier) VerifyFromBroker(ctx context.Context, consumer, provider, version string) (*VerificationResult, error) {
	if v.broker == nil {
		return nil, errors.New("pact: VerifyFromBroker requires WithBrokerClient")
	}
	body, err := v.broker.Fetch(ctx, consumer, provider, version)
	if err != nil {
		return nil, err
	}
	return v.VerifyPactBytes(ctx, body)
}

// PublishResults POSTs a verification outcome back to the broker.
// Spec: pact-foundation /pacts/.../verification-results.
func (v *Verifier) PublishResults(ctx context.Context, consumer, provider, version string, res *VerificationResult) error {
	if v.broker == nil {
		return errors.New("pact: PublishResults requires WithBrokerClient")
	}
	if v.providerVersion == "" {
		return errors.New("pact: PublishResults requires WithProviderVersion")
	}
	if res == nil {
		return errors.New("pact: PublishResults requires a result")
	}
	payload := map[string]any{
		"success":           res.OK(),
		"providerApplicationVersion": v.providerVersion,
		"verifiedBy": map[string]any{
			"implementation": "mockarty-go-sdk",
			"version":        "1",
		},
		"testResults": flattenInteractionResults(res),
	}
	if v.providerBranch != "" {
		payload["buildUrl"] = ""
		payload["branch"] = v.providerBranch
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("pact: marshal verification results: %w", err)
	}
	path := "/pacts/provider/" + url.PathEscape(provider) +
		"/consumer/" + url.PathEscape(consumer) +
		"/pact-version/" + url.PathEscape(version) +
		"/verification-results"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		v.broker.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, vs := range brokerAuthHeader(v.broker) {
		for _, val := range vs {
			req.Header.Add(k, val)
		}
	}
	resp, err := v.broker.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pact: PublishResults HTTP %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ----------------------------------------------------------------------
// internals
// ----------------------------------------------------------------------

func (v *Verifier) verifyInteractions(ctx context.Context, ins []Interaction) (*VerificationResult, error) {
	res := &VerificationResult{
		Provider:  v.providerName,
		StartedAt: time.Now().UTC(),
	}
	for _, in := range ins {
		ir := v.verifyOne(ctx, in)
		res.Interactions = append(res.Interactions, ir)
	}
	res.FinishedAt = time.Now().UTC()
	return res, nil
}

func (v *Verifier) verifyOne(ctx context.Context, in Interaction) InteractionResult {
	ir := InteractionResult{Description: in.Description}
	if err := v.setUpStates(ctx, in.ProviderStates); err != nil {
		ir.Error = "state setup: " + err.Error()
		return ir
	}
	if len(in.ProviderStates) > 0 {
		ir.State = in.ProviderStates[0].Name
	}

	// Apply the per-request timeout via context — works with both the
	// default http.Client and any user-supplied client (since
	// http.Client.Timeout would be overridden by WithHTTPClient).
	reqCtx, cancel := v.requestContext(ctx)
	defer cancel()

	req, err := buildHTTPRequest(reqCtx, v.providerURL, in.Request)
	if err != nil {
		ir.Error = "build request: " + err.Error()
		return ir
	}
	if v.requestFilter != nil {
		if err := v.requestFilter(reqCtx, req); err != nil {
			ir.Error = "request filter: " + err.Error()
			return ir
		}
	}
	resp, err := v.http.Do(req)
	if err != nil {
		ir.Error = "transport: " + err.Error()
		return ir
	}
	defer resp.Body.Close()
	ir.StatusCode = resp.StatusCode
	// Cap the body — a misbehaving provider should not OOM the
	// verifier; 16 MiB matches the broker.Fetch cap.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		ir.Error = "read body: " + err.Error()
		return ir
	}

	ir.Mismatches = compareResponse(in.Response, resp.StatusCode, resp.Header, body)
	ir.Passed = len(ir.Mismatches) == 0 && ir.Error == ""
	return ir
}

// requestContext derives a context with the verifier's per-request
// timeout. If requestTimeout <= 0 the parent context is returned
// as-is, with a no-op cancel.
func (v *Verifier) requestContext(parent context.Context) (context.Context, context.CancelFunc) {
	if v.requestTimeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, v.requestTimeout)
}

func (v *Verifier) setUpStates(ctx context.Context, states []ProviderState) error {
	for _, st := range states {
		if fn, ok := v.stateHandlers[st.Name]; ok {
			if err := fn(ctx, st.Name, st.Params); err != nil {
				return err
			}
			continue
		}
		if v.stateSetupURL != "" {
			if err := v.callStateSetup(ctx, st); err != nil {
				return err
			}
			continue
		}
		// No handler / URL — silently skip; pact V3 allows states to
		// be informational. Strict mode could be a future option.
	}
	return nil
}

func (v *Verifier) callStateSetup(ctx context.Context, st ProviderState) error {
	body, err := json.Marshal(map[string]any{
		"state":  st.Name,
		"params": st.Params,
		"action": "setup",
	})
	if err != nil {
		return fmt.Errorf("marshal state setup: %w", err)
	}
	reqCtx, cancel := v.requestContext(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, v.stateSetupURL,
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := v.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("state-setup HTTP %d", resp.StatusCode)
	}
	return nil
}

func buildHTTPRequest(ctx context.Context, base string, r Request) (*http.Request, error) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}
	u, err := url.Parse(base + r.Path)
	if err != nil {
		return nil, err
	}
	if len(r.Query) > 0 {
		q := u.Query()
		for k, vs := range r.Query {
			for _, val := range vs {
				q.Add(k, val)
			}
		}
		u.RawQuery = q.Encode()
	}
	var body io.Reader
	if r.Body != nil {
		raw, err := bodyToBytes(r.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	for k, vs := range r.Headers {
		for _, val := range vs {
			req.Header.Add(k, val)
		}
	}
	if r.Body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func bodyToBytes(b any) ([]byte, error) {
	switch v := b.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return json.Marshal(stripMatchersForBody(b))
	}
}

// compareResponse runs the matcher engine on status + headers + body.
func compareResponse(expected Response, actualStatus int, actualHeaders http.Header, actualBody []byte) []MatchMismatch {
	var miss []MatchMismatch
	if expected.Status != 0 && expected.Status != actualStatus {
		miss = append(miss, MatchMismatch{
			Path:     "$.status",
			Reason:   "status mismatch",
			Expected: expected.Status,
			Actual:   actualStatus,
		})
	}
	for k, vs := range expected.Headers {
		got := actualHeaders.Values(k)
		if len(got) == 0 {
			miss = append(miss, MatchMismatch{
				Path:     "$.headers." + k,
				Reason:   "expected header missing",
				Expected: vs,
				Actual:   nil,
			})
			continue
		}
		// For each expected value, look for it as either an exact match
		// of one of the actual values OR as a parameter-of-a-parameter
		// list (e.g. expected "application/json" matches actual
		// "application/json; charset=utf-8"). Substring search across
		// comma-joined values would false-match overlapping tokens
		// (`"text/plain"` is a substring of `"application/json,text/plain"`
		// even when the producer never sent "text/plain" on its own).
		for _, want := range vs {
			if !headerValueMatches(got, want) {
				miss = append(miss, MatchMismatch{
					Path:     "$.headers." + k,
					Reason:   "header value mismatch",
					Expected: want,
					Actual:   strings.Join(got, ","),
				})
			}
		}
	}
	if expected.Body != nil {
		miss = append(miss, bodyMismatches(expected.Body, actualBody)...)
	}
	return miss
}

// headerValueMatches reports whether the expected header value `want`
// is present in any of the actual header values `got`. A value
// matches when it equals `got[i]` verbatim OR when it equals the
// pre-`;` token of `got[i]` (Content-Type-style parameter stripping:
// expected "application/json" matches actual "application/json; charset=utf-8").
func headerValueMatches(got []string, want string) bool {
	wantTrim := strings.TrimSpace(want)
	for _, g := range got {
		if g == want {
			return true
		}
		// Strip parameters after a `;` and compare the bare token.
		bare := g
		if i := strings.IndexByte(g, ';'); i >= 0 {
			bare = strings.TrimSpace(g[:i])
		}
		if bare == wantTrim {
			return true
		}
	}
	return false
}

// brokerAuthHeader returns the header(s) a BrokerClient would attach
// to outbound requests (so the verifier can piggy-back the same auth
// when publishing results).
func brokerAuthHeader(c *BrokerClient) http.Header {
	h := http.Header{}
	if c.token != "" {
		h.Set("Authorization", "Bearer "+c.token)
		return h
	}
	if c.username != "" || c.password != "" {
		// Reuse the broker's applyAuth helper would be cleaner, but
		// http.Header doesn't share with http.Request.Header directly
		// — applyAuth is request-bound. Recompute via the same scheme.
		req, _ := http.NewRequest(http.MethodGet, c.baseURL, nil)
		c.applyAuth(req)
		if a := req.Header.Get("Authorization"); a != "" {
			h.Set("Authorization", a)
		}
	}
	return h
}

func flattenInteractionResults(res *VerificationResult) []map[string]any {
	out := make([]map[string]any, 0, len(res.Interactions))
	for _, ir := range res.Interactions {
		row := map[string]any{
			"interactionDescription": ir.Description,
			"success":                ir.Passed,
		}
		if ir.State != "" {
			row["providerState"] = ir.State
		}
		if ir.Error != "" {
			row["error"] = ir.Error
		}
		if len(ir.Mismatches) > 0 {
			ms := make([]map[string]any, 0, len(ir.Mismatches))
			for _, m := range ir.Mismatches {
				ms = append(ms, map[string]any{
					"path":     m.Path,
					"reason":   m.Reason,
					"expected": m.Expected,
					"actual":   m.Actual,
				})
			}
			row["mismatches"] = ms
		}
		out = append(out, row)
	}
	return out
}

// ----------------------------------------------------------------------
// pact JSON parser (V3 + V4 union shape)
// ----------------------------------------------------------------------

// parsedPact is a permissive view over a pact JSON file — covers V3
// (`providerState` singular, no `type`) and V4 (`providerStates`
// plural, optional `type=Synchronous/HTTP`).
type parsedPact struct {
	Interactions []Interaction
}

func parsePactDoc(raw []byte) (*parsedPact, error) {
	var doc struct {
		Interactions []map[string]any `json:"interactions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("pact: parse: %w", err)
	}
	out := &parsedPact{}
	for _, ix := range doc.Interactions {
		in, err := decodeInteraction(ix)
		if err != nil {
			return nil, err
		}
		out.Interactions = append(out.Interactions, in)
	}
	return out, nil
}

func decodeInteraction(ix map[string]any) (Interaction, error) {
	var in Interaction
	in.Description, _ = ix["description"].(string)
	in.Type, _ = ix["type"].(string)

	// V4 plural form.
	if arr, ok := ix["providerStates"].([]any); ok {
		for _, s := range arr {
			if m, ok := s.(map[string]any); ok {
				ps := ProviderState{}
				ps.Name, _ = m["name"].(string)
				if p, ok := m["params"].(map[string]any); ok {
					ps.Params = p
				}
				in.ProviderStates = append(in.ProviderStates, ps)
			}
		}
	}
	// V3 singular form.
	if s, ok := ix["providerState"].(string); ok && s != "" && len(in.ProviderStates) == 0 {
		in.ProviderStates = []ProviderState{{Name: s}}
	}

	if req, ok := ix["request"].(map[string]any); ok {
		in.Request = decodeRequest(req)
	}
	if resp, ok := ix["response"].(map[string]any); ok {
		in.Response = decodeResponse(resp)
	}
	return in, nil
}

func decodeRequest(m map[string]any) Request {
	r := Request{}
	r.Method, _ = m["method"].(string)
	r.Path, _ = m["path"].(string)
	r.Headers = decodeHeaders(m["headers"])
	r.Query = decodeQuery(m["query"])
	r.Body = m["body"]
	return r
}

func decodeResponse(m map[string]any) Response {
	r := Response{}
	switch s := m["status"].(type) {
	case float64:
		r.Status = int(s)
	case string:
		if n, err := strconv.Atoi(s); err == nil {
			r.Status = n
		}
	}
	r.Headers = decodeHeaders(m["headers"])
	r.Body = m["body"]
	return r
}

func decodeHeaders(v any) map[string][]string {
	out := map[string][]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for k, val := range m {
		switch vv := val.(type) {
		case string:
			out[k] = []string{vv}
		case []any:
			for _, s := range vv {
				if str, ok := s.(string); ok {
					out[k] = append(out[k], str)
				}
			}
		}
	}
	return out
}

func decodeQuery(v any) map[string][]string {
	out := map[string][]string{}
	switch q := v.(type) {
	case map[string]any:
		for k, val := range q {
			switch vv := val.(type) {
			case string:
				out[k] = []string{vv}
			case []any:
				for _, s := range vv {
					if str, ok := s.(string); ok {
						out[k] = append(out[k], str)
					}
				}
			}
		}
	case string:
		u, err := url.ParseQuery(q)
		if err == nil {
			for k, vs := range u {
				out[k] = vs
			}
		}
	}
	return out
}
