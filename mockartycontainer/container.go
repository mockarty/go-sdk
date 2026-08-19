// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// MockartyContainer wraps a running `mockarty/cli:latest-mock`
// instance. The zero value is not usable — always construct through
// New(ctx, opts...).
//
// The struct is safe for concurrent use after Start returns: every
// exported method routes through the embedded testcontainers.Container
// (itself thread-safe) or the local sync.RWMutex protecting the
// cached endpoint URLs.
type MockartyContainer struct {
	mu sync.RWMutex

	cfg       *config
	container testcontainers.Container

	mockURL    string
	metricsURL string
	httpClient *http.Client
}

// New starts the container and waits until /health returns 200. It is
// the single entry point — callers pass the options and get back a
// ready-to-hit instance, or an error.
//
// On error the underlying container (if it was started) is terminated
// before returning, so callers do not need to defer Stop on a failed
// New.
func New(ctx context.Context, opts ...Option) (*MockartyContainer, error) {
	cfg := newConfig()
	for _, o := range opts {
		if err := o(cfg); err != nil {
			return nil, err
		}
	}

	// Bind mounts. testcontainers-go v0.27 SILENTLY DROPS bind mounts supplied
	// via the deprecated ContainerMounts/GenericBindMountSource API, so stubs
	// never reached the container (empty /mocks -> a hard-to-debug 404). Use the
	// supported HostConfigModifier + hc.Mounts path instead, and resolve every
	// host path to ABSOLUTE — Docker rejects relative bind sources, so a common
	// idiom like WithMappings("./fixtures") otherwise no-ops.
	absHost := func(p string) string {
		if a, err := filepath.Abs(p); err == nil {
			return a
		}
		return p
	}
	var dockerMounts []mount.Mount
	for _, hostPath := range cfg.stubFiles {
		dockerMounts = append(dockerMounts, mount.Mount{
			Type: mount.TypeBind, ReadOnly: true,
			Source: absHost(hostPath), Target: StubsMount + "/" + filepath.Base(hostPath),
		})
	}
	// WithMappings(dir) — entire directory mounted at /mocks.
	if cfg.mappingsDir != "" {
		dockerMounts = append(dockerMounts, mount.Mount{
			Type: mount.TypeBind, ReadOnly: true,
			Source: absHost(cfg.mappingsDir), Target: MappingsMount,
		})
	}
	// WithHAR(path) — single file mounted at /har/traffic.har.
	if cfg.harFile != "" {
		dockerMounts = append(dockerMounts, mount.Mount{
			Type: mount.TypeBind, ReadOnly: true,
			Source: absHost(cfg.harFile), Target: HARMount,
		})
	}

	env := map[string]string{
		FormatEnv: string(cfg.format),
	}
	if cfg.mappingsDir != "" {
		env[MockDirEnv] = MappingsMount
	}
	if cfg.harFile != "" {
		env[HARReplayEnv] = HARMount
	}
	for k, v := range cfg.envs {
		env[k] = v
	}

	exposed := []string{MockPort, MetricsPort}
	if cfg.hostPort > 0 {
		// Host-pin: testcontainers honours `<host>:<container>/proto`
		// only inside the ContainerRequest.HostConfigModifier path,
		// but the cheaper alternative — bind the host-side port via
		// the exposed-port form — works on every supported driver.
		exposed = []string{fmt.Sprintf("%d:%s", cfg.hostPort, MockPort), MetricsPort}
	}

	req := testcontainers.ContainerRequest{
		Image:        cfg.image,
		ExposedPorts: exposed,
		Env:          env,
		Cmd:          cfg.cmd,
		// Bind stubs/mappings/HAR via HostConfigModifier (hc.Mounts) — the
		// deprecated ContainerMounts API is silently dropped on testcontainers-go
		// v0.27, which left /mocks empty and every seeded stub 404ing.
		HostConfigModifier: func(hc *dockercontainer.HostConfig) {
			if len(dockerMounts) > 0 {
				hc.Mounts = append(hc.Mounts, dockerMounts...)
			}
		},
		// Wait on the WireMock-compat admin health endpoint served on
		// the SAME 8080 listener as the mocks. The earlier draft polled
		// /health on port 9090 (Prometheus metrics port), which the
		// mock-serve subcommand doesn't bind — the container blocked
		// until startup timeout and then errored. Review #109/H2.
		WaitingFor: wait.ForHTTP("/__admin/health").
			WithPort(MockPort).
			WithStatusCodeMatcher(func(status int) bool { return status == http.StatusOK }).
			WithStartupTimeout(cfg.startupTimeout),
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("mockartycontainer: start image %q: %w", cfg.image, err)
	}

	mc := &MockartyContainer{
		cfg:        cfg,
		container:  c,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
	if err := mc.refreshEndpoints(ctx); err != nil {
		_ = c.Terminate(ctx) // best effort
		return nil, err
	}
	if cfg.logger != nil {
		// Best-effort log streaming. Errors are logged to the user's
		// writer (not returned) because losing logs must never break a
		// otherwise-healthy container start.
		go func() {
			r, err := c.Logs(context.Background())
			if err != nil {
				fmt.Fprintf(cfg.logger, "mockartycontainer: open log stream: %v\n", err)
				return
			}
			defer r.Close()
			_, _ = io.Copy(cfg.logger, r)
		}()
	}
	return mc, nil
}

// refreshEndpoints resolves the host+port pair testcontainers assigned
// to the container and caches the two endpoint URLs.
func (m *MockartyContainer) refreshEndpoints(ctx context.Context) error {
	mockEP, err := m.container.PortEndpoint(ctx, MockPort, "http")
	if err != nil {
		return fmt.Errorf("mockartycontainer: resolve mock endpoint: %w", err)
	}
	metricsEP, err := m.container.PortEndpoint(ctx, MetricsPort, "http")
	if err != nil {
		return fmt.Errorf("mockartycontainer: resolve metrics endpoint: %w", err)
	}
	m.mu.Lock()
	m.mockURL = mockEP
	m.metricsURL = metricsEP
	m.mu.Unlock()
	return nil
}

// URL is the canonical base URL of the running mock — points at the
// path-multiplexed listener that accepts both WireMock-compat and
// Mockarty-native traffic. Use this for in-test HTTP clients.
func (m *MockartyContainer) URL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mockURL
}

