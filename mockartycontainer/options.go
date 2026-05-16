// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer

import (
	"fmt"
	"path/filepath"
	"strings"
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
	// dialect to expect when auto-detect is disabled.
	FormatEnv = "MOCKARTY_STUB_FORMAT"
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
	image     string
	format    Format
	stubFiles []string
	envs      map[string]string
	cmd       []string
}

func newConfig() *config {
	return &config{
		image:  DefaultImage,
		format: FormatAuto,
		envs:   map[string]string{},
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
