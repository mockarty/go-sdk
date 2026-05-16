// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package fuzz_test

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/fuzz"
)

// fullHTTPTarget builds a representative HTTP target every test in this
// file can reuse.  Centralising the construction avoids drift between
// tests when the DSL grows.
func fullHTTPTarget(t *testing.T) *fuzz.Target {
	t.Helper()
	return fuzz.NewTarget("full-login",
		fuzz.WithDescription("login fuzz"),
		fuzz.WithNamespace("default"),
		fuzz.WithHTTPEndpoint("POST", "https://api.example.com", "/api/v1/login"),
		fuzz.WithHeader("X-Test", "1"),
		fuzz.WithAuthHeader("Bearer abc"),
		fuzz.WithSeedCorpus(
			fuzz.Seed("valid", `{"u":"a","p":"b"}`),
			fuzz.Seed("missing-pw", `{"u":"a"}`),
		),
		fuzz.WithMutator(fuzz.MutatorJSON),
		fuzz.WithMutator(fuzz.MutatorString),
		fuzz.WithDuration(2*time.Minute),
		fuzz.WithRequestTimeout(5*time.Second),
		fuzz.WithMaxRequests(1000),
		fuzz.WithConcurrency(8),
		fuzz.WithMaxRPS(50),
		fuzz.WithStopOnFinding(true),
		fuzz.WithFollowRedirects(false),
		fuzz.WithVerifyFindings(true),
		fuzz.WithStrategy(fuzz.StrategySecurity),
		fuzz.WithReporter(fuzz.ReporterAllure),
		fuzz.WithStatusCodeAlerts(500, 502, 503),
		fuzz.WithAssertion(fuzz.AssertStatus(200, 299)),
		fuzz.WithAssertion(fuzz.AssertNoCrash()),
		fuzz.WithAssertion(fuzz.AssertResponseTimeUnder(5*time.Second)),
		fuzz.WithAssertion(fuzz.AssertNoErrorInBody(`(?i)panic`)),
		fuzz.WithExpectedStatus(200),
		fuzz.WithAssertedJSONPath("$.token"),
		fuzz.WithTag("ci"),
		fuzz.WithTag("nightly"),
	)
}

func TestTargetValidateRequiresProtocol(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("bare")
	if err := tgt.Validate(); err == nil {
		t.Fatal("expected error when no protocol endpoint set")
	}
}

func TestTargetValidateRequiresName(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget(" ",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
	)
	if err := tgt.Validate(); err == nil {
		t.Fatal("expected error on whitespace-only name")
	}
}

func TestTargetHTTPRequiresMethodAndURL(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t", fuzz.WithHTTPEndpoint("", "", ""))
	if err := tgt.Validate(); err == nil {
		t.Fatal("expected error when http method/url empty")
	}
}

func TestTargetPOSTRequiresSeeds(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("POST", "https://x", "/"),
	)
	if err := tgt.Validate(); err == nil {
		t.Fatal("expected error when POST has no seeds")
	}
}

func TestTargetGETNoSeedsOK(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
	)
	if err := tgt.Validate(); err != nil {
		t.Fatalf("GET should not require seeds: %v", err)
	}
}

func TestTargetUnknownMutatorRejected(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.Mutator("totally-fake")),
	)
	if err := tgt.Validate(); err == nil {
		t.Fatal("expected error for unknown built-in mutator")
	}
}

func TestTargetCustomMutatorPassesThrough(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithCustomMutator("org-internal-js", `function mutate(b){return b;}`),
	)
	if err := tgt.Validate(); err != nil {
		t.Fatalf("custom mutator should pass through: %v", err)
	}
}

func TestRegisterMutator(t *testing.T) {
	t.Parallel()
	const key = fuzz.Mutator("test-register-mutator-key")
	fuzz.RegisterMutator(key, "boundary_values")
	if !key.Valid() {
		t.Fatal("registered mutator must report Valid()")
	}
	if cats := key.CategoriesFor(); len(cats) != 1 || cats[0] != "boundary_values" {
		t.Fatalf("CategoriesFor=%v want [boundary_values]", cats)
	}
}

func TestListMutatorsSorted(t *testing.T) {
	t.Parallel()
	list := fuzz.ListMutators()
	if len(list) < 8 {
		t.Fatalf("expected ≥8 built-in mutators, got %d", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1] >= list[i] {
			t.Fatalf("ListMutators not sorted: %v", list)
		}
	}
}

func TestAssertionsValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		a    fuzz.Assertion
		ok   bool
	}{
		{"status-ok", fuzz.AssertStatus(200, 299), true},
		{"status-bad", fuzz.AssertStatus(500, 200), false},
		{"crash-ok", fuzz.AssertNoCrash(), true},
		{"resp-time-ok", fuzz.AssertResponseTimeUnder(2 * time.Second), true},
		{"resp-time-zero", fuzz.AssertResponseTimeUnder(0), true},
		{"regex-empty", fuzz.AssertNoErrorInBody(""), false},
		{"regex-ok", fuzz.AssertNoErrorInBody(`panic`), true},
		{"contains-empty", fuzz.AssertBodyContains(""), false},
		{"contains-ok", fuzz.AssertBodyContains(`hello`), true},
		{"status-class-2", fuzz.AssertStatusClass(2), true},
		{"status-class-out-of-range", fuzz.AssertStatusClass(9), true}, // clamps to 2xx
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tgt := fuzz.NewTarget("t",
				fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
				fuzz.WithAssertion(tc.a),
			)
			err := tgt.Validate()
			gotOK := err == nil
			if gotOK != tc.ok {
				t.Fatalf("validate ok=%v want %v, err=%v", gotOK, tc.ok, err)
			}
		})
	}
}

func TestToJSONShapeMatchesServerSchema(t *testing.T) {
	t.Parallel()
	tgt := fullHTTPTarget(t)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}

	// Hard schema requirements — these are what the server-side
	// FuzzConfig type expects, see internal/fuzzing/config.go.
	for _, key := range []string{
		"name", "targetBaseUrl", "sourceType", "strategy",
		"seedRequests", "options", "sdk",
	} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing top-level key %q in transpiled JSON", key)
		}
	}

	sdk, ok := raw["sdk"].(map[string]any)
	if !ok {
		t.Fatalf("sdk must be an object, got %T", raw["sdk"])
	}
	if sdk["sdk"] != "mockarty-go" {
		t.Errorf("sdk.sdk=%v want mockarty-go", sdk["sdk"])
	}
	if sdk["version"] != fuzz.SDKVersion {
		t.Errorf("sdk.version=%v want %v", sdk["version"], fuzz.SDKVersion)
	}

	if raw["sourceType"] != "manual" {
		t.Errorf("sourceType=%v want manual", raw["sourceType"])
	}
	if raw["targetBaseUrl"] != "https://api.example.com" {
		t.Errorf("targetBaseUrl=%v want origin", raw["targetBaseUrl"])
	}

	opts := raw["options"].(map[string]any)
	if got := opts["maxDuration"]; got != "2m0s" {
		t.Errorf("options.maxDuration=%v want 2m0s", got)
	}
	if got := opts["stopOnFinding"]; got != true {
		t.Errorf("options.stopOnFinding=%v want true", got)
	}
}

func TestToJSONRoundTrip(t *testing.T) {
	t.Parallel()
	src := fullHTTPTarget(t)
	data, err := src.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	round, err := fuzz.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if got := round.Name(); got != src.Name() {
		t.Errorf("name lost: %q -> %q", src.Name(), got)
	}
	if got := round.Description(); got != src.Description() {
		t.Errorf("description lost: %q -> %q", src.Description(), got)
	}
	if got := round.Namespace(); got != src.Namespace() {
		t.Errorf("namespace lost: %q -> %q", src.Namespace(), got)
	}
	if got := round.Protocol(); got != src.Protocol() {
		t.Errorf("protocol lost: %q -> %q", src.Protocol(), got)
	}
	if got := len(round.Seeds()); got != len(src.Seeds()) {
		t.Errorf("seeds lost: %d -> %d", len(src.Seeds()), got)
	}
	if got := len(round.Mutators()); got != len(src.Mutators()) {
		t.Errorf("mutators lost: %d -> %d", len(src.Mutators()), got)
	}
	if got := len(round.Assertions()); got != len(src.Assertions()) {
		t.Errorf("assertions lost: %d -> %d", len(src.Assertions()), got)
	}
	if err := round.Validate(); err != nil {
		t.Errorf("round-tripped target fails validate: %v", err)
	}
}

