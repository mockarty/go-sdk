// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"errors"
	"strings"
	"time"
)

// SDKVersion is the wire-format tag stamped into every emitted target.
// Bumping it is a coordinated change: server-side and other SDKs must
// recognise the new tag.
const SDKVersion = "wave3.fuzz.v1"

// Reporter selects which artefact the runtime should emit alongside the
// raw findings.  Reporters are run server-side after the campaign ends.
type Reporter string

const (
	// ReporterDefault leaves the choice to the server (typically JSON).
	ReporterDefault Reporter = ""
	// ReporterAllure produces an allure-results compatible bundle.
	ReporterAllure Reporter = "allure"
	// ReporterJUnit produces a JUnit XML report.
	ReporterJUnit Reporter = "junit"
	// ReporterHTML produces a self-contained HTML report.
	ReporterHTML Reporter = "html"
	// ReporterJSON produces a structured JSON report.
	ReporterJSON Reporter = "json"
)

// Valid reports whether r is a known reporter value.
func (r Reporter) Valid() bool {
	switch r {
	case ReporterDefault, ReporterAllure, ReporterJUnit, ReporterHTML, ReporterJSON:
		return true
	}
	return false
}

// Strategy mirrors the server-side strategy enum (fuzzing.Strategy*).
// The default is StrategyMutation which is the broadest applicable.
type Strategy string

const (
	StrategyMutation    Strategy = "mutation"
	StrategySecurity    Strategy = "security"
	StrategySchemaAware Strategy = "schema_aware"
	StrategyAll         Strategy = "all"
)

// Valid reports whether s is a known strategy value.
func (s Strategy) Valid() bool {
	switch s {
	case StrategyMutation, StrategySecurity, StrategySchemaAware, StrategyAll:
		return true
	}
	return false
}

// Option mutates a Target at construction time.  Per Mockarty's
// architecture principles (composition over inheritance, extensibility
// first) every tunable goes through this functional-option seam.
type Option func(*Target)

// Target is the in-memory description of one fuzz campaign.  Build it
// with NewTarget + With* options; call ToJSON / WriteTo / hand to a
// Runner to actually execute.
//
// Fields are ordered for struct-field alignment efficiency (8-byte
// fields first, then strings/slices/maps, then smaller types) per
// CLAUDE.md "Struct Field Alignment".
type Target struct {
	endpoint         endpoint
	namespace        string
	strategy         Strategy
	reporter         Reporter
	name             string
	description      string
	coverage         CoverageHint
	seeds            []SeedRequest
	mutators         []customMutator
	assertions       []Assertion
	tags             []string
	statusCodeAlerts []int
	maxRequests      int64
	timeoutPerReq    time.Duration
	duration         time.Duration
	concurrency      int
	maxRPS           int
	stopOnFinding    bool
	followRedirects  bool
	verifyFindings   bool
}

// NewTarget builds a Target.  name is the human-readable label shown in
// the admin UI's run list and in Allure reports.
func NewTarget(name string, opts ...Option) *Target {
	t := &Target{
		name:           name,
		strategy:       StrategyMutation,
		reporter:       ReporterDefault,
		verifyFindings: true,
	}
	for _, o := range opts {
		if o != nil {
			o(t)
		}
	}
	return t
}

// WithDescription sets the campaign description.
func WithDescription(d string) Option { return func(t *Target) { t.description = d } }

// WithNamespace overrides the namespace the runner posts the target to.
// When unset the Runner falls back to its own bound namespace.
func WithNamespace(ns string) Option { return func(t *Target) { t.namespace = ns } }

// WithStrategy selects the broad fuzz strategy. Defaults to
// StrategyMutation.
func WithStrategy(s Strategy) Option { return func(t *Target) { t.strategy = s } }

// WithReporter selects the report flavour.
func WithReporter(r Reporter) Option { return func(t *Target) { t.reporter = r } }

// WithDuration caps the wall-clock duration of the campaign.  Zero or
// negative leaves it to the server default.
func WithDuration(d time.Duration) Option { return func(t *Target) { t.duration = d } }

// WithRequestTimeout caps a single fuzzed request.
func WithRequestTimeout(d time.Duration) Option { return func(t *Target) { t.timeoutPerReq = d } }

// WithMaxRequests caps total requests across the campaign.
func WithMaxRequests(n int64) Option { return func(t *Target) { t.maxRequests = n } }

// WithConcurrency caps in-flight requests.
func WithConcurrency(n int) Option { return func(t *Target) { t.concurrency = n } }

// WithMaxRPS caps the request rate.
func WithMaxRPS(n int) Option { return func(t *Target) { t.maxRPS = n } }

// WithStopOnFinding shuts the campaign down on the first finding.
// Useful in CI gates where any deviation is a fail.
func WithStopOnFinding(b bool) Option { return func(t *Target) { t.stopOnFinding = b } }

// WithFollowRedirects opts into HTTP-redirect following (the default
// is to NOT follow — redirects often hide the real signal).
func WithFollowRedirects(b bool) Option { return func(t *Target) { t.followRedirects = b } }

// WithVerifyFindings opts in/out of the engine's verification pass
// (a second request that re-issues the same payload to confirm the
// finding is reproducible).  Default true.
func WithVerifyFindings(b bool) Option { return func(t *Target) { t.verifyFindings = b } }

