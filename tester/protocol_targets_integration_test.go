// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package tester

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mockarty/mockarty-go/protocols/s3"
	"github.com/mockarty/mockarty-go/protocols/smtp"
)

// These tests exercise the s3 / smtp / socket.io facets against a live
// Mockarty testbackend (cmd/testbackend). They are skipped unless the
// backend URL is provided, so the default `go test ./...` stays hermetic.
//
//	# from the main repo:
//	go run ./cmd/testbackend &
//	MOCKARTY_TESTBACKEND_URL=http://127.0.0.1:18770 \
//	MOCKARTY_TESTBACKEND_SMTP=127.0.0.1:18772 \
//	  go test ./tester/ -run TestIntegration
func testbackendURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("MOCKARTY_TESTBACKEND_URL")
	if u == "" {
		t.Skip("set MOCKARTY_TESTBACKEND_URL to run protocol-target integration tests")
	}
	return strings.TrimRight(u, "/")
}

func TestIntegrationS3AgainstTestbackend(t *testing.T) {
	base := testbackendURL(t)
	cli := s3.NewClient(base + "/s3")
	tst := New()
	key := "sdk-it-" + time.Now().Format("150405.000000") + ".txt"
	tst.S3(cli).Put("mockarty-test", key).
		Body("integration-body").ContentType("text/plain").Meta("owner", "sdk").
		ExpectOK().ExpectStatus(200)
	tst.S3(cli).Get("mockarty-test", key).
		ExpectOK().ExpectBodyEquals("integration-body").ExpectContentType("text/plain")
	tst.S3(cli).Head("mockarty-test", key).ExpectExists()
	tst.S3(cli).List("mockarty-test").ExpectKey(key)
	tst.S3(cli).Delete("mockarty-test", key).ExpectOK().ExpectStatus(204)
	tst.S3(cli).Get("mockarty-test", key).ExpectError().ExpectStatus(404)
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("S3 integration failed: %v", tst.Errors())
	}
}

func TestIntegrationSMTPAgainstTestbackend(t *testing.T) {
	base := testbackendURL(t)
	smtpAddr := os.Getenv("MOCKARTY_TESTBACKEND_SMTP")
	if smtpAddr == "" {
		smtpAddr = "127.0.0.1:18772"
	}
	cli := smtp.NewClient(smtpAddr, smtp.WithPlainAuth("alice", "secret", "127.0.0.1"))
	rcpt := "it-" + time.Now().Format("150405.000000") + "@corp"
	subj := "Integration " + rcpt

	tst := New()
	tst.SMTP(cli).Send("sender@corp", rcpt).
		Subject(subj).Body("integration body").ExpectAccepted()
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("SMTP send failed: %v", tst.Errors())
	}

	// Assert receipt via the testbackend inbox surface.
	resp, err := http.Get(base + "/smtp/inbox?to=" + rcpt)
	if err != nil {
		t.Fatalf("inbox fetch: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Messages []struct {
			From     string `json:"from"`
			Subject  string `json:"subject"`
			AuthUser string `json:"authUser"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode inbox: %v", err)
	}
	if len(out.Messages) == 0 {
		t.Fatalf("message not delivered to inbox for %s", rcpt)
	}
	m := out.Messages[len(out.Messages)-1]
	if m.Subject != subj || m.AuthUser != "alice" {
		t.Fatalf("delivered message mismatch: %+v", m)
	}
}

func TestIntegrationSocketIOAgainstTestbackend(t *testing.T) {
	base := testbackendURL(t)
	tst := New()
	tst.SocketIO(base).
		Connect().
		Emit("echo", map[string]any{"hello": "world"}).
		Emit("greet", "Integration").
		Collect(3 * time.Second).
		ExpectConnected().
		ExpectEvent("echo").
		ExpectEvent("greeting").
		ExpectEventJSONPath("greeting", "$.msg", "hello Integration").
		ExpectEventArgContains("echo", `"hello":"world"`)
	tst.Finish()
	if !tst.OK() {
		t.Fatalf("Socket.IO integration failed: %v", tst.Errors())
	}
}
