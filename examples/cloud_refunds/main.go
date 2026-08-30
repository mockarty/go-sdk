package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_TOKEN")))
	// The API token needs the exact operator:commerce:write scope.
	refunds, err := client.CloudRefunds().ListRefunds(context.Background())
	if err != nil {
		panic(err)
	}
	operationID := os.Getenv("REFUND_OPERATION_ID")
	var selected *mockarty.CloudRefundIncident
	for i := range refunds {
		if refunds[i].OperationID == operationID {
			selected = &refunds[i]
			break
		}
	}
	if selected == nil || os.Getenv("REFUND_IDEMPOTENCY_KEY") == "" {
		panic("REFUND_OPERATION_ID must select a listed refund and REFUND_IDEMPOTENCY_KEY is required")
	}
	result, err := client.CloudRefunds().ResolveRefund(context.Background(), selected.OperationID,
		mockarty.CloudRefundRetry, "provider_recovery_retry", selected.Generation, os.Getenv("REFUND_IDEMPOTENCY_KEY"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("refund %s is %s at generation %d\n", result.Refund.OperationID, result.Refund.Status, result.Refund.Generation)
}
