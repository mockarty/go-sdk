// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Package telemetry is the shared step-capture vocabulary used by every
// mockarty-go protocol client (gRPC, Kafka, RabbitMQ, SOAP, GraphQL,
// SSE, WebSocket). It defines:
//
//   - Step: the protocol-agnostic record of one captured operation.
//   - StepRecorder: the narrow seam a protocol client emits Steps through.
//   - NopRecorder: the zero-cost default for tests that don't care.
//   - ExternalRunsRecorder: the production adapter — buffers steps and
//     flushes them into a mockarty-go externalruns.Client so a CI job's
//     TCM external run shows a per-call timeline.
//
// Why a separate package?
//
// Three sibling protocol packages need the same recorder shape AND the
// same externalruns adapter. Duplicating ~150 LOC × 3 is worse than one
// 200 LOC package they all import. The package has zero dependencies
// outside mockarty-go/externalruns + the Go stdlib, so importing it
// from a protocol client does not pull in unrelated transports.
package telemetry

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/mockarty/mockarty-go/externalruns"
)

// Step is the protocol-agnostic record produced for each captured
// operation. It maps 1:1 to externalruns.Step; keeping a separate
// type means users plugging in their own sinks (logs, Honeycomb,
// custom dashboards) don't have to take an externalruns dependency.
type Step struct {
	StartedAt time.Time
	// EndedAt is StartedAt + Duration; kept separately so callers
	// don't have to recompute when fanning out to multiple sinks.
	EndedAt    time.Time
	Parameters map[string]string
	// Key is a stable identifier for the step. Use NewStepKey to
	// build it so retries server-side de-dup as intended.
	Key string
	// Name is the human-readable label shown in the TCM run. Typical
	// values: gRPC full method ("acme.UserService/GetUser"), Kafka
	// topic ("publish:orders"), RabbitMQ routing key, etc.
	Name string
	// Status — "passed" / "failed" / "broken" / "skipped". Empty
	// defaults to "passed" when forwarded to externalruns.
	Status string
	// Message is the error message (or empty on success).
	Message string
	// Duration is EndedAt-StartedAt, pre-computed so the recorder
	// doesn't have to repeat the subtraction.
	Duration time.Duration
}

// StepRecorder is the narrow seam through which a protocol client
// reports per-operation steps. Implementations MUST be safe for
// concurrent use — every protocol client in this SDK can fire
// multiple operations in parallel from a single test.
type StepRecorder interface {
	// Record stores one completed step. Called once per
	// invocation after the operation returns (or fails). The
	// implementation should not block — callers typically buffer
	// + flush in the background.
	Record(ctx context.Context, step Step)
}

// NopRecorder is the zero-value default — drops every step on the
// floor. Protocol clients pick it up when no recorder is configured
// so test code that doesn't want TCM step capture works out of the
// box without extra wiring.
type NopRecorder struct{}

// Record implements StepRecorder.
func (NopRecorder) Record(context.Context, Step) {}

// ExternalRunsRecorder feeds captured steps into an
// externalruns.Client. One recorder is bound to ONE runID —
// typically the run the test job already opened via Client.CreateRun
// before exercising the service under test.
//
// Steps are buffered and flushed when the batch reaches 32 entries
// OR the 250 ms flush ticker fires. The recorder owns a background
// goroutine that drains the channel; call Close to flush + stop it.
type ExternalRunsRecorder struct {
	runs   *externalruns.Client
	runID  string
	ch     chan Step
	done   chan struct{}
	closed atomic.Bool
}

// NewExternalRunsRecorder wires a recorder that streams steps into
// the supplied externalruns.Client under runID. Nil client / empty
// runID yield a recorder whose Record no-ops (silently — the test
// code can still call Record from inside a non-CI run without
// crashing).
func NewExternalRunsRecorder(runs *externalruns.Client, runID string) *ExternalRunsRecorder {
	r := &ExternalRunsRecorder{
		runs:  runs,
		runID: runID,
		ch:    make(chan Step, 64),
		done:  make(chan struct{}),
	}
	go r.loop()
	return r
}

