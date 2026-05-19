// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultImage is the canonical CLI image baked from cmd/cli with
	// the `mock serve` entrypoint. Override via WithImage for forks /
	// private registries.
	DefaultImage = "mockarty/cli:latest-mock"

	// MockPort is the unified HTTP listener inside the container. The
	// CLI multiplexes WireMock-compat, Mockoon-compat and Mockarty
	// native traffic on this single port (path-prefix based routing).
	MockPort = "8080/tcp"

	// MetricsPort exposes Prometheus metrics + /health.
	MetricsPort = "9090/tcp"

	// StubsMount is the in-container directory the CLI scans on
	// startup for stub files.
	StubsMount = "/data/stubs"

	// FormatEnv is the env-var the CLI reads to decide which stub
	// dialect to expect when auto-detect is disabled. MUST match the
	// CLI's applyMockServeEnv reader at cmd/cli/cmd/mock_serve_env.go —
	// a mismatch silently leaves the container in auto-detect.
	// Review #109/H1 caught the old "MOCKARTY_STUB_FORMAT" name was
	// ignored; renamed to match the CLI side.
	FormatEnv = "MOCKARTY_MOCK_FORMAT"

	// MockDirEnv tells the CLI which in-container directory to scan
	// for stub files at startup. Set by WithMappings.
	MockDirEnv = "MOCKARTY_MOCK_DIR"

	// HARReplayEnv points the CLI at a HAR file to replay (in-container
	// path). Set by WithHAR.
	HARReplayEnv = "MOCKARTY_HAR_REPLAY"

	// MockPortEnv overrides the CLI's listen port. Set by WithPort.
	MockPortEnv = "MOCKARTY_MOCK_PORT"

	// MappingsMount is the in-container directory mapped by WithMappings.
	MappingsMount = "/mocks"

	// HARMount is the in-container path mapped by WithHAR.
	HARMount = "/har/traffic.har"
)

// Format is the stub-dialect mode passed to the container.
type Format string

const (
	// FormatAuto lets the CLI sniff each file (default).
	FormatAuto Format = "auto"
	// FormatWireMock forces WireMock JSON parsing.
	FormatWireMock Format = "wiremock"
	// FormatMockarty forces Mockarty native JSON parsing.
	FormatMockarty Format = "mockarty"
	// FormatMockoon forces Mockoon-3.x environment JSON parsing.
	FormatMockoon Format = "mockoon"
)

// validFormats keeps the set of accepted dialect values. Adding a new
// dialect = one line here, no switch elsewhere.
var validFormats = map[Format]struct{}{
	FormatAuto:     {},
	FormatWireMock: {},
	FormatMockarty: {},
	FormatMockoon:  {},
}

// config holds the assembled container blueprint before Start. It is
// populated exclusively through functional Options so each future knob
// (env vars, networks, extra mounts) is one new Option without
// touching call-sites.
type config struct {
	envs           map[string]string
	logger         io.Writer
	image          string
	format         Format
	mappingsDir    string // host path mounted at /mocks
	harFile        string // host file mounted at /har/traffic.har
	stubFiles      []string
	cmd            []string
	startupTimeout time.Duration
	hostPort       int // 0 = let docker assign
}

func newConfig() *config {
	return &config{
		image:          DefaultImage,
		format:         FormatAuto,
		envs:           map[string]string{},
		startupTimeout: 60 * time.Second,
	}
}

// Option mutates the container blueprint. Options compose freely; later
// options win when they overlap.
type Option func(*config) error

// WithImage overrides the docker image reference. The image is expected
// to be a CLI build with the `mock serve` entrypoint baked in.
func WithImage(ref string) Option {
	return func(c *config) error {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return fmt.Errorf("mockartycontainer: image must not be empty")
		}
		c.image = ref
		return nil
	}
}

// WithFormat selects the stub dialect ("auto" by default).
func WithFormat(f Format) Option {
	return func(c *config) error {
		if _, ok := validFormats[f]; !ok {
			return fmt.Errorf("mockartycontainer: unknown format %q (valid: auto|wiremock|mockarty|mockoon)", f)
		}
		c.format = f
		return nil
	}
}

