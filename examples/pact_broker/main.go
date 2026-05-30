// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: publish a consumer pact + check can-i-deploy.
//
// Run against a real Pact Broker by setting PACT_BROKER_BASE_URL +
// PACT_BROKER_TOKEN (or PACT_BROKER_USERNAME/PASSWORD) in env.
//
//	PACT_BROKER_BASE_URL=https://broker.example.com \
//	PACT_BROKER_TOKEN=... \
//	go run ./examples/pact_broker
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/mockarty/mockarty-go/pact"
)

const samplePact = `{
  "consumer": {"name": "OrderClient"},
  "provider": {"name": "OrderAPI"},
  "interactions": [
    {
      "description": "a request for order 42",
      "request":  {"method": "GET", "path": "/orders/42"},
      "response": {"status": 200, "body": {"id": 42, "status": "open"}}
    }
  ],
  "metadata": {"pactSpecification": {"version": "4.0"}}
}`

func main() {
	ctx := context.Background()
	c, err := pact.NewBrokerClient()
	if err != nil {
		log.Fatalf("broker: %v", err)
	}

	consumerVersion := os.Getenv("GIT_COMMIT")
	if consumerVersion == "" {
		consumerVersion = "0.0.0-local"
	}
	branch := os.Getenv("GIT_BRANCH")
	tags := []string{"ci"}

	if err := c.Publish(ctx, []byte(samplePact), consumerVersion, branch, tags); err != nil {
		log.Fatalf("publish: %v", err)
	}
	fmt.Printf("Published OrderClient@%s to broker.\n", consumerVersion)

	res, err := c.CanIDeploy(ctx, "OrderClient", consumerVersion, "production")
	if err != nil {
		log.Fatalf("can-i-deploy: %v", err)
	}
	if !res.Deployable {
		fmt.Printf("BLOCKED: %s\n", res.Reason)
		os.Exit(2)
	}
	fmt.Println("Deployable to production.")

	body, err := c.FetchLatest(ctx, "OrderClient", "OrderAPI")
	if err != nil {
		if errors.Is(err, pact.ErrBrokerPactNotFound) {
			fmt.Println("(no published pact yet)")
			return
		}
		log.Fatalf("fetch: %v", err)
	}
	fmt.Printf("Latest pact: %d bytes\n", len(body))
}
