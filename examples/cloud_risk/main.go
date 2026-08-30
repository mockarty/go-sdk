package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_TOKEN")))
	cases, err := client.CloudRisk().ListCases(context.Background(), "open", 50)
	if err != nil {
		panic(err)
	}
	fmt.Printf("open risk cases: %d\n", len(cases))
	// ReleaseEnforcement automatically uses a stable idempotency key, so an
	// exact retry after a lost response cannot release twice.
}
