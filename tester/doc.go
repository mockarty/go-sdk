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
// HTTP is the first protocol facet shipped; Kafka / gRPC / RabbitMQ
// follow the same shape. When the SDK is used inside a test wrapped with
// allure.WithTest, every step is also captured in the Allure result JSON.
//
// Variable interpolation: any string passed to .Header / .JSON / .Body
// or used as part of the request path is scanned for "{{name}}" tokens
// and substituted with values previously stored by .Extract. Missing
// names render as the literal "{{name}}" so failures are visible.
package tester