// Record enqueues a step. Non-blocking on the happy path; falls back
// to a synchronous flush when the buffer is full so a long-running
// suite can't lose steps to channel back-pressure.
func (r *ExternalRunsRecorder) Record(ctx context.Context, s Step) {
	if r == nil || r.runs == nil || r.runID == "" || r.closed.Load() {
		return
	}
	select {
	case r.ch <- s:
	default:
		// Buffer full — drain one synchronously.
		r.flushOne(ctx, s)
	}
}

// Close drains pending steps and stops the background goroutine.
// Safe to call multiple times — second + subsequent calls no-op.
func (r *ExternalRunsRecorder) Close() {
	if r == nil || !r.closed.CompareAndSwap(false, true) {
		return
	}
	close(r.ch)
	<-r.done
}

func (r *ExternalRunsRecorder) loop() {
	defer close(r.done)
	const batchSize = 32
	const flushInterval = 250 * time.Millisecond
	buf := make([]externalruns.Step, 0, batchSize)
	tick := time.NewTicker(flushInterval)
	defer tick.Stop()
	flush := func() {
		if len(buf) == 0 || r.runs == nil {
			return
		}
		_ = r.runs.AddSteps(context.Background(), r.runID, buf)
		buf = buf[:0]
	}
	for {
		select {
		case s, ok := <-r.ch:
			if !ok {
				flush()
				return
			}
			buf = append(buf, ToExternalStep(s))
			if len(buf) >= batchSize {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

func (r *ExternalRunsRecorder) flushOne(ctx context.Context, s Step) {
	if r.runs == nil {
		return
	}
	_ = r.runs.AddSteps(ctx, r.runID, []externalruns.Step{ToExternalStep(s)})
}

// ToExternalStep converts a telemetry.Step into an externalruns.Step
// suitable for AddSteps. Exposed for tests + custom recorders that
// want to ship steps through their own externalruns.Client batches.
func ToExternalStep(s Step) externalruns.Step {
	status := externalruns.Status(s.Status)
	if status == "" {
		status = externalruns.StatusPassed
	}
	ended := s.EndedAt
	if ended.IsZero() && !s.StartedAt.IsZero() {
		ended = s.StartedAt.Add(s.Duration)
	}
	return externalruns.Step{
		StartedAt:  s.StartedAt,
		FinishedAt: &ended,
		Parameters: s.Parameters,
		StepKey:    s.Key,
		Name:       s.Name,
		Status:     status,
		Message:    s.Message,
		DurationMS: s.Duration.Milliseconds(),
	}
}

// NewStepKey builds a stable per-call key from a method/topic name +
// a monotonic counter. Protocol clients use this so retries of the
// same operation de-dup server-side on (namespace, run, step_key).
func NewStepKey(name string, seq uint64) string {
	return fmt.Sprintf("%s#%d", name, seq)
}

// CapPreview truncates `body` to at most `cap` bytes, rounding the
// cut DOWN to a valid UTF-8 rune boundary so the returned string is
// never half-a-rune. When truncation happens, the literal marker
// `…(truncated <N>B)` (U+2026 ellipsis) is appended. The marker
// shape is identical across every mockarty-go protocol client, the
// Python SDK, and the Java SDK — change it in lock-step or break
// cross-language step diffs.
//
// `cap == 0` returns the empty string (disables capture).
func CapPreview(body []byte, cap int) string {
	if cap == 0 || len(body) == 0 {
		return ""
	}
	if len(body) <= cap {
		return string(body)
	}
	// Round down to a valid rune boundary so we never emit a half-
	// truncated multi-byte sequence (`u8.RuneError`).
	cut := cap
	for cut > 0 && !utf8.RuneStart(body[cut]) {
		cut--
	}
	if cut == 0 {
		// Defensive: body[0] is always a rune start, so this branch
		// only fires when the first byte was invalid UTF-8. Fall
		// back to the byte cap to avoid losing the preview entirely.
		cut = cap
	}
	return string(body[:cut]) + "…(truncated " + strconv.Itoa(len(body)-cut) + "B)"
}

// CapPreviewString is a convenience for the string overload — saves
// a `[]byte(s)` allocation in the common case.
func CapPreviewString(s string, cap int) string {
	if cap == 0 || s == "" {
		return ""
	}
	if len(s) <= cap {
		return s
	}
	return CapPreview([]byte(s), cap)
}
