// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: grpc_client — call a gRPC service (real or Mockarty-mocked)
// from a CI test script with auto-step capture into a Mockarty TCM
// external run.
//
// Two flavours of the same call are shown:
//
//  1. Reflection-driven — server speaks grpc.reflection, the client
//     discovers the method shape at runtime.
//  2. .proto-file driven — air-gapped / hardened servers without
//     reflection; the test points the client at the schema file.
//
// Steps end up in a TCM external run via externalruns.Client. Run from
// CI, the timeline shows per-RPC duration / status / error message in
// the Mockarty admin UI.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
	mgrpc "github.com/mockarty/mockarty-go/protocols/grpc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminBase := getenv("MOCKARTY_BASE_URL", "http://localhost:5770")
	apiToken := getenv("MOCKARTY_API_TOKEN", "your-api-key")
	namespace := getenv("MOCKARTY_NAMESPACE", "sandbox")
	grpcTarget := getenv("GRPC_TARGET", "localhost:50051")

	// 1) Open an external run on the admin so per-call steps stream
	//    into the TCM timeline. In a real CI job you'd open it before
	//    the suite and Close at the end.
	runsClient, err := externalruns.NewClient(adminBase, namespace, apiToken)
	if err != nil {
		log.Fatalf("externalruns.NewClient: %v", err)
	}
	run, err := runsClient.CreateRun(ctx, externalruns.CreateRunRequest{
		Name:      "grpc smoke",
		Framework: "go-sdk-grpc-example",
	})
	if err != nil {
		log.Fatalf("CreateRun: %v", err)
	}
	defer func() {
		_ = runsClient.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{})
	}()

	recorder := mgrpc.NewExternalRunsRecorder(runsClient, run.ID)
	defer recorder.Close()

	// 2) Dial — reflection on by default, plaintext for local. Flip
	//    WithReflection(false) + WithProtoFile(...) for air-gapped
	//    servers without grpc.reflection.
	conn, err := mgrpc.Dial(ctx, grpcTarget,
		mgrpc.WithRecorder(recorder),
		mgrpc.WithMetadata(map[string]string{
			"x-tenant": "demo",
			"x-job":    os.Getenv("CI_JOB_ID"),
		}),
	)
	if err != nil {
		log.Fatalf("mgrpc.Dial: %v", err)
	}
	defer conn.Close()

	// 3) Discovery — sanity-check the surface we're about to exercise.
	services, err := conn.ListServices(ctx)
	if err != nil {
		log.Printf("ListServices (continuing): %v", err)
	} else {
		fmt.Printf("server exposes %d services\n", len(services))
		for _, s := range services {
			fmt.Printf("  - %s\n", s)
		}
	}

	// 4) Unary invoke — JSON in, JSON out. Method name is the gRPC
	//    fully-qualified form "package.Service/Method".
	var resp map[string]any
	err = conn.InvokeJSON(ctx,
		"acme.UserService/GetUser",
		map[string]any{"id": "u-42"},
		&resp,
	)
	if err != nil {
		log.Printf("GetUser failed: %v", err)
	} else {
		fmt.Printf("GetUser → %+v\n", resp)
	}

	// 5) Air-gapped flavour: point at .proto file when the server
	//    doesn't expose reflection. Uncomment + tweak when needed.
	//
	//	connAir, err := mgrpc.Dial(ctx, grpcTarget,
	//	    mgrpc.WithRecorder(recorder),
	//	    mgrpc.WithReflection(false),
	//	    mgrpc.WithProtoFile("acme/user.proto"),
	//	    mgrpc.WithImportDir("./protos", "./third_party"),
	//	)
	//	defer connAir.Close()
	//	_ = connAir.InvokeJSON(ctx, "acme.UserService/GetUser", ...)
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
