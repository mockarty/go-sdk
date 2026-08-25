// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_CLOUD_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_CLOUD_TOKEN")))
	projection, err := client.CloudEntitlements().Get(context.Background(), os.Getenv("MOCKARTY_SPACE_ID"))
	if err != nil {
		panic(err)
	}
	// This is an unsigned inspection projection. Do not use it as an offline
	// licence or as a substitute for a server-side authorization decision.
	fmt.Printf("space=%s plan=%s revision=%d\n", projection.Snapshot.SpaceID, projection.Snapshot.Plan, projection.Revision)
}
