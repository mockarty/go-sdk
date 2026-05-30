// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the Mockarty SDK License Agreement. See LICENSE file for details.

// Example: kafka_client — produce + consume against a Kafka broker
// (real or Mockarty-mocked) with auto-step capture into a TCM run.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mockarty/mockarty-go/externalruns"
	"github.com/mockarty/mockarty-go/protocols/kafka"
	"github.com/mockarty/mockarty-go/protocols/telemetry"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminBase := getenv("MOCKARTY_BASE_URL", "http://localhost:5770")
	apiToken := getenv("MOCKARTY_API_TOKEN", "your-api-key")
	namespace := getenv("MOCKARTY_NAMESPACE", "sandbox")
	broker := getenv("KAFKA_BROKER", "localhost:9092")
	topic := getenv("KAFKA_TOPIC", "orders")

	runs, err := externalruns.NewClient(adminBase, namespace, apiToken)
	if err != nil {
		log.Fatalf("externalruns.NewClient: %v", err)
	}
	run, err := runs.CreateRun(ctx, externalruns.CreateRunRequest{
		Name:      "kafka smoke",
		Framework: "go-sdk-kafka-example",
	})
	if err != nil {
		log.Fatalf("CreateRun: %v", err)
	}
	defer func() {
		_ = runs.FinishRun(ctx, run.ID, externalruns.FinishRunRequest{})
	}()

	recorder := telemetry.NewExternalRunsRecorder(runs, run.ID)
	defer recorder.Close()

	cli, err := kafka.NewClient([]string{broker},
		kafka.WithRecorder(recorder),
		kafka.WithAutoTopicCreation(true),
	)
	if err != nil {
		log.Fatalf("kafka.NewClient: %v", err)
	}
	defer cli.Close()

	if err := cli.Produce(ctx, topic, "order-42", map[string]any{
		"id":     42,
		"amount": 19.99,
	}, map[string]string{"x-tenant": "demo"}); err != nil {
		log.Fatalf("Produce: %v", err)
	}

	msgs, err := cli.Consume(ctx, kafka.ConsumeOptions{
		Topic:       topic,
		GroupID:     "smoke-" + run.ID,
		MaxMessages: 1,
	})
	if err != nil {
		log.Fatalf("Consume: %v", err)
	}
	for _, m := range msgs {
		fmt.Printf("got message: key=%s partition=%d offset=%d body=%s\n",
			m.Key, m.Partition, m.Offset, string(m.Value))
	}
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
