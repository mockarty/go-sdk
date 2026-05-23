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

// CapPreview boundaries — ASCII + UTF-8 multi-byte rune handling.
// The review (B4) flagged a half-truncated rune in the previous
// implementation; these tests pin the fix.

func TestCapPreview_EmptyCapReturnsEmpty(t *testing.T) {
	if got := CapPreview([]byte("hello"), 0); got != "" {
		t.Fatalf("cap=0 should return empty, got %q", got)
	}
}

func TestCapPreview_EmptyBodyReturnsEmpty(t *testing.T) {
	if got := CapPreview(nil, 10); got != "" {
		t.Fatalf("nil body should return empty, got %q", got)
	}
	if got := CapPreview([]byte{}, 10); got != "" {
		t.Fatalf("empty body should return empty, got %q", got)
	}
}

func TestCapPreview_CapLargerThanBody(t *testing.T) {
	if got := CapPreview([]byte("hello"), 100); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
}

func TestCapPreview_AsciiTruncationMarker(t *testing.T) {
	body := []byte("0123456789ABCDEF") // 16 bytes
	got := CapPreview(body, 5)
	want := "01234…(truncated 11B)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCapPreview_UTF8RuneBoundaryRoundsDown(t *testing.T) {
	// "Привет" = "П"(D0 9F) "р"(D1 80) "и"(D0 B8) "в"(D0 B2)
	// "е"(D0 B5) "т"(D1 82). Each Cyrillic char is 2 bytes in
	// UTF-8 → "Привет" total length = 12 bytes.
	// Cutting at cap=5 lands BETWEEN the bytes of "и" (offset 5
	// is the trailing B8 byte of "и") — the buggy old slice
	// would have produced "Пр\xd0" + invalid rune. The fixed
	// CapPreview rounds DOWN to the last rune start (offset 4)
	// → "Пр" preserved + truncated marker.
	body := []byte("Привет")
	if len(body) != 12 {
		t.Fatalf("test data sanity: len(Привет) = %d, want 12", len(body))
	}
	got := CapPreview(body, 5)
	// Verify the prefix is a valid UTF-8 run (no replacement char).
	for i, r := range got {
		if r == 0xFFFD {
			t.Fatalf("CapPreview emitted replacement char at byte %d in %q", i, got)
		}
		if i > 4 { // stop after the marker prefix
			break
		}
	}
	// The preserved prefix is "Пр" (4 bytes), followed by the marker.
	want := "Пр…(truncated 8B)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCapPreview_UTF8CutAlreadyOnRuneStartKeepsFullPrefix(t *testing.T) {
	// cap=4 lands exactly between "р" and "и" — already a rune
	// boundary. The fix must NOT round down further; the prefix
	// stays "Пр".
	body := []byte("Привет")
	got := CapPreview(body, 4)
	want := "Пр…(truncated 8B)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCapPreviewString_Equivalence(t *testing.T) {
	// String overload returns the same shape — easy round-trip
	// guarantee for the protocol clients that hand strings in.
	if got, want := CapPreviewString("Привет", 5), "Пр…(truncated 8B)"; got != want {
		t.Fatalf("string overload got %q, want %q", got, want)
	}
	if got := CapPreviewString("", 10); got != "" {
		t.Fatalf("empty string should return empty, got %q", got)
	}
}
