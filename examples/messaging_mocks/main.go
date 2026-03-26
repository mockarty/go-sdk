// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: messaging_mocks — Kafka, RabbitMQ, and SMTP mock examples.
//
// This example demonstrates:
//   - Kafka topic mock with output routing
//   - RabbitMQ queue mock with exchange and routing key
//   - SMTP mock with sender/recipient conditions
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
		"kafka-order-events",
		"rabbitmq-notifications",
		"smtp-welcome-email",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll messaging example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. Kafka topic mock
	// -----------------------------------------------------------------------
	// Consumes from "order.created" topic and publishes a response
	// to "order.confirmed" topic.
	kafkaMock := mockarty.NewMockBuilder().
		ID("kafka-order-events").
		KafkaConfig(func(k *mockarty.KafkaBuilder) {
			k.Topic("order.created").
				ServerName("kafka-main").
				ConsumerGroup("mockarty-consumer").
				OutputTopic("order.confirmed").
				OutputBrokers("localhost:9092").
				OutputKey("$.body.orderId").
				OutputHeaders(map[string]string{
					"x-correlation-id": "$.fake.UUID",
					"x-source":         "mockarty",
				}).
				BodyCondition("$.body.type", mockarty.AssertEquals, "NEW_ORDER")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"orderId":           "$.body.orderId",
				"status":            "CONFIRMED",
				"confirmedAt":       "$.fake.DateISO",
				"estimatedDelivery": "$.fake.Date",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, kafkaMock); err != nil {
		log.Fatalf("Failed to create Kafka mock: %v", err)
	}
	fmt.Println("[1] Created Kafka mock: order.created -> order.confirmed")

	// -----------------------------------------------------------------------
	// 2. RabbitMQ queue mock
	// -----------------------------------------------------------------------
	// Consumes from "notifications" queue and publishes a response
	// to the "notification.results" queue.
	rabbitMock := mockarty.NewMockBuilder().
		ID("rabbitmq-notifications").
		RabbitMQConfig(func(r *mockarty.RabbitMQBuilder) {
			r.Queue("notifications").
				Exchange("notifications.exchange").
				RoutingKey("notification.send").
				ServerName("rabbit-main").
				OutputURL("amqp://guest:guest@localhost:5672/").
				OutputExchange("notifications.exchange").
				OutputRoutingKey("notification.result").
				OutputQueue("notification.results").
				OutputProps(&mockarty.RabbitMQOutputProps{
					ContentType:   "application/json",
					DeliveryMode:  2, // persistent
					CorrelationID: "$.body.correlationId",
				}).
				BodyCondition("$.body.channel", mockarty.AssertEquals, "email")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.JSONBody(map[string]any{
				"notificationId": "$.fake.UUID",
				"channel":        "email",
				"status":         "SENT",
				"sentAt":         "$.fake.DateISO",
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, rabbitMock); err != nil {
		log.Fatalf("Failed to create RabbitMQ mock: %v", err)
	}
	fmt.Println("[2] Created RabbitMQ mock: notifications -> notification.results")

	// -----------------------------------------------------------------------
	// 3. SMTP mock
	// -----------------------------------------------------------------------
	// Matches emails sent to new users and returns a mock SMTP response.
	smtpMock := mockarty.NewMockBuilder().
		ID("smtp-welcome-email").
		SMTPConfig(func(s *mockarty.SMTPBuilder) {
			s.ServerName("smtp-mock").
				SenderCondition("", mockarty.AssertContains, "noreply@").
				RecipientCondition("", mockarty.AssertNotEmpty, nil).
				SubjectCondition("", mockarty.AssertContains, "Welcome")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(250). // SMTP 250 OK
					JSONBody(map[string]any{
					"messageId": "$.fake.UUID",
					"status":    "accepted",
					"message":   "Message queued for delivery",
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, smtpMock); err != nil {
		log.Fatalf("Failed to create SMTP mock: %v", err)
	}
	fmt.Println("[3] Created SMTP mock: welcome email handler")

	fmt.Println("\nAll messaging mock examples created successfully!")
}
