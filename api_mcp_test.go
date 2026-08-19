// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mcpTestServer emulates the admin's streamable-HTTP /mcp endpoint. It records
// the API key seen and can reply in JSON or SSE mode.
func mcpTestServer(t *testing.T, sse bool, wantKey string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if wantKey != "" && r.Header.Get("X-API-Key") != wantKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req jsonrpcRequest
		_ = json.Unmarshal(body, &req)

		// Notifications (no id) get a 202 with no body.
		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result any
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "sess-123")
			result = map[string]any{"protocolVersion": "2025-03-26", "serverInfo": map[string]any{"name": "mockarty"}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{
				{"name": "list_mocks", "description": "List mocks"},
				{"name": "create_mock", "description": "Create a mock"},
			}}
		case "tools/call":
			var p struct {
				Name string `json:"name"`
			}
			pb, _ := json.Marshal(req.Params)
			_ = json.Unmarshal(pb, &p)
			result = map[string]any{"content": []map[string]any{
				{"type": "text", "text": `{"tool":"` + p.Name + `","ok":true}`},
			}}
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
			return
		}

		frame := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}
		fb, _ := json.Marshal(frame)
		if sse {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("event: message\ndata: " + string(fb) + "\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fb)
	}))
}

func TestMCP_ListAndCall_JSON(t *testing.T) {
	srv := mcpTestServer(t, false, "mk_key")
	defer srv.Close()

	c := NewClient(srv.URL, WithAPIKey("mk_key"))
	tools, err := c.MCP().ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) != 2 || tools[0].Name != "list_mocks" {
		t.Fatalf("unexpected tools: %+v", tools)
	}
	res, err := c.MCP().CallTool(context.Background(), "create_mock", map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(res.Text(), `"tool":"create_mock"`) {
		t.Fatalf("unexpected result text: %q", res.Text())
	}
}

func TestMCP_ListAndCall_SSE(t *testing.T) {
	srv := mcpTestServer(t, true, "")
	defer srv.Close()

	c := NewClient(srv.URL)
	tools, err := c.MCP().ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools (SSE): %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools over SSE, got %d", len(tools))
	}
	res, err := c.MCP().CallTool(context.Background(), "list_mocks", nil)
	if err != nil {
		t.Fatalf("CallTool (SSE): %v", err)
	}
	if res.Text() == "" {
		t.Fatal("empty SSE tool result")
	}
}

func TestMCP_SessionIDEchoed(t *testing.T) {
	var sawSession bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonrpcRequest
		_ = json.Unmarshal(body, &req)
		if req.Method == "tools/list" && r.Header.Get("Mcp-Session-Id") == "sess-xyz" {
			sawSession = true
		}
		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Mcp-Session-Id", "sess-xyz")
		w.Header().Set("Content-Type", "application/json")
		var result any = map[string]any{}
		if req.Method == "tools/list" {
			result = map[string]any{"tools": []any{}}
		}
		fb, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		_, _ = w.Write(fb)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.MCP().ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if !sawSession {
		t.Fatal("session id from initialize was not echoed on the follow-up request")
	}
}

func TestMCP_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req jsonrpcRequest
		_ = json.Unmarshal(body, &req)
		if req.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "initialize" {
			fb, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
			_, _ = w.Write(fb)
			return
		}
		fb, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID,
			"error": map[string]any{"code": -32601, "message": "method not found"}})
		_, _ = w.Write(fb)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.MCP().CallTool(context.Background(), "nope", nil)
	if err == nil || !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("expected rpc error surfaced, got %v", err)
	}
}
