// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// transpiledConfig is the canonical wire shape the Mockarty admin server
// stores in the fuzz_configs table.  Field names line up with
// internal/fuzzing/config.go FuzzConfig (server) and api_fuzzing.go
// FuzzingConfig (this SDK's existing client).
//
// Optional fields are pointers / omitempty so the server's defaults
// apply when the SDK leaves them blank.
type transpiledConfig struct {
	Coverage          *CoverageHint    `json:"coverage,omitempty"`
	SDK               sdkAnnotation    `json:"sdk"`
	Name              string           `json:"name"`
	Description       string           `json:"description,omitempty"`
	Namespace         string           `json:"namespace,omitempty"`
	TargetBaseURL     string           `json:"targetBaseUrl"`
	SourceType        string           `json:"sourceType"`
	Protocol          string           `json:"protocol,omitempty"`
	Strategy          string           `json:"strategy"`
	ID                string           `json:"id,omitempty"`
	Reporter          string           `json:"reporter,omitempty"`
	SeedRequests      []seedWireFormat `json:"seedRequests,omitempty"`
	Assertions        []Assertion      `json:"assertions,omitempty"`
	Mutators          []wireMutator    `json:"mutators,omitempty"`
	Tags              []string         `json:"tags,omitempty"`
	PayloadCategories []string         `json:"payloadCategories,omitempty"`
	Options           wireOptions      `json:"options"`
}