// WithStubFile mounts a host-side stub file into /data/stubs/ inside
// the container. May be called repeatedly to mount multiple files.
func WithStubFile(hostPath string) Option {
	return func(c *config) error {
		hostPath = strings.TrimSpace(hostPath)
		if hostPath == "" {
			return fmt.Errorf("mockartycontainer: stub file path must not be empty")
		}
		abs, err := filepath.Abs(hostPath)
		if err != nil {
			return fmt.Errorf("mockartycontainer: resolve stub path %q: %w", hostPath, err)
		}
		c.stubFiles = append(c.stubFiles, abs)
		return nil
	}
}

// WithEnv injects an extra environment variable into the container.
// Useful for advanced CLI flags exposed via env (telemetry, log level,
// JWT secret).
func WithEnv(key, value string) Option {
	return func(c *config) error {
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("mockartycontainer: env key must not be empty")
		}
		c.envs[key] = value
		return nil
	}
}

// WithCmd overrides the default container CMD. Use sparingly — the
// image's baked-in entrypoint already wires the right flags. Mainly
// here for niche cases (verbose logging, dump-config debugging).
func WithCmd(cmd ...string) Option {
	return func(c *config) error {
		c.cmd = append([]string(nil), cmd...)
		return nil
	}
}

// WithMappings bind-mounts a host directory containing stub files
// (WireMock / Mockoon / native Mockarty JSON) into the running
// container at MappingsMount and tells the CLI to load it on startup
// via MockDirEnv. Drop-in replacement for the WireMock
// `withMappingFromResource` ergonomic.
//
// The path is resolved to an absolute path; the directory must exist
// at New-time (the bind-mount will otherwise fail inside Docker).
func WithMappings(hostDir string) Option {
	return func(c *config) error {
		hostDir = strings.TrimSpace(hostDir)
		if hostDir == "" {
			return fmt.Errorf("mockartycontainer: mappings dir must not be empty")
		}
		abs, err := filepath.Abs(hostDir)
		if err != nil {
			return fmt.Errorf("mockartycontainer: resolve mappings dir %q: %w", hostDir, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("mockartycontainer: mappings dir %q: %w", abs, err)
		}
		if !fi.IsDir() {
			return fmt.Errorf("mockartycontainer: mappings path %q is not a directory", abs)
		}
		c.mappingsDir = abs
		return nil
	}
}

// WithHAR bind-mounts a host-side HAR file into the container at
// HARMount and tells the CLI to replay the captured traffic via
// HARReplayEnv. Combine with WithMappings to layer hand-crafted stubs
// on top of recorded traffic.
func WithHAR(hostPath string) Option {
	return func(c *config) error {
		hostPath = strings.TrimSpace(hostPath)
		if hostPath == "" {
			return fmt.Errorf("mockartycontainer: HAR path must not be empty")
		}
		abs, err := filepath.Abs(hostPath)
		if err != nil {
			return fmt.Errorf("mockartycontainer: resolve HAR path %q: %w", hostPath, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("mockartycontainer: HAR file %q: %w", abs, err)
		}
		if fi.IsDir() {
			return fmt.Errorf("mockartycontainer: HAR path %q is a directory, want file", abs)
		}
		c.harFile = abs
		return nil
	}
}

// WithPort pins the host-side TCP port. Pass 0 (the default) to let
// docker pick an ephemeral port — recommended for parallel test
// suites. Pin a specific port only when an external consumer hard-codes
// it (browser dev-tools, legacy CI script).
func WithPort(hostPort int) Option {
	return func(c *config) error {
		if hostPort < 0 || hostPort > 65535 {
			return fmt.Errorf("mockartycontainer: host port %d out of range", hostPort)
		}
		c.hostPort = hostPort
		return nil
	}
}

// WithLogger streams the container's stdout+stderr to the supplied
// writer (e.g. os.Stderr, a testing.T's writer, a *bytes.Buffer). When
// unset the container logs are only available on demand via Logs().
func WithLogger(w io.Writer) Option {
	return func(c *config) error {
		c.logger = w
		return nil
	}
}

// WithStartupTimeout overrides the default 60s wait-for-ready timeout.
// Bump this when running on a slow CI runner or when pulling the image
// over a constrained network for the first time.
func WithStartupTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return fmt.Errorf("mockartycontainer: startup timeout must be positive")
		}
		c.startupTimeout = d
		return nil
	}
}
