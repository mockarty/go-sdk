// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package mockarty

import (
	"encoding/json"
	"testing"
)

// A Mock can now carry an S3 (object-storage) request context — the SDK was
// missing the type entirely, so an S3 mock authored via the SDK silently
// dropped its bucket/rules. Assert the wire shape round-trips under the "s3"
// key the server expects.
func TestMock_S3RequestContext_RoundTrips(t *testing.T) {
	m := Mock{
		ID: "s3-mock",
		S3: &S3RequestContext{
			Bucket:    "reports",
			KeyPrefix: "2026/",
			Endpoint:  "s3.local",
			Rules: []S3OperationRule{
				{
					ID:        "r1",
					Operation: "GetObject",
					Priority:  10,
					Enabled:   true,
					Match:     S3RuleMatch{KeyPattern: "*.json"},
					Response: S3RuleResponse{
						Type:        "inline",
						InlineBody:  "eyJvayI6dHJ1ZX0=",
						ContentType: "application/json",
						UserMeta:    map[string]string{"team": "qa"},
					},
				},
			},
			Defaults: &S3Defaults{
				PassthroughToBackend: false,
				DefaultErrorCode:     "NoSuchKey",
				Owner:                &S3Owner{ID: "mockarty", DisplayName: "Mockarty"},
				ExtraBuckets:         []string{"logs", "tmp"},
			},
			Toxicity: []ToxicityRule{
				{Type: "latency", Probability: 50, Params: map[string]interface{}{"ms": 200},
					Scope: ToxicityScope{Operations: []string{"GetObject"}}},
			},
		},
	}

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	// The server keys the S3 context under "s3".
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if _, ok := probe["s3"]; !ok {
		t.Fatalf("marshalled mock must carry the 's3' key: %s", raw)
	}

	var back Mock
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.S3 == nil {
		t.Fatal("S3 context lost on round-trip")
	}
	if back.S3.Bucket != "reports" || back.S3.KeyPrefix != "2026/" {
		t.Fatalf("top-level S3 fields lost: %+v", back.S3)
	}
	if len(back.S3.Rules) != 1 || back.S3.Rules[0].Operation != "GetObject" ||
		!back.S3.Rules[0].Enabled || back.S3.Rules[0].Response.Type != "inline" {
		t.Fatalf("S3 rule lost: %+v", back.S3.Rules)
	}
	if back.S3.Rules[0].Response.UserMeta["team"] != "qa" {
		t.Fatalf("rule UserMeta lost: %+v", back.S3.Rules[0].Response)
	}
	if back.S3.Defaults == nil || back.S3.Defaults.Owner == nil ||
		back.S3.Defaults.Owner.ID != "mockarty" || len(back.S3.Defaults.ExtraBuckets) != 2 {
		t.Fatalf("S3 defaults lost: %+v", back.S3.Defaults)
	}
	if len(back.S3.Toxicity) != 1 || back.S3.Toxicity[0].Type != "latency" {
		t.Fatalf("S3 toxicity lost: %+v", back.S3.Toxicity)
	}
}
