// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package pact

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMessagePact_ToJSON_V4Shape(t *testing.T) {
	mp := NewMessagePact("OrderConsumer", "OrderEvents")
	mp.Given("user 42 exists").
		ExpectsToReceive("an order-created event").
		WithMetadata(map[string]string{"topic": "orders"}).
		WithContent(map[string]any{
			"orderId": 42,
			"status":  "open",
		})

	raw, err := mp.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if cn, _ := doc["consumer"].(map[string]any)["name"].(string); cn != "OrderConsumer" {
		t.Errorf("consumer name: %q", cn)
	}
	ix, ok := doc["interactions"].([]any)
	if !ok || len(ix) != 1 {
		t.Fatalf("expected 1 interaction, got %v", doc["interactions"])
	}
	first := ix[0].(map[string]any)
	if first["type"] != MessageInteractionType {
		t.Errorf("type = %q, want %q", first["type"], MessageInteractionType)
	}
	if first["description"] != "an order-created event" {
		t.Errorf("description = %v", first["description"])
	}
	contents, _ := first["contents"].(map[string]any)
	if contents["contentType"] != "application/json" {
		t.Errorf("contentType = %v", contents["contentType"])
	}
}

func TestMessagePact_RequiresConsumerProvider(t *testing.T) {
	mp := &MessagePact{specVersion: SpecV4}
	if _, err := mp.ToJSON(); err == nil {
		t.Fatal("expected error without consumer + provider")
	}
}

func TestMessagePact_V3Shape(t *testing.T) {
	mp := NewMessagePact("c", "p").WithSpecVersion(SpecV3)
	mp.Given("state").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"k": "v"})
	raw, err := mp.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	if _, ok := doc["messages"]; !ok {
		t.Errorf("V3 should emit 'messages' array")
	}
	if _, ok := doc["interactions"]; ok {
		t.Errorf("V3 should NOT emit 'interactions'")
	}
}

func TestMessagePact_VerifyHappyPath(t *testing.T) {
	mp := NewMessagePact("c", "p")
	mp.Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 7})

	var sawBytes []byte
	err := mp.Verify(func(content []byte, _ map[string]string) error {
		sawBytes = content
		return nil
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !strings.Contains(string(sawBytes), `"id"`) {
		t.Errorf("consumer did not see body: %q", string(sawBytes))
	}
}

func TestMessagePact_VerifyConsumerRejects(t *testing.T) {
	mp := NewMessagePact("c", "p")
	mp.Given("st").ExpectsToReceive("msg").WithContent(map[string]any{"id": 7})

	err := mp.Verify(func(_ []byte, _ map[string]string) error {
		return errors.New("cannot parse")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot parse") {
		t.Errorf("error did not propagate: %v", err)
	}
}

func TestMessagePact_VerifyNilHandler(t *testing.T) {
	mp := NewMessagePact("c", "p")
	if err := mp.Verify(nil); err == nil {
		t.Fatal("expected error on nil handler")
	}
}

func TestMessagePact_WriteFile(t *testing.T) {
	mp := NewMessagePact("OrderConsumer", "OrderEvents")
	mp.Given("st").ExpectsToReceive("msg").WithContent(map[string]any{"id": 1})

	dir := t.TempDir()
	path, err := mp.WriteFile(dir)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("file not under dir: %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(b), MessageInteractionType) {
		t.Errorf("written pact missing V4 message type")
	}
}

func TestParseMessagePactDoc_V4(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").
		Given("st").
		ExpectsToReceive("msg").
		WithMetadata(map[string]string{"k": "v"}).
		WithContent(map[string]any{"id": 1}).
		pact.ToJSON()
	msgs, err := parseMessagePactDoc(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if msgs[0].Description != "msg" {
		t.Errorf("description: %q", msgs[0].Description)
	}
	if msgs[0].Metadata["k"] != "v" {
		t.Errorf("metadata not parsed: %+v", msgs[0].Metadata)
	}
}

func TestParseMessagePactDoc_V3(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").WithSpecVersion(SpecV3).
		Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 1}).
		pact.ToJSON()
	msgs, err := parseMessagePactDoc(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1, got %d", len(msgs))
	}
	if msgs[0].Description != "msg" {
		t.Errorf("V3 description: %q", msgs[0].Description)
	}
	if msgs[0].States[0].Name != "st" {
		t.Errorf("V3 state: %+v", msgs[0].States)
	}
}

func TestVerifier_VerifyMessages_HappyPath(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").
		Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 7}).
		pact.ToJSON()

	v, _ := NewVerifier(
		WithProviderURL("http://x"),
		WithMessageProducer("msg", func(_ context.Context, _ string, _ []ProviderState) ([]byte, map[string]string, error) {
			return []byte(`{"id": 7}`), nil, nil
		}),
	)
	res, err := v.VerifyMessagePactBytes(context.Background(), raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected pass; got %+v", res.Interactions)
	}
}

