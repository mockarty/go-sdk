// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"strings"
	"time"

	"github.com/mockarty/mockarty-go/protocols/smtp"
)

// SMTPSender is the minimal contract the SMTP facet needs. `*smtp.Client`
// from `protocols/smtp` satisfies it directly; tests pass an in-memory
// fake so no real SMTP server is required.
type SMTPSender interface {
	Send(msg smtp.Message) (smtp.SendResult, error)
}

// SMTPFacet is the SMTP entry point reached via Tester.SMTP(sender).
type SMTPFacet struct {
	t      *Tester
	sender SMTPSender
}

// SMTP returns the SMTP facet bound to the supplied sender.
//
//	cli := smtp.NewClient("localhost:18772",
//	  smtp.WithPlainAuth("user", "pass", "localhost"))
//	t.SMTP(cli).
//	  Send("alice@corp", "bob@corp").
//	  Subject("Invoice {{id}}").
//	  Body("Please find attached.").
//	  ExpectAccepted()
func (t *Tester) SMTP(sender SMTPSender) *SMTPFacet {
	t.flushPending()
	return &SMTPFacet{t: t, sender: sender}
}

// Send starts an SMTP send chain with the given sender and recipients.
func (f *SMTPFacet) Send(from string, to ...string) *SMTPStep {
	vars := f.t.snapshotVars()
	rcpts := make([]string, len(to))
	for i, r := range to {
		rcpts[i] = interpolate(r, vars)
	}
	step := &SMTPStep{
		t:      f.t,
		sender: f.sender,
		msg: smtp.Message{
			From:    interpolate(from, vars),
			To:      rcpts,
			Headers: map[string]string{},
		},
	}
	f.t.setPending(step)
	return step
}

// SMTPStep is one SMTP send.
type SMTPStep struct {
	t      *Tester
	sender SMTPSender
	msg    smtp.Message
	result smtp.SendResult

	sent       bool
	committed  bool
	abortChain bool
	startedAt  time.Time
	endedAt    time.Time
	err        error
	failures   []string
}

// Subject sets the mail subject, {{var}}-interpolated.
func (s *SMTPStep) Subject(subj string) *SMTPStep {
	if s.guardSent("Subject") {
		return s
	}
	s.msg.Subject = interpolate(subj, s.t.snapshotVars())
	return s
}

// Body sets the mail body, {{var}}-interpolated.
func (s *SMTPStep) Body(body string) *SMTPStep {
	if s.guardSent("Body") {
		return s
	}
	s.msg.Body = interpolate(body, s.t.snapshotVars())
	return s
}

// Header adds an extra header (e.g. "Content-Type"), {{var}}-interpolated.
func (s *SMTPStep) Header(k, v string) *SMTPStep {
	if s.guardSent("Header") {
		return s
	}
	s.msg.Headers[k] = interpolate(v, s.t.snapshotVars())
	return s
}

// ExpectAccepted asserts the server accepted the message (no error).
func (s *SMTPStep) ExpectAccepted() *SMTPStep {
	if !s.ensureSent() {
		return s
	}
	if s.err != nil {
		s.fail("ExpectAccepted: " + s.err.Error())
	}
	return s
}

// ExpectRejected asserts the server rejected the message (error returned).
// Use for negative tests against a mock configured to reject.
func (s *SMTPStep) ExpectRejected() *SMTPStep {
	s.ensureSent()
	if s.err == nil {
		s.fail("ExpectRejected: message was accepted")
	}
	return s
}

// ExpectErrorContains asserts the send error message contains sub.
func (s *SMTPStep) ExpectErrorContains(sub string) *SMTPStep {
	s.ensureSent()
	if s.err == nil {
		s.fail("ExpectErrorContains: no error")
		return s
	}
	if !strings.Contains(s.err.Error(), sub) {
		s.fail("ExpectErrorContains: " + sub + " not found in: " + s.err.Error())
	}
	return s
}

// Raw returns the rendered wire message (escape hatch for custom checks).
func (s *SMTPStep) Raw() string {
	s.ensureSent()
	return s.result.Raw
}

// Done finalises the step.
func (s *SMTPStep) Done() *Tester {
	s.commit()
	s.t.clearPending(s)
	return s.t
}

func (s *SMTPStep) fail(msg string) { s.failures = append(s.failures, msg) }

func (s *SMTPStep) guardSent(method string) bool {
	if s.sent {
		s.fail(method + "() called after send")
		return true
	}
	return false
}

func (s *SMTPStep) ensureSent() bool {
	if s.sent {
		return !s.abortChain
	}
	s.sent = true
	if s.t.shouldAbort() {
		s.abortChain = true
		s.fail("skipped: fail-fast triggered by earlier step")
		return false
	}
	s.startedAt = time.Now()
	s.result, s.err = s.sender.Send(s.msg)
	s.endedAt = time.Now()
	return true
}

func (s *SMTPStep) commit() {
	if s.committed {
		return
	}
	s.committed = true
	if !s.sent {
		s.ensureSent()
	}
	status := 250 // accepted
	if s.err != nil {
		status = 550 // generic rejection marker for the report
	}
	rec := StepRecord{
		Protocol:     "smtp",
		Method:       "send",
		Name:         "smtp send " + s.msg.From + " → " + strings.Join(s.msg.To, ","),
		URL:          strings.Join(s.msg.To, ","),
		StatusOrCode: status,
		StartedAt:    s.startedAt,
		EndedAt:      s.endedAt,
		Failures:     append([]string(nil), s.failures...),
	}
	s.t.recordStep(rec)
	emitAllureStep(s.t.ctx, rec)
}
