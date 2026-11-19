package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mockarty "github.com/mockarty/go-sdk"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_SERVICE_ACCOUNT_CREDENTIAL")))
	run, err := client.CloudServiceAccount().DispatchRun(context.Background(), os.Getenv("MOCKARTY_SPACE_ID"), mockarty.CloudServiceAccountRunRequest{
		Deadline: time.Now().Add(10 * time.Minute), ProjectID: os.Getenv("MOCKARTY_PROJECT_ID"),
		Capability: "execute", Class: "batch", Payload: json.RawMessage(`{"suite":"smoke"}`),
	}, os.Getenv("MOCKARTY_IDEMPOTENCY_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(run.ID, run.State, run.Revision)
}
