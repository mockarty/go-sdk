// Copyright (c) 2024-2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: sse_mocks — SSE (Server-Sent Events) mock examples.
//
// This example demonstrates:
//   - SSE event chain with multiple events
//   - Looping events with heartbeat
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
		"sse-order-updates",
		"sse-metrics-stream",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll SSE example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. SSE event chain with multiple events
	// -----------------------------------------------------------------------
	// Simulates an order status update stream: order received -> processing
	// -> shipped -> delivered, each with a delay between events.
	orderSSE := mockarty.NewMockBuilder().
		ID("sse-order-updates").
		SSEConfig(func(s *mockarty.SSEBuilder) {
			s.EventPath("/events/orders").
				EventName("order-update").
				Description("Order status update stream")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.SSEChain(&mockarty.SSEEventChain{
				Events: []mockarty.SSEEvent{
					{
						EventName: "order.received",
						ID:        "evt-1",
						Data: map[string]any{
							"orderId": "ORD-12345",
							"status":  "RECEIVED",
							"message": "Order has been received",
						},
						Delay: 0, // immediate
					},
					{
						EventName: "order.processing",
						ID:        "evt-2",
						Data: map[string]any{
							"orderId": "ORD-12345",
							"status":  "PROCESSING",
							"message": "Order is being processed",
						},
						Delay: 1000, // 1 second after previous event
					},
					{
						EventName: "order.shipped",
						ID:        "evt-3",
						Data: map[string]any{
							"orderId":    "ORD-12345",
							"status":     "SHIPPED",
							"message":    "Order has been shipped",
							"trackingId": "TRK-98765",
						},
						Delay: 2000, // 2 seconds after previous event
					},
					{
						EventName: "order.delivered",
						ID:        "evt-4",
						Data: map[string]any{
							"orderId": "ORD-12345",
							"status":  "DELIVERED",
							"message": "Order has been delivered",
						},
						Delay: 3000, // 3 seconds after previous event
					},
				},
				Loop: false,
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, orderSSE); err != nil {
		log.Fatalf("Failed to create order SSE mock: %v", err)
	}
	fmt.Println("[1] Created SSE mock: /events/orders (4-event chain)")

	// -----------------------------------------------------------------------
	// 2. Looping SSE events with heartbeat
	// -----------------------------------------------------------------------
	// Simulates a metrics stream that loops every 5 seconds and sends
	// a heartbeat every 10 seconds. Limited to 100 loops and 600 seconds.
	metricsSSE := mockarty.NewMockBuilder().
		ID("sse-metrics-stream").
		SSEConfig(func(s *mockarty.SSEBuilder) {
			s.EventPath("/events/metrics").
				EventName("metrics").
				Description("System metrics stream")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.SSEChain(&mockarty.SSEEventChain{
				Events: []mockarty.SSEEvent{
					{
						EventName: "cpu",
						Data: map[string]any{
							"metric": "cpu_usage",
							"value":  "$.fake.Float64",
							"unit":   "percent",
						},
						Delay: 0,
					},
					{
						EventName: "memory",
						Data: map[string]any{
							"metric": "memory_usage",
							"value":  "$.fake.Float64",
							"unit":   "percent",
						},
						Delay: 1000,
					},
					{
						EventName: "disk",
						Data: map[string]any{
							"metric": "disk_usage",
							"value":  "$.fake.Float64",
							"unit":   "percent",
						},
						Delay: 1000,
					},
				},
				Loop:      true,
				LoopDelay: 5000,  // 5 seconds between loop iterations
				Heartbeat: 10000, // heartbeat every 10 seconds
				MaxLoops:  100,   // stop after 100 iterations
				MaxTime:   600,   // or stop after 600 seconds (10 minutes)
			})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, metricsSSE); err != nil {
		log.Fatalf("Failed to create metrics SSE mock: %v", err)
	}
	fmt.Println("[2] Created SSE mock: /events/metrics (looping with heartbeat)")

	fmt.Println("\nAll SSE mock examples created successfully!")
}
