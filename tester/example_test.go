// Copyright (c) 2026 Mockarty. All rights reserved.

package tester_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/mockarty/mockarty-go/tester"
)

// Example shows the canonical chain shape: login → extract token →
// POST a downstream call carrying the token. The Tester executes
// lazily on the first Expect / Extract in each chain.
func Example() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok-abc"})
		case "/me":
			if r.Header.Get("Authorization") != "Bearer tok-abc" {
				http.Error(w, "unauthorized", 401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"user": "alice"})
		}
	}))
	defer srv.Close()

	t := tester.New(
		tester.WithBaseURL(srv.URL),
		tester.WithContext(context.Background()),
	)
	t.HTTP().GET("/login").
		ExpectStatus(200).
		Extract("$.token", "token")
	t.HTTP().GET("/me").
		Header("Authorization", "Bearer {{token}}").
		ExpectStatus(200).
		ExpectJSONPath("$.user", "alice")
	t.Finish()

	fmt.Println("ok:", t.OK(), "steps:", len(t.Report()))
	// Output: ok: true steps: 2
}
