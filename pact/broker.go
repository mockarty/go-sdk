// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package pact

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Pact Broker client — RFC-style HTTP wrapper around the canonical
// Pact Broker API surface (https://docs.pact.io/pact_broker/api_docs).
//
// Works identically against:
//   - Mockarty's built-in broker (POSTs to /api/v1/pact-broker via
//     the same Pact Broker shape the OSS Ruby broker exposes)
//   - SmartBear's Pactflow
//   - The OSS pact-broker docker image
//
// Standard cross-SDK env vars (mirrors pact-jvm + pact-python + pact-go):
//   PACT_BROKER_BASE_URL   broker base URL
//   PACT_BROKER_TOKEN      bearer token (Pactflow style)
//   PACT_BROKER_USERNAME   basic-auth user (OSS broker style)
//   PACT_BROKER_PASSWORD   basic-auth pass
//   PACT_DO_NOT_TRACK      "1" disables anonymous usage telemetry
//
// Both auth schemes are common in the wild; the first non-empty pair
// wins (token > basic). Callers can override per-call via Options.

// BrokerClient is the thin HTTP wrapper around the Pact Broker API.
// Goroutine-safe; one instance per broker target.
type BrokerClient struct {
	baseURL  string
	token    string
	username string
	password string
	http     *http.Client
}

// BrokerOption mutates dial-time configuration. Functional-options
// matching the rest of mockarty-go.
type BrokerOption func(*brokerCfg)

type brokerCfg struct {
	baseURL    string
	token      string
	username   string
	password   string
	httpClient *http.Client
	timeout    time.Duration
}

