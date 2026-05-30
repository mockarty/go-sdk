// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Package kafka is the Mockarty Go SDK's Kafka test client. It is built
// for CI/CD test scripts that exercise a Kafka producer/consumer (real
// or Mockarty-mocked) and stream the per-call timeline into a TCM
// external run.
//
// # What it gives you
//
//  1. **Produce / Consume with step capture.** Each Produce and each
//     Consume call is timed, classified (passed/failed/broken), and
//     reported to a telemetry.StepRecorder. The default adapter
//     ships steps into a mockarty-go externalruns.Client, so the
//     CI run's TCM external run already shows a per-call timeline
//     with topic / partition / offset / payload preview.
//
//  2. **Air-gapped friendly.** Built on segmentio/kafka-go — pure Go,
//     no librdkafka, no CGO. The same binary runs in a distroless
//     container against MinIO-fronted Mockarty mocks.
//
//  3. **JSON-shaped payloads.** Convenience helpers marshal/unmarshal
//     map[string]any / structs through encoding/json so test code
//     doesn't have to drag a separate codec in.
//
// # What it is NOT
//
// This is a TESTING client. It is not designed for high-throughput
// production pipelines — every call records a step, and the buffered
// recorder will back-pressure under heavy concurrency. Don't ship it
// inside a service binary.
//
// Admin / topic-management surface (DescribeCluster, AlterConfig, ACLs)
// is out of scope — the owner-rule for mockarty-go is "expose only the
// surface useful from CI/CD scripts and tests". Create topics for
// test setup; everything else lives in admin tooling.
package kafka
