package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	items, err := client.Experience().Search(context.Background(), mockarty.ExperienceSearchRequest{Query: "payment retry", Limit: 5})
	if err != nil {
		panic(err)
	}
	for _, item := range items.Results {
		fmt.Printf("%s: %s (%s)\n", item.Kind, item.Text, item.Source)
	}
}