// seedWireFormat is the on-wire shape of one SeedRequest.  It must
// remain a byte-identical superset of fuzzing.FuzzSeedRequest so the
// server can decode without translation.
type seedWireFormat struct {
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	PathParams  map[string]string `json:"pathParams,omitempty"`
	ID          string            `json:"id"`
	Name        string            `json:"name,omitempty"`
	Method      string            `json:"method,omitempty"`
	URL         string            `json:"url,omitempty"`
	Path        string            `json:"path,omitempty"`
	Body        string            `json:"body,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
}

// wireOptions mirrors fuzzing.FuzzOptions (server) plus the SDK-only
// transport adjuncts the engine needs to wire up a non-HTTP target.
type wireOptions struct {
	CustomHeaders    map[string]string `json:"customHeaders,omitempty"`
	GraphQLOperation string            `json:"graphqlOperation,omitempty"`
	GraphQLPath      string            `json:"graphqlPath,omitempty"`
	WebSocketURL     string            `json:"websocketUrl,omitempty"`
	GraphQLQuery     string            `json:"graphqlQuery,omitempty"`
	GRPCAddress      string            `json:"grpcAddress,omitempty"`
	GraphQLEndpoint  string            `json:"graphqlEndpoint,omitempty"`
	TimeoutPerReq    string            `json:"timeoutPerReq,omitempty"`
	MaxDuration      string            `json:"maxDuration,omitempty"`
	KafkaBrokers     string            `json:"kafkaBrokers,omitempty"`
	KafkaTopic       string            `json:"kafkaTopic,omitempty"`
	AMQPURL          string            `json:"amqpUrl,omitempty"`
	AMQPExchange     string            `json:"amqpExchange,omitempty"`
	AMQPRoutingKey   string            `json:"amqpRoutingKey,omitempty"`
	GRPCServices     []string          `json:"grpcServices,omitempty"`
	GRPCMethods      []string          `json:"grpcMethods,omitempty"`
	StatusCodeAlerts []int             `json:"statusCodeAlerts,omitempty"`
	MaxRequests      int64             `json:"maxRequests,omitempty"`
	MaxRPS           int               `json:"maxRps,omitempty"`
	Concurrency      int               `json:"concurrency,omitempty"`
	GRPCUseTLS       bool              `json:"grpcUseTls,omitempty"`
	StopOnCritical   bool              `json:"stopOnCritical,omitempty"`
	StopOnFinding    bool              `json:"stopOnFinding,omitempty"`
	FollowRedirects  bool              `json:"followRedirects,omitempty"`
	VerifyFindings   bool              `json:"verifyFindings,omitempty"`
}

// wireMutator is the on-wire shape of one selected mutator.
type wireMutator struct {
	Name     string `json:"name"`
	JSConfig string `json:"jsConfig,omitempty"`
}

// sdkAnnotation stamps the producer SDK identity into every config so
// the broker can audit which client built it.
type sdkAnnotation struct {
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

// ToJSON renders the target as canonical Mockarty fuzz-config JSON.
//
// Returns an error if Validate fails.  The output is stable for a given
// Target (no map iteration leaks): seed slices keep their insertion
// order; map fields are emitted with sorted keys via the seedWireFormat
// wrapper.
func (t *Target) ToJSON() ([]byte, error) {
	cfg, err := t.toWire()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// WriteTo serialises and writes to a file. Parent directories are
// created with 0o755 if they do not exist.
func (t *Target) WriteTo(path string) error {
	if path == "" {
		return fmt.Errorf("fuzz: WriteTo requires a non-empty path")
	}
	data, err := t.ToJSON()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("fuzz: mkdir %q: %w", dir, err)
		}
	}
	return os.WriteFile(path, data, 0o644)
}

// toWire applies validation and projects Target onto the wire schema.
func (t *Target) toWire() (*transpiledConfig, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}

	cfg := &transpiledConfig{
		Name:        t.name,
		Description: t.description,
		Namespace:   t.namespace,
		Strategy:    string(t.strategy),
		Reporter:    string(t.reporter),
		Protocol:    string(t.endpoint.protocol),
		Tags:        append([]string(nil), t.tags...),
		SDK:         sdkAnnotation{SDK: "mockarty-go", Version: SDKVersion},
	}

	// Seeds: project to wire format and fill in IDs.
	cfg.SeedRequests = make([]seedWireFormat, len(t.seeds))
	for i, s := range t.seeds {
		id := s.ID
		if id == "" {
			id = fmt.Sprintf("seed-%d", i)
		}
		cfg.SeedRequests[i] = seedWireFormat{
			ID:          id,
			Name:        s.Name,
			Method:      s.Method,
			URL:         s.URL,
			Path:        s.Path,
			Body:        s.Body,
			ContentType: s.ContentType,
			Headers:     copyMap(s.Headers),
			QueryParams: copyMap(s.QueryParams),
			PathParams:  copyMap(s.PathParams),
		}
	}

	// Endpoint projection — collapse the typed endpoint variants down
	// to the flat FuzzConfig fields the server already understands.
	switch t.endpoint.protocol {
	case ProtocolHTTP:
		cfg.SourceType = "manual"
		cfg.TargetBaseURL = t.endpoint.address
		// Method/path travel via the first seed if the caller didn't
		// specify per-seed values — otherwise the engine respects per-seed.
		for i := range cfg.SeedRequests {
			if cfg.SeedRequests[i].Method == "" {
				cfg.SeedRequests[i].Method = t.endpoint.method
			}
			if cfg.SeedRequests[i].Path == "" {
				cfg.SeedRequests[i].Path = t.endpoint.path
			}
		}
	case ProtocolGRPC:
		cfg.SourceType = "grpc"
		cfg.TargetBaseURL = t.endpoint.address
	case ProtocolGraphQL:
		cfg.SourceType = "graphql"
		cfg.TargetBaseURL = t.endpoint.address
	case ProtocolSOAP:
		cfg.SourceType = "manual"
		cfg.TargetBaseURL = t.endpoint.address
	case ProtocolWebSocket:
		cfg.SourceType = "websocket"
		cfg.TargetBaseURL = t.endpoint.address
	case ProtocolKafka:
		cfg.SourceType = "kafka"
		cfg.TargetBaseURL = "kafka://" + t.endpoint.address
	case ProtocolRabbitMQ:
		cfg.SourceType = "rabbitmq"
		cfg.TargetBaseURL = t.endpoint.address
	}

	// Options projection.
	cfg.Options = wireOptions{
		CustomHeaders:    copyMap(t.endpoint.headers),
		MaxRequests:      t.maxRequests,
		MaxRPS:           t.maxRPS,
		Concurrency:      t.concurrency,
		StatusCodeAlerts: append([]int(nil), t.statusCodeAlerts...),
		StopOnFinding:    t.stopOnFinding,
		FollowRedirects:  t.followRedirects,
		VerifyFindings:   t.verifyFindings,
	}
	if t.duration > 0 {
		cfg.Options.MaxDuration = t.duration.String()
	}
	if t.timeoutPerReq > 0 {
		cfg.Options.TimeoutPerReq = t.timeoutPerReq.String()
	}

	// Protocol-specific Options pour-over.
	switch t.endpoint.protocol {
	case ProtocolGRPC:
		cfg.Options.GRPCAddress = t.endpoint.address
		cfg.Options.GRPCServices = []string{t.endpoint.grpcService}
		cfg.Options.GRPCMethods = []string{t.endpoint.grpcMethod}
		cfg.Options.GRPCUseTLS = t.endpoint.grpcUseTLS
	case ProtocolGraphQL:
		cfg.Options.GraphQLEndpoint = t.endpoint.address
		cfg.Options.GraphQLPath = t.endpoint.graphqlPath
		cfg.Options.GraphQLOperation = t.endpoint.graphqlOpName
		cfg.Options.GraphQLQuery = t.endpoint.graphqlQuery
	case ProtocolWebSocket:
		cfg.Options.WebSocketURL = t.endpoint.address
	case ProtocolKafka:
		cfg.Options.KafkaBrokers = t.endpoint.address
		cfg.Options.KafkaTopic = t.endpoint.path
	case ProtocolRabbitMQ:
		// Path was packed as "exchange::routingKey" — decode it here.
		ex, rk := splitRabbitPath(t.endpoint.path)
		cfg.Options.AMQPURL = t.endpoint.address
		cfg.Options.AMQPExchange = ex
		cfg.Options.AMQPRoutingKey = rk
	}

	// Mutators + payload categories.
	cfg.Mutators = make([]wireMutator, 0, len(t.mutators))
	categories := make(map[string]struct{}, 8)
	for _, m := range t.mutators {
		cfg.Mutators = append(cfg.Mutators, wireMutator{Name: string(m.Name), JSConfig: m.JSConfig})
		for _, c := range m.Name.CategoriesFor() {
			categories[c] = struct{}{}
		}
	}
	if len(categories) > 0 {
		cfg.PayloadCategories = make([]string, 0, len(categories))
		for c := range categories {
			cfg.PayloadCategories = append(cfg.PayloadCategories, c)
		}
		sort.Strings(cfg.PayloadCategories)
	}

	// Coverage hint — only emit when non-empty.
	if len(t.coverage.ResponseStatusCodes) > 0 || len(t.coverage.JSONPaths) > 0 {
		cp := CoverageHint{
			ResponseStatusCodes: append([]int(nil), t.coverage.ResponseStatusCodes...),
			JSONPaths:           append([]string(nil), t.coverage.JSONPaths...),
		}
		cfg.Coverage = &cp
	}

	// Assertions — copy to a fresh slice so callers' mutations don't
	// leak into already-emitted JSON.
	if len(t.assertions) > 0 {
		cfg.Assertions = make([]Assertion, len(t.assertions))
		copy(cfg.Assertions, t.assertions)
	}

	return cfg, nil
}

// FromJSON parses a previously-emitted target JSON back into a Target.
// Lossy in the sense that server-default fields stay zero; the round-
// trip is "logical equivalence" not byte equality.
func FromJSON(data []byte) (*Target, error) {
	var cfg transpiledConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("fuzz: parse JSON: %w", err)
	}
	t := &Target{
		name:             cfg.Name,
		description:      cfg.Description,
		namespace:        cfg.Namespace,
		strategy:         Strategy(cfg.Strategy),
		reporter:         Reporter(cfg.Reporter),
		tags:             append([]string(nil), cfg.Tags...),
		statusCodeAlerts: append([]int(nil), cfg.Options.StatusCodeAlerts...),
		maxRequests:      cfg.Options.MaxRequests,
		maxRPS:           cfg.Options.MaxRPS,
		concurrency:      cfg.Options.Concurrency,
		stopOnFinding:    cfg.Options.StopOnFinding,
		followRedirects:  cfg.Options.FollowRedirects,
		verifyFindings:   cfg.Options.VerifyFindings,
	}
	if cfg.Options.MaxDuration != "" {
		if d, err := time.ParseDuration(cfg.Options.MaxDuration); err == nil {
			t.duration = d
		}
	}
	if cfg.Options.TimeoutPerReq != "" {
		if d, err := time.ParseDuration(cfg.Options.TimeoutPerReq); err == nil {
			t.timeoutPerReq = d
		}
	}
	// Seeds.
	t.seeds = make([]SeedRequest, len(cfg.SeedRequests))
	for i, s := range cfg.SeedRequests {
		t.seeds[i] = SeedRequest{
			ID:          s.ID,
			Name:        s.Name,
			Method:      s.Method,
			URL:         s.URL,
			Path:        s.Path,
			Body:        s.Body,
			ContentType: s.ContentType,
			Headers:     copyMap(s.Headers),
			QueryParams: copyMap(s.QueryParams),
			PathParams:  copyMap(s.PathParams),
		}
	}
	// Mutators.
	t.mutators = make([]customMutator, len(cfg.Mutators))
	for i, m := range cfg.Mutators {
		t.mutators[i] = customMutator{Name: Mutator(m.Name), JSConfig: m.JSConfig}
	}
	// Assertions.
	t.assertions = append([]Assertion(nil), cfg.Assertions...)
	// Coverage.
	if cfg.Coverage != nil {
		t.coverage = CoverageHint{
			ResponseStatusCodes: append([]int(nil), cfg.Coverage.ResponseStatusCodes...),
			JSONPaths:           append([]string(nil), cfg.Coverage.JSONPaths...),
		}
	}
	// Endpoint reconstruction — prefer the explicit Protocol field (an
	// SDK-side annotation) so SOAP / HTTP can be distinguished even
	// though they share SourceType="manual" on the server.  Fall back
	// to SourceType when Protocol is absent (configs emitted by other
	// SDKs or hand-written by the user).
	t.endpoint = endpoint{
		address: cfg.TargetBaseURL,
		headers: copyMap(cfg.Options.CustomHeaders),
	}
	src := cfg.SourceType
	if cfg.Protocol != "" {
		src = cfg.Protocol
	}
	switch src {
	case "manual", string(ProtocolHTTP), "":
		t.endpoint.protocol = ProtocolHTTP
		if len(cfg.SeedRequests) > 0 {
			t.endpoint.method = cfg.SeedRequests[0].Method
			t.endpoint.path = cfg.SeedRequests[0].Path
		}
	case string(ProtocolSOAP):
		t.endpoint.protocol = ProtocolSOAP
		if len(cfg.SeedRequests) > 0 {
			t.endpoint.method = cfg.SeedRequests[0].Method
			t.endpoint.path = cfg.SeedRequests[0].Path
		}
	case "grpc":
		t.endpoint.protocol = ProtocolGRPC
		if cfg.Options.GRPCAddress != "" {
			t.endpoint.address = cfg.Options.GRPCAddress
		}
		if len(cfg.Options.GRPCServices) > 0 {
			t.endpoint.grpcService = cfg.Options.GRPCServices[0]
		}
		if len(cfg.Options.GRPCMethods) > 0 {
			t.endpoint.grpcMethod = cfg.Options.GRPCMethods[0]
		}
		t.endpoint.grpcUseTLS = cfg.Options.GRPCUseTLS
	case "graphql":
		t.endpoint.protocol = ProtocolGraphQL
		t.endpoint.graphqlPath = cfg.Options.GraphQLPath
		t.endpoint.graphqlOpName = cfg.Options.GraphQLOperation
		t.endpoint.graphqlQuery = cfg.Options.GraphQLQuery
	case "kafka":
		t.endpoint.protocol = ProtocolKafka
		t.endpoint.address = cfg.Options.KafkaBrokers
		t.endpoint.path = cfg.Options.KafkaTopic
	case "rabbitmq":
		t.endpoint.protocol = ProtocolRabbitMQ
		t.endpoint.address = cfg.Options.AMQPURL
		t.endpoint.path = cfg.Options.AMQPExchange + "::" + cfg.Options.AMQPRoutingKey
	case "websocket":
		t.endpoint.protocol = ProtocolWebSocket
		t.endpoint.address = cfg.Options.WebSocketURL
	}
	return t, nil
}

func splitRabbitPath(packed string) (string, string) {
	for i := 0; i < len(packed)-1; i++ {
		if packed[i] == ':' && packed[i+1] == ':' {
			return packed[:i], packed[i+2:]
		}
	}
	return packed, ""
}

func copyMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
