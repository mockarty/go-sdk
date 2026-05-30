// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"errors"
	"strings"
	"testing"

	"github.com/mockarty/mockarty-go/protocols/smtp"
)

// fakeSMTP is an in-memory SMTPSender. Captures sent messages; can be
// forced to reject.
type fakeSMTP struct {
	sent      []smtp.Message
	rejectErr error
}

func (f *fakeSMTP) Send(msg smtp.Message) (smtp.SendResult, error) {
	if f.rejectErr != nil {
		return smtp.SendResult{}, f.rejectErr
	}
	f.sent = append(f.sent, msg)
	return smtp.SendResult{Raw: "From: " + msg.From + "\r\nSubject: " + msg.Subject + "\r\n\r\n" + msg.Body}, nil
}

func TestSMTPSendAccepted(t *testing.T) {
	srv := &fakeSMTP{}
	tst := New()
	tst.SMTP(srv).
		Send("alice@corp", "bob@corp", "carol@corp").
		Subject("Invoice 42").
		Body("Please pay.").
		Header("X-Priority", "1").
		ExpectAccepted()
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
	if len(srv.sent) != 1 {
		t.Fatalf("want 1 sent, got %d", len(srv.sent))
	}
	got := srv.sent[0]
	if got.From != "alice@corp" || len(got.To) != 2 || got.Subject != "Invoice 42" {
		t.Fatalf("unexpected message: %+v", got)
	}
	if got.Headers["X-Priority"] != "1" {
		t.Fatalf("header not propagated: %+v", got.Headers)
	}
}

func TestSMTPInterpolation(t *testing.T) {
	srv := &fakeSMTP{}
	tst := New()
	tst.SetVar("id", "INV-7")
	tst.SetVar("rcpt", "dyn@corp")
	tst.SMTP(srv).
		Send("sys@corp", "{{rcpt}}").
		Subject("Order {{id}}").
		Body("Ref {{id}}").
		ExpectAccepted()
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("expected pass, got: %v", tst.Errors())
	}
	got := srv.sent[0]
	if got.To[0] != "dyn@corp" || got.Subject != "Order INV-7" || !strings.Contains(got.Body, "INV-7") {
		t.Fatalf("interpolation failed: %+v", got)
	}
}

func TestSMTPNegative(t *testing.T) {
	cases := []struct {
		name    string
		run     func(tst *Tester, srv *fakeSMTP)
		wantErr bool
	}{
		{
			name: "rejected-but-expected-accepted-fails",
			run: func(tst *Tester, srv *fakeSMTP) {
				srv.rejectErr = errors.New("550 mailbox unavailable")
				tst.SMTP(srv).Send("a@x", "b@y").ExpectAccepted()
			},
			wantErr: true,
		},
		{
			name: "rejected-and-expect-rejected-passes",
			run: func(tst *Tester, srv *fakeSMTP) {
				srv.rejectErr = errors.New("550 mailbox unavailable")
				tst.SMTP(srv).Send("a@x", "b@y").ExpectRejected().ExpectErrorContains("550")
			},
			wantErr: false,
		},
		{
			name: "accepted-but-expect-rejected-fails",
			run: func(tst *Tester, srv *fakeSMTP) {
				tst.SMTP(srv).Send("a@x", "b@y").ExpectRejected()
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &fakeSMTP{}
			tst := New()
			tc.run(tst, srv)
			tst.Finish()
			if tst.OK() == tc.wantErr {
				t.Fatalf("OK()=%v wantErr=%v errs=%v", tst.OK(), tc.wantErr, tst.Errors())
			}
		})
	}
}
