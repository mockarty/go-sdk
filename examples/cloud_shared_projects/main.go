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
	ctx := context.Background()
	requestID := os.Getenv("MOCKARTY_REQUEST_ID")
	var project *mockarty.CloudSharedProject
	var err error
	if requestID == "" {
		project, err = client.CloudSharedProjects().Create(ctx, os.Getenv("MOCKARTY_SPACE_ID"), "SDK example", json.RawMessage(`{"version":1}`))
	} else {
		project, err = client.CloudSharedProjects().CreateWithRequestID(ctx, os.Getenv("MOCKARTY_SPACE_ID"), "SDK example", json.RawMessage(`{"version":1}`), requestID)
	}
	if err != nil {
		panic(err)
	}
	fmt.Printf("created %s revision %d\n", project.ID, project.Revision)
}
