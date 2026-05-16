// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunnerOption mutates a Runner at construction time.
type RunnerOption func(*Runner)

// Runner is the thin REST + subprocess wrapper that ties the in-memory
// Target to the actual fuzz runtime.  A Runner is safe for concurrent
// use by multiple goroutines; each Submit / Wait / Stream / LocalSpawn
// is independent.
type Runner struct {
	httpClient *http.Client
	baseURL    *url.URL
	token      string
	namespace  string
	cliPath    string
	userAgent  string
	pollPeriod time.Duration
}

const (
	defaultRunnerUserAgent = "mockarty-go-sdk/fuzz/" + SDKVersion
	defaultPollPeriod      = 2 * time.Second
	defaultCLIName         = "mockarty-cli"
)

// WithRunnerHTTPClient injects a caller-owned *http.Client.
func WithRunnerHTTPClient(c *http.Client) RunnerOption {
	return func(r *Runner) {
		if c != nil {
			r.httpClient = c
		}
	}
}

// WithRunnerTimeout caps a single HTTP round-trip.  Ignored when a
// custom *http.Client is supplied (set the timeout on it instead).
func WithRunnerTimeout(d time.Duration) RunnerOption {
	return func(r *Runner) {
		if d > 0 {
			r.httpClient.Timeout = d
		}
	}
}

// WithRunnerPollPeriod controls how often Wait polls the admin server
// for completion.  Smaller values are more responsive; larger values
// are gentler on the server.  Default 2s.
func WithRunnerPollPeriod(d time.Duration) RunnerOption {
	return func(r *Runner) {
		if d > 0 {
			r.pollPeriod = d
		}
	}
}

// WithRunnerCLIPath overrides the mockarty-cli binary path used by
// LocalSpawn.  Defaults to whichever "mockarty-cli" exec.LookPath finds
// on $PATH.
func WithRunnerCLIPath(path string) RunnerOption {
	return func(r *Runner) {
		if strings.TrimSpace(path) != "" {
			r.cliPath = path
		}
	}
}

// WithRunnerUserAgent overrides the User-Agent header.
func WithRunnerUserAgent(ua string) RunnerOption {
	return func(r *Runner) {
		if strings.TrimSpace(ua) != "" {
			r.userAgent = ua
		}
	}
}

// NewRunner builds a Runner. baseURL is the admin server origin,
// namespace is the bound Mockarty namespace (used when the Target's own
// namespace is empty), and token is the API key sent as X-API-Key.
//
// At least baseURL+token are required for any Submit/Wait/Stream call;
// LocalSpawn does not need them and accepts a Runner constructed with
// all three empty.
func NewRunner(baseURL, namespace, token string, opts ...RunnerOption) *Runner {
	r := &Runner{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      strings.TrimSpace(token),
		namespace:  strings.TrimSpace(namespace),
		userAgent:  defaultRunnerUserAgent,
		pollPeriod: defaultPollPeriod,
	}
	if u, err := url.Parse(strings.TrimSpace(baseURL)); err == nil && u.Host != "" {
		u.Path = strings.TrimRight(u.Path, "/")
		r.baseURL = u
	}
	for _, o := range opts {
		if o != nil {
			o(r)
		}
	}
	return r
}

// BaseURL returns the configured admin URL (empty when none).
func (r *Runner) BaseURL() string {
	if r.baseURL == nil {
		return ""
	}
	return r.baseURL.String()
}

// Namespace returns the bound namespace.
func (r *Runner) Namespace() string { return r.namespace }

