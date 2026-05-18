// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"strings"
	"testing"
)

// TestGetQueriesDeclareIDType is a regression test for a GraphQL type mismatch
// where root *Get queries declared `$id: String!` but Shopify's product(id:),
// customer(id:), order(id:), fulfillmentOrder(id:), and inventoryItem(id:)
// root query arguments expect `ID!`. The previous typing caused the API to
// reject calls with: `Type mismatch on variable $id and argument id
// (String! / ID!)`.
func TestGetQueriesDeclareIDType(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"CustomersGetQuery", CustomersGetQuery},
		{"FulfillmentOrdersGetQuery", FulfillmentOrdersGetQuery},
		{"InventoryItemsGetQuery", InventoryItemsGetQuery},
		{"OrdersGetQuery", OrdersGetQuery},
		{"ProductsGetQuery", ProductsGetQuery},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(tc.query, "$id: ID!") {
				t.Errorf("%s must declare $id as ID! to match Shopify's root query argument type; got: %s", tc.name, tc.query)
			}
			if strings.Contains(tc.query, "$id: String!") {
				t.Errorf("%s must not declare $id as String! (Shopify expects ID!); got: %s", tc.name, tc.query)
			}
		})
	}
}
