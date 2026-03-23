# mockarty-go

Official Go SDK for [Mockarty](https://mockarty.com) — a powerful mocking platform for HTTP, gRPC, MCP, GraphQL, SOAP, SSE, WebSocket, Kafka, RabbitMQ, and SMTP services.

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
versions, err := client.Mocks().Versions(ctx, "chain-id")
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
err := client.Stores().GlobalSet(ctx, "counter", 42)
err := client.Stores().GlobalDelete(ctx, "key1", "key2")

// Chain store
store, err := client.Stores().ChainGet(ctx, "chain-id")
err := client.Stores().ChainSet(ctx, "chain-id", "status", "completed")
err := client.Stores().ChainDelete(ctx, "chain-id", "key")
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

MIT License - see [LICENSE](LICENSE) for details.