// Submit posts the target to the admin server's POST /api/v1/fuzzing/run.
// The returned *JobID is then fed to Wait or Stream.
func (r *Runner) Submit(ctx context.Context, t *Target) (*JobID, error) {
	if err := r.requireRemote(); err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errors.New("fuzz: Submit requires a non-nil Target")
	}
	body, err := t.toWire()
	if err != nil {
		return nil, err
	}
	if body.Namespace == "" {
		body.Namespace = r.namespace
	}
	var job JobID
	if err := r.doJSON(ctx, http.MethodPost, "/api/v1/fuzzing/run", body, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// Wait polls the admin server's GET /api/v1/fuzzing/results/:id until
// the campaign reaches a terminal state (completed / failed / cancelled)
// or ctx is cancelled.
func (r *Runner) Wait(ctx context.Context, jobID string) (*Result, error) {
	if err := r.requireRemote(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(jobID) == "" {
		return nil, errors.New("fuzz: Wait requires a non-empty jobID")
	}
	ticker := time.NewTicker(r.pollPeriod)
	defer ticker.Stop()

	endpoint := "/api/v1/fuzzing/results/" + url.PathEscape(jobID)
	for {
		var res Result
		if err := r.doJSON(ctx, http.MethodGet, endpoint, nil, &res); err != nil {
			return nil, err
		}
		switch strings.ToLower(res.Status) {
		case "completed", "failed", "cancelled", "stopped", "error":
			return &res, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Stream opens a server-sent-events channel that emits an Event per
// progress update.  The channel closes when the campaign terminates,
// ctx is cancelled, or the connection drops.
//
// The implementation is intentionally minimal: it parses `data: …` SSE
// lines into Event values.  Other SSE frame types (event:, id:) are
// ignored — they are not part of the contract.
func (r *Runner) Stream(ctx context.Context, jobID string) (<-chan Event, error) {
	if err := r.requireRemote(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(jobID) == "" {
		return nil, errors.New("fuzz: Stream requires a non-empty jobID")
	}
	endpoint := r.endpointURL("/api/v1/fuzzing/run/" + url.PathEscape(jobID) + "/stream")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fuzz: build stream request: %w", err)
	}
	r.applyHeaders(req)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := r.httpClient.Do(req) //nolint:bodyclose // closed in goroutine or on error path
	if err != nil {
		return nil, fmt.Errorf("fuzz: open stream: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("fuzz: stream HTTP %d: %s", resp.StatusCode, string(raw))
	}
	out := make(chan Event, 8)
	go func() {
		defer close(out)
		defer func() { _ = resp.Body.Close() }()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1<<16), 1<<20) // tolerate large events
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var ev Event
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				ev = Event{Kind: "error", Message: err.Error(), Timestamp: time.Now()}
			}
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}

// LocalSpawn writes the target to a temp file and invokes
// `mockarty-cli fuzz run <file> --json` as a subprocess.  The stdout
// is decoded as a Result.  This is the offline / no-admin path.
//
// On Windows the same approach works as long as mockarty-cli is on PATH
// or supplied via WithRunnerCLIPath.
func (r *Runner) LocalSpawn(ctx context.Context, t *Target) (*Result, error) {
	if t == nil {
		return nil, errors.New("fuzz: LocalSpawn requires a non-nil Target")
	}
	data, err := t.ToJSON()
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "mockarty-fuzz-*.json")
	if err != nil {
		return nil, fmt.Errorf("fuzz: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("fuzz: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("fuzz: close temp: %w", err)
	}

	cli := r.cliPath
	if cli == "" {
		cli = defaultCLIName
	}
	if !filepath.IsAbs(cli) {
		if resolved, lookupErr := exec.LookPath(cli); lookupErr == nil {
			cli = resolved
		}
	}

	cmd := exec.CommandContext(ctx, cli, "fuzz", "run", tmpPath, "--json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("fuzz: %s fuzz run: %w (stderr=%s)", cli, err, strings.TrimSpace(stderr.String()))
	}
	var res Result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return nil, fmt.Errorf("fuzz: parse %s stdout: %w (out=%s)", cli, err, truncForErr(stdout.String()))
	}
	return &res, nil
}

// Stop cancels a running campaign by ID.
func (r *Runner) Stop(ctx context.Context, jobID string) error {
	if err := r.requireRemote(); err != nil {
		return err
	}
	if strings.TrimSpace(jobID) == "" {
		return errors.New("fuzz: Stop requires a non-empty jobID")
	}
	return r.doJSON(ctx, http.MethodPost, "/api/v1/fuzzing/run/"+url.PathEscape(jobID)+"/stop", nil, nil)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (r *Runner) requireRemote() error {
	if r.baseURL == nil {
		return errors.New("fuzz: Runner has no admin URL configured (pass baseURL to NewRunner)")
	}
	if r.token == "" {
		return errors.New("fuzz: Runner has no API token configured")
	}
	return nil
}

func (r *Runner) endpointURL(path string) string {
	u := *r.baseURL
	// Preserve any prefix path baked into baseURL (e.g. server mounted at /mock/).
	prefix := strings.TrimRight(r.baseURL.Path, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u.Path = prefix + path
	return u.String()
}

func (r *Runner) applyHeaders(req *http.Request) {
	req.Header.Set("X-API-Key", r.token)
	req.Header.Set("User-Agent", r.userAgent)
	req.Header.Set("Accept", "application/json")
}

func (r *Runner) doJSON(ctx context.Context, method, path string, body, respOut any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("fuzz: marshal %s %s: %w", method, path, err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.endpointURL(path), reader)
	if err != nil {
		return fmt.Errorf("fuzz: build %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	r.applyHeaders(req)
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fuzz: %s %s: %w", method, path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		return fmt.Errorf("fuzz: %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncForErr(string(raw)))
	}
	if respOut == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(respOut); err != nil {
		return fmt.Errorf("fuzz: decode %s %s: %w", method, path, err)
	}
	return nil
}

func truncForErr(s string) string {
	const max = 256
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
