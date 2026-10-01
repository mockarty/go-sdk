package main

import (
	"context"
	mockarty "github.com/mockarty/mockarty-go"
	"os"
)

// Run inside a selected job. The test runner supplies its stable ID/fullName;
// never print the private delivery context. Complete only after uploads finish.
func main() {
	job, err := mockarty.LoadCIJobContext(os.Getenv(mockarty.CIContextEnvironment))
	if err != nil {
		panic("selected job context unavailable")
	}
	c := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_TOKEN")), mockarty.WithNamespace(job.Namespace))
	status := "passed"
	_, err = c.ExternalRuns().Report(context.Background(), "", mockarty.ExternalRunRequest{ExternalID: os.Getenv("TEST_EXTERNAL_ID"), FullName: os.Getenv("TEST_FULL_NAME"), Status: status})
	if err != nil {
		status = "failed"
	}
	_, completeErr := c.ExternalRuns().CompleteGroup(context.Background(), "", job.Callback(), status)
	if err != nil || completeErr != nil {
		os.Exit(1)
	}
}
