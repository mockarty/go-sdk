// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

// Example: programmatic Mockarty mock-server container.
//
// Spawns the `mockarty/cli:latest-mock` image via testcontainers-go,
// hands it a directory of WireMock-format stubs, and exercises three
// endpoints. Mirrors the WireMock-testcontainers ergonomic so existing
// users can swap container classes without rewriting their tests.
//
// Run with Docker available:
//
//	cd sdk/go-sdk/examples/testcontainer_mockarty
//	go run .
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/mockartycontainer"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Mount the fixtures dir as /mocks inside the container. The CLI
	// reads MOCKARTY_MOCK_DIR (set automatically by WithMappings) and
	// loads every JSON file at startup.
	c, err := mockartycontainer.Run(ctx,
		mockartycontainer.WithImage("mockarty/cli:latest-mock"),
		mockartycontainer.WithFormat(mockartycontainer.FormatAuto),
		mockartycontainer.WithMappings("./fixtures"),
		mockartycontainer.WithLogger(os.Stderr),
		mockartycontainer.WithStartupTimeout(90*time.Second),
	)
	if err != nil {
		log.Fatalf("start container: %v", err)
	}
	defer func() {
		if err := c.Terminate(ctx); err != nil {
			log.Printf("terminate: %v", err)
		}
	}()

	fmt.Printf("Mockarty mock listening at %s\n", c.URL())

	// 1) Hit the pre-loaded stub.
	if status, body, err := hit(c.URL() + "/api/users/1"); err != nil {
		log.Fatalf("GET /api/users/1: %v", err)
	} else if status != 200 {
		log.Fatalf("/api/users/1 status=%d want=200", status)
	} else {
		fmt.Printf("GET /api/users/1 -> 200 %s\n", body)
	}

	// 2) Register a second stub at runtime via the WireMock-compat admin API.
	if err := c.AddWireMockStub(ctx, map[string]any{
		"request":  map[string]any{"method": "GET", "url": "/api/runtime"},
		"response": map[string]any{"status": 200, "body": `{"added":"at-runtime"}`},
	}); err != nil {
		log.Fatalf("AddWireMockStub: %v", err)
	}
	if status, _, err := hit(c.URL() + "/api/runtime"); err != nil || status != 200 {
		log.Fatalf("/api/runtime status=%d err=%v", status, err)
	}
	fmt.Println("GET /api/runtime -> 200 (runtime-added stub OK)")

	// 3) Reset wipes everything; the previously-loaded fixture is gone.
	if err := c.Reset(ctx); err != nil {
		log.Fatalf("reset: %v", err)
	}
	if status, _, _ := hit(c.URL() + "/api/users/1"); status == 200 {
		log.Fatalf("/api/users/1 still served after reset")
	}
	fmt.Println("Reset OK — previously-loaded stubs cleared")
}

func hit(url string) (int, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}
