// Upload result JSON, nested steps, and small media from an Allure directory.
// Media preflight rejects missing files and any file above 256 KiB (1 MiB /
// 32 unique files per result; 32 MiB per batch). Keep larger source media for
// a separate multipart transfer. Container fixtures are not imported here.
package main

import (
	"context"
	mockarty "github.com/mockarty/mockarty-go"
	"log"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: allure_upload <allure-results-directory>")
	}
	client := mockarty.NewClient(os.Getenv("MOCKARTY_SERVER"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")), mockarty.WithNamespace(os.Getenv("MOCKARTY_NAMESPACE")))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	results, err := client.ExternalRuns().UploadAllureDir(ctx, os.Args[1], mockarty.AllureUploadOptions{AutoCreate: true, OnError: "raise"})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("uploaded %d results", len(results))
}
