// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: s3_smtp_socketio — author one autotest that exercises S3,
// SMTP and Socket.IO endpoints through the Mockarty SDK Tester, the same
// way you'd test HTTP / gRPC / Kafka.
//
// Point it at a Mockarty testbackend (cmd/testbackend) or any
// S3-compatible / SMTP / Socket.IO server:
//
//	go run ./cmd/testbackend &   # from the main repo
//	S3_ENDPOINT=http://localhost:18770/s3 \
//	SMTP_ADDR=localhost:18772 \
//	SOCKETIO_URL=http://localhost:18770 \
//	  go run ./examples/s3_smtp_socketio
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/protocols/s3"
	"github.com/mockarty/mockarty-go/protocols/smtp"
	"github.com/mockarty/mockarty-go/tester"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s3Endpoint := getenv("S3_ENDPOINT", "http://localhost:18770/s3")
	smtpAddr := getenv("SMTP_ADDR", "localhost:18772")
	socketURL := getenv("SOCKETIO_URL", "http://localhost:18770")

	t := tester.New(tester.WithContext(ctx))

	// ── S3: put → get → list → delete ─────────────────────────────────
	s3cli := s3.NewClient(s3Endpoint)
	key := "report-" + time.Now().Format("150405") + ".csv"
	t.S3(s3cli).Put("mockarty-test", key).
		Body("region,sales\neu,42\n").
		ContentType("text/csv").
		Meta("owner", "finance").
		ExpectOK().ExpectStatus(200).ExtractETag("etag")
	t.S3(s3cli).Get("mockarty-test", key).
		ExpectOK().
		ExpectContentType("text/csv").
		ExpectBodyContains("eu,42").
		ExpectMeta("owner", "finance")
	t.S3(s3cli).List("mockarty-test").ExpectKey(key)
	t.S3(s3cli).Delete("mockarty-test", key).ExpectOK().ExpectStatus(204)

	// ── SMTP: send an authenticated mail ──────────────────────────────
	// net/smtp's AUTH PLAIN requires the auth host to match the server
	// host we connect to, so derive it from the address.
	smtpHost, _, _ := net.SplitHostPort(smtpAddr)
	if smtpHost == "" {
		smtpHost = "localhost"
	}
	mail := smtp.NewClient(smtpAddr, smtp.WithPlainAuth("user", "pass", smtpHost))
	t.SMTP(mail).
		Send("billing@corp", "customer@corp").
		Subject("Your invoice").
		Body("Please find your invoice attached.").
		ExpectAccepted()

	// ── Socket.IO: connect → emit → assert echoed events ──────────────
	t.SocketIO(socketURL).
		Connect().
		Emit("greet", "World").
		Collect(2 * time.Second).
		ExpectConnected().
		ExpectEvent("greeting").
		ExpectEventJSONPath("greeting", "$.msg", "hello World")

	t.Finish()

	if t.OK() {
		fmt.Printf("PASS — %d steps\n", len(t.Report()))
		return
	}
	fmt.Println("FAIL:")
	for _, e := range t.Errors() {
		fmt.Printf("  - %v\n", e)
	}
	os.Exit(1)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
