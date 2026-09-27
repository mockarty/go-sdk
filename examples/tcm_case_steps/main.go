// Create a manual case, read/edit its steps, then save a version twice.
package main

import (
	"context"
	"fmt"
	mockarty "github.com/mockarty/mockarty-go"
	"log"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	api := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")), mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE"))).TCM()
	c, err := api.CreateCase(ctx, "", mockarty.TCMObject{"name": "Login smoke", "expectedResult": "Signed in", "steps": []mockarty.TCMObject{{"stepUid": "login", "name": "Sign in", "mode": "manual", "executorType": "manual", "action": "Enter valid credentials", "expectedResult": "Home page"}}})
	if err != nil {
		log.Fatal(err)
	}
	caseID := fmt.Sprint(c["id"])
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
	fmt.Printf("case %s version %v; unchanged save deduplicated\n", caseID, saved["version"])
}
