// Create a manual case, read/edit its steps, then save a version twice.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	mockarty "github.com/mockarty/go-sdk"
	"io"
	"log"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	api := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")), mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE"))).TCM()
	sourceID := os.Getenv("MOCKARTY_SOURCE_CASE_ID")
	if sourceID == "" {
		sourceID = "sdk-demo:login-smoke"
	}
	caseName := os.Getenv("MOCKARTY_CASE_NAME")
	if caseName == "" {
		caseName = "Login smoke"
	}
	c, err := api.CreateCase(ctx, "", mockarty.TCMObject{"name": caseName, "externalId": sourceID, "expectedResult": "Signed in", "customFields": []mockarty.TCMObject{{"name": "Component", "value": "authentication"}}, "steps": []mockarty.TCMObject{{"stepUid": "login", "name": "Sign in", "mode": "manual", "executorType": "manual", "action": "Enter valid credentials", "expectedResult": "Home page"}}})
	if err != nil {
		log.Fatal(err)
	}
	caseID := fmt.Sprint(c["id"])
	loaded, err := api.GetCase(ctx, "", caseID)
	if err != nil || loaded["name"] != caseName {
		log.Fatalf("case lookup failed: %v", err)
	}
	if _, err := api.ListCases(ctx, "", nil); err != nil {
		log.Fatal(err)
	}
	version, err := api.GetCaseVersion(ctx, "", caseID, fmt.Sprint(c["currentVersionId"]))
	if err != nil {
		log.Fatal(err)
	}
	rows, ok := version["steps"].([]any)
	if !ok || len(rows) == 0 {
		log.Fatal("created version contains no steps")
	}
	steps := make([]mockarty.TCMObject, 0, len(rows))
	for _, row := range rows {
		step, ok := row.(map[string]any)
		if !ok {
			log.Fatal("invalid step response")
		}
		steps = append(steps, step)
	}
	// Edit the loaded object so expectedAttachmentId and executorConfig survive.
	steps[0]["expectedResult"] = "Dashboard displays the signed-in user"
	saved, err := api.SaveCaseSteps(ctx, "", caseID, steps, "Clarify expected result")
	if err != nil {
		log.Fatal(err)
	}
	repeated, err := api.SaveCaseSteps(ctx, "", caseID, steps, "Clarify expected result")
	if err != nil {
		log.Fatal(err)
	}
	if saved["id"] != repeated["id"] || repeated["dedupSkipped"] != true {
		log.Fatal("unchanged save did not return the same version")
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
	if err != nil {
		log.Fatal(err)
	}
	filename := os.Getenv("MOCKARTY_ATTACHMENT_NAME")
	if filename == "" {
		filename = "login.png"
	}
	attachment, err := api.UploadAttachment(ctx, "", "tcm_case", caseID, filename, "image/png", bytes.NewReader(png))
	if err != nil {
		log.Fatal(err)
	}
	// Imported launch files: ListAttachments(ctx, namespace, "test_plan_run", runID) reads all pages (up to 10,000), keeping report ownership.
	media, err := api.ListAttachments(ctx, "", "tcm_case", caseID)
	namePreserved := false
	for _, item := range media {
		if item.ID == attachment.ID && item.OriginalName == filename {
			namePreserved = true
		}
	}
	if err != nil || !namePreserved {
		log.Fatalf("attachment name not preserved: %v", err)
	}
	raw, err := api.DownloadAttachment(ctx, "", attachment.ID)
	if err != nil {
		log.Fatal(err)
	}
	readPNG, readErr := io.ReadAll(raw)
	closeErr := raw.Close()
	if readErr != nil || closeErr != nil || !bytes.Equal(readPNG, png) {
		log.Fatalf("attachment bytes changed: read=%v close=%v", readErr, closeErr)
	}
	run, err := api.RunCase(ctx, "", caseID, mockarty.TCMObject{"mode": "manual"})
	if err != nil {
		log.Fatal(err)
	}
	runID := fmt.Sprint(run["id"])
	if _, err := api.GetCaseRun(ctx, "", runID); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("case %s version %v, PNG %s (%d bytes), run %s; unchanged save deduplicated\n", caseID, saved["version"], attachment.ID, len(png), runID)
}
