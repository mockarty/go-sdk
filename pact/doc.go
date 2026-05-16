// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package pact is a pure-Go consumer DSL that emits Pact V3 and V4
// contract files (.json) for the Mockarty platform and any other Pact-
// compatible broker / provider.
//
// The package is the Go side of Mockarty's Wave 2 SDK strategy (see
// docs/research/SDK_FRAMEWORK_PLAN.md §3 Pact compat). It is intentionally
// FFI-free: there is no libpact_ffi, no CGO, no Rust runtime. The DSL is
// implemented in pure Go on top of stdlib JSON and net/http/httptest so
// the SDK stays cross-compilable and embeddable in any test binary.
//
// # Design constraints
//
// SDK is a THIN layer — this package only:
//
//   - Builds a Pact contract in memory via a fluent builder.
//   - Spins up an ephemeral httptest.Server bound to the consumer's
//     declared interactions so the user's HTTP client can be pointed at it
//     during `go test`.
//   - Serialises the accumulated contract to a V3- or V4-shaped pact.json
//     file when the mock server is closed.
//
// It never:
//
//   - Talks to a Mockarty admin server.
//   - Performs provider-side verification. (Verification is the provider's
//     responsibility and runs server-side via Mockarty's
//     `internal/contract/pact_matcher.go` OR via the standard Pact CLI
//     tools.)
//   - Embeds a long-lived multi-tenant mock server. (That is the CLI /
//     admin's job — WireMock-compatible product surface.)
//
// # Spec coverage
//
// Both V3 (https://github.com/pact-foundation/pact-specification/tree/version-3)
// and V4 (https://github.com/pact-foundation/pact-specification/tree/version-4)
// are first-class. The user picks one with [WithSpecVersion]:
//
//   - V3 emits flat `$.body.X` matching-rule paths, a single
//     `providerState`, and the legacy `matchingRules` block.
//   - V4 emits nested category paths (`body`, `header`, `query`, `path`),
//     plural `providerStates`, the rich `matchingRules` block with typed
//     matcher entries, and an interaction `type` of
//     `Synchronous/HTTP` (HTTP/sync is the supported interaction type
//     today; async messaging is on the follow-up backlog).
//
// V4 plugin runtime is wired through the `pact/plugins` subpackage:
// import `_ "github.com/mockarty/mockarty-go/pact/plugins/protobuf"`
// (or `.../grpc`) to enable runtime payload validation for the
// `application/x-protobuf` / `application/grpc` content types.
// Third-party plugins implement `pact/plugins.Plugin` and call
// `plugins.Register` from their init().
//
// # Quick start
//
//	func TestPayment(t *testing.T) {
//	    p := pact.NewConsumer("OrderService",
//	        pact.WithProvider("PaymentService"),
//	        pact.WithSpecVersion(pact.SpecV4),
//	        pact.WithOutputDir(t.TempDir()),
//	    )
//	    p.AddInteraction().
//	        Given("payment service is up").
//	        UponReceiving("a charge request").
//	        WithRequest(http.MethodPost, "/charge").
//	        WithHeader("Content-Type", "application/json").
//	        WithJSONBody(map[string]any{"amount": pact.Like(100)}).
//	        WillRespondWith(200).
//	        WithJSONBody(map[string]any{"id": pact.Like("abc")})
//
//	    srv, err := p.Start(context.Background())
//	    if err != nil { t.Fatal(err) }
//	    defer srv.Close()
//
//	    // ...point your real client at srv.URL() and exercise the request...
//
//	    if err := srv.Verify(); err != nil { t.Fatal(err) }
//	}
//
// # Follow-up backlog
//
//   - Real HTTP/2 gRPC transport (current runtime is payload-level only).
//   - Asynchronous and Synchronous/Messages interaction types.
//   - Provider-side verification client (consumes the pact.json this
//     package emits and replays it against a live provider).
package pact
