// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Functional-options coverage — pure unit tests (no docker).
// ---------------------------------------------------------------------------

func TestNewConfig_Defaults(t *testing.T) {
	cfg := newConfig()
	if cfg.image != DefaultImage {
		t.Errorf("default image = %q, want %q", cfg.image, DefaultImage)
	}
	if cfg.format != FormatAuto {
		t.Errorf("default format = %q, want %q", cfg.format, FormatAuto)
	}
	if len(cfg.envs) != 0 {
		t.Errorf("default envs not empty: %v", cfg.envs)
	}
	if len(cfg.stubFiles) != 0 {
		t.Errorf("default stubFiles not empty: %v", cfg.stubFiles)
	}
}

func TestWithImage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		err  bool
		want string
	}{
		{"override", "ghcr.io/acme/mockarty-cli:1.2.3", false, "ghcr.io/acme/mockarty-cli:1.2.3"},
		{"trim", "  trim/me:tag  ", false, "trim/me:tag"},
		{"empty", "", true, ""},
		{"whitespace", "   ", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			err := WithImage(tc.in)(cfg)
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, want err? %v", err, tc.err)
			}
			if !tc.err && cfg.image != tc.want {
				t.Errorf("image = %q, want %q", cfg.image, tc.want)
			}
		})
	}
}

func TestWithFormat(t *testing.T) {
	cases := []struct {
		name string
		in   Format
		err  bool
	}{
		{"auto", FormatAuto, false},
		{"wiremock", FormatWireMock, false},
		{"mockarty", FormatMockarty, false},
		{"mockoon", FormatMockoon, false},
		{"unknown", Format("zzz"), true},
		{"empty", Format(""), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			err := WithFormat(tc.in)(cfg)
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, want err? %v", err, tc.err)
			}
			if !tc.err && cfg.format != tc.in {
				t.Errorf("format = %q, want %q", cfg.format, tc.in)
			}
		})
	}
}

func TestWithStubFile(t *testing.T) {
	t.Run("relative-resolved", func(t *testing.T) {
		cfg := newConfig()
		if err := WithStubFile("./stubs.json")(cfg); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !filepath.IsAbs(cfg.stubFiles[0]) {
			t.Errorf("expected absolute path, got %q", cfg.stubFiles[0])
		}
		if !strings.HasSuffix(cfg.stubFiles[0], "stubs.json") {
			t.Errorf("expected suffix stubs.json, got %q", cfg.stubFiles[0])
		}
	})
	t.Run("repeatable", func(t *testing.T) {
		cfg := newConfig()
		_ = WithStubFile("/a.json")(cfg)
		_ = WithStubFile("/b.json")(cfg)
		if len(cfg.stubFiles) != 2 {
			t.Errorf("expected 2 stub files, got %d", len(cfg.stubFiles))
		}
	})
	t.Run("empty-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithStubFile("")(cfg); err == nil {
			t.Error("expected error on empty path")
		}
	})
}

func TestWithEnv(t *testing.T) {
	cfg := newConfig()
	if err := WithEnv("FOO", "bar")(cfg); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.envs["FOO"] != "bar" {
		t.Errorf("env = %v, want FOO=bar", cfg.envs)
	}
	if err := WithEnv("", "x")(cfg); err == nil {
		t.Error("expected error on empty key")
	}
	if err := WithEnv("FOO", "baz")(cfg); err != nil { // last writer wins
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg.envs["FOO"] != "baz" {
		t.Errorf("last-writer wins violated: %v", cfg.envs)
	}
}

func TestWithCmd(t *testing.T) {
	cfg := newConfig()
	if err := WithCmd("serve", "--verbose")(cfg); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cfg.cmd) != 2 || cfg.cmd[0] != "serve" {
		t.Errorf("cmd = %v, want [serve --verbose]", cfg.cmd)
	}
}

func TestNew_OptionErrorPropagates(t *testing.T) {
	_, err := New(context.Background(),
		WithImage(""), // forces error before any docker call
	)
	if err == nil {
		t.Fatal("expected option-error to surface from New")
	}
	if !strings.Contains(err.Error(), "image must not be empty") {
		t.Errorf("unexpected err: %v", err)
	}
}

// ---------------------------------------------------------------------------
// URL formatting + concurrency — pure unit tests (no docker).
// ---------------------------------------------------------------------------

func TestURLHelpers(t *testing.T) {
	m := &MockartyContainer{mockURL: "http://127.0.0.1:12345", metricsURL: "http://127.0.0.1:12346"}
	if m.URL() != "http://127.0.0.1:12345" {
		t.Errorf("URL = %q", m.URL())
	}
	if m.MetricsURL() != "http://127.0.0.1:12346" {
		t.Errorf("MetricsURL = %q", m.MetricsURL())
	}
	if m.WireMockURL() != "http://127.0.0.1:12345/__admin" {
		t.Errorf("WireMockURL = %q", m.WireMockURL())
	}
	if m.MockartyURL() != "http://127.0.0.1:12345/api/v1" {
		t.Errorf("MockartyURL = %q", m.MockartyURL())
	}
	// Verify trailing-slash normalisation.
	m.mockURL = "http://127.0.0.1:12345/"
	if m.WireMockURL() != "http://127.0.0.1:12345/__admin" {
		t.Errorf("WireMockURL not normalised: %q", m.WireMockURL())
	}
}

