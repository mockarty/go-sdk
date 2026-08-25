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
	queue, err := client.Experience().ListReview(context.Background(), mockarty.ExperienceReviewListRequest{State: "candidate", Limit: 20})
	if err != nil {
		panic(err)
	}
	for _, candidate := range queue.Items {
		fmt.Printf("review %s %s v%d: %s\n", candidate.State, candidate.ID, candidate.Version, candidate.Source)
	}
}