func TestProtocolMatrix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		wantSource  string
		wantBaseURL string
		opts        []fuzz.Option
	}{
		{
			name: "grpc",
			opts: []fuzz.Option{
				fuzz.WithGRPCEndpoint("svc:9090", "pkg.Service", "Method", true),
				fuzz.WithSeed(fuzz.Seed("s", "{}")),
				fuzz.WithMutator(fuzz.MutatorGRPC),
			},
			wantSource:  "grpc",
			wantBaseURL: "svc:9090",
		},
		{
			name: "graphql",
			opts: []fuzz.Option{
				fuzz.WithGraphQLEndpoint("https://api.example.com", "/graphql", "Login", `query Login { ok }`),
				fuzz.WithMutator(fuzz.MutatorGraphQL),
			},
			wantSource:  "graphql",
			wantBaseURL: "https://api.example.com",
		},
		{
			name: "soap",
			opts: []fuzz.Option{
				fuzz.WithSOAPEndpoint("https://api.example.com", "/svc", "urn:do"),
				fuzz.WithSeed(fuzz.Seed("s", `<env/>`)),
				fuzz.WithMutator(fuzz.MutatorXML),
			},
			wantSource:  "manual",
			wantBaseURL: "https://api.example.com",
		},
		{
			name: "websocket",
			opts: []fuzz.Option{
				fuzz.WithWebSocketEndpoint("wss://chat.example.com/ws"),
				fuzz.WithSeed(fuzz.Seed("s", `{"m":"hi"}`)),
				fuzz.WithMutator(fuzz.MutatorJSON),
			},
			wantSource:  "websocket",
			wantBaseURL: "wss://chat.example.com/ws",
		},
		{
			name: "kafka",
			opts: []fuzz.Option{
				fuzz.WithKafkaEndpoint("broker1:9092,broker2:9092", "events"),
				fuzz.WithSeed(fuzz.Seed("s", `{"x":1}`)),
				fuzz.WithMutator(fuzz.MutatorJSON),
			},
			wantSource:  "kafka",
			wantBaseURL: "kafka://broker1:9092,broker2:9092",
		},
		{
			name: "rabbitmq",
			opts: []fuzz.Option{
				fuzz.WithRabbitMQEndpoint("amqp://localhost:5672/", "ex", "rk"),
				fuzz.WithSeed(fuzz.Seed("s", `{"x":1}`)),
				fuzz.WithMutator(fuzz.MutatorJSON),
			},
			wantSource:  "rabbitmq",
			wantBaseURL: "amqp://localhost:5672/",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tgt := fuzz.NewTarget(tc.name, tc.opts...)
			data, err := tgt.ToJSON()
			if err != nil {
				t.Fatalf("ToJSON: %v", err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatal(err)
			}
			if raw["sourceType"] != tc.wantSource {
				t.Errorf("sourceType=%v want %v", raw["sourceType"], tc.wantSource)
			}
			if raw["targetBaseUrl"] != tc.wantBaseURL {
				t.Errorf("targetBaseUrl=%v want %v", raw["targetBaseUrl"], tc.wantBaseURL)
			}
			// Confirm round-trip preserves protocol identity.
			round, err := fuzz.FromJSON(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := round.Protocol(); got != tgt.Protocol() {
				t.Errorf("round-trip protocol: %v -> %v", tgt.Protocol(), got)
			}
		})
	}
}

func TestSeedsHelpers(t *testing.T) {
	t.Parallel()
	s1 := fuzz.Seed("a", `{"k":"v"}`)
	if s1.ID == "" {
		t.Fatal("Seed should generate a non-empty ID")
	}
	s2 := fuzz.SeedBytes("b", []byte{0x00, 0x01, 0x02})
	if s2.Body != "\x00\x01\x02" {
		t.Errorf("SeedBytes body lost: %q", s2.Body)
	}
	s3 := fuzz.SeedHTTP("c", "POST", "/x", "body")
	if s3.Method != "POST" || s3.Path != "/x" {
		t.Errorf("SeedHTTP fields lost: %+v", s3)
	}
}

func TestSeedFileMissing(t *testing.T) {
	t.Parallel()
	_, err := fuzz.SeedFile("/non/existent/path.json")
	if err == nil {
		t.Fatal("expected error for missing seed file")
	}
}

func TestEmptySeedCorpusForPOSTFails(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("POST", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorJSON),
	)
	if err := tgt.Validate(); err == nil || !strings.Contains(err.Error(), "seed") {
		t.Fatalf("expected seed-corpus error, got %v", err)
	}
}

func TestUnicodeInSeedSurvivesRoundTrip(t *testing.T) {
	t.Parallel()
	payload := "{\"msg\":\"hello-привет\"}"
	tgt := fuzz.NewTarget("u",
		fuzz.WithHTTPEndpoint("POST", "https://x", "/"),
		fuzz.WithSeed(fuzz.Seed("unicode", payload)),
		fuzz.WithMutator(fuzz.MutatorJSON),
	)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	round, err := fuzz.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if round.Seeds()[0].Body != payload {
		t.Errorf("unicode payload lost: %q != %q", round.Seeds()[0].Body, payload)
	}
}

