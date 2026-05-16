# `pact` — Pact V3 + V4 consumer DSL for Mockarty Go SDK

This package is a pure-Go consumer-side DSL that emits Pact `pact.json`
contract files in either the V3 or V4 specification. It contains no
Rust FFI, no `libpact_ffi`, and no CGO — every test pulls in just the
standard library plus `github.com/google/uuid` (already a transitive
dep of the parent SDK).

## Quick start

```go
package payments_test

import (
    "bytes"
    "context"
    "encoding/json"
    "io"
    "net/http"
    "testing"

    "github.com/mockarty/mockarty-go/pact"
)

func TestPaymentClient(t *testing.T) {
    p := pact.NewConsumer("OrderService",
        pact.WithProvider("PaymentService"),
        pact.WithSpecVersion(pact.SpecV4),
        pact.WithOutputDir(t.TempDir()),
    )

    p.AddInteraction().
        Given("payment service is up").
        UponReceiving("a charge request").
        WithRequest(http.MethodPost, "/charge").
        WithHeader("Content-Type", "application/json").
        WithJSONBody(map[string]any{"amount": pact.Like(100)}).
        WillRespondWith(200).
        WithHeader("Content-Type", "application/json").
        WithJSONBody(map[string]any{"id": pact.Like("abc")})

    srv, err := p.Start(context.Background())
    if err != nil {
        t.Fatal(err)
    }
    defer srv.Close()

    // Point your real client at srv.URL() and exercise the request.
    body, _ := json.Marshal(map[string]any{"amount": 999})
    resp, _ := http.Post(srv.URL()+"/charge", "application/json", bytes.NewReader(body))
    defer resp.Body.Close()
    out, _ := io.ReadAll(resp.Body)
    t.Logf("got %d %s", resp.StatusCode, out)

    if err := srv.Verify(); err != nil {
        t.Fatal(err)
    }
}
```

When the test ends, `srv.Close()` writes
`<OutputDir>/OrderService-PaymentService.json` ready to be uploaded
to a Pact broker or fed into a provider verification run.

## V3 vs V4

Pass `pact.WithSpecVersion(pact.SpecV3)` or `pact.WithSpecVersion(pact.SpecV4)`
to pick a dialect. The default is V4.

| Aspect              | V3 (`SpecV3` = `"3.0.0"`)             | V4 (`SpecV4` = `"4.0"`)                   |
|---------------------|---------------------------------------|-------------------------------------------|
| Provider states     | Single `providerState`                | Plural `providerStates[]` with `params`   |
| Interaction `type`  | absent                                | `Synchronous/HTTP` (Phase 1)              |
| matchingRules shape | Flat (`"$.body.id": {matchers:[...]}`) | Nested by category (`body`/`header`/...)  |
| Plugins             | Not emitted (silently dropped)        | Recorded in `metadata.plugins` (no-op)    |
| Async/message       | Not supported in Phase 1              | Not supported in Phase 1                  |

The same DSL works in both modes; the writer produces the right shape
on serialisation. Both reference fixtures used in the schema round-trip
tests live in `testdata/`.

## Matcher reference

| Function                              | Pact name        | V3 | V4 | Notes                                                            |
|---------------------------------------|------------------|----|----|------------------------------------------------------------------|
| `pact.Like(example)`                  | `type`           | x  | x  | Type-only match; the canonical default                           |
| `pact.MatchType(example)`             | `type` (V4)      |    | x  | V4-styled alias of `Like`                                        |
| `pact.Term(example, regex)`           | `regex`          | x  | x  | Regex match on string values                                     |
| `pact.Regex(example, regex)`          | `regex`          | x  | x  | Alias of `Term` for parity with pact-jvm / pact-js               |
| `pact.Integer(example)`               | `integer`        | x  | x  |                                                                  |
| `pact.Decimal(example)`               | `decimal`        | x  | x  |                                                                  |
| `pact.Boolean(example)`               | `boolean`        | x  | x  |                                                                  |
| `pact.EachLike(example, min)`         | `type` + `min`   | x  | x  | Wraps example in `[example]`                                     |
| `pact.MinType(example, min)`          | `type` + `min`   | x  | x  | Like EachLike, no child Like                                     |
| `pact.MaxType(example, max)`          | `type` + `max`   | x  | x  |                                                                  |
| `pact.MinMaxType(example, min, max)`  | `type` + bounds  | x  | x  |                                                                  |
| `pact.EachKeyLike(example)`           | `values`         |    | x  | V3: degrades to `type`                                            |
| `pact.EachKey(matcher)`               | `each-key`       |    | x  |                                                                  |
| `pact.EachValue(matcher)`             | `each-value`     |    | x  |                                                                  |
| `pact.ArrayContains(variants...)`     | `arrayContains`  |    | x  |                                                                  |
| `pact.Equality(example)`              | `equality`       |    | x  | V3: verifiers default to equality so still safe                  |

The mock server is intentionally **permissive** when matching incoming
requests against a declared matcher (it only checks the JSON type,
not the regex / equality assertions). Strict assertions live on the
provider side: the resulting `pact.json` is the contract, and the
provider verifier (Mockarty admin, Pact CLI, etc.) is what tightens
the screws.

## Provider verification

This package does NOT verify the contract against a real provider —
that is the provider team's responsibility. Two production paths:

1. **Mockarty admin** — POST your generated `pact.json` to the
   contract-broker endpoint. Mockarty's `internal/contract/pact_matcher.go`
   runs the verification side and writes a `PactVerificationResult`
   back. This is the integrated path: same UI, same dashboards, same
   RBAC.

2. **Pact CLI** — every standard Pact tool reads the same JSON. Point
   `pact-cli verify` at the file, or upload to any pact broker
   (pactflow.io, your own self-hosted broker, etc.). The contract is
   spec-compliant; no Mockarty-specific extension is required for
   verification.

## What this package is NOT

- **Not a long-lived mock server.** The `MockServer` returned by
  `Start` dies with the test process. It's a per-test consumer mock,
  not a multi-tenant fake-backend service. That product lives in
  Mockarty admin (WireMock-compatible) and `mockarty-cli`.
- **Not a verifier.** No reverse direction: replaying a `pact.json`
  against a live provider is out of scope here. See "Provider
  verification" above.
- **Not a plugin runtime.** `WithPlugin` records metadata for V4
  round-trip fidelity only; the SDK does not load gRPC plugins or
  speak HTTP/2 in Phase 1.

## Phase 2 follow-ups

The Wave 2 plan (`docs/research/SDK_FRAMEWORK_PLAN.md` §3) flags the
following items for a subsequent phase:

- **V4 plugin runtime** — gRPC plugin client + HTTP/2 transport so
  protobuf / gRPC contracts can be served by the consumer mock.
- **Asynchronous and Synchronous/Messages interactions** —
  `Asynchronous/Messages` (V3 + V4) and `Synchronous/Messages` (V4
  only) for event-driven contracts.
- **Provider-side verification client** — a Go entry-point that
  consumes the `pact.json` this package writes and replays it
  against a live provider, mirroring the standard Pact `verify`
  command without Rust FFI.
- **Contract publication** — direct POST to a Pact broker (
  pactflow / Mockarty admin) without writing a temporary file.
