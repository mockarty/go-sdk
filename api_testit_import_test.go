package mockarty

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTestITImport_EmptyExport(t *testing.T) {
	c := &Client{namespace: "sandbox"}
	_, err := c.TestITImport(context.Background(), "sandbox", nil)
	if err == nil {
		t.Fatal("expected error for empty export JSON")
	}
}

func TestTestITImportResult_JSONRoundTrip(t *testing.T) {
	raw := `{"created":3,"updated":0,"placed":3,"failed":1,"configsCreated":2,"errors":["work item 42: empty name"]}`
	var result TestITImportResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Created != 3 || result.Failed != 1 || result.ConfigsCreated != 2 {
		t.Errorf("wrong counts: %+v", result)
	}
	if len(result.Errors) != 1 || result.Errors[0] != "work item 42: empty name" {
		t.Errorf("wrong errors: %v", result.Errors)
	}
}
