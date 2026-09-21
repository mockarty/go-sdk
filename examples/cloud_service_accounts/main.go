package main

import (
	"context"
	"fmt"
	"log"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	ctx := context.Background()
	if os.Getenv("MOCKARTY_SERVICE_ACCOUNT_CREDENTIAL") != "" {
		client = mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_SERVICE_ACCOUNT_CREDENTIAL")))
		identity, err := client.CloudServiceAccount().Self(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("service account %s has %d binding(s)\n", identity.ServiceAccount.Name, len(identity.Bindings))
		return
	}
	page, err := client.CloudServiceAccounts().List(ctx, os.Getenv("MOCKARTY_SPACE_ID"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d service account(s), %d allowance remaining\n", len(page.ServiceAccounts), page.Capacity.Remaining)
}
