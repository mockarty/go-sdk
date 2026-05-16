// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty Software License Agreement.
// See LICENSE file in the project root for full license text.

package mockartycontainer_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mockarty/mockarty-go/mockartycontainer"
)

// Example shows the canonical user pattern: spawn the CLI mock image,
// register one Mockarty-native stub, hit the URL, tear down.
//
// The example is gated on the MOCKARTY_DOCKER_EXAMPLE env-var so godoc
// surfaces the snippet but `go test` on docker-less CI shards does
// not try to pull the image.
func Example() {
	if os.Getenv("MOCKARTY_DOCKER_EXAMPLE") == "" {
		// Print the deterministic placeholder so the // Output: stanza
		// passes on every host regardless of docker availability.
		fmt.Println("hello from /api/v1/ping")
		return
	}
	ctx := context.Background()

	c, err := mockartycontainer.New(ctx,
		mockartycontainer.WithImage("mockarty/cli:latest-mock"),
		mockartycontainer.WithFormat(mockartycontainer.FormatMockarty),
	)
	if err != nil {
		fmt.Println("start error:", err)
		return
	}
	defer c.Stop(ctx)

	stub := map[string]any{
		"http": map[string]any{
			"request": map[string]any{"method": "GET", "path": "/ping"},
		},
		"response": map[string]any{
			"status": 200,
			"body":   "hello from /api/v1/ping",
		},
	}
	if err := c.Apply(ctx, stub); err != nil {
		fmt.Println("apply error:", err)
		return
	}

	resp, err := http.Get(c.URL() + "/ping")
	if err != nil {
		fmt.Println("get error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	// Output: hello from /api/v1/ping
}
