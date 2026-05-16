// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact

import (
	"context"
	"errors"
	"log"
	"sync"
)

// Option mutates [Consumer] at construction time. Functional options
// are used per Mockarty's architecture principles (extensibility-first,
// composition over inheritance).
type Option func(*Consumer)

// Consumer is the top-level builder for a single consumer-provider
// contract. One Consumer represents one consumer service paired with
// one provider service; multiple providers means multiple Consumer
// instances.
//
// Consumer is NOT safe for concurrent construction (one goroutine
// builds, then forks to many test goroutines using Start). The Start
// method returns a [MockServer] which IS safe for concurrent use.
type Consumer struct {
	logger         *log.Logger
	consumer       string
	provider       string
	outputDir      string
	specVersion    SpecVersion
	interactions   []*Interaction
	plugins        []PluginEntry
	pluginRuntimes []pluginBinding
	mu             sync.Mutex
	closed         bool
}

// pluginBinding pairs a runtime Plugin (from pact/plugins.Default)
// with the per-Consumer configuration the user supplied via
// WithPlugin. The mock server dispatches incoming requests through
// these bindings in declaration order; first match wins.
type pluginBinding struct {
	plugin pluginRuntime
	config map[string]any
}

// pluginRuntime is the local interface satisfied by
// pact/plugins.Plugin. Keeping the interface local avoids a hard
// import-cycle when the plugins package itself wants to depend on the
// pact root package (e.g. for reusing strict_matcher helpers in a
// future plugin). The runtime hook in pluginLookup is provided by an
// init() in plugin_runtime.go.
type pluginRuntime interface {
	Name() string
	Version() string
	SupportedContentTypes() []string
	MatchRequest(ctx context.Context, expected map[string]any, actual []byte, contentType string) error
	GenerateResponse(ctx context.Context, expected map[string]any, contentType string) ([]byte, error)
}

// pluginLookup is set at package init time by plugin_runtime.go to
// avoid an import cycle (pact → pact/plugins → pact). Tests can swap
// the hook for fakes by replacing the variable.
var pluginLookup = func(name string) (pluginRuntime, bool) { return nil, false }

// NewConsumer builds a Consumer. consumer is the service-under-test
// name; provider is set via [WithProvider] (defaults to
// "UnknownProvider" — strict broker setups will reject this, on
// purpose).
func NewConsumer(consumer string, opts ...Option) *Consumer {
	c := &Consumer{
		consumer:    consumer,
		provider:    "UnknownProvider",
		specVersion: SpecV4,
		outputDir:   "./pacts",
	}
	for _, o := range opts {
		if o != nil {
			o(c)
		}
	}
	if !c.specVersion.Valid() {
		c.specVersion = SpecV4
	}
	return c
}

// WithProvider names the provider service.
func WithProvider(name string) Option { return func(c *Consumer) { c.provider = name } }

// WithSpecVersion selects V3 vs V4 output. Unknown values fall back to
// V4 at NewConsumer.
func WithSpecVersion(v SpecVersion) Option { return func(c *Consumer) { c.specVersion = v } }

// WithOutputDir sets the directory in which the pact.json file is
// written when the mock server closes. The directory is created on
// first write if it does not exist.
func WithOutputDir(dir string) Option { return func(c *Consumer) { c.outputDir = dir } }

// WithLogger redirects internal warnings (plugin stubs, mock-server
// teardown errors when no t.Logf is available, etc.).
func WithLogger(l *log.Logger) Option { return func(c *Consumer) { c.logger = l } }

// WithPlugin records a V4 plugin manifest in the emitted pact file
// AND wires the plugin into the in-process mock server so live
// requests get validated through the plugin's MatchRequest hook.
//
// Plugin lookup is performed via the package-level
// `pact/plugins.Default` registry: import the relevant plugin
// subpackage (e.g. `_ "github.com/mockarty/mockarty-go/pact/plugins/protobuf"`)
// once and it will self-register through its init().
//
// If the plugin name is not registered when this option is applied the
// metadata is still recorded (so downstream verifiers see the
// declaration) but the mock server falls back to the JSON-shape
// matcher engine. A warning is logged unless `WithLogger` was used to
// silence stdout.
func WithPlugin(name, version string, config map[string]any) Option {
	return func(c *Consumer) {
		entry := PluginEntry{
			Name:          name,
			Version:       version,
			Configuration: config,
		}
		c.plugins = append(c.plugins, entry)
		// Bind the plugin's runtime to this Consumer so the mock
		// server can dispatch requests by content type.
		if p, ok := pluginLookup(name); ok {
			c.pluginRuntimes = append(c.pluginRuntimes, pluginBinding{
				plugin: p,
				config: cloneConfig(config),
			})
			return
		}
		// Plugin not registered — log so the caller knows the metadata
		// will round-trip but the mock won't enforce the plugin's
		// per-request matcher.
		msg := "pact: WithPlugin(%q) — plugin not registered in pact/plugins; metadata recorded, runtime fallback to JSON-shape matcher"
		if c.logger != nil {
			c.logger.Printf(msg, name)
		} else {
			log.Printf(msg, name)
		}
	}
}