func TestVeryLongSeedSurvivesRoundTrip(t *testing.T) {
	t.Parallel()
	payload := "{\"msg\":\"hello-привет\"}"
	tgt := fuzz.NewTarget("long",
		fuzz.WithHTTPEndpoint("POST", "https://x", "/"),
		fuzz.WithSeed(fuzz.Seed("long", payload)),
		fuzz.WithMutator(fuzz.MutatorBytes),
	)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	round, err := fuzz.FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON: %v", err)
	}
	if len(round.Seeds()[0].Body) != len(payload) {
		t.Errorf("long body truncated: got %d want %d", len(round.Seeds()[0].Body), len(payload))
	}
}

func TestParallelBuildingRaceClean(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tgt := fuzz.NewTarget("p",
				fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
				fuzz.WithMutator(fuzz.MutatorURL),
			)
			if _, err := tgt.ToJSON(); err != nil {
				t.Errorf("[%d] ToJSON: %v", i, err)
			}
			_ = tgt.Validate()
		}(i)
	}
	wg.Wait()
}

func TestStopOnFindingWithDurationCoexist(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("t",
		fuzz.WithHTTPEndpoint("POST", "https://x", "/"),
		fuzz.WithSeed(fuzz.Seed("s", "{}")),
		fuzz.WithMutator(fuzz.MutatorJSON),
		fuzz.WithDuration(5*time.Minute),
		fuzz.WithStopOnFinding(true),
	)
	if err := tgt.Validate(); err != nil {
		t.Fatalf("duration + stop-on-finding should coexist: %v", err)
	}
}

func TestPayloadCategoriesAggregatedFromMutators(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("c",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
		fuzz.WithMutator(fuzz.MutatorString),
	)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	cats, _ := raw["payloadCategories"].([]any)
	if len(cats) == 0 {
		t.Fatal("expected payloadCategories to be populated from mutator presets")
	}
	// MutatorURL contributes path_traversal; MutatorString contributes naughty_strings.
	found := map[string]bool{}
	for _, c := range cats {
		found[c.(string)] = true
	}
	if !found["path_traversal"] || !found["naughty_strings"] {
		t.Errorf("expected derived categories, got %v", cats)
	}
}

func TestCoverageHintEmittedWhenSet(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("c",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
		fuzz.WithCoverageHint(fuzz.CoverageHint{
			ResponseStatusCodes: []int{200, 404},
			JSONPaths:           []string{"$.id"},
		}),
	)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if !strings.Contains(string(data), `"coverage"`) {
		t.Fatal("expected coverage block in JSON")
	}
}

func TestCoverageHintOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("c",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
	)
	data, err := tgt.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if strings.Contains(string(data), `"coverage"`) {
		t.Fatal("coverage block should be absent when empty")
	}
}

func TestNilOptionTolerated(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("n",
		nil,
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
	)
	if err := tgt.Validate(); err != nil {
		t.Fatalf("nil option must be skipped: %v", err)
	}
}

func TestFromJSONInvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := fuzz.FromJSON([]byte("not json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestWriteToCreatesParentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/nested/deep/target.json"
	tgt := fuzz.NewTarget("w",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
	)
	if err := tgt.WriteTo(path); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
}

func TestWriteToRejectsEmptyPath(t *testing.T) {
	t.Parallel()
	tgt := fuzz.NewTarget("w",
		fuzz.WithHTTPEndpoint("GET", "https://x", "/"),
		fuzz.WithMutator(fuzz.MutatorURL),
	)
	if err := tgt.WriteTo(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestProtocolValid(t *testing.T) {
	t.Parallel()
	for _, p := range []fuzz.Protocol{
		fuzz.ProtocolHTTP, fuzz.ProtocolGRPC, fuzz.ProtocolGraphQL,
		fuzz.ProtocolSOAP, fuzz.ProtocolWebSocket,
		fuzz.ProtocolKafka, fuzz.ProtocolRabbitMQ,
	} {
		if !p.Valid() {
			t.Errorf("%q must be Valid", p)
		}
	}
	if fuzz.Protocol("nope").Valid() {
		t.Error("unknown protocol must not be Valid")
	}
}

func TestReporterValid(t *testing.T) {
	t.Parallel()
	for _, r := range []fuzz.Reporter{
		fuzz.ReporterDefault, fuzz.ReporterAllure, fuzz.ReporterJUnit,
		fuzz.ReporterHTML, fuzz.ReporterJSON,
	} {
		if !r.Valid() {
			t.Errorf("%q must be Valid", r)
		}
	}
}

func TestStrategyValid(t *testing.T) {
	t.Parallel()
	for _, s := range []fuzz.Strategy{
		fuzz.StrategyMutation, fuzz.StrategySecurity, fuzz.StrategySchemaAware, fuzz.StrategyAll,
	} {
		if !s.Valid() {
			t.Errorf("%q must be Valid", s)
		}
	}
}