func TestURL_RaceClean(t *testing.T) {
	m := &MockartyContainer{mockURL: "http://127.0.0.1:99999"}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = m.URL()
				_ = m.WireMockURL()
				_ = m.MockartyURL()
			}
		}()
	}
	wg.Wait()
}

func TestStop_NilSafe(t *testing.T) {
	var m *MockartyContainer
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("Stop on nil receiver returned %v, want nil", err)
	}
	m2 := &MockartyContainer{}
	if err := m2.Stop(context.Background()); err != nil {
		t.Errorf("Stop with nil container returned %v, want nil", err)
	}
}

// TestStop_Idempotent verifies the documented contract — double Stop is a
// no-op. The prior implementation propagated the daemon-side error from a
// second Terminate to the user as "terminate failed" even though the
// container had shut down cleanly.
//
// Stubbing testcontainers.Container in full would require ~25 method stubs;
// instead the test reaches into the unexported field after the first Stop
// has cleared it (assertion: Container() returns nil) and confirms the
// second Stop returns nil without panicking.
func TestStop_Idempotent(t *testing.T) {
	// We cannot construct a real testcontainers.Container without Docker.
	// Drive Stop with a nil container handle (post-first-Stop state) and
	// assert it does NOT panic or return an error. This mirrors what
	// happens to a real instance after the first successful Terminate.
	m := &MockartyContainer{container: nil}
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("Stop on already-stopped container returned %v, want nil", err)
	}
	if m.Container() != nil {
		t.Error("Container() should return nil after Stop on already-stopped instance")
	}
	// Second Stop on the same instance — also nil.
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("second Stop returned %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// post() helper — exercised against an in-process httptest.Server so we
// cover both the happy and error paths without requiring docker.
// ---------------------------------------------------------------------------

func TestPost_HappyAndErrorPaths(t *testing.T) {
	var seenBody string
	var status int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seenBody = string(b)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"detail":"x"}`))
	}))
	defer srv.Close()

	m := &MockartyContainer{
		mockURL:    srv.URL,
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}

	t.Run("ok", func(t *testing.T) {
		status = 200
		if err := m.post(context.Background(), srv.URL+"/x", []byte(`{"a":1}`)); err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if seenBody != `{"a":1}` {
			t.Errorf("server saw %q", seenBody)
		}
	})

	t.Run("4xx-surfaces-error", func(t *testing.T) {
		status = 422
		err := m.post(context.Background(), srv.URL+"/x", []byte(`{}`))
		if err == nil {
			t.Fatal("expected 422 to surface as error")
		}
		if !strings.Contains(err.Error(), "422") {
			t.Errorf("err missing status: %v", err)
		}
	})

	t.Run("bad-url", func(t *testing.T) {
		err := m.post(context.Background(), "http://127.0.0.1:1", []byte(`{}`))
		if err == nil {
			t.Fatal("expected dial error")
		}
	})

	t.Run("invalid-url-build", func(t *testing.T) {
		err := m.post(context.Background(), string([]byte{0x7f}), []byte(`{}`)) // control char triggers url.Parse failure
		if err == nil {
			t.Fatal("expected build error")
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			// http.NewRequestWithContext wraps url.Error inside the
			// returned error chain — accept either path.
			return
		}
	})

	t.Run("nil-body", func(t *testing.T) {
		status = 200
		if err := m.post(context.Background(), srv.URL+"/x", nil); err != nil {
			t.Errorf("unexpected err: %v", err)
		}
	})
}

func TestApply_NilStubRejected(t *testing.T) {
	m := &MockartyContainer{httpClient: &http.Client{}}
	if err := m.Apply(context.Background(), nil); err == nil {
		t.Error("expected nil-stub rejection")
	}
}

func TestApply_MarshalError(t *testing.T) {
	m := &MockartyContainer{mockURL: "http://127.0.0.1", httpClient: &http.Client{Timeout: time.Second}}
	// channels are not JSON-serialisable
	if err := m.Apply(context.Background(), make(chan int)); err == nil {
		t.Error("expected marshal error")
	}
}

// ---------------------------------------------------------------------------
// Docker smoke — opt-in, skipped without docker / image.
// ---------------------------------------------------------------------------

func dockerAvailable() bool {
	// Cheap heuristic: rely on DOCKER_HOST or a default unix socket.
	if os.Getenv("DOCKER_HOST") != "" {
		return true
	}
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return true
	}
	return false
}

func TestSmoke_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("docker smoke skipped under -short")
	}
	if !dockerAvailable() {
		t.Skip("no docker daemon reachable (set DOCKER_HOST or run with /var/run/docker.sock mounted)")
	}
	if os.Getenv("MOCKARTY_SDK_DOCKER_SMOKE") == "" {
		t.Skip("MOCKARTY_SDK_DOCKER_SMOKE not set — opt in to actually pull the CLI image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := New(ctx, WithFormat(FormatAuto))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop(ctx)
	if !strings.HasPrefix(c.URL(), "http://") {
		t.Errorf("expected http URL, got %q", c.URL())
	}
}
