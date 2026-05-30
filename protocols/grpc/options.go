// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package grpc

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Option mutates the client's dial-time configuration. Pass options to
// Dial / NewClient — functional-options pattern, same shape as the rest
// of mockarty-go.
type Option func(*config)

// config holds the dial-time configuration. Unexported so users can't
// poke at it after construction.
type config struct {
	recorder   StepRecorder
	creds      credentials.TransportCredentials
	metadata   map[string]string
	dialOpts   []grpc.DialOption
	protoFiles []string
	importDirs []string
	timeout    time.Duration
	// useReflection enables server-side reflection as the descriptor
	// source. When false, the client relies on protoFiles (passed via
	// WithProtoFile) for descriptor lookup; falling back to reflection
	// is opt-in to avoid surprise round-trips from a test that "just
	// wanted to send one RPC".
	useReflection bool
	// payloadCap is the max bytes of request/response captured in a
	// step's Parameters map. Default 1 KiB so a large gRPC payload
	// can't blow up the TCM run row.
	payloadCap int
}

func defaultConfig() *config {
	return &config{
		recorder:      nopRecorder{},
		timeout:       30 * time.Second,
		useReflection: true,
		payloadCap:    1024,
	}
}

// WithRecorder wires a step recorder. Nil = drop steps (default).
// Typical use: WithRecorder(NewExternalRunsRecorder(runs, runID)).
func WithRecorder(r StepRecorder) Option {
	return func(c *config) {
		if r == nil {
			c.recorder = nopRecorder{}
			return
		}
		c.recorder = r
	}
}

// WithTLS pins the transport credentials. Default is insecure (plaintext)
// — appropriate for in-cluster Mockarty mocks and local dev; mandatory
// to flip for prod endpoints.
func WithTLS(creds credentials.TransportCredentials) Option {
	return func(c *config) { c.creds = creds }
}

// WithDialOption appends a raw grpc.DialOption — escape hatch for users
// who need balancer/keepalive/etc. Don't reach for it unless the typed
// options above can't express what you need.
func WithDialOption(opts ...grpc.DialOption) Option {
	return func(c *config) { c.dialOpts = append(c.dialOpts, opts...) }
}

// WithMetadata sets default headers applied to every RPC. Useful for
// Authorization / x-tenant-id / x-request-id constants. Per-call
// metadata still wins through gRPC's normal context metadata channel.
func WithMetadata(md map[string]string) Option {
	return func(c *config) {
		if c.metadata == nil {
			c.metadata = make(map[string]string, len(md))
		}
		for k, v := range md {
			c.metadata[k] = v
		}
	}
}

// WithProtoFile registers one or more .proto source files as a
// descriptor source. The loader resolves imports against ImportDirs
// passed via WithImportDir; if you compile-locally / vendored your
// protos, pass the repo root as an import dir so transitive imports
// resolve.
//
// When multiple files are registered the client searches them in
// order. Server reflection (enabled by default) still wins — flip
// WithReflection(false) to force file-only.
func WithProtoFile(paths ...string) Option {
	return func(c *config) { c.protoFiles = append(c.protoFiles, paths...) }
}

// WithImportDir adds a directory the proto-file loader searches when
// resolving `import "..."` statements. Typically the repo root and/or
// `third_party/`.
func WithImportDir(dirs ...string) Option {
	return func(c *config) { c.importDirs = append(c.importDirs, dirs...) }
}

// WithReflection toggles the server-reflection descriptor source.
// Default true. Flip to false when the server doesn't expose
// grpc.reflection (e.g. production hardened build) and you have
// .proto files registered via WithProtoFile.
func WithReflection(enabled bool) Option {
	return func(c *config) { c.useReflection = enabled }
}

// WithTimeout sets the default per-call timeout. Default 30s. The
// per-call ctx still wins — InvokeJSON honours ctx.Deadline() before
// falling back to this.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithPayloadCaptureBytes caps the JSON payload recorded in a step's
// Parameters map. Default 1024. Set to 0 to disable payload capture
// (status / method / duration still recorded).
func WithPayloadCaptureBytes(n int) Option {
	return func(c *config) {
		if n < 0 {
			n = 0
		}
		c.payloadCap = n
	}
}