func TestVerifier_VerifyMessages_ContentMismatch(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").
		Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 7}).
		pact.ToJSON()

	v, _ := NewVerifier(
		WithProviderURL("http://x"),
		WithMessageProducer("msg", func(_ context.Context, _ string, _ []ProviderState) ([]byte, map[string]string, error) {
			return []byte(`{"id": 999}`), nil, nil
		}),
	)
	res, _ := v.VerifyMessagePactBytes(context.Background(), raw)
	if res.OK() {
		t.Fatal("expected failure")
	}
	if len(res.Interactions[0].Mismatches) == 0 {
		t.Error("expected mismatches")
	}
}

func TestVerifier_VerifyMessages_NoProducer(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").
		Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 7}).
		pact.ToJSON()

	v, _ := NewVerifier(WithProviderURL("http://x"))
	res, _ := v.VerifyMessagePactBytes(context.Background(), raw)
	if res.OK() {
		t.Fatal("expected failure when no producer registered")
	}
	if !strings.Contains(res.Interactions[0].Error, "no MessageProducer") {
		t.Errorf("error: %q", res.Interactions[0].Error)
	}
}

func TestVerifier_VerifyMessages_ProducerErrors(t *testing.T) {
	raw, _ := NewMessagePact("c", "p").
		Given("st").
		ExpectsToReceive("msg").
		WithContent(map[string]any{"id": 7}).
		pact.ToJSON()

	v, _ := NewVerifier(
		WithProviderURL("http://x"),
		WithMessageProducer("msg", func(_ context.Context, _ string, _ []ProviderState) ([]byte, map[string]string, error) {
			return nil, nil, errors.New("kafka down")
		}),
	)
	res, _ := v.VerifyMessagePactBytes(context.Background(), raw)
	if !strings.Contains(res.Interactions[0].Error, "kafka down") {
		t.Errorf("producer error not propagated: %q", res.Interactions[0].Error)
	}
}

// TestMessagePact_V3SingularStateAlwaysEmitted regression: strict V3
// verifiers only read `providerState` (singular); we used to emit only
// the plural `providerStates` when len > 1, dropping the primary state
// for those verifiers. The serialiser now ALWAYS includes the singular
// field plus the plural for V3+ extensions.
func TestMessagePact_V3SingularStateAlwaysEmitted(t *testing.T) {
	mp := NewMessagePact("c", "p").WithSpecVersion(SpecV3)
	// Two states — pre-fix the singular field would be missing.
	mp.GivenWithParams("state-A", map[string]any{"k": "v"})
	mp.messages[0].States = append(mp.messages[0].States,
		ProviderState{Name: "state-B"})
	mp.messages[0].Description = "msg"
	mp.messages[0].Contents = map[string]any{"id": 1}

	raw, err := mp.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	msg := doc["messages"].([]any)[0].(map[string]any)
	if msg["providerState"] != "state-A" {
		t.Errorf("singular providerState = %v, want state-A", msg["providerState"])
	}
	if _, ok := msg["providerStates"].([]any); !ok {
		t.Errorf("plural providerStates should also be emitted for multi-state")
	}
}

// FuzzParseMessagePactDoc — never panic on adversarial bytes.
func FuzzParseMessagePactDoc(f *testing.F) {
	for _, s := range []string{
		``,
		`{}`,
		`{"interactions":[{"type":"Asynchronous/Messages","contents":{}}]}`,
		`{"messages":[{"description":""}]}`,
		`{"interactions":[null]}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseMessagePactDoc panicked: %v", r)
			}
		}()
		_, _ = parseMessagePactDoc(raw)
	})
}
