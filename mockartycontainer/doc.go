// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package mockartycontainer is a thin testcontainers-go wrapper around
// the `mockarty/cli:latest-mock` image. It is the drop-in replacement
// for WireMockContainer / MockServer / MockoonContainer in user
// integration tests.
//
// The package is the Go side of Mockarty's Wave 4 SDK strategy (see
// docs/research/SDK_FRAMEWORK_PLAN.md rev 3 §6.4). Like the other Wave
// 3/4 packages it is intentionally THIN:
//
//   - It does NOT embed a mock engine. The container itself runs the
//     real `mockarty-cli mock serve` process baked into the image.
//   - It DOES manage the docker lifecycle (pull, start, wait for
//     /health, stop, clean up) via testcontainers-go.
//   - It DOES expose two complementary URLs — WireMock-compat (8080)
//     and Mockarty native (also 8080, separate path namespace) — so
//     existing WireMock test bodies and Mockarty-native test bodies
//     can both target the same running container.
//
// Quick start:
//
//	c, err := mockartycontainer.New(ctx,
//	    mockartycontainer.WithImage("mockarty/cli:latest-mock"),
//	    mockartycontainer.WithFormat("auto"),
//	    mockartycontainer.WithStubFile("./testdata/stubs.json"),
//	)
//	if err != nil { t.Fatal(err) }
//	defer c.Stop(ctx)
//	resp, _ := http.Get(c.URL() + "/api/v1/orders/42")
//
// Docker is a hard prerequisite. Tests that import this package MUST
// skip (not fail) when no docker daemon is reachable so the wider
// SDK test suite stays green on CI shards without docker.
package mockartycontainer
