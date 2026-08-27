// Copyright (c) 2026 Mockarty. All rights reserved.

package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	run, err := client.PageAnalyzer().Run(context.Background(), mockarty.PageAnalyzerRunRequest{
		TargetURL: "https://example.com",
		Options:   &mockarty.PageAnalyzerOptions{CheckResources: true, FollowRedirects: true},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("result=%s status=%s\n", run.ResultID, run.Status)
}
