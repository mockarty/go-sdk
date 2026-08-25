package main

import (
	"context"
	"log"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace("engineering"))
	settings, err := client.NamespaceSettings().GetAutonomySettings(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	events, payloads, window := 365, 30, 90
	settings.JournalEventRetentionDays = &events
	settings.JournalPayloadRetentionDays = &payloads
	settings.RunWindowMinutes = &window
	_, err = client.NamespaceSettings().SaveAutonomySettingsWithOptions(context.Background(), settings,
		mockarty.AutonomySettingsSaveOptions{RequestID: "safety-change-2026-08-25"})
	if err != nil {
		log.Fatal(err)
	}
}
