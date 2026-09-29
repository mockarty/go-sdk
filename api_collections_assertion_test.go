// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"testing"
)

func TestCollectionAssertionRetainsSkipped(t *testing.T) {
	var result TestResult
	if err := json.Unmarshal([]byte(`{"assertions":[{"name":"optional","passed":false,"skipped":true}]}`), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Assertions) != 1 {
		t.Fatal("assertion lost")
	}
	raw, err := json.Marshal(result.Assertions[0])
	if err != nil {
		t.Fatal(err)
	}
	var assertion map[string]interface{}
	if err := json.Unmarshal(raw, &assertion); err != nil {
		t.Fatal(err)
	}
	if assertion["skipped"] != true {
		t.Fatalf("skipped flag lost: %s", raw)
	}
}
