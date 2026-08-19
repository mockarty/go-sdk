// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// MCPAPI is a ready-to-use Model Context Protocol (MCP) client that speaks to a
// Mockarty MCP endpoint (the admin node's streamable-HTTP `/mcp`). It lets SDK
// users discover and call the same agent-facing tool surface an AI agent would,
// programmatically — list the tools the server exposes, then call them with
// typed arguments and read structured results. Auth reuses the SDK client's
// API key; feature/licence gating is enforced server-side.
//
// The client is safe for concurrent use. It performs the MCP `initialize`
// handshake lazily on first use and reuses the negotiated session for the rest
// of its lifetime.
type MCPAPI struct {
	client   *Client
	endpoint string

	mu          sync.Mutex
	initialized bool
	sessionID   string
	nextID      int64
}

// MCPTool describes one tool advertised by the MCP server.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// MCPContent is one content block of a tool result. Type is usually "text".
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPToolResult is the structured result of a tools/call.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// Text returns the concatenated text of every text content block — the common
// case for a Mockarty tool that returns a JSON string.
func (r *MCPToolResult) Text() string {
	var b strings.Builder
	for _, c := range r.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

// MCP returns the MCP client bound to this SDK client's server + credentials.
func (c *Client) MCP() *MCPAPI {
	if c.mcpAPI == nil {
		c.mcpAPI = &MCPAPI{
			client:   c,
			endpoint: strings.TrimRight(c.baseURL, "/") + "/mcp",
		}
	}
	return c.mcpAPI
}

// jsonrpcRequest is a single JSON-RPC 2.0 request frame.
type jsonrpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// jsonrpcResponse is a single JSON-RPC 2.0 response frame.
type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("mcp: rpc error %d: %s", e.Code, e.Message)
}

// Initialize performs the MCP handshake explicitly. It is called automatically
// by ListTools / CallTool, so most callers never need it — use it to fail fast
// on an unreachable server or a bad token.
func (m *MCPAPI) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureInit(ctx)
}

// ensureInit runs the initialize handshake once. Caller must hold m.mu.
func (m *MCPAPI) ensureInit(ctx context.Context) error {
	if m.initialized {
		return nil
	}
	params := map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mockarty-go-sdk", "version": "1.0.0"},
	}
	if _, err := m.call(ctx, "initialize", params); err != nil {
		return fmt.Errorf("mcp: handshake with %s failed (check the token): %w", m.endpoint, err)
	}
	// Per the MCP spec, send the initialized notification after handshake.
	_ = m.notify(ctx, "notifications/initialized", nil)
	m.initialized = true
	return nil
}

// ListTools returns every tool the MCP server advertises.
func (m *MCPAPI) ListTools(ctx context.Context) ([]MCPTool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureInit(ctx); err != nil {
		return nil, err
	}
	raw, err := m.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []MCPTool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/list: %w", err)
	}
	return out.Tools, nil
}

// CallTool invokes a tool by name with the given arguments and returns its
// structured result. args may be nil for a no-argument tool.
func (m *MCPAPI) CallTool(ctx context.Context, name string, args map[string]any) (*MCPToolResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ensureInit(ctx); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	raw, err := m.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return nil, err
	}
	var res MCPToolResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("mcp: decode tools/call result for %q: %w", name, err)
	}
	return &res, nil
}

// call sends a JSON-RPC request expecting a response and returns its Result.
// Caller must hold m.mu.
func (m *MCPAPI) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	m.nextID++
	reqBody := jsonrpcRequest{JSONRPC: "2.0", ID: m.nextID, Method: method, Params: params}
	respFrame, err := m.roundtrip(ctx, reqBody, true)
	if err != nil {
		return nil, err
	}
	if respFrame.Error != nil {
		return nil, respFrame.Error
	}
	return respFrame.Result, nil
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (m *MCPAPI) notify(ctx context.Context, method string, params any) error {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}
	_, err := m.roundtripRaw(ctx, body, false)
	return err
}

// roundtrip marshals a typed request, POSTs it, and parses one JSON-RPC frame.
func (m *MCPAPI) roundtrip(ctx context.Context, req jsonrpcRequest, wantResponse bool) (*jsonrpcResponse, error) {
	data, err := m.roundtripRaw(ctx, req, wantResponse)
	if err != nil || !wantResponse {
		return nil, err
	}
	var frame jsonrpcResponse
	if err := json.Unmarshal(data, &frame); err != nil {
		return nil, fmt.Errorf("mcp: decode response frame: %w (body: %s)", err, truncate(string(data), 200))
	}
	return &frame, nil
}

// roundtripRaw POSTs the given body to the MCP endpoint and, when wantResponse
// is true, returns the JSON-RPC response bytes. It transparently handles both
// direct application/json responses and text/event-stream (SSE) framing, and
// captures/echoes the Mcp-Session-Id negotiated at initialize time.
func (m *MCPAPI) roundtripRaw(ctx context.Context, body any, wantResponse bool) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if m.client.apiKey != "" {
		httpReq.Header.Set(headerAPIKey, m.client.apiKey)
	}
	if m.client.namespace != "" {
		httpReq.Header.Set("X-Mockarty-Namespace", m.client.namespace)
	}
	if m.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", m.sessionID)
	}

	resp, err := m.client.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mcp: cannot reach %s: %w", m.endpoint, err)
	}
	defer resp.Body.Close()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		m.sessionID = sid
	}
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("mcp: HTTP %d from %s: %s", resp.StatusCode, m.endpoint, strings.TrimSpace(string(snippet)))
	}
	if !wantResponse {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, nil
	}

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return readSSEResponse(resp.Body)
	}
	return io.ReadAll(resp.Body)
}

// readSSEResponse extracts the first JSON-RPC frame carried in an SSE stream —
// the streamable-HTTP transport wraps the response in `data:` lines.
func readSSEResponse(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var data strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data.WriteString(strings.TrimSpace(line[len("data:"):]))
			continue
		}
		if line == "" && data.Len() > 0 {
			return []byte(data.String()), nil // end of one SSE event
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mcp: read SSE stream: %w", err)
	}
	if data.Len() == 0 {
		return nil, fmt.Errorf("mcp: empty SSE response")
	}
	return []byte(data.String()), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
