package externalruns

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSelectedCIIncrementalWritesRefuseBeforeHTTP(t *testing.T) {
	t.Setenv("MOCKARTY_CI_CONTEXT", "private-context")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.Write([]byte(`{}`)) }))
	defer server.Close()
	client, err := NewClient(server.URL, "qa", "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	writes := map[string]func() error{
		"create": func() error {
			_, e := client.CreateRun(context.Background(), CreateRunRequest{Name: "test", Framework: "go"})
			return e
		},
		"steps": func() error {
			return client.AddSteps(context.Background(), "run-1", []Step{{StepKey: "step", Name: "step", Status: StatusPassed}})
		},
		"attachment": func() error {
			return client.AttachReport(context.Background(), "run-1", "proof.txt", []byte("proof"), "text/plain")
		},
		"finish": func() error {
			return client.FinishRun(context.Background(), "run-1", FinishRunRequest{Status: StatusPassed})
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			if e := write(); !errors.Is(e, ErrInvalidRequest) {
				t.Fatalf("selected incremental write must refuse: %v", e)
			}
		})
	}
	if calls != 0 {
		t.Fatal("unsupported selected lifecycle reached HTTP")
	}
	// Read operations continue to use the ordinary authenticated namespace API.
	if _, e := client.GetRun(context.Background(), "run-1"); e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatal("readonly lifecycle behavior changed")
	}
}
