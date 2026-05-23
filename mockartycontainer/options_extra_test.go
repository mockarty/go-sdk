// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// New options — pure unit tests (no docker).
// ---------------------------------------------------------------------------

func TestWithMappings(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		dir := t.TempDir()
		cfg := newConfig()
		if err := WithMappings(dir)(cfg); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if !filepath.IsAbs(cfg.mappingsDir) {
			t.Errorf("mappingsDir not absolute: %q", cfg.mappingsDir)
		}
	})
	t.Run("empty-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithMappings("")(cfg); err == nil {
			t.Error("expected empty error")
		}
	})
	t.Run("missing-dir-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithMappings("/definitely/missing/abcxyz123")(cfg); err == nil {
			t.Error("expected missing-dir error")
		}
	})
	t.Run("file-not-dir-rejected", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "x.json")
		if err := os.WriteFile(f, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := newConfig()
		if err := WithMappings(f)(cfg); err == nil {
			t.Error("expected not-a-dir error")
		}
	})
}

func TestWithHAR(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		f := filepath.Join(t.TempDir(), "x.har")
		if err := os.WriteFile(f, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := newConfig()
		if err := WithHAR(f)(cfg); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if !filepath.IsAbs(cfg.harFile) {
			t.Errorf("harFile not absolute: %q", cfg.harFile)
		}
	})
	t.Run("empty-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithHAR("")(cfg); err == nil {
			t.Error("expected empty error")
		}
	})
	t.Run("missing-file-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithHAR("/no/such/file.har")(cfg); err == nil {
			t.Error("expected missing-file error")
		}
	})
	t.Run("dir-not-file-rejected", func(t *testing.T) {
		cfg := newConfig()
		if err := WithHAR(t.TempDir())(cfg); err == nil {
			t.Error("expected is-a-dir error")
		}
	})
}

func TestWithPort(t *testing.T) {
	cases := []struct {
		name string
		port int
		err  bool
	}{
		{"zero-ephemeral", 0, false},
		{"valid", 8080, false},
		{"max", 65535, false},
		{"negative", -1, true},
		{"too-high", 70000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			err := WithPort(tc.port)(cfg)
			if (err != nil) != tc.err {
				t.Fatalf("err=%v want-err=%v", err, tc.err)
			}
			if !tc.err && cfg.hostPort != tc.port {
				t.Errorf("hostPort=%d", cfg.hostPort)
			}
		})
	}
}

func TestWithLogger(t *testing.T) {
	cfg := newConfig()
	var buf bytes.Buffer
	if err := WithLogger(&buf)(cfg); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cfg.logger != &buf {
		t.Error("logger not stored")
	}
	// nil is allowed — disables streaming.
	if err := WithLogger(nil)(cfg); err != nil {
		t.Fatalf("nil logger should be allowed: %v", err)
	}
	if cfg.logger != nil {
		t.Error("nil logger should clear the writer")
	}
}

