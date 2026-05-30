// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

package smtp

import (
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	raw := buildMessage(Message{
		From:    "a@x",
		To:      []string{"b@y", "c@z"},
		Subject: "Hi",
		Body:    "line1\r\nline2",
		Headers: map[string]string{"X-Test": "1", "Subject": "dup-ignored"},
	})
	for _, want := range []string{"From: a@x\r\n", "To: b@y, c@z\r\n", "Subject: Hi\r\n", "X-Test: 1\r\n", "\r\nline1"} {
		if !strings.Contains(raw, want) {
			t.Errorf("buildMessage missing %q in:\n%s", want, raw)
		}
	}
	// The duplicate Subject header in Headers must not be emitted twice.
	if strings.Count(raw, "Subject:") != 1 {
		t.Errorf("duplicate Subject header:\n%s", raw)
	}
}

func TestSendValidation(t *testing.T) {
	cli := NewClient("127.0.0.1:0")
	if _, err := cli.Send(Message{To: []string{"b@y"}}); err == nil {
		t.Error("expected error on empty From")
	}
	if _, err := cli.Send(Message{From: "a@x"}); err == nil {
		t.Error("expected error on no recipients")
	}
}
