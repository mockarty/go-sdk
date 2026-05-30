// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: multi_protocol — showcase Tester DSL across protocols
// that don't need external infrastructure.
//
// kitchen_sink/ covers the canonical adoption shape — HTTP + GraphQL +
// upstream-tracker side-channels + TCM upload. This sibling example
// extends the chain to two more facets that testbackend serves
// natively: SSE for streaming events and SOAP for legacy-XML
// envelopes. Use this when you need to prove multi-protocol coverage
// in a single test job without spinning up Kafka / RabbitMQ.
//
// For Kafka / RabbitMQ / gRPC / DB facets, see the dedicated examples:
//   sdk/go-sdk/examples/kafka_client/
//   sdk/go-sdk/examples/rabbitmq_client/
//   sdk/go-sdk/examples/grpc_client/
// Those require docker-spun brokers / databases; this example is
// 100% standalone given a running testbackend.
//
// Run it:
//
//   mockarty-testbackend &              # 18770
//   TESTBACKEND_URL=http://127.0.0.1:18770 go run .

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/tester"
)

func main() {
	backend := envOr("TESTBACKEND_URL", "http://127.0.0.1:18770")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t := tester.New(
		tester.WithBaseURL(backend),
		tester.WithContext(ctx),
	)

	// ── HTTP — establish a baseline (also seeds {{token}} for later) ──
	t.HTTP().GET("/api/v1/token-chain/issue").
		ExpectStatus(200).
		Extract("$.token", "token")

	// ── GraphQL — typed query with variables ─────────────────────────
	t.GraphQL(backend+"/graphql").
		Query(`query GetUser($id: ID!) { user(id: $id) { name role } }`,
			map[string]any{"id": "user-2"}).
		ExpectStatus(200).
		ExpectNoErrors().
		ExpectField("$.data.user.role", "user")

	// ── SSE — subscribe to the notifications stream, assert event
	// count + the JSON-path of the first event's payload.
	// Listen for 2s so testbackend emits at least one event.
	t.SSE(backend+"/events/notifications").
		Subscribe().
		Listen(2 * time.Second).
		ExpectMinEvents(1)

	// ── SOAP — POST a SOAP envelope and assert the response status. ──
	// testbackend's /soap endpoint accepts a generic envelope and
	// echoes a fixed Body — useful for proving the SOAP transport
	// works end-to-end without standing up a real SOAP service.
	// testbackend recognises a fixed set of operations (GetUser,
	// CreateUser, ListUsers, …); see cmd/testbackend/handlers/soap.go.
	t.SOAP(backend+"/soap").
		Call("GetUser", `<GetUser><userId>user-1</userId></GetUser>`).
		ExpectStatus(200).
		ExpectNoFault()

	t.Finish()

	if !t.OK() {
		fmt.Fprintln(os.Stderr, "multi_protocol: failed steps:")
		for _, e := range t.Errors() {
			fmt.Fprintln(os.Stderr, "  -", e)
		}
		os.Exit(1)
	}
	fmt.Println("multi_protocol: ok — http+graphql+sse+soap")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
