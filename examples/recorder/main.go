// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: recorder — Traffic recording API usage with configs, CA, and entry operations.
//
// This example demonstrates:
//   - Start, stop, restart a recording session
//   - List and manage sessions and entries
//   - Create mocks from recorded traffic
//   - Recorder configs management (save, list, export, delete)
//   - CA certificate operations (status, generate, download)
//   - Entry annotation and replay
//   - Session modifications
//   - Available ports
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	// -----------------------------------------------------------------------
	// 1. Recorder configs management
	// -----------------------------------------------------------------------
	fmt.Println("--- Recorder Configs ---")

	// Save a reusable recorder configuration
	recConfig, err := client.Recorder().SaveConfig(ctx, &mockarty.RecorderConfig{
		Name:      "Production API Recorder",
		TargetURL: "https://api.example.com",
		Port:      8443,
	})
	if err != nil {
		fmt.Printf("Save recorder config returned: %v\n", err)
	} else {
		fmt.Printf("Saved recorder config: id=%s, name=%s, port=%d\n",
			recConfig.ID, recConfig.Name, recConfig.Port)

		defer func() {
			_ = client.Recorder().DeleteConfig(ctx, recConfig.ID)
			fmt.Println("\nRecorder config cleaned up.")
		}()
	}

	// List all recorder configs
	configs, err := client.Recorder().ListConfigs(ctx)
	if err != nil {
		fmt.Printf("List configs returned: %v\n", err)
	} else {
		fmt.Printf("Found %d recorder configs:\n", len(configs))
		for _, c := range configs {
			fmt.Printf("  - %s (id=%s, target=%s, port=%d)\n",
				c.Name, c.ID, c.TargetURL, c.Port)
		}
	}

	// Export a recorder config
	if recConfig != nil {
		exportData, err := client.Recorder().ExportConfig(ctx, recConfig.ID)
		if err != nil {
			fmt.Printf("Export config returned: %v\n", err)
		} else {
			fmt.Printf("Exported recorder config: %d bytes\n", len(exportData))
		}
	}

	// -----------------------------------------------------------------------
	// 2. CA Certificate operations
	// -----------------------------------------------------------------------
	fmt.Println("\n--- CA Certificate Operations ---")

	// Check current CA status
	caStatus, err := client.Recorder().GetCAStatus(ctx)
	if err != nil {
		fmt.Printf("CA status returned: %v\n", err)
	} else {
		fmt.Println("CA certificate status:")
		for key, val := range caStatus {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// Generate a new CA certificate (for HTTPS interception)
	err = client.Recorder().GenerateCA(ctx)
	if err != nil {
		fmt.Printf("Generate CA returned: %v\n", err)
		fmt.Println("(CA may already exist or feature may not be available)")
	} else {
		fmt.Println("CA certificate generated successfully")
	}

	// Download the CA certificate to install in your browser/OS trust store
	caCert, err := client.Recorder().DownloadCA(ctx)
	if err != nil {
		fmt.Printf("Download CA returned: %v\n", err)
	} else {
		fmt.Printf("Downloaded CA certificate: %d bytes\n", len(caCert))
		fmt.Println("Install this certificate in your OS/browser trust store for HTTPS recording")
	}

	// -----------------------------------------------------------------------
	// 3. Check available ports
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Available Ports ---")

	ports, err := client.Recorder().GetPorts(ctx)
	if err != nil {
		fmt.Printf("Get ports returned: %v\n", err)
	} else {
		fmt.Println("Available recorder proxy ports:")
		for key, val := range ports {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 4. Start a recording session
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Start Recording Session ---")

	session, err := client.Recorder().StartRecording(ctx, &mockarty.RecorderSession{
		Name:      "API Traffic Capture",
		TargetURL: "https://api.example.com",
	})
	if err != nil {
		fmt.Printf("Start recording returned: %v\n", err)
		fmt.Println("(Ensure Mockarty supports the recorder feature)")
		listExistingSessions(ctx, client)
		return
	}
	fmt.Printf("Recording session started: id=%s, status=%s\n", session.ID, session.Status)

	defer func() {
		_ = client.Recorder().StopRecording(ctx, session.ID)
		_ = client.Recorder().DeleteSession(ctx, session.ID)
		fmt.Println("\nRecording session cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 5. List recording sessions
	// -----------------------------------------------------------------------
	fmt.Println("\n--- List Recording Sessions ---")

	sessions, err := client.Recorder().ListSessions(ctx)
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}

	fmt.Printf("Found %d recording sessions:\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  - %s (id=%s, status=%s, entries=%d, target=%s)\n",
			s.Name, s.ID, s.Status, s.EntryCount, s.TargetURL)
	}

	// -----------------------------------------------------------------------
	// 6. Session modifications (request/response rewriting rules)
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Session Modifications ---")

	// Set up modifications that alter traffic as it passes through the recorder.
	// This is useful for redacting sensitive data or injecting test headers.
	err = client.Recorder().UpdateModifications(ctx, session.ID, map[string]any{
		"requestHeaders": map[string]any{
			"add": map[string]string{
				"X-Test-Header": "recorded-via-mockarty",
			},
			"remove": []string{"Authorization"},
		},
		"responseHeaders": map[string]any{
			"add": map[string]string{
				"X-Recorded": "true",
			},
		},
	})
	if err != nil {
		fmt.Printf("Update modifications returned: %v\n", err)
	} else {
		fmt.Println("Session modifications configured")
	}

	// Get current modifications
	mods, err := client.Recorder().GetModifications(ctx, session.ID)
	if err != nil {
		fmt.Printf("Get modifications returned: %v\n", err)
	} else {
		fmt.Println("Current modifications:")
		for key, val := range mods {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}

	// -----------------------------------------------------------------------
	// 7. Get recorded entries and annotate/replay them
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Recorded Entries ---")

	fmt.Println("Recording traffic... (send requests to the proxy to capture them)")
	time.Sleep(2 * time.Second)

	entries, err := client.Recorder().GetEntries(ctx, session.ID)
	if err != nil {
		fmt.Printf("Get entries returned: %v\n", err)
	} else {
		fmt.Printf("Found %d recorded entries:\n", len(entries))
		for _, e := range entries {
			ts := "unknown"
			if e.Timestamp > 0 {
				ts = time.Unix(e.Timestamp/1000, 0).Format(time.RFC3339)
			}
			fmt.Printf("  - %s %s -> %d (%d ms, at %s)\n",
				e.Method, e.Path, e.StatusCode, e.Duration, ts)
		}

		// Annotate and replay entries if available
		if len(entries) > 0 {
			entryID := entries[0].ID

			// Annotate an entry with notes for team collaboration
			fmt.Println("\n--- Annotate Entry ---")
			err = client.Recorder().AnnotateEntry(ctx, session.ID, entryID, map[string]any{
				"note":     "This endpoint returns user profile data",
				"priority": "high",
				"tags":     []string{"user-api", "profile"},
			})
			if err != nil {
				fmt.Printf("Annotate entry returned: %v\n", err)
			} else {
				fmt.Println("Entry annotated successfully")
			}

			// Replay a recorded request to verify it still works
			fmt.Println("\n--- Replay Entry ---")
			err = client.Recorder().ReplayEntry(ctx, session.ID, entryID)
			if err != nil {
				fmt.Printf("Replay entry returned: %v\n", err)
			} else {
				fmt.Println("Entry replayed successfully")
			}
		}
	}

	// -----------------------------------------------------------------------
	// 8. Create mocks from recorded traffic
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Create Mocks from Recording ---")

	createReq := map[string]any{
		"namespace": "sandbox",
		"prefix":    "recorded-",
	}

	mocks, err := client.Recorder().CreateMocksFromSession(ctx, session.ID, createReq)
	if err != nil {
		fmt.Printf("Create mocks returned: %v\n", err)
		fmt.Println("(This is expected if no entries were recorded)")
	} else {
		fmt.Printf("Created %d mocks from recorded traffic:\n", len(mocks))
		for _, m := range mocks {
			route := ""
			if m.HTTP != nil {
				route = m.HTTP.Route
			}
			fmt.Printf("  - %s (%s)\n", m.ID, route)
		}
	}

	// -----------------------------------------------------------------------
	// 9. Restart recording
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Restart Recording ---")

	err = client.Recorder().RestartRecording(ctx, session.ID)
	if err != nil {
		fmt.Printf("Restart recording returned: %v\n", err)
	} else {
		fmt.Println("Recording restarted (entries cleared, recording resumed)")
	}

	// -----------------------------------------------------------------------
	// 10. Stop recording and export
	// -----------------------------------------------------------------------
	fmt.Println("\n--- Stop Recording ---")

	if err := client.Recorder().StopRecording(ctx, session.ID); err != nil {
		fmt.Printf("Stop recording returned: %v\n", err)
	} else {
		fmt.Println("Recording stopped.")
	}

	fmt.Println("\n--- Export Session ---")

	harData, err := client.Recorder().ExportSession(ctx, session.ID)
	if err != nil {
		fmt.Printf("Export returned: %v\n", err)
	} else {
		fmt.Printf("Exported session as HAR: %d bytes\n", len(harData))
	}

	fmt.Println("\nRecorder examples completed!")
}

// listExistingSessions demonstrates working with already-existing sessions
// when creating new ones is not available.
func listExistingSessions(ctx context.Context, client *mockarty.Client) {
	fmt.Println("\n--- Listing Existing Sessions ---")

	sessions, err := client.Recorder().ListSessions(ctx)
	if err != nil {
		fmt.Printf("List sessions returned: %v\n", err)
		return
	}

	fmt.Printf("Found %d existing sessions:\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  - %s (id=%s, status=%s, entries=%d)\n",
			s.Name, s.ID, s.Status, s.EntryCount)
	}
}
