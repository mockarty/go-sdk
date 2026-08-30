package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	providers, err := client.CloudOAuthProviders().List(context.Background())
	if err != nil {
		panic(err)
	}
	for _, provider := range providers {
		fmt.Printf("%s enabled=%t revision=%d secret-configured=%t\n",
			provider.Provider, provider.Enabled, provider.ConfigRevision, provider.SecretConfigured)
	}
}
