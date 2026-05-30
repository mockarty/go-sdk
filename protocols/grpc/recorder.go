// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package grpc

import (
	"github.com/mockarty/mockarty-go/externalruns"
	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

// Step is re-exported from protocols/telemetry. The protocol-agnostic
// type lives there because the same shape is used by every
// mockarty-go protocol client (gRPC, Kafka, RabbitMQ, …) — kept here
// as an alias so existing call sites (mgrpc.Step{...}) keep working.
type Step = telemetry.Step

// StepRecorder is re-exported from protocols/telemetry. Implement
// telemetry.StepRecorder directly when wiring a custom sink — the
// alias is purely for package ergonomics.
type StepRecorder = telemetry.StepRecorder

// ExternalRunsRecorder is re-exported for the same reason. Use either
// import; both name the same concrete type.
type ExternalRunsRecorder = telemetry.ExternalRunsRecorder

// NewExternalRunsRecorder wires a recorder that streams steps into an
// externalruns.Client. See telemetry.NewExternalRunsRecorder for
// semantics — this is a thin re-export.
func NewExternalRunsRecorder(runs *externalruns.Client, runID string) *ExternalRunsRecorder {
	return telemetry.NewExternalRunsRecorder(runs, runID)
}

// NewStepKey is re-exported. Identical to telemetry.NewStepKey;
// kept under mgrpc.NewStepKey so the gRPC client's tests can build
// keys without dragging the telemetry import.
func NewStepKey(method string, seq uint64) string {
	return telemetry.NewStepKey(method, seq)
}

// nopRecorder is the internal zero-value used by defaultConfig when no
// WithRecorder option is supplied. Re-uses the canonical telemetry
// implementation so a future tweak (instrumentation, debug logging)
// only needs to land in one place.
type nopRecorder = telemetry.NopRecorder

// stepKey is the unexported alias used inside InvokeJSON so the
// existing call sites keep compiling.
func stepKey(method string, seq uint64) string {
	return telemetry.NewStepKey(method, seq)
}

// toExternalStep is the unexported alias used by client_test.go. The
// real conversion lives in telemetry.ToExternalStep.
func toExternalStep(s Step) externalruns.Step {
	return telemetry.ToExternalStep(s)
}
