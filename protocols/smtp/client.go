// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Package smtp is the Mockarty Go SDK's minimal SMTP test client. It
// sends a mail over SMTP (plain or AUTH PLAIN) and reports the server's
// acceptance so CI/CD test scripts can assert that a mock — or a real
// MTA — accepted a message. Receipt-side assertions (did the mailbox get
// it?) are done separately against whatever inbox the test target
// exposes; this client owns the send side.
//
// # Air-gapped friendly
//
// Built on the standard library net/smtp — no CGO, no external module.
//
// # Out of scope
//
// STARTTLS negotiation, DKIM signing, and bounce parsing are NOT
// implemented — the owner-rule for mockarty-go is "expose only the
// surface useful from CI/CD scripts and tests".
package smtp

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

// Client is an SMTP test client bound to a fixed server address.
type Client struct {
	auth smtp.Auth
	addr string
}

// Option configures a Client.
type Option func(*Client)

// WithPlainAuth installs AUTH PLAIN credentials. host must match the
// SMTP server host the client connects to (net/smtp enforces this).
func WithPlainAuth(username, password, host string) Option {
	return func(c *Client) {
		c.auth = smtp.PlainAuth("", username, password, host)
	}
}

// WithAuth installs a custom smtp.Auth (e.g. CRAM-MD5).
func WithAuth(a smtp.Auth) Option {
	return func(c *Client) { c.auth = a }
}

// NewClient constructs a client against addr (host:port), e.g.
// "localhost:18772".
func NewClient(addr string, opts ...Option) *Client {
	c := &Client{addr: addr}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Message is one outbound mail.
type Message struct {
	Headers map[string]string
	From    string
	Subject string
	Body    string
	To      []string
}

// SendResult reports the outcome of a Send.
type SendResult struct {
	SentAt time.Time
	// Raw is the exact wire payload (headers + body) handed to the server,
	// useful for debugging / recording.
	Raw string
}

// Send transmits msg via the configured server. It builds an RFC 5322
// message (From / To / Subject + any extra headers, then a blank line,
// then the body) and hands it to net/smtp.SendMail, which performs the
// EHLO/AUTH/MAIL/RCPT/DATA exchange. A server-side rejection (4xx/5xx)
// surfaces as a non-nil error.
func (c *Client) Send(msg Message) (SendResult, error) {
	if msg.From == "" {
		return SendResult{}, fmt.Errorf("mockarty smtp: empty From")
	}
	if len(msg.To) == 0 {
		return SendResult{}, fmt.Errorf("mockarty smtp: no recipients")
	}
	raw := buildMessage(msg)
	if err := smtp.SendMail(c.addr, c.auth, msg.From, msg.To, []byte(raw)); err != nil {
		return SendResult{Raw: raw}, fmt.Errorf("mockarty smtp: send: %w", err)
	}
	return SendResult{Raw: raw, SentAt: time.Now()}, nil
}

// buildMessage renders an RFC 5322 message from the structured fields.
func buildMessage(msg Message) string {
	var sb strings.Builder
	sb.WriteString("From: " + msg.From + "\r\n")
	if len(msg.To) > 0 {
		sb.WriteString("To: " + strings.Join(msg.To, ", ") + "\r\n")
	}
	if msg.Subject != "" {
		sb.WriteString("Subject: " + msg.Subject + "\r\n")
	}
	for k, v := range msg.Headers {
		// Skip headers we already emitted to avoid duplicates.
		switch strings.ToLower(k) {
		case "from", "to", "subject":
			continue
		}
		sb.WriteString(k + ": " + v + "\r\n")
	}
	sb.WriteString("\r\n")
	sb.WriteString(msg.Body)
	return sb.String()
}
