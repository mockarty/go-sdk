<p align="center">
  <img src="https://raw.githubusercontent.com/mockarty/mockarty-go/main/logo.svg" alt="Mockarty" width="400">
</p>

<h1 align="center">Go SDK</h1>

<p align="center">
  Official Go client library for <a href="https://mockarty.ru">Mockarty</a> — a multi-protocol mock server for HTTP, gRPC, MCP, GraphQL, SOAP, SSE, WebSocket, Kafka, RabbitMQ, and SMTP.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/mockarty/mockarty-go"><img src="https://pkg.go.dev/badge/github.com/mockarty/mockarty-go.svg" alt="Go Reference"></a>
  <a href="https://github.com/mockarty/mockarty-go/blob/main/LICENSE"><img src="https://img.shields.io/github/license/mockarty/mockarty-go" alt="License"></a>
</p>

## Installation

```bash
go get github.com/mockarty/mockarty-go
```

**Requirements:** Go 1.21+ | Zero external dependencies (stdlib only)

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    mockarty "github.com/mockarty/mockarty-go"
)

func main() {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey("mk_your_api_key"),
        mockarty.WithNamespace("sandbox"),
    )

    // Create a mock using the fluent builder
    mock := mockarty.NewMockBuilder().
        ID("get-user").
        HTTP(func(h *mockarty.HTTPBuilder) {
            h.Route("/api/users/:id").
              Method("GET")
        }).
        Response(func(r *mockarty.ResponseBuilder) {
            r.Status(200).
              Header("Content-Type", "application/json").
              JSONBody(map[string]any{
                  "id":    "$.pathParam.id",
                  "name":  "$.fake.FirstName",
                  "email": "$.fake.Email",
              })
        }).
        Build()

    resp, err := client.Mocks().Create(context.Background(), mock)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created mock: %s (overwritten: %v)\n", resp.Mock.ID, resp.Overwritten)
}
```

## Client Configuration

```go
client := mockarty.NewClient("http://localhost:5770",
    mockarty.WithAPIKey("mk_..."),           // API key authentication
    mockarty.WithNamespace("production"),     // Default namespace (default: "sandbox")
    mockarty.WithTimeout(10 * time.Second),  // HTTP timeout (default: 30s)
    mockarty.WithRetry(3, time.Second),      // Retry config with exponential back-off
    mockarty.WithHTTPClient(customClient),   // Custom HTTP client
    mockarty.WithLogger(slog.Default()),     // Custom structured logger
)
```

## API Reference

### Mocks

```go
// CRUD
resp, err := client.Mocks().Create(ctx, mock)
mock, err := client.Mocks().Get(ctx, "mock-id")
list, err := client.Mocks().List(ctx, &mockarty.ListMocksOptions{
    Namespace: "production",
    Tags:      []string{"users"},
    Limit:     20,
})
updated, err := client.Mocks().Update(ctx, "mock-id", mock)
err := client.Mocks().Delete(ctx, "mock-id")
err := client.Mocks().Restore(ctx, "mock-id")
err := client.Mocks().Purge(ctx, "mock-id")

// Batch operations
err := client.Mocks().BatchCreate(ctx, mocks)
err := client.Mocks().BatchDelete(ctx, ids)
err := client.Mocks().BatchRestore(ctx, ids)

// Logs and versions
logs, err := client.Mocks().Logs(ctx, "mock-id", &mockarty.LogsOptions{Limit: 50})
versions, err := client.Mocks().GetChain(ctx, "chain-id")
```

### Namespaces

```go
err := client.Namespaces().Create(ctx, "production")
namespaces, err := client.Namespaces().List(ctx)
```

### Stores

```go
// Global store
store, err := client.Stores().GlobalGet(ctx)
err := client.Stores().GlobalSet(ctx, "counter", "42")
err := client.Stores().GlobalDelete(ctx, "key1")
err := client.Stores().GlobalDeleteMany(ctx, "key1", "key2") // multiple keys

// Chain store
store, err := client.Stores().ChainGet(ctx, "chain-id")
err := client.Stores().ChainSet(ctx, "chain-id", "status", "completed")
err := client.Stores().ChainDelete(ctx, "chain-id", "key")
err := client.Stores().ChainDeleteMany(ctx, "chain-id", "key1", "key2") // multiple keys
```

### Health

```go
health, err := client.Health().Check(ctx)
err := client.Health().Live(ctx)
err := client.Health().Ready(ctx)
```

### Collections & Performance

```go
collections, err := client.Collections().List(ctx)
result, err := client.Collections().Execute(ctx, "collection-id")

