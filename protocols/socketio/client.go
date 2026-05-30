// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Package socketio is the Mockarty Go SDK's minimal Socket.IO v4
// (Engine.IO v4) test client over the WebSocket transport. It lets
// CI/CD test scripts connect to a Socket.IO server (a Mockarty mock or a
// real server), connect a namespace, emit events, and collect inbound
// events for assertion — distinct from a raw WebSocket because it speaks
// the Engine.IO/Socket.IO framing (handshake, ping/pong, packet types).
//
// # Transport
//
// WebSocket only (the SDK connects with EIO=4&transport=websocket).
// HTTP long-polling is intentionally not implemented — air-gapped CI
// uses WS, and a single transport keeps the client small.
//
// # Air-gapped friendly
//
// Built on github.com/gorilla/websocket (already a mockarty-go dep) —
// pure Go, no CGO.
//
// # Out of scope
//
// Binary attachments (Socket.IO packet type 5/6), acknowledgement
// callbacks, and the polling transport are NOT implemented — the
// owner-rule for mockarty-go is "expose only the surface useful from
// CI/CD scripts and tests".
package socketio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Engine.IO packet type prefixes.
const (
	eioOpen    = '0'
	eioClose   = '1'
	eioPing    = '2'
	eioPong    = '3'
	eioMessage = '4'
)

// Socket.IO packet type prefixes (carried inside an eioMessage).
const (
	sioConnect      = '0'
	sioConnectError = '4'
	sioEvent        = '2'
)

// Event is one inbound Socket.IO event.
type Event struct {
	ReceivedAt time.Time
	Name       string
	Namespace  string
	// Args holds the event arguments after the name (raw JSON each).
	Args []json.RawMessage
}

// Client is a Socket.IO connection. Not safe for concurrent emit from
// multiple goroutines — one Client per test goroutine.
type Client struct {
	conn       *websocket.Conn
	handshake  map[string]any
	mu         sync.Mutex
	inbound    []Event
	connectErr string
	connected  bool
	closed     bool
}

// Option configures the dial.
type Option func(*dialOptions)

type dialOptions struct {
	header  http.Header
	timeout time.Duration
}

// WithHeader adds a handshake header (e.g. Authorization).
func WithHeader(k, v string) Option {
	return func(d *dialOptions) {
		if d.header == nil {
			d.header = http.Header{}
		}
		d.header.Set(k, v)
	}
}

// WithDialTimeout overrides the dial timeout (default 10s).
func WithDialTimeout(t time.Duration) Option {
	return func(d *dialOptions) {
		if t > 0 {
			d.timeout = t
		}
	}
}

// Dial connects to a Socket.IO server. url may be ws://host or
// http://host (auto-rewritten); the /socket.io/ path and
// EIO=4&transport=websocket query are appended automatically when
// absent. It reads the Engine.IO open handshake before returning.
func Dial(ctx context.Context, url string, opts ...Option) (*Client, error) {
	do := &dialOptions{timeout: 10 * time.Second}
	for _, o := range opts {
		o(do)
	}
	wsURL := normalizeURL(url)

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = do.timeout
	conn, resp, err := dialer.DialContext(ctx, wsURL, do.header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("mockarty socketio: dial %s: %w", wsURL, err)
	}
	c := &Client{conn: conn}

	// Read the Engine.IO open handshake.
	_ = conn.SetReadDeadline(time.Now().Add(do.timeout))
	mt, data, rerr := conn.ReadMessage()
	if rerr != nil || mt != websocket.TextMessage || len(data) == 0 || data[0] != eioOpen {
		_ = conn.Close()
		return nil, fmt.Errorf("mockarty socketio: expected open handshake, got %q (err=%v)", data, rerr)
	}
	_ = json.Unmarshal(data[1:], &c.handshake)
	return c, nil
}

// Connect performs the Socket.IO namespace CONNECT and waits for the
// server's CONNECT ack (or CONNECT_ERROR). Pass "/" for the default
// namespace. Must be called before Emit.
func (c *Client) Connect(namespace string, waitFor time.Duration) error {
	if namespace == "" {
		namespace = "/"
	}
	pkt := string(eioMessage) + string(sioConnect)
	if namespace != "/" {
		pkt += namespace + ","
	}
	if err := c.write(pkt); err != nil {
		return err
	}
	deadline := time.Now().Add(waitFor)
	for time.Now().Before(deadline) {
		typ, ns, body, err := c.readSocketIO(time.Until(deadline))
		if err != nil {
			return fmt.Errorf("mockarty socketio: connect read: %w", err)
		}
		switch typ {
		case sioConnect:
			if nsMatch(ns, namespace) {
				c.connected = true
				return nil
			}
		case sioConnectError:
			c.connectErr = body
			return fmt.Errorf("mockarty socketio: connect error: %s", body)
		}
	}
	return fmt.Errorf("mockarty socketio: connect to %q timed out", namespace)
}

