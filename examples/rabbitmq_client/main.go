// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: rabbitmq_client — declare + publish + consume against a
// RabbitMQ broker (real or Mockarty-mocked) with auto-step capture.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
	"github.com/mockarty/mockarty-go/protocols/rabbitmq"
	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminBase := getenv("MOCKARTY_BASE_URL", "http://localhost:5770")
	apiToken := getenv("MOCKARTY_API_TOKEN", "your-api-key")
	namespace := getenv("MOCKARTY_NAMESPACE", "sandbox")
	amqpURL := getenv("AMQP_URL", "amqp://guest:guest@localhost:5672/")
	queue := getenv("QUEUE", "orders")

	runs, err := externalruns.NewClient(adminBase, namespace, apiToken)
	if err != nil {
		log.Fatalf("externalruns.NewClient: %v", err)
	}
	run, err := runs.CreateRun(ctx, externalruns.CreateRunRequest{
		Name:      "rabbitmq smoke",
		Framework: "go-sdk-rabbitmq-example",
	})
	if err != nil {
		log.Fatalf("CreateRun: %v", err)
	}
	defer func() {
		_ = runs.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{})
	}()

	recorder := telemetry.NewExternalRunsRecorder(runs, run.ID)
	defer recorder.Close()

	cli, err := rabbitmq.NewClient(amqpURL, rabbitmq.WithRecorder(recorder))
	if err != nil {
		log.Fatalf("rabbitmq.NewClient: %v", err)
	}
	defer cli.Close()

	if err := cli.DeclareQueue(ctx, queue, rabbitmq.DeclareQueueOptions{Durable: true}); err != nil {
		log.Fatalf("DeclareQueue: %v", err)
	}
	if err := cli.Publish(ctx, "", queue, map[string]any{
		"id":     42,
		"status": "ready",
	}, nil); err != nil {
		log.Fatalf("Publish: %v", err)
	}
	msgs, err := cli.Consume(ctx, rabbitmq.ConsumeOptions{
		Queue:       queue,
		MaxMessages: 1,
	})
	if err != nil {
		log.Fatalf("Consume: %v", err)
	}
	for _, m := range msgs {
		fmt.Printf("got message: routing=%s body=%s\n", m.RoutingKey, string(m.Body))
	}
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
