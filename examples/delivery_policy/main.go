// Copyright (c) 2026 Mockarty. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	created, err := client.DeliveryPolicy().Create(context.Background(), mockarty.DeliveryPolicyEnvironmentRequest{
		ID: "staging", ProjectID: "payments", Class: "staging", Profile: "standard",
		AuditID: "change-123", EvidenceID: "review-123",
	}, "payments-staging-v1")
	if err != nil {
		panic(err)
	}
	fmt.Printf("environment=%s revision=%d etag=%s\n", created.ID, created.Revision, created.ETag)
}
