// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"encoding/json"
	"testing"

	"github.com/mockarty/mockarty-go/pact"
)

// TestSyncMessagePact verifies the consumer DSL emits a V4
// Synchronous/Messages interaction: contents = the request, response[] = the
// expected replies (each with its own contents + matchingRules).
func TestSyncMessagePact(t *testing.T) {
	mp := pact.NewMessagePact("rpc-consumer", "rpc-provider")
	mp.Given("a user exists").
		ExpectsToReceive("a get-user request/response").
		WithContent(map[string]any{"op": "getUser", "id": pact.Integer(7)}).
		ExpectsResponse(map[string]any{"id": pact.Integer(7), "name": pact.Like("Alice")}).
		WithResponseMetadata(map[string]string{"status": "ok"})

	raw, err := mp.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ix := doc["interactions"].([]any)[0].(map[string]any)
	if ix["type"] != "Synchronous/Messages" {
		t.Fatalf("type = %v, want Synchronous/Messages", ix["type"])
	}
	// Request lives under contents.
	if _, ok := ix["contents"].(map[string]any); !ok {
		t.Fatalf("expected request contents, got %v", ix["contents"])
	}
	// Replies live under response[].
	resp, ok := ix["response"].([]any)
	if !ok || len(resp) != 1 {
		t.Fatalf("expected 1 response, got %v", ix["response"])
	}
	r0 := resp[0].(map[string]any)
	if _, ok := r0["contents"].(map[string]any); !ok {
		t.Errorf("response[0] missing contents: %v", r0)
	}
	if md, _ := r0["metadata"].(map[string]any); md["status"] != "ok" {
		t.Errorf("response metadata not carried: %v", r0["metadata"])
	}
	// The Like/Integer matchers in the reply must have produced matchingRules.
	if _, ok := r0["matchingRules"]; !ok {
		t.Errorf("expected response matchingRules from the matchers, got none")
	}
}

// TestAsyncMessageStillAsync guards that an interaction WITHOUT ExpectsResponse
// still serialises as Asynchronous/Messages (no regression).
func TestAsyncMessageStillAsync(t *testing.T) {
	mp := pact.NewMessagePact("c", "p")
	mp.Given("s").ExpectsToReceive("evt").WithContent(map[string]any{"a": 1})
	raw, _ := mp.ToJSON()
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	ix := doc["interactions"].([]any)[0].(map[string]any)
	if ix["type"] != "Asynchronous/Messages" {
		t.Fatalf("type = %v, want Asynchronous/Messages", ix["type"])
	}
	if _, ok := ix["response"]; ok {
		t.Errorf("async message must not carry a response array")
	}
}