func TestWithStartupTimeout(t *testing.T) {
	cfg := newConfig()
	if err := WithStartupTimeout(5 * time.Second)(cfg); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if cfg.startupTimeout != 5*time.Second {
		t.Errorf("startupTimeout=%s", cfg.startupTimeout)
	}
	for _, bad := range []time.Duration{0, -1 * time.Second} {
		if err := WithStartupTimeout(bad)(cfg); err == nil {
			t.Errorf("expected error for %s", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// AddWireMockStub / AddMockartyMock — exercised against an in-process
// httptest server (no docker).
// ---------------------------------------------------------------------------

func TestAddStubHelpers(t *testing.T) {
	var capturedPath, capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.WriteHeader(201)
	}))
	defer srv.Close()

	m := &MockartyContainer{
		mockURL:    srv.URL,
		httpClient: &http.Client{Timeout: 2 * time.Second},
	}

	t.Run("wiremock", func(t *testing.T) {
		err := m.AddWireMockStub(context.Background(), map[string]any{
			"request":  map[string]any{"method": "GET", "url": "/x"},
			"response": map[string]any{"status": 200},
		})
		if err != nil {
			t.Fatalf("AddWireMockStub: %v", err)
		}
		if capturedPath != "/__admin/mappings" {
			t.Errorf("path=%q", capturedPath)
		}
		if !strings.Contains(capturedBody, `"method":"GET"`) {
			t.Errorf("body missing request: %q", capturedBody)
		}
	})

	t.Run("mockarty", func(t *testing.T) {
		err := m.AddMockartyMock(context.Background(), map[string]any{
			"id":   "x",
			"http": map[string]any{"httpMethod": "GET", "route": "/y"},
		})
		if err != nil {
			t.Fatalf("AddMockartyMock: %v", err)
		}
		if capturedPath != "/api/v1/mocks" {
			t.Errorf("path=%q", capturedPath)
		}
	})

	t.Run("nil-rejected", func(t *testing.T) {
		if err := m.AddWireMockStub(context.Background(), nil); err == nil {
			t.Error("expected nil rejection")
		}
		if err := m.AddMockartyMock(context.Background(), nil); err == nil {
			t.Error("expected nil rejection")
		}
	})

	t.Run("raw-bytes-passthrough", func(t *testing.T) {
		raw := []byte(`{"request":{"method":"POST","url":"/raw"},"response":{"status":204}}`)
		if err := m.AddWireMockStub(context.Background(), raw); err != nil {
			t.Fatalf("raw bytes: %v", err)
		}
		if !strings.Contains(capturedBody, `"/raw"`) {
			t.Errorf("raw bytes not forwarded: %q", capturedBody)
		}
	})

	t.Run("empty-bytes-rejected", func(t *testing.T) {
		if err := m.AddWireMockStub(context.Background(), []byte("   ")); err == nil {
			t.Error("expected empty-bytes rejection")
		}
	})

	t.Run("string-passthrough", func(t *testing.T) {
		if err := m.AddMockartyMock(context.Background(), `{"id":"s","http":{"httpMethod":"GET","route":"/s"}}`); err != nil {
			t.Fatalf("string: %v", err)
		}
		if !strings.Contains(capturedBody, `"id":"s"`) {
			t.Errorf("string not forwarded: %q", capturedBody)
		}
	})
}

// ---------------------------------------------------------------------------
// WaitReady — drives the in-process httptest server.
// ---------------------------------------------------------------------------

func TestWaitReady(t *testing.T) {
	t.Run("ready-immediately", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
		}))
		defer srv.Close()
		m := &MockartyContainer{
			metricsURL: srv.URL,
			httpClient: &http.Client{Timeout: 2 * time.Second},
		}
		if err := m.WaitReady(context.Background(), WithReadyTimeout(2*time.Second)); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})

	t.Run("times-out-on-503", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(503)
		}))
		defer srv.Close()
		m := &MockartyContainer{
			metricsURL: srv.URL,
			httpClient: &http.Client{Timeout: 1 * time.Second},
		}
		err := m.WaitReady(context.Background(),
			WithReadyTimeout(200*time.Millisecond),
			WithReadyInterval(50*time.Millisecond),
		)
		if err == nil {
			t.Error("expected deadline error")
		}
	})

	t.Run("ctx-cancel-honored", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
		}))
		defer srv.Close()
		m := &MockartyContainer{
			metricsURL: srv.URL,
			httpClient: &http.Client{Timeout: 1 * time.Second},
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		err := m.WaitReady(ctx,
			WithReadyTimeout(5*time.Second),
			WithReadyInterval(20*time.Millisecond),
		)
		if err == nil {
			t.Error("expected ctx cancellation error")
		}
	})
}

// ---------------------------------------------------------------------------
// MustRun — drives a fake TestingT.
// ---------------------------------------------------------------------------

type fakeT struct {
	fatalCalled bool
	cleanups    []func()
}

func (f *fakeT) Helper() {}
func (f *fakeT) Fatalf(format string, args ...any) {
	f.fatalCalled = true
}
func (f *fakeT) Cleanup(fn func()) { f.cleanups = append(f.cleanups, fn) }

// MustRun with an option that errors before any docker call surfaces
// the failure via t.Fatalf.
func TestMustRun_OptionErrorTriggersFatal(t *testing.T) {
	ft := &fakeT{}
	_ = MustRun(context.Background(), ft, WithImage(""))
	if !ft.fatalCalled {
		t.Error("MustRun did not call t.Fatalf on option error")
	}
}

func TestRun_Alias(t *testing.T) {
	// Verifies Run is wired to the same path as New (fails fast on
	// bad image without ever reaching docker).
	_, err := Run(context.Background(), WithImage(""))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTerminate_Alias(t *testing.T) {
	m := &MockartyContainer{}
	if err := m.Terminate(context.Background()); err != nil {
		t.Errorf("Terminate on zero-value container returned %v, want nil", err)
	}
}
