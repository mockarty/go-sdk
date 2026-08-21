package main

import (
	"context"
	"fmt"
	"os"

	mockarty "github.com/mockarty/mockarty-go"
)

func main() {
	client := mockarty.NewClient(os.Getenv("MOCKARTY_BASE_URL"), mockarty.WithAPIKey(os.Getenv("MOCKARTY_API_KEY")))
	report, err := client.Economics().GetUsage(context.Background(), mockarty.LLMUsageQuery{GroupBy: "profile", Days: 30})
	if err != nil {
		panic(err)
	}
	fmt.Printf("calls=%d tokens=%d unpriced=%d\n", report.Totals.Calls, report.Totals.TotalTokens, report.UnpricedCalls)
	statement, err := client.Economics().DownloadUsageStatement(context.Background(), mockarty.LLMUsageStatementQuery{Limit: 100})
	if err != nil {
		panic(err)
	}
	fmt.Printf("statement_bytes=%d\n", len(statement))
	resourcePrices, err := client.Economics().ListResourcePrices(context.Background(), mockarty.ResourcePriceQuery{
		EventKind: "tool_call", Provider: "mockarty-agent", Resource: "run_api_test", Limit: 20,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("tool_price_versions=%d\n", len(resourcePrices.ResourcePrices))
}
