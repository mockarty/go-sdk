// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
)

func TestNopRecorder_NeverPanics(t *testing.T) {
	var r NopRecorder
	r.Record(context.Background(), Step{Key: "k", Name: "n"})
}

func TestExternalRunsRecorder_NilClient_DropsSilently(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	defer r.Close()
	r.Record(context.Background(), Step{Key: "k", Name: "n"})
}

func TestExternalRunsRecorder_CloseIdempotent(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	r.Close()
	r.Close()
}

func TestExternalRunsRecorder_RecordAfterCloseNoops(t *testing.T) {
	r := NewExternalRunsRecorder(nil, "")
	r.Close()
	r.Record(context.Background(), Step{Key: "k", Name: "n"})
}

func TestToExternalStep_StatusDefaultsPassed(t *testing.T) {
	out := ToExternalStep(Step{Key: "k", Name: "n"})
	if out.Status != externalruns.StatusPassed {
		t.Fatalf("status = %q, want passed", out.Status)
	}
}

func TestToExternalStep_EndedAtDerivedFromDuration(t *testing.T) {
	start := time.Now()
	out := ToExternalStep(Step{
		StartedAt: start,
		Duration:  150 * time.Millisecond,
		Key:       "k",
	})
	want := start.Add(150 * time.Millisecond)
	if out.FinishedAt == nil || !out.FinishedAt.Equal(want) {
		t.Fatalf("FinishedAt = %v, want %v", out.FinishedAt, want)
	}
}

func TestToExternalStep_FieldMappingComplete(t *testing.T) {
	start := time.Now()
	end := start.Add(50 * time.Millisecond)
	out := ToExternalStep(Step{
		StartedAt:  start,
		EndedAt:    end,
		Parameters: map[string]string{"k": "v"},
		Key:        "key-1",
		Name:       "topic/op",
		Status:     "failed",
		Message:    "bad",
		Duration:   50 * time.Millisecond,
	})
	if out.StepKey != "key-1" || out.Name != "topic/op" {
		t.Fatalf("name/key wrong: %+v", out)
	}
	if out.Status != externalruns.StatusFailed {
		t.Fatalf("status = %q", out.Status)
	}
	if out.Message != "bad" {
		t.Fatalf("message = %q", out.Message)
	}
	if out.DurationMS != 50 {
		t.Fatalf("DurationMS = %d, want 50", out.DurationMS)
	}
	if out.Parameters["k"] != "v" {
		t.Fatalf("parameters not preserved")
	}
}

func TestNewStepKey_StableShape(t *testing.T) {
	if got := NewStepKey("topic/op", 42); got != "topic/op#42" {
		t.Fatalf("NewStepKey = %q", got)
	}
}