// WireMockURL is the same listener exposed with the WireMock admin
// API prefix already appended (callers can append `/mappings` etc.).
// The CLI image serves the WireMock admin API verbatim under
// `/__admin/`.
func (m *MockartyContainer) WireMockURL() string {
	return strings.TrimRight(m.URL(), "/") + "/__admin"
}

// MockartyURL is the same listener with the Mockarty native admin API
// prefix appended. Callers append `/mocks`, `/stores`, etc.
func (m *MockartyContainer) MockartyURL() string {
	return strings.TrimRight(m.URL(), "/") + "/api/v1"
}

// MetricsURL is the Prometheus / health endpoint host:port.
func (m *MockartyContainer) MetricsURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.metricsURL
}

// Apply registers one Mockarty-native Mock against the running
// container. The stub payload mirrors the JSON shape accepted by the
// admin node's POST /api/v1/mocks.
//
// Caller passes anything JSON-serialisable: a `mockarty.Mock`, a raw
// `map[string]any`, or a struct from a generated DTO. We keep the
// signature intentionally loose so the container package does not pull
// a hard dependency on the parent module (and avoids an import cycle
// during refactors).
func (m *MockartyContainer) Apply(ctx context.Context, stub any) error {
	if stub == nil {
		return errors.New("mockartycontainer: stub must not be nil")
	}
	body, err := json.Marshal(stub)
	if err != nil {
		return fmt.Errorf("mockartycontainer: marshal stub: %w", err)
	}
	return m.post(ctx, m.MockartyURL()+"/mocks", body)
}

