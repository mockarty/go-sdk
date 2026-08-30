package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_TOKEN")))
	result, err := client.CloudRefunds().ResolveRefund(context.Background(), os.Getenv("REFUND_OPERATION_ID"),
		mockarty.CloudRefundRetry, "provider_recovery_retry", 4, "refund-resolution:example-1")
	if err != nil {
		panic(err)
	}
	fmt.Printf("refund %s is %s at generation %d\n", result.Refund.OperationID, result.Refund.Status, result.Refund.Generation)
}