// WithBrokerURL pins the base URL (e.g. "https://pact.acme.com").
// Trailing slashes stripped — callers don't have to be careful.
func WithBrokerURL(u string) BrokerOption {
	return func(c *brokerCfg) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithBrokerToken sets the Pactflow-style bearer token. Wins over
// basic-auth when both are configured.
func WithBrokerToken(t string) BrokerOption {
	return func(c *brokerCfg) { c.token = t }
}

// WithBrokerBasicAuth sets HTTP Basic credentials for the OSS broker.
func WithBrokerBasicAuth(user, pass string) BrokerOption {
	return func(c *brokerCfg) { c.username = user; c.password = pass }
}

// WithBrokerHTTPClient lets callers inject a pre-configured client
// (custom TLS, proxy, retries). When unset a 30s-timeout default is
// used.
func WithBrokerHTTPClient(c *http.Client) BrokerOption {
	return func(cfg *brokerCfg) { cfg.httpClient = c }
}

// WithBrokerTimeout sets the per-request timeout for the default
// HTTP client. Ignored when WithBrokerHTTPClient is supplied.
func WithBrokerTimeout(d time.Duration) BrokerOption {
	return func(c *brokerCfg) { c.timeout = d }
}

// NewBrokerClient constructs a BrokerClient. At minimum, a base URL
// is required (either via option or PACT_BROKER_BASE_URL).
func NewBrokerClient(opts ...BrokerOption) (*BrokerClient, error) {
	cfg := &brokerCfg{
		baseURL:  strings.TrimRight(os.Getenv("PACT_BROKER_BASE_URL"), "/"),
		token:    os.Getenv("PACT_BROKER_TOKEN"),
		username: os.Getenv("PACT_BROKER_USERNAME"),
		password: os.Getenv("PACT_BROKER_PASSWORD"),
		timeout:  30 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.baseURL == "" {
		return nil, errors.New("pact broker: base URL is required (PACT_BROKER_BASE_URL or WithBrokerURL)")
	}
	if cfg.httpClient == nil {
		cfg.httpClient = &http.Client{Timeout: cfg.timeout}
	}
	return &BrokerClient{
		baseURL:  cfg.baseURL,
		token:    cfg.token,
		username: cfg.username,
		password: cfg.password,
		http:     cfg.httpClient,
	}, nil
}

// Publish uploads a pact JSON document for one consumer/provider pair.
//
// pact:           the JSON bytes (typically from PactWriter / Pact.JSON())
// consumerVersion: the consumer application version this pact
//
//	describes — usually a git SHA or semver tag. The
//	broker uses (consumer, version) as the unique key.
//
// branch:          optional consumer branch name (Pactflow feature).
//
//	Pass "" to skip.
//
// tags:            optional list of tags (e.g. ["prod", "release"]).
//
// On success the broker returns 200/201 with a Hal+JSON envelope —
// we don't parse it (callers rarely need the broker's view); just
// check the status code.
//
// Errors:
//   - transport / 4xx / 5xx surface verbatim with the response body
//     included so CI logs show why the publish failed.
func (b *BrokerClient) Publish(ctx context.Context, pact []byte, consumerVersion, branch string, tags []string) error {
	consumer, provider, err := extractConsumerProvider(pact)
	if err != nil {
		return err
	}
	if strings.TrimSpace(consumerVersion) == "" {
		return errors.New("pact broker: consumerVersion is required")
	}
	endpoint := fmt.Sprintf("%s/pacts/provider/%s/consumer/%s/version/%s",
		b.baseURL,
		url.PathEscape(provider),
		url.PathEscape(consumer),
		url.PathEscape(consumerVersion))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(pact))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if branch != "" {
		req.Header.Set("X-Pact-Consumer-Branch", branch)
	}
	b.applyAuth(req)
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("pact broker: publish: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return fmt.Errorf("pact broker: publish HTTP %d: %s", resp.StatusCode, string(body))
	}
	// Apply tags (each is a separate PUT in the canonical broker API).
	// Collect failures and report them together, but DO NOT short-circuit
	// — the pact has already been published; we still want every other
	// tag attempted so a partial tag set is at least maximised.
	var tagErrs []error
	for _, tag := range tags {
		if err := b.tagConsumer(ctx, consumer, consumerVersion, tag); err != nil {
			tagErrs = append(tagErrs, fmt.Errorf("tag %q: %w", tag, err))
		}
	}
	if len(tagErrs) > 0 {
		return fmt.Errorf("pact broker: publish succeeded but %d tag(s) failed: %w",
			len(tagErrs), errors.Join(tagErrs...))
	}
	return nil
}

// Fetch retrieves a specific consumer-version's pact JSON.
//
// version may be "latest" — the broker resolves it to the most
// recent published version.
func (b *BrokerClient) Fetch(ctx context.Context, consumer, provider, version string) ([]byte, error) {
	if consumer == "" || provider == "" {
		return nil, errors.New("pact broker: consumer + provider are required")
	}
	if version == "" {
		version = "latest"
	}
	endpoint := fmt.Sprintf("%s/pacts/provider/%s/consumer/%s/version/%s",
		b.baseURL,
		url.PathEscape(provider),
		url.PathEscape(consumer),
		url.PathEscape(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	b.applyAuth(req)
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pact broker: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrBrokerPactNotFound
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("pact broker: fetch HTTP %d: %s", resp.StatusCode, string(body))
	}
	// Cap the fetch body — a misbehaving/hostile broker should not be
	// able to OOM the consumer. 16 MiB matches the desktop push budget
	// and is ~3 orders of magnitude over realistic pact sizes.
	return io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
}

// FetchLatest is shorthand for Fetch(ctx, consumer, provider, "latest").
func (b *BrokerClient) FetchLatest(ctx context.Context, consumer, provider string) ([]byte, error) {
	return b.Fetch(ctx, consumer, provider, "latest")
}

// CanIDeployResult is the response shape of /can-i-deploy. Mirrors
// the broker's JSON envelope; we expose only the fields CI scripts
// actually branch on. Full response is also stashed on Raw for
// callers that want the matrix details.
type CanIDeployResult struct {
	Deployable bool            `json:"deployable"`
	Reason     string          `json:"reason,omitempty"`
	Raw        json.RawMessage `json:"-"`
}

// CanIDeploy queries the broker's deployment-safety matrix.
//
//	pacticipant:  the application asking to deploy (consumer OR provider)
//	version:      that application's version
//	toEnvironment: target environment (e.g. "production"). Pass "" to
//	              fall back to the legacy `latest=true` query shape.
//
// Returns Deployable=true when ALL relevant pacts have been verified
// in the target environment.
func (b *BrokerClient) CanIDeploy(ctx context.Context, pacticipant, version, toEnvironment string) (*CanIDeployResult, error) {
	if pacticipant == "" || version == "" {
		return nil, errors.New("pact broker: pacticipant + version are required")
	}
	q := url.Values{}
	q.Set("pacticipant", pacticipant)
	q.Set("version", version)
	if toEnvironment != "" {
		q.Set("environment", toEnvironment)
	} else {
		q.Set("latest", "true")
	}
	endpoint := fmt.Sprintf("%s/can-i-deploy?%s", b.baseURL, q.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	b.applyAuth(req)
	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pact broker: can-i-deploy: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("pact broker: can-i-deploy HTTP %d: %s", resp.StatusCode, string(body))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Summary struct {
			Deployable bool   `json:"deployable"`
			Reason     string `json:"reason"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("pact broker: decode can-i-deploy: %w", err)
	}
	return &CanIDeployResult{
		Deployable: parsed.Summary.Deployable,
		Reason:     parsed.Summary.Reason,
		Raw:        body,
	}, nil
}

// ErrBrokerPactNotFound is the typed sentinel for 404 from Fetch.
var ErrBrokerPactNotFound = errors.New("pact broker: pact not found")

// tagConsumer applies a single tag to a (consumer, version) pair.
// The broker exposes PUT /pacticipants/{name}/versions/{version}/tags/{tag}.
func (b *BrokerClient) tagConsumer(ctx context.Context, consumer, version, tag string) error {
	endpoint := fmt.Sprintf("%s/pacticipants/%s/versions/%s/tags/%s",
		b.baseURL,
		url.PathEscape(consumer),
		url.PathEscape(version),
		url.PathEscape(tag))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	b.applyAuth(req)
	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("pact broker: tag %q: %w", tag, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return fmt.Errorf("pact broker: tag %q HTTP %d: %s", tag, resp.StatusCode, string(body))
	}
	return nil
}

// applyAuth sets the Authorization header. Bearer wins over Basic
// when both are configured (matches the precedence the official
// pact-broker CLI uses).
func (b *BrokerClient) applyAuth(req *http.Request) {
	if b.token != "" {
		req.Header.Set("Authorization", "Bearer "+b.token)
		return
	}
	if b.username != "" || b.password != "" {
		credentials := b.username + ":" + b.password
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	}
}

// extractConsumerProvider pulls the top-level consumer.name +
// provider.name out of a pact JSON document. Pact V3 + V4 use the
// same shape, so one parser fits both. Returns an error when the
// fields are missing — that's a contract violation upstream
// (PactWriter / Pact.JSON() always populate them).
func extractConsumerProvider(pact []byte) (consumer, provider string, err error) {
	var head struct {
		Consumer struct{ Name string } `json:"consumer"`
		Provider struct{ Name string } `json:"provider"`
	}
	if err := json.Unmarshal(pact, &head); err != nil {
		return "", "", fmt.Errorf("pact broker: parse pact JSON: %w", err)
	}
	if head.Consumer.Name == "" || head.Provider.Name == "" {
		return "", "", errors.New("pact broker: pact JSON missing consumer.name or provider.name")
	}
	return head.Consumer.Name, head.Provider.Name, nil
}