// WithStatusCodeAlerts overrides the status codes the engine treats as
// implicit findings (in addition to whatever AssertStatus declares).
// Server default: 500/502/503/504.
func WithStatusCodeAlerts(codes ...int) Option {
	cp := append([]int(nil), codes...)
	return func(t *Target) { t.statusCodeAlerts = cp }
}

// WithTag appends a free-form label (e.g. "ci", "nightly").
func WithTag(tag string) Option {
	return func(t *Target) {
		if tag == "" {
			return
		}
		t.tags = append(t.tags, tag)
	}
}

// Name returns the configured target name.
func (t *Target) Name() string { return t.name }

// Description returns the configured description.
func (t *Target) Description() string { return t.description }

// Namespace returns the configured namespace (empty when unset).
func (t *Target) Namespace() string { return t.namespace }

// Protocol returns the target's transport.  Returns "" before any
// With*Endpoint has been called.
func (t *Target) Protocol() Protocol { return t.endpoint.protocol }

// Seeds returns a copy of the current seed corpus.
func (t *Target) Seeds() []SeedRequest {
	out := make([]SeedRequest, len(t.seeds))
	copy(out, t.seeds)
	return out
}

// Mutators returns the names of all selected mutators.
func (t *Target) Mutators() []Mutator {
	out := make([]Mutator, 0, len(t.mutators))
	for _, m := range t.mutators {
		out = append(out, m.Name)
	}
	return out
}

// Assertions returns a copy of the configured assertions.
func (t *Target) Assertions() []Assertion {
	out := make([]Assertion, len(t.assertions))
	copy(out, t.assertions)
	return out
}

// Validate runs the local sanity-check pass.  Returns nil when the
// target can be transpiled, otherwise a descriptive error.  The Runner
// calls Validate before sending the target to the server so the failure
// surfaces in Go and not as an opaque 400.
func (t *Target) Validate() error {
	if t == nil {
		return errors.New("fuzz: nil Target")
	}
	if strings.TrimSpace(t.name) == "" {
		return errors.New("fuzz: target name is required")
	}
	if !t.endpoint.protocol.Valid() {
		return errors.New("fuzz: no protocol endpoint set (call WithHTTPEndpoint, WithGRPCEndpoint, …)")
	}
	if !t.strategy.Valid() {
		return errors.New("fuzz: invalid strategy")
	}
	if !t.reporter.Valid() {
		return errors.New("fuzz: invalid reporter")
	}
	if t.endpoint.protocol == ProtocolHTTP {
		if t.endpoint.method == "" {
			return errors.New("fuzz: WithHTTPEndpoint requires a non-empty method")
		}
		if t.endpoint.address == "" {
			return errors.New("fuzz: WithHTTPEndpoint requires a non-empty baseURL")
		}
	}
	if t.endpoint.protocol == ProtocolGRPC {
		if t.endpoint.address == "" || t.endpoint.grpcService == "" || t.endpoint.grpcMethod == "" {
			return errors.New("fuzz: WithGRPCEndpoint requires address, service, and method")
		}
	}
	if t.endpoint.protocol == ProtocolGraphQL && t.endpoint.address == "" {
		return errors.New("fuzz: WithGraphQLEndpoint requires a baseURL")
	}
	if t.endpoint.protocol == ProtocolWebSocket && t.endpoint.address == "" {
		return errors.New("fuzz: WithWebSocketEndpoint requires a URL")
	}
	if t.endpoint.protocol == ProtocolKafka && (t.endpoint.address == "" || t.endpoint.path == "") {
		return errors.New("fuzz: WithKafkaEndpoint requires brokers and topic")
	}
	if t.endpoint.protocol == ProtocolRabbitMQ && t.endpoint.address == "" {
		return errors.New("fuzz: WithRabbitMQEndpoint requires an AMQP URL")
	}

	// Mutators: built-ins must be in the registry; custom (with JSConfig)
	// pass through to the server unverified.
	for _, m := range t.mutators {
		if m.JSConfig != "" {
			continue
		}
		if !m.Name.Valid() {
			return errors.New("fuzz: unknown mutator " + string(m.Name) + " (register via RegisterMutator or supply JSConfig)")
		}
	}

	for _, a := range t.assertions {
		if err := validateAssertion(a); err != nil {
			return err
		}
	}

	if t.duration < 0 || t.timeoutPerReq < 0 {
		return errors.New("fuzz: durations must be non-negative")
	}
	if t.maxRequests < 0 || int64(t.concurrency) < 0 || int64(t.maxRPS) < 0 {
		return errors.New("fuzz: count caps must be non-negative")
	}

	// Sanity: if there are no seeds AND the protocol is one that *requires*
	// a seed corpus (HTTP non-GET, gRPC, GraphQL with no inline query),
	// fail loudly.  GET requests can fuzz the path/query without a body
	// seed so they are tolerated.
	requiresSeeds := false
	switch t.endpoint.protocol {
	case ProtocolHTTP:
		m := strings.ToUpper(t.endpoint.method)
		if m == "POST" || m == "PUT" || m == "PATCH" {
			requiresSeeds = true
		}
	case ProtocolGRPC, ProtocolKafka, ProtocolRabbitMQ, ProtocolSOAP, ProtocolWebSocket:
		requiresSeeds = true
	case ProtocolGraphQL:
		if t.endpoint.graphqlQuery == "" {
			requiresSeeds = true
		}
	}
	if requiresSeeds && len(t.seeds) == 0 {
		return errors.New("fuzz: this protocol/method needs at least one seed (use WithSeedCorpus / WithSeed)")
	}

	return nil
}
