// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: callbacks — Webhook callback examples.
//
// This example demonstrates:
//   - HTTP callback on mock hit
//   - Kafka callback
//   - RabbitMQ callback
//   - Callback triggers (on_success, on_error, always)
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := mockarty.NewClient("http://localhost:5770",
		mockarty.WithAPIKey("your-api-key"),
		mockarty.WithNamespace("sandbox"),
	)

	mockIDs := []string{
		"callback-http",
		"callback-kafka",
		"callback-rabbitmq",
		"callback-triggers",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll callback example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. HTTP callback on mock hit
	// -----------------------------------------------------------------------
	// When this mock is called, it also fires a POST request to the
	// configured webhook URL with the request details.
	httpCallbackMock := mockarty.NewMockBuilder().
		ID("callback-http").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/orders").Method("POST")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(201).JSONBody(map[string]any{
				"orderId": "$.fake.UUID",
				"status":  "CREATED",
			})
		}).
		Callbacks(mockarty.Callback{
			Type:   mockarty.CallbackTypeHTTP,
			URL:    "https://webhook.example.com/order-created",
			Method: "POST",
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer webhook-secret-token",
			},
			Body: map[string]any{
				"event":   "order.created",
				"orderId": "$.fake.UUID",
				"payload": "$.body",
			},
			Timeout:    5000, // 5 second timeout
			RetryCount: 3,    // retry up to 3 times
			RetryDelay: 1000, // 1 second between retries
			Async:      true, // don't block the mock response
			Trigger:    mockarty.TriggerOnSuccess,
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, httpCallbackMock); err != nil {
		log.Fatalf("Failed to create HTTP callback mock: %v", err)
	}
	fmt.Println("[1] Created mock with HTTP callback (POST to webhook URL)")

	// -----------------------------------------------------------------------
	// 2. Kafka callback
	// -----------------------------------------------------------------------
	// When this mock is hit, a message is published to a Kafka topic.
	kafkaCallbackMock := mockarty.NewMockBuilder().
		ID("callback-kafka").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/payments").Method("POST")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"paymentId": "$.fake.UUID",
				"status":    "PROCESSED",
			})
		}).
		Callbacks(mockarty.Callback{
			Type:         mockarty.CallbackTypeKafka,
			KafkaBrokers: "localhost:9092",
			KafkaTopic:   "payment.events",
			KafkaKey:      "$.body.paymentId",
			Body: map[string]any{
				"event":     "payment.processed",
				"paymentId": "$.body.paymentId",
				"amount":    "$.body.amount",
				"currency":  "$.body.currency",
			},
			Trigger: mockarty.TriggerOnSuccess,
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, kafkaCallbackMock); err != nil {
		log.Fatalf("Failed to create Kafka callback mock: %v", err)
	}
	fmt.Println("[2] Created mock with Kafka callback (publish to payment.events)")

	// -----------------------------------------------------------------------
	// 3. RabbitMQ callback
	// -----------------------------------------------------------------------
	// When this mock is hit, a message is published to a RabbitMQ exchange.
	rabbitCallbackMock := mockarty.NewMockBuilder().
		ID("callback-rabbitmq").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/users/:id/notify").Method("POST")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"notified": true,
			})
		}).
		Callbacks(mockarty.Callback{
			Type:             mockarty.CallbackTypeRabbitMQ,
			RabbitURL:        "amqp://guest:guest@localhost:5672/",
			RabbitExchange:   "notifications",
			RabbitRoutingKey: "user.notification",
			Body: map[string]any{
				"userId":  "$.pathParam.id",
				"type":    "$.body.type",
				"message": "$.body.message",
			},
			Trigger: mockarty.TriggerAlways, // fire even if mock returns an error
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, rabbitCallbackMock); err != nil {
		log.Fatalf("Failed to create RabbitMQ callback mock: %v", err)
	}
	fmt.Println("[3] Created mock with RabbitMQ callback (publish to notifications exchange)")

	// -----------------------------------------------------------------------
	// 4. Multiple callbacks with different triggers
	// -----------------------------------------------------------------------
	// This mock has three callbacks, each fired under different conditions:
	//   - on_success: notify analytics
	//   - on_error: alert ops team
	//   - always: audit log
	multiCallbackMock := mockarty.NewMockBuilder().
		ID("callback-triggers").
		HTTP(func(h *mockarty.HTTPBuilder) {
			h.Route("/api/critical-operation").Method("POST")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).JSONBody(map[string]any{
				"result": "completed",
			})
		}).
		Callbacks(
			// Fires only when the mock responds successfully
			mockarty.Callback{
				Type:    mockarty.CallbackTypeHTTP,
				URL:     "https://analytics.example.com/events",
				Method:  "POST",
				Trigger: mockarty.TriggerOnSuccess,
				Body: map[string]any{
					"event": "operation.success",
				},
				Async: true,
			},
			// Fires only when an error occurs
			mockarty.Callback{
				Type:    mockarty.CallbackTypeHTTP,
				URL:     "https://alerts.example.com/ops",
				Method:  "POST",
				Trigger: mockarty.TriggerOnError,
				Body: map[string]any{
					"event":    "operation.failed",
					"severity": "HIGH",
				},
				Async: true,
			},
			// Fires on every mock hit regardless of outcome
			mockarty.Callback{
				Type:    mockarty.CallbackTypeHTTP,
				URL:     "https://audit.example.com/log",
				Method:  "POST",
				Trigger: mockarty.TriggerAlways,
				Body: map[string]any{
					"event":     "operation.attempted",
					"timestamp": "$.fake.DateISO",
				},
				Async: true,
			},
		).
		Build()

	if _, err := client.Mocks().Create(ctx, multiCallbackMock); err != nil {
		log.Fatalf("Failed to create multi-callback mock: %v", err)
	}
	fmt.Println("[4] Created mock with 3 callbacks (on_success, on_error, always)")

	fmt.Println("\nAll callback examples created successfully!")
}