// cloneConfig produces a defensive copy so post-construction mutation
// of the caller's map does not bleed into the consumer's snapshot.
func cloneConfig(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// SpecVersion returns the version selected at construction time.
//
// Helpful in tests that want to assert the consumer is talking the
// expected dialect to its broker.
func (c *Consumer) SpecVersion() SpecVersion { return c.specVersion }

// ConsumerName returns the consumer service name.
func (c *Consumer) ConsumerName() string { return c.consumer }

// ProviderName returns the provider service name.
func (c *Consumer) ProviderName() string { return c.provider }

// OutputDir returns the configured output directory.
func (c *Consumer) OutputDir() string { return c.outputDir }

// AddInteraction returns a fresh [InteractionBuilder] for one new
// consumer-provider exchange. The builder is bound to this Consumer;
// finalising it (via the implicit terminal call WillRespondWith /
// WithJSONBody on the response side) leaves the interaction in
// `Consumer.Interactions()` ready to be served by [MockServer].
//
// Multiple interactions accumulate in the order they are added; the
// mock server tries them in that order when matching an incoming
// request.
func (c *Consumer) AddInteraction() *InteractionBuilder {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		// Defensively reject mutations after writes — the contract is sealed.
		return nil
	}
	ix := &Interaction{}
	c.interactions = append(c.interactions, ix)
	return &InteractionBuilder{
		consumer:    c,
		ix:          ix,
		specVersion: c.specVersion,
	}
}

// Interactions returns a snapshot of every interaction added so far.
// The returned slice is a copy; mutating it does not affect the
// Consumer.
func (c *Consumer) Interactions() []Interaction {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Interaction, len(c.interactions))
	for i, ix := range c.interactions {
		out[i] = *ix
	}
	return out
}

// Start spins up the in-process mock server bound to all interactions
// added so far. The returned [MockServer] exposes URL() for the user's
// HTTP client to point at, Verify() to assert call coverage at
// teardown, and Close() to shut down and write the pact.json file.
//
// Calling AddInteraction after Start panics — the contract must be
// fully built before the mock starts serving traffic. This matches
// pact-jvm and pact-python semantics; deferring is intentional.
func (c *Consumer) Start(ctx context.Context) (*MockServer, error) {
	if ctx == nil {
		return nil, errors.New("pact: Start requires a non-nil context")
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, errors.New("pact: Consumer is closed; build a new one")
	}
	snapshot := make([]Interaction, len(c.interactions))
	for i, ix := range c.interactions {
		snapshot[i] = *ix
	}
	c.mu.Unlock()
	return newMockServer(c, snapshot)
}

// finalize is called by the mock server on Close — it seals the
// consumer (no more AddInteraction) and triggers the writer.
func (c *Consumer) finalize() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
}

// snapshotForWriter returns the current state of the Consumer in a
// shape the writer can serialise.
func (c *Consumer) snapshotForWriter() PactFile {
	c.mu.Lock()
	defer c.mu.Unlock()
	ixs := make([]Interaction, len(c.interactions))
	for i, ix := range c.interactions {
		ixs[i] = *ix
	}
	return PactFile{
		Consumer: Participant{Name: c.consumer},
		Provider: Participant{Name: c.provider},
		Metadata: Metadata{
			PactSpecification: PactSpec{Version: string(c.specVersion)},
			Plugins:           append([]PluginEntry(nil), c.plugins...),
			MockarSDK: &SDKAnnotation{
				SDK:     "mockarty-go",
				Version: SDKVersion,
			},
		},
		Interactions: ixs,
	}
}

// SDKVersion is the Go SDK release tag stamped into the pact metadata
// so brokers can audit which client wrote the file.
const SDKVersion = "wave2.pact.v1"