task, err := client.Perf().Run(ctx, &mockarty.PerfConfig{...})
err := client.Perf().Stop(ctx, "task-id")
results, err := client.Perf().Results(ctx)
```

## Mock Builder

The fluent builder supports all Mockarty protocols:

```go
// HTTP mock
mock := mockarty.NewMockBuilder().
    ID("user-api").
    HTTP(func(h *mockarty.HTTPBuilder) {
        h.Route("/api/users/:id").
          Method("GET").
          HeaderCondition("Authorization", mockarty.AssertNotEmpty, nil)
    }).
    Response(func(r *mockarty.ResponseBuilder) {
        r.Status(200).JSONBody(map[string]any{"name": "$.fake.FirstName"})
    }).
    Build()

// gRPC mock
mock := mockarty.NewMockBuilder().
    ID("grpc-user").
    GRPC(func(g *mockarty.GRPCBuilder) {
        g.Service("UserService").Method("GetUser")
    }).
    Response(func(r *mockarty.ResponseBuilder) {
        r.JSONBody(map[string]any{"name": "John"})
    }).
    Build()

// OneOf responses (random or sequential)
mock := mockarty.NewMockBuilder().
    ID("flaky-service").
    HTTP(func(h *mockarty.HTTPBuilder) {
        h.Route("/api/data").Method("GET")
    }).
    OneOfConfig(mockarty.OneOfOrderRandom,
        func(r *mockarty.ResponseBuilder) { r.Status(200).JSONBody("ok") },
        func(r *mockarty.ResponseBuilder) { r.Status(500).Error("boom") },
    ).
    Build()

// Proxy mock
mock := mockarty.NewMockBuilder().
    ID("proxy").
    HTTP(func(h *mockarty.HTTPBuilder) {
        h.Route("/api/external").Method("GET")
    }).
    ProxyTo("https://real-api.example.com").
    Build()
```

## Protocol Clients

Drive the system under test directly from CI scripts. Each protocol client
captures every call as a TCM step (start/end/duration/status/payload
preview) so the external run shows a per-call timeline at the end:

- `protocols/grpc`     — JSON-shaped gRPC client with reflection / `.proto` file source
- `protocols/kafka`    — Produce / Consume on segmentio/kafka-go (pure Go)
- `protocols/rabbitmq` — Publish / Consume / DeclareQueue on amqp091-go (pure Go)
- `protocols/telemetry` — shared `Step` / `StepRecorder` / `ExternalRunsRecorder`

```go
import (
    "github.com/mockarty/mockarty-go/externalruns"
    mgrpc "github.com/mockarty/mockarty-go/protocols/grpc"
    "github.com/mockarty/mockarty-go/protocols/telemetry"
)

runs, _ := externalruns.NewClient(adminURL, "sandbox", apiToken)
run, _ := runs.CreateRun(ctx, externalruns.CreateRunRequest{Name: "smoke", Framework: "go-test"})
defer runs.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{})

rec := telemetry.NewExternalRunsRecorder(runs, run.ID); defer rec.Close()
conn, _ := mgrpc.Dial(ctx, "service:50051", mgrpc.WithRecorder(rec))
defer conn.Close()
var resp map[string]any
_ = conn.InvokeJSON(ctx, "acme.UserService/GetUser", map[string]any{"id": "u-42"}, &resp)
```

Full cross-language reference (Go / Python / Java side-by-side, every
protocol, options, classification rules, troubleshooting):
**[SDK Protocol Clients](https://mockarty.ru/docs/sdk-protocol-clients)**.

## Testing Helpers

```go
func TestUserAPI(t *testing.T) {
    client := mockarty.NewClient("http://localhost:5770",
        mockarty.WithAPIKey("mk_test_key"),
    )

    // Auto-cleanup on test end
    client.SetupNamespaceT(t, "test-ns")

    mock := client.CreateMockT(t, mockarty.NewMockBuilder().
        ID("test-user-get").
        Namespace("test-ns").
        HTTP(func(h *mockarty.HTTPBuilder) {
            h.Route("/api/users/1").Method("GET")
        }).
        Response(func(r *mockarty.ResponseBuilder) {
            r.Status(200).JSONBody(map[string]any{"id": "1", "name": "Test"})
        }).
        Build(),
    )
    // mock is auto-deleted when test ends
    _ = mock
}
```

## Fluent Tester DSL

For end-to-end tests that exercise multiple protocols, the
`tester` sub-package provides a fluent chain shaped after
JUnit + RestAssured + k6 — but driving any of Mockarty's nine
supported transports:

```go
import (
    "github.com/mockarty/mockarty-go/tester"
    "github.com/mockarty/mockarty-go/protocols/kafka"
)

