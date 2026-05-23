// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Package grpc is the Mockarty Go SDK's gRPC test client. It is built for
// the use case the rest of mockarty-go is built for: writing CI/CD test
// scripts that exercise a gRPC service (real or Mockarty-mocked) and
// stream the per-call timeline back into Mockarty as a TCM external run.
//
// # What it gives you
//
// Three things that are otherwise tedious to assemble in test code:
//
//  1. **JSON-shaped Invoke.** You name the fully-qualified gRPC method
//     ("acme.UserService/GetUser") and hand it a JSON request body. The
//     client looks up the method descriptor (via server reflection or
//     a .proto file you point it at), marshals JSON → protobuf, sends
//     the RPC, and marshals the reply back to JSON for you. No
//     hand-generated stubs, no protoc step in your test repo.
//
//  2. **Auto-step capture.** Every call is timed and reported to a
//     StepRecorder you wire in. The default adapter ships steps into
//     a mockarty-go externalruns.Client, so when a CI job runs your
//     tests the TCM external run already shows a per-RPC timeline
//     with status / duration / error message — same UX you'd get
//     from allure-pytest's @step decorator.
//
//  3. **Discoverable surface.** Reflection-based listing of the
//     services / methods the server exposes. Useful for sanity checks
//     ("is the mock actually wired to MethodX?") and for generating a
//     human-readable manifest of what your test plan touches.
//
// # What it is NOT
//
// This is a TESTING client, not a generated stub library. It uses
// reflection for both message descriptors and for codec routing, which
// is great for test ergonomics and terrible for hot-path production
// traffic. Don't ship it inside a service binary; do ship it inside
// your test job.
//
// Client-streaming and bidirectional-streaming RPCs are out of scope in
// v1 because (a) they double the API surface and (b) the CI test cases
// that need them are rare. Unary and server-streaming are supported —
// the latter exposes a stream you iterate, with each message recorded
// as a sub-step under the parent RPC step.
//
// # Quick start
//
//	conn, _ := mgrpc.Dial(ctx, "localhost:50051",
//	    mgrpc.WithRecorder(mgrpc.NewExternalRunsRecorder(runs, runID)),
//	)
//	defer conn.Close()
//
//	var resp map[string]any
//	err := conn.InvokeJSON(ctx,
//	    "acme.UserService/GetUser",
//	    map[string]any{"id": "u-42"},
//	    &resp,
//	)
//
// # Package boundary
//
// This subpackage lives under sdk/go-sdk/protocols/grpc so future
// protocol clients (Kafka, RabbitMQ, SOAP, GraphQL, SSE, WebSocket)
// can mount alongside without bloating the root mockarty-go import
// graph. Each protocol package is independent — pulling in
// `protocols/grpc` does not drag in `protocols/kafka`, and vice
// versa, so a script that only mocks HTTP doesn't pay the gRPC
// dependency cost.
package grpc
