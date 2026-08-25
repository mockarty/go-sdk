package mockarty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNamespaceSettingsAutonomyRetentionRoundTrip(t *testing.T) {
	var putBodies []map[string]any
	var putHeaders []http.Header
	etag := `"` + strings.Repeat("a", 64) + `"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/autotester/settings" || (r.Method != http.MethodGet && r.Method != http.MethodPut) {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if r.Method == http.MethodPut {
			putHeaders = append(putHeaders, r.Header.Clone())
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("body=%+v err=%v", body, err)
			}
			putBodies = append(putBodies, body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"defaultAutonomy": "auto", "defaultBudget": map[string]any{"tokensTotal": 100},
			"defaultContextRefs": []map[string]string{{"kind": "spec", "value": "openapi.yaml"}}, "journalEventRetentionDays": 365,
			"journalPayloadRetentionDays": 30, "updatedAt": "2026-08-23T00:00:00Z", "etag": etag})
		// keep the wall in the merge document so a retention-only save cannot erase it
	}))
	defer server.Close()
	client := NewClient(server.URL)
	days := 365
	got, err := client.NamespaceSettings().SaveAutonomySettingsWithOptions(context.Background(),
		&AutonomyNamespaceSettings{JournalEventRetentionDays: &days}, AutonomySettingsSaveOptions{RequestID: "stable-save-1"})
	if err != nil || got.JournalPayloadRetentionDays == nil || *got.JournalPayloadRetentionDays != 30 {
		t.Fatalf("save=%+v err=%v", got, err)
	}
	if _, err := client.NamespaceSettings().GetAutonomySettings(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The merge carries the server's current non-nil override; it must not
	// accidentally send null or omit it from a full-replacement document.
	if got := putBodies[0]["journalPayloadRetentionDays"]; got != float64(30) {
		t.Fatalf("payload override not preserved, body=%v", putBodies[0])
	}
	// Additive run-wall support must be presence-safe and explicitly clearable.
	window := 90
	if _, err := client.NamespaceSettings().SaveAutonomySettingsWithOptions(context.Background(),
		&AutonomyNamespaceSettings{RunWindowMinutes: &window}, AutonomySettingsSaveOptions{RequestID: "window-save-1"}); err != nil {
		t.Fatal(err)
	}
	if putBodies[len(putBodies)-1]["runWindowMinutes"] != float64(90) {
		t.Fatalf("run window not sent: %v", putBodies[len(putBodies)-1])
	}
	if _, err := client.NamespaceSettings().ClearAutonomyRunWindow(context.Background(), AutonomySettingsSaveOptions{RequestID: "window-clear-1"}); err != nil {
		t.Fatal(err)
	}
	if putBodies[len(putBodies)-1]["runWindowMinutes"] != nil {
		t.Fatalf("run window clear must send null: %v", putBodies[len(putBodies)-1])
	}
	if putBodies[0]["defaultAutonomy"] != "auto" {
		t.Fatalf("partial retention save erased autonomy: %v", putBodies[0])
	}
	if putHeaders[0].Get("Idempotency-Key") != "stable-save-1" || putHeaders[0].Get("If-Match") != etag {
		t.Fatalf("conditional headers=%v", putHeaders[0])
	}
	budget, _ := putBodies[0]["defaultBudget"].(map[string]any)
	refs, _ := putBodies[0]["defaultContextRefs"].([]any)
	if budget["tokensTotal"] != float64(100) || len(refs) != 1 {
		t.Fatalf("partial retention save erased budget/refs: %v", putBodies[0])
	}
	if _, err := client.NamespaceSettings().SaveAutonomySettingsWithOptions(context.Background(),
		&AutonomyNamespaceSettings{DefaultBudget: AutonomyDefaultBudget{}},
		AutonomySettingsSaveOptions{RequestID: "clear-budget-1", ReplaceDefaultBudget: true}); err != nil {
		t.Fatal(err)
	}
	zeroBudget, _ := putBodies[3]["defaultBudget"].(map[string]any)
	if zeroBudget["tokensTotal"] != float64(0) || zeroBudget["tokensPerDay"] != float64(0) || zeroBudget["usdCap"] != float64(0) {
		t.Fatalf("explicit zero budget was not sent: %v", putBodies[3])
	}
	if putHeaders[3].Get("Idempotency-Key") != "clear-budget-1" {
		t.Fatalf("stable zero-budget request id not sent: %v", putHeaders[3])
	}
	if _, err := client.NamespaceSettings().ClearAutonomyRetentionWithOptions(context.Background(), true, false,
		AutonomySettingsSaveOptions{RequestID: "stable-clear-1"}); err != nil {
		t.Fatal(err)
	}
	if got := putBodies[4]["journalEventRetentionDays"]; got != nil {
		t.Fatalf("explicit clear must send null, body=%v", putBodies[4])
	}
	if got := putBodies[4]["journalPayloadRetentionDays"]; got != float64(30) {
		t.Fatalf("unselected override must be preserved, body=%v", putBodies[4])
	}
	if putHeaders[4].Get("Idempotency-Key") != "stable-clear-1" {
		t.Fatalf("stable clear request id not sent: %v", putHeaders[4])
	}
	if _, err := client.NamespaceSettings().ClearAutonomyRetention(context.Background(), false, false); err == nil {
		t.Fatal("empty clear selection must fail")
	}
}