func TestUserSignupFlow(t *testing.T) {
    tt := tester.New(tester.WithBaseURL("http://localhost:8080"))
    defer tt.Finish()

    tt.HTTP().POST("/signup").
        JSON(map[string]any{"email": "a@b.c"}).
        ExpectStatus(201).
        Extract("$.token", "token")

    tt.HTTP().GET("/me").
        Header("Authorization", "Bearer {{token}}").
        ExpectStatus(200).
        ExpectJSONPath("$.email", "a@b.c")

    kfk, _ := kafka.NewClient([]string{"localhost:9092"})
    tt.Kafka(kfk).Consume("user.signups").
        Max(1).
        ExpectMessageContains(0, "a@b.c")

    if !tt.OK() {
        t.Fatalf("%v", tt.Errors())
    }
}
```

Facets shipped: `HTTP()`, `Kafka(broker)`, `GRPC(client)`,
`GraphQL(endpoint)`, `RabbitMQ(broker)`, `SSE(endpoint)`,
`WebSocket(url)`, `SOAP(endpoint)`, `DB(conn)`. Each chain emits
one Allure step automatically (wrap the test with
`allure.WithTest(ctx, "...")` and the result file lands in
`$ALLURE_RESULTS_DIR`). Group calls with `.Wrap("name", fn)`,
retry with `.Eventually(within, interval, fn)`, fan-out with
`.Parallel(branchA, branchB)`.

See [`tester/doc.go`](./tester/doc.go) for the full vocabulary.

Upload a Tester chain as a TCM external run in one call:

```go
import (
    mockarty "github.com/mockarty/mockarty-go"
    "github.com/mockarty/mockarty-go/tester"
)

t := tester.New(tester.WithBaseURL("http://localhost:8080"))
t.HTTP().GET("/me").ExpectStatus(200)
t.Finish()

client := mockarty.NewClient("http://...", mockarty.WithAPIKey("..."), mockarty.WithNamespace("qa"))
_, err := client.ExternalRuns().Report(ctx, "",
    t.ToExternalRun(tester.ExternalRunOptions{
        CaseName:   "me-endpoint",
        AutoCreate: true,
    }),
)
```

`Tester.ToExternalRun(opts)` maps Tester report to
`ExternalRunRequest`: per-step `Protocol/Method/URL/StatusOrCode` go
into `Metadata`, multi-failure errors join with `"; "`, run
duration computed from first/last step timestamps. Same vocabulary
as the Python (`tester.to_report_kwargs`) and Java
(`ExternalRunBridge`) SDKs.

## Examples

The [`examples/`](./examples/) directory has 30+ runnable programs
covering every facet of the SDK. The two most useful starting points
for adopters:

| Example | What it shows |
|---------|---------------|
| [`kitchen_sink/`](./examples/kitchen_sink/) | Full adopter showcase — token-chain → GraphQL → all `Expect*` assertions → `Wrap` grouping → Jira issue create → GitLab pipeline poll → TCM upload. Runs against the testbackend; ~1 s end-to-end. |
| [`multi_protocol/`](./examples/multi_protocol/) | Tester DSL chain across HTTP + GraphQL + SSE + SOAP in one job, zero external infrastructure. Proves the DSL really works over event streams and XML envelopes. |

For protocol-specific examples that need real brokers, see
[`kafka_client/`](./examples/kafka_client/),
[`rabbitmq_client/`](./examples/rabbitmq_client/),
[`grpc_client/`](./examples/grpc_client/) — each ships a
`docker-compose.yml`.

For reporting integration: [`ci_cd_pipeline/`](./examples/ci_cd_pipeline/)
shows JUnit + Allure upload from a CI step;
[`agent_tasks/`](./examples/agent_tasks/) shows how the Tester DSL
emits TCM external runs.

## Test Container

For tests that need a fresh, isolated mock server per package, the
`mockartycontainer` sub-package spawns the `mockarty/cli:latest-mock`
Docker image via testcontainers-go. Drop-in replacement for
`wiremock-testcontainers`. See [SDK Test Container](https://mockarty.ru/docs/sdk-testcontainer)
and the [`examples/testcontainer_mockarty/`](./examples/testcontainer_mockarty/)
example.

## Error Handling

```go
import "errors"

_, err := client.Mocks().Get(ctx, "nonexistent")
if errors.Is(err, mockarty.ErrNotFound) {
    // handle 404
}

var apiErr *mockarty.APIError
if errors.As(err, &apiErr) {
    fmt.Printf("Status: %d, Message: %s\n", apiErr.StatusCode, apiErr.Message)
}
```

## License

This SDK is proprietary software, **not** open source. It is licensed under the
**Mockarty SDK License Agreement** — see [LICENSE](LICENSE) for the full terms.

- **Free** for evaluation, learning, and non-commercial / community use.
- **Commercial use requires a valid, paid Mockarty subscription.** Using this
  SDK in production or for commercial advantage without a subscription is not
  permitted.

For commercial subscriptions and licensing inquiries, see
[mockarty.ru](https://mockarty.ru) or contact orlovich.artem@gmail.com.