// Emit sends an event with the given name and JSON-marshalled args on the
// namespace. Pass "/" for the default namespace.
func (c *Client) Emit(namespace, event string, args ...any) error {
	if namespace == "" {
		namespace = "/"
	}
	arr := make([]any, 0, len(args)+1)
	arr = append(arr, event)
	arr = append(arr, args...)
	payload, err := json.Marshal(arr)
	if err != nil {
		return fmt.Errorf("mockarty socketio: marshal emit args: %w", err)
	}
	pkt := string(eioMessage) + string(sioEvent)
	if namespace != "/" {
		pkt += namespace + ","
	}
	pkt += string(payload)
	return c.write(pkt)
}

// Collect reads inbound frames for the given window, accumulating EVENT
// packets (and answering Engine.IO pings with pongs). Returns the events
// received during the window. Cumulative events are also retained and
// returned by Events().
func (c *Client) Collect(window time.Duration) ([]Event, error) {
	deadline := time.Now().Add(window)
	start := len(c.inbound)
	for time.Now().Before(deadline) {
		typ, ns, body, err := c.readSocketIO(time.Until(deadline))
		if err != nil {
			// Timeout ends the window cleanly (consistent with the SDK's
			// WS/SSE listen-window semantics).
			break
		}
		if typ == sioEvent {
			if ev, ok := parseEvent(ns, body); ok {
				c.inbound = append(c.inbound, ev)
			}
		}
	}
	out := make([]Event, len(c.inbound)-start)
	copy(out, c.inbound[start:])
	return out, nil
}

// Events returns all events collected so far (escape hatch).
func (c *Client) Events() []Event {
	out := make([]Event, len(c.inbound))
	copy(out, c.inbound)
	return out
}

// Handshake returns the Engine.IO open-handshake fields (sid, etc).
func (c *Client) Handshake() map[string]any { return c.handshake }

// Close closes the connection. Idempotent.
func (c *Client) Close() error {
	if c == nil || c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// ── internals ───────────────────────────────────────────────────────────

func (c *Client) write(s string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(s)); err != nil {
		return fmt.Errorf("mockarty socketio: write: %w", err)
	}
	return nil
}

// readSocketIO reads one Engine.IO frame, transparently answering pings
// and skipping non-message frames, and returns the Socket.IO packet type,
// namespace, and the JSON body tail of the next message frame.
func (c *Client) readSocketIO(timeout time.Duration) (sioType byte, namespace, body string, err error) {
	if timeout <= 0 {
		timeout = 10 * time.Millisecond
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(timeout))
	for {
		mt, data, rerr := c.conn.ReadMessage()
		if rerr != nil {
			return 0, "", "", rerr
		}
		if mt != websocket.TextMessage || len(data) == 0 {
			continue
		}
		switch data[0] {
		case eioPing:
			// Reply pong, preserving any probe payload.
			_ = c.write(string(eioPong) + string(data[1:]))
			continue
		case eioPong, eioOpen:
			continue
		case eioClose:
			return 0, "", "", fmt.Errorf("server closed connection")
		case eioMessage:
			sio := data[1:]
			if len(sio) == 0 {
				continue
			}
			typ := sio[0]
			rest := string(sio[1:])
			ns := "/"
			if strings.HasPrefix(rest, "/") {
				if comma := strings.IndexByte(rest, ','); comma >= 0 {
					ns = rest[:comma]
					rest = rest[comma+1:]
				}
			}
			// Strip a leading ack id if present.
			rest = strings.TrimLeft(rest, "0123456789")
			return typ, ns, rest, nil
		}
	}
}

// parseEvent decodes a Socket.IO EVENT body `["name", arg...]`.
func parseEvent(ns, body string) (Event, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(body), &arr); err != nil || len(arr) == 0 {
		return Event{}, false
	}
	var name string
	if err := json.Unmarshal(arr[0], &name); err != nil {
		return Event{}, false
	}
	return Event{
		ReceivedAt: time.Now(),
		Namespace:  ns,
		Name:       name,
		Args:       arr[1:],
	}, true
}

func nsMatch(got, want string) bool {
	if got == "" {
		got = "/"
	}
	if want == "" {
		want = "/"
	}
	return got == want
}

// normalizeURL rewrites http→ws / https→wss and appends the standard
// /socket.io/ path + EIO=4&transport=websocket query when missing.
func normalizeURL(url string) string {
	u := url
	if strings.HasPrefix(u, "http://") {
		u = "ws://" + strings.TrimPrefix(u, "http://")
	} else if strings.HasPrefix(u, "https://") {
		u = "wss://" + strings.TrimPrefix(u, "https://")
	}
	// Split off any existing query.
	base, query := u, ""
	if q := strings.IndexByte(u, '?'); q >= 0 {
		base, query = u[:q], u[q+1:]
	}
	if !strings.Contains(base, "/socket.io") {
		base = strings.TrimRight(base, "/") + "/socket.io/"
	}
	if !strings.Contains(query, "EIO=") {
		if query != "" {
			query += "&"
		}
		query += "EIO=4&transport=websocket"
	}
	return base + "?" + query
}
