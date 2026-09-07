package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	namespace := os.Getenv("MOCKARTY_NAMESPACE")
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"),
		mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")),
		mockarty.WithNamespace(namespace))
	snapshot, err := client.Connections().Create(context.Background(), mockarty.ConnectionDescriptor{
		ContractVersion: "mockarty.connection/v1",
		Namespace:       namespace,
		ID:              "gitlab-prod",
		Kind:            "gitlab",
		Endpoint:        "https://gitlab.example.com/api/v4",
		TargetPolicy: mockarty.ConnectionTargetPolicy{
			Schemes: []string{"https"}, Hosts: []string{"gitlab.example.com"},
			Ports: []uint16{443}, PathPrefixes: []string{"/api/v4"},
		},
		SecretRefs:          []mockarty.ConnectionSecretRef{{StoreID: "team-vault", Key: "gitlab-token", Version: 3}},
		AllowedOperationIDs: []string{"gitlab.pipeline.observe"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("connection revision %d: %s\n", snapshot.Descriptor.Revision, snapshot.Digest)
}