// Reset wipes runtime state on the running container — clears every
// applied stub, history of requests, store contents and counters.
// Maps to DELETE /__admin/reset (WireMock-compat) which the CLI image
// also wires to its Mockarty internals.
func (m *MockartyContainer) Reset(ctx context.Context) error {
	return m.post(ctx, m.WireMockURL()+"/reset", nil)
}

// Logs returns the container's stdout+stderr stream up to "now". It is
// a one-shot snapshot (does not block); for live tailing call it
// repeatedly or use the underlying testcontainers reader directly.
func (m *MockartyContainer) Logs(ctx context.Context) (string, error) {
	r, err := m.container.Logs(ctx)
	if err != nil {
		return "", fmt.Errorf("mockartycontainer: open log stream: %w", err)
	}
	defer r.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return "", fmt.Errorf("mockartycontainer: read log stream: %w", err)
	}
	return buf.String(), nil
}

// Stop terminates and removes the container.
//
// Stop is idempotent: the first call terminates the underlying
// container and clears the handle; subsequent calls (and calls on a
// never-started MockartyContainer) return nil without re-issuing the
// terminate RPC. testcontainers.Container.Terminate is NOT idempotent
// on the daemon side — a second Terminate after the container is gone
// returns an error from the docker daemon, which we previously
// propagated as a "terminate" failure to the user even though the
// container had already shut down cleanly.
func (m *MockartyContainer) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	c := m.container
	m.container = nil // clear under the lock so concurrent Stop is also a no-op
	m.mu.Unlock()
	if c == nil {
		return nil
	}
	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("mockartycontainer: terminate: %w", err)
	}
	return nil
}

// Container returns the underlying testcontainers handle for advanced
// users (network attach, custom exec, etc.). Most users should not
// need it. Returns nil after Stop.
func (m *MockartyContainer) Container() testcontainers.Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.container
}

// Terminate is an alias for Stop — provided so test bodies that use
// `defer c.Terminate(ctx)` (the canonical testcontainers-go idiom)
// read naturally. Both methods are safe to call multiple times.
func (m *MockartyContainer) Terminate(ctx context.Context) error {
	return m.Stop(ctx)
}

// Run is an alternative entry point for callers who prefer the
// testcontainers naming convention. Identical to New.
func Run(ctx context.Context, opts ...Option) (*MockartyContainer, error) {
	return New(ctx, opts...)
}

// TestingT is the minimal subset of testing.TB used by MustRun. We
// avoid importing "testing" at top-level so the package stays usable
// from non-test code paths too (e.g. integration harness binaries).
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// MustRun is the t-aware convenience constructor: it starts a container
// (failing the test on any error) and registers a Cleanup hook so the
// container is terminated when the test ends. The returned *Container
// is ready for traffic.
//
//	func TestMyAPI(t *testing.T) {
//	    ctx := context.Background()
//	    c := mockartycontainer.MustRun(ctx, t,
//	        mockartycontainer.WithMappings("./mocks"),
//	    )
//	    resp, _ := http.Get(c.URL() + "/api/users/1")
//	    ...
//	}
func MustRun(ctx context.Context, t TestingT, opts ...Option) *MockartyContainer {
	t.Helper()
	c, err := New(ctx, opts...)
	if err != nil {
		t.Fatalf("mockartycontainer.MustRun: %v", err)
		return nil // unreachable, satisfies the compiler.
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = c.Stop(stopCtx)
	})
	return c
}

// post is the shared JSON POST helper used by Apply / Reset.
func (m *MockartyContainer) post(ctx context.Context, url string, body []byte) error {
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, rdr)
	if err != nil {
		return fmt.Errorf("mockartycontainer: build request: %w", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mockartycontainer: POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mockartycontainer: POST %s returned %d: %s",
			url, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
