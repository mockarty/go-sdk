package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	project, err := client.CloudSharedProjects().Create(context.Background(), os.Getenv("MOCKARTY_SPACE_ID"), "SDK example", json.RawMessage(`{"version":1}`))
	if err != nil {
		panic(err)
	}
	fmt.Printf("created %s revision %d\n", project.ID, project.Revision)
}
