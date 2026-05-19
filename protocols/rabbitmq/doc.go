// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Package rabbitmq is the Mockarty Go SDK's RabbitMQ test client. It
// is built for CI/CD test scripts that exercise an AMQP 0.9.1
// producer/consumer (real or Mockarty-mocked) and stream the per-call
// timeline into a TCM external run.
//
// # Surface
//
//   - NewClient(url, opts...)               — dial a broker
//   - cli.Publish(ctx, exchange, key, p)    — fire-and-forget / confirm
//   - cli.Consume(ctx, queue, opts)         — pull N messages
//   - cli.DeclareQueue(ctx, name, opts)     — create / mirror a queue
//
// Every call records a telemetry.Step (passed/failed/broken) so the
// configured externalruns.Client shows a per-publish-and-consume
// timeline in the TCM run.
//
// # Air-gapped friendly
//
// Built on github.com/rabbitmq/amqp091-go — pure Go, no CGO. Same
// binary runs against Mockarty-fronted RabbitMQ mocks in distroless.
//
// # Out of scope
//
// Admin surface (UserCreate, PermissionSet, FederationConfig, …) is
// not exposed — the owner-rule for mockarty-go is "expose only the
// surface useful from CI/CD scripts and tests".
package rabbitmq
