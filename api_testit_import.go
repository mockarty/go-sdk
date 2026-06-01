package mockarty

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrTestITImportFailed is returned when a Test IT import operation fails.
var ErrTestITImportFailed = errors.New("mockarty: testit import failed")

// TestITImport imports a Test IT bulk-export JSON into Mockarty's TCM domain.
// The export is the raw JSON obtained from Test IT's export function (WorkItems,
// Sections, Attributes, Configurations).  Returns a summary of what was created
// and any per-row errors (the import is best-effort; individual work-item
// failures are collected, not fatal).
//
// The underlying endpoint is POST /api/v1/namespaces/:ns/tcm/import/testit.
func (c *Client) TestITImport(ctx context.Context, namespace string, exportJSON json.RawMessage) (*TestITImportResult, error) {
	if len(exportJSON) == 0 {
		return nil, fmt.Errorf("%w: export JSON is empty", ErrTestITImportFailed)
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/tcm/import/testit", namespace)
	var result TestITImportResult
	if err := c.do(ctx, "POST", path, exportJSON, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TestITImportResult is the response from a Test IT bulk import.
type TestITImportResult struct {
	Created        int      `json:"created"`
	Updated        int      `json:"updated"`
	Placed         int      `json:"placed"`
	Failed         int      `json:"failed"`
	ConfigsCreated int      `json:"configsCreated"`
	Errors         []string `json:"errors,omitempty"`
}
