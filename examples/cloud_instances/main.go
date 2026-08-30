package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_CLOUD_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_CLOUD_TOKEN")))
	result, err := client.CloudInstances().Create(context.Background(), os.Getenv("MOCKARTY_CLOUD_SPACE_ID"), "Managed beta", "example-create-1")
	if err != nil {
		panic(err)
	}
	// Store the one-time password in a secret manager. Never log it.
	fmt.Printf("instance %s admitted; bootstrap available=%t\n", result.Instance.ID, result.Bootstrap != nil && result.Bootstrap.Available)
}
