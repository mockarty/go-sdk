// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockarty

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponseBuilder_Script(t *testing.T) {
	m := NewMockBuilder().
		ID("calc").
		HTTP(func(h *HTTPBuilder) { h.Route("/calc/:op").Method("POST") }).
		Response(func(r *ResponseBuilder) {
			r.Script(`response.json({ ok: true });`)
		}).
		Build()

	if m.Response == nil || m.Response.Script == nil {
		t.Fatal("script not set on response")
	}
	if !strings.Contains(m.Response.Script.Code, "response.json") {
		t.Fatalf("code = %q", m.Response.Script.Code)
	}
	if m.Response.Script.AllowNet {
		t.Fatal("AllowNet should default false")
	}

	b, err := json.Marshal(m.Response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"script"`) || !strings.Contains(string(b), `"code"`) {
		t.Fatalf("serialized response missing script: %s", b)
	}
	// AllowNet false must be omitted (omitempty).
	if strings.Contains(string(b), "allowNet") {
		t.Fatalf("allowNet should be omitted when false: %s", b)
	}
}

func TestResponseBuilder_ScriptWithNet(t *testing.T) {
	m := NewMockBuilder().
		ID("fetch").
		HTTP(func(h *HTTPBuilder) { h.Route("/fetch").Method("GET") }).
		Response(func(r *ResponseBuilder) { r.ScriptWithNet(`mk.http.send({url: env.get("U")});`) }).
		Build()
	if m.Response.Script == nil || !m.Response.Script.AllowNet {
		t.Fatalf("ScriptWithNet should set AllowNet: %+v", m.Response.Script)
	}
}
