package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	ctx := context.Background()
	spaceID := os.Getenv("MOCKARTY_CLOUD_SPACE_ID")

	redemptions, err := client.CloudCustomer().ListLoyaltyRedemptions(ctx, spaceID, "", 25)
	if err != nil {
		panic(err)
	}
	fmt.Printf("loyalty: %#v\n", redemptions)

	cases, err := client.CloudOperations().ListSupportCases(ctx, "open", "", 50)
	if err != nil {
		panic(err)
	}
	fmt.Printf("operator cases: %#v\n", cases)
}
