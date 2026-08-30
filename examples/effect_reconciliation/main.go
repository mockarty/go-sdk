package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	page, err := client.EffectReconciliation().ListQueue(context.Background(), mockarty.EffectReconciliationListOptions{Limit: 20})
	if err != nil {
		panic(err)
	}
	fmt.Printf("unresolved effects: %d\n", len(page.Items))
}
