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
	page, err := client.MediaDelivery().ListFenced(context.Background(), "transcribe")
	if err != nil {
		panic(err)
	}
	fmt.Printf("fenced deliveries: %d\n", page.Count)
}
