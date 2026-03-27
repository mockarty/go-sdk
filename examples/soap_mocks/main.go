// Copyright (c) 2026 Mockarty. All rights reserved.
// Licensed under the MIT License. See LICENSE file for details.

// Example: soap_mocks — SOAP protocol mock examples.
//
// This example demonstrates:
//   - SOAP service/method mock
//   - SOAP fault response
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
		"soap-get-account",
		"soap-fault",
	}
	defer func() {
		for _, id := range mockIDs {
			_ = client.Mocks().Delete(ctx, id)
		}
		fmt.Println("\nAll SOAP example mocks cleaned up.")
	}()

	// -----------------------------------------------------------------------
	// 1. SOAP service/method mock
	// -----------------------------------------------------------------------
	// Mocks the AccountService.GetAccount operation.
	soapMock := mockarty.NewMockBuilder().
		ID("soap-get-account").
		SOAPConfig(func(s *mockarty.SOAPBuilder) {
			s.Service("AccountService").
				Method("GetAccount").
				Action("http://example.com/AccountService/GetAccount").
				Path("/ws/account")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(200).
				JSONBody(map[string]any{
					"Envelope": map[string]any{
						"Body": map[string]any{
							"GetAccountResponse": map[string]any{
								"AccountId":   "ACC-001",
								"AccountName": "$.fake.CompanyName",
								"Balance":     "$.fake.Amount",
								"Currency":    "USD",
								"Status":      "ACTIVE",
							},
						},
					},
				})
		}).
		Build()

	if _, err := client.Mocks().Create(ctx, soapMock); err != nil {
		log.Fatalf("Failed to create SOAP mock: %v", err)
	}
	fmt.Println("[1] Created SOAP mock: AccountService.GetAccount")

	// -----------------------------------------------------------------------
	// 2. SOAP fault response
	// -----------------------------------------------------------------------
	// Returns a SOAP fault when the account is not found.
	faultMock := mockarty.NewMockBuilder().
		ID("soap-fault").
		SOAPConfig(func(s *mockarty.SOAPBuilder) {
			s.Service("AccountService").
				Method("GetAccount").
				Path("/ws/account").
				BodyCondition("$.body.accountId", mockarty.AssertEquals, "INVALID")
		}).
		Response(func(r *mockarty.ResponseBuilder) {
			r.Status(500).
				SOAPFaultConfig(&mockarty.SOAPFault{
					FaultCode:   "soap:Client",
					FaultString: "Account not found",
					FaultActor:  "http://example.com/AccountService",
					Detail:      "The account with ID 'INVALID' does not exist in the system.",
					HTTPStatus:  500,
				})
		}).
		Priority(10). // match before the generic GetAccount mock
		Build()

	if _, err := client.Mocks().Create(ctx, faultMock); err != nil {
		log.Fatalf("Failed to create SOAP fault mock: %v", err)
	}
	fmt.Println("[2] Created SOAP fault mock: AccountService.GetAccount (not found)")

	fmt.Println("\nAll SOAP mock examples created successfully!")
}
