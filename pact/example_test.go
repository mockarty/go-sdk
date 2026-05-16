// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package pact_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mockarty/mockarty-go/pact"
)

// Example shows the canonical consumer test pattern: declare the
// expected interactions, point the real client at the mock URL,
// exercise the request, then verify and close.
//
// The pact file written by Close is `<consumer>-<provider>.json` under
// WithOutputDir.
func Example() {
	tmp, _ := os.MkdirTemp("", "pact-example-*")
	defer os.RemoveAll(tmp)

	p := pact.NewConsumer("OrderService",
		pact.WithProvider("PaymentService"),
		pact.WithSpecVersion(pact.SpecV4),
		pact.WithOutputDir(tmp),
	)
	p.AddInteraction().
		Given("payment service is up").
		UponReceiving("a charge request").
		WithRequest(http.MethodPost, "/charge").
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"amount": pact.Like(100)}).
		WillRespondWith(200).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{"id": pact.Like("abc")})

	srv, err := p.Start(context.Background())
	if err != nil {
		fmt.Println("start error:", err)
		return
	}
	defer srv.Close()

	// Exercise the request against the in-process mock.
	body, _ := json.Marshal(map[string]any{"amount": 999})
	resp, err := http.Post(srv.URL()+"/charge", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("post error:", err)
		return
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Println("status:", resp.StatusCode)
	fmt.Println("body:", string(out))

	if err := srv.Verify(); err != nil {
		fmt.Println("verify:", err)
		return
	}
	fmt.Println("verified")

	// Output:
	// status: 200
	// body: {"id":"abc"}
	// verified
}
