# `fuzz/` — Mockarty Go SDK Fuzz DSL

Language-side, fluent description of a Mockarty fuzz campaign.
The package builds an in-memory `Target`, transpiles it to canonical
Mockarty fuzz-config JSON, and hands the JSON to either the admin
server (`Runner.Submit`) or the `mockarty-cli` subprocess
(`Runner.LocalSpawn`).

The SDK is intentionally THIN: it does **not** embed a fuzz engine.
The Mockarty server / CLI runs the actual campaign — this package is
the Go front door to the same JSON config the UI / REST API / file-on-
disk paths also produce.

## Quick start

```go
import (
    "context"
    "time"

    "github.com/mockarty/mockarty-go/fuzz"
)

target := fuzz.NewTarget("login-stress",
    fuzz.WithDescription("Stress-test the login endpoint"),
    fuzz.WithHTTPEndpoint("POST", "https://api.example.com", "/api/v1/login"),
    fuzz.WithSeedCorpus(
        fuzz.Seed("valid",      `{"username":"admin","password":"secret"}`),
        fuzz.Seed("missing-pw", `{"username":"admin"}`),
    ),
    fuzz.WithMutator(fuzz.MutatorJSON),
    fuzz.WithDuration(5 * time.Minute),
    fuzz.WithStopOnFinding(true),
    fuzz.WithReporter(fuzz.ReporterAllure),
    fuzz.WithAssertion(fuzz.AssertNoCrash()),
    fuzz.WithAssertion(fuzz.AssertResponseTimeUnder(5 * time.Second)),
)

runner := fuzz.NewRunner("https://mockarty.example.com", "default", "tok-abc")
job, err := runner.Submit(context.Background(), target)
// ...
res, err := runner.Wait(context.Background(), job.ID)
```

## Surface table

| Axis | Values |
|------|--------|
| Protocols | `WithHTTPEndpoint`, `WithGRPCEndpoint`, `WithGraphQLEndpoint`, `WithSOAPEndpoint`, `WithWebSocketEndpoint`, `WithKafkaEndpoint`, `WithRabbitMQEndpoint` |
| Mutators (built-in) | `MutatorJSON`, `MutatorXML`, `MutatorBytes`, `MutatorString`, `MutatorURL`, `MutatorHeader`, `MutatorGRPC`, `MutatorGraphQL` |
| Mutators (custom) | `WithCustomMutator(name, jsConfig)` — opaque JS blob the server-side engine evaluates |
| Assertions | `AssertStatus`, `AssertStatusClass`, `AssertNoCrash`, `AssertResponseTimeUnder`, `AssertNoErrorInBody`, `AssertBodyContains` |
| Strategies | `StrategyMutation` (default), `StrategySecurity`, `StrategySchemaAware`, `StrategyAll` |
| Reporters | `ReporterDefault`, `ReporterAllure`, `ReporterJUnit`, `ReporterHTML`, `ReporterJSON` |
| Seeds | `Seed(name, body)`, `SeedBytes`, `SeedFile`, `SeedHTTP`, `WithSeedCorpus`, `WithSeed` |
| Coverage hints | `WithCoverageHint`, `WithExpectedStatus`, `WithAssertedJSONPath` |
| Stop conditions | `WithDuration`, `WithMaxRequests`, `WithStopOnFinding`, `WithConcurrency`, `WithMaxRPS` |

## Execution paths

| Path | When to use | API |
|------|-------------|-----|
| `Runner.Submit` + `Runner.Wait` | Long-running admin server, central reports | `runner.Submit(ctx, target)` |
| `Runner.Stream` | Live progress / findings | `runner.Stream(ctx, jobID)` returns `<-chan Event` |
| `Runner.LocalSpawn` | Offline / dev iteration | `runner.LocalSpawn(ctx, target)` (invokes `mockarty-cli fuzz run`) |
| `Target.WriteTo` | Check JSON into the repo for CI | `target.WriteTo("target.json")` then `mockarty-cli fuzz run target.json` |
| `Target.ToJSON` | Hand-off / inspection | `target.ToJSON()` returns canonical config bytes |
| `FromJSON` | Round-trip / round-tripping CI artefacts back into Go | `fuzz.FromJSON(data)` |

## Custom mutator example

```go
const jsTransform = `function mutate(payload) { return payload.toUpperCase(); }`

target := fuzz.NewTarget("uppercase-everything",
    fuzz.WithHTTPEndpoint("POST", "https://api.example.com", "/x"),
    fuzz.WithSeed(fuzz.Seed("base", `{"hello":"world"}`)),
    fuzz.WithCustomMutator("uppercase", jsTransform),
)
```

Custom mutators bypass the SDK's local validation — the server is the
source of truth for whether the JS payload is acceptable.

## Schema parity

The emitted JSON is a strict superset of the server-side
`fuzzing.FuzzConfig` type (`internal/fuzzing/config.go`):

* `targetBaseUrl`, `sourceType`, `strategy`, `seedRequests`, `options`,
  `payloadCategories` — server-readable today.
* `protocol`, `assertions`, `coverage`, `mutators`, `reporter`, `tags`,
  `sdk` — SDK-side annotations; server treats unknown fields as opaque
  metadata (forward-compatible).

## Phase 2 follow-ups

* **Distributed fuzz across runner pool** — sharding the seed corpus
  across multiple runners with cross-shard finding dedup.
* **Dictionary-based mutators** — AFL-style keyword corpora with
  per-target dictionary upload.
* **Streaming Allure step emission** — wire the `<-chan Event` SSE
  feed through the `allure/` sibling package so each new finding
  becomes an Allure step in real time.
* **Recorder / contract import helpers** — `fuzz.FromRecorder(session)`,
  `fuzz.FromOpenAPI(spec)`, `fuzz.FromPact(pactJSON)` — currently the
  existing `api_fuzzing.go` REST surface covers this server-side.
