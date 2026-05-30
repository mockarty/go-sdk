// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Package tester is the fluent test-writing surface of the Mockarty SDK.
//
// Each chain call records one Allure step and accumulates assertions.
// The test reads in this order:
//
//	t := tester.New(tester.WithBaseURL("http://localhost:8080"))
//	t.HTTP().GET("/api/v1/users/42").
//	    ExpectStatus(200).
//	    ExpectJSONPath("$.name", "Alice").
//	    Extract("$.token", "token")
//	t.HTTP().POST("/api/v1/orders").
//	    Header("X-Auth", "Bearer {{token}}").
//	    JSON(map[string]any{"userId": 42}).
//	    ExpectStatus(201)
//	if !t.OK() {
//	    for _, e := range t.Errors() { fmt.Println(e) }
//	}
//
// HTTP is the first protocol facet shipped; GraphQL, SOAP, WebSocket,
// SSE, gRPC, Kafka, RabbitMQ, DB, S3, SMTP and Socket.IO follow the same
// shape. When the SDK is used inside a test wrapped with allure.WithTest,
// every step is also captured in the Allure result JSON.
//
// S3 / SMTP / Socket.IO facets (CI-friendly minimal clients live in
// protocols/{s3,smtp,socketio}):
//
//	s3cli := s3.NewClient("http://localhost:8080/s3")
//	t.S3(s3cli).Put("reports", "q1.csv").Body("a,b,c").ExpectOK()
//	t.S3(s3cli).Get("reports", "q1.csv").ExpectBodyContains("a,b,c")
//
//	mail := smtp.NewClient("localhost:1025",
//	    smtp.WithPlainAuth("user", "pass", "localhost"))
//	t.SMTP(mail).Send("a@x", "b@y").Subject("Hi").Body("...").ExpectAccepted()
//
//	t.SocketIO("http://localhost:8080").Connect().
//	    Emit("greet", "World").Collect(2*time.Second).
//	    ExpectEvent("greeting").ExpectEventJSONPath("greeting", "$.msg", "hello World")
//
// Variable interpolation: any string passed to .Header, .JSON, .Body
// (text/* + form-urlencoded), or used as part of the request path is
// scanned for "{{name}}" tokens and substituted with values previously
// stored by .Extract. Missing names render as the literal "{{name}}"
// so failures are visible.
//
// Timing: interpolation happens at builder-call time, NOT at send time.
// That works because chains are linear — by the time the second chain
// reads {{token}}, the first chain has already finished (its commit
// fired when the second .HTTP() was called, which runs the first
// chain's actual HTTP request and then its .Extract). Don't try to
// reference a variable extracted by the same step you're building —
// the value isn't there yet.
package tester
