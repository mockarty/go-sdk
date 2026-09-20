package main

import (
	"context"
	"log"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	identities, err := client.CloudIdentity().List(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("linked sign-in methods: %d", len(identities))
	// Email changes require a Cloud cabinet session and two distinct mailbox
	// proofs. A password alone never completes the change.
	if address := os.Getenv("MOCKARTY_NEW_EMAIL"); address != "" {
		result, err := client.CloudIdentity().RequestEmailChange(context.Background(), mockarty.CloudEmailChangeRequest{NewEmail: address})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("email change stage: %s", result.Stage)
	}
}
