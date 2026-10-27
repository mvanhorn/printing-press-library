// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package speccontract

import (
	"context"
	"net/http"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func TestPublishedOperationsExist(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		doc, method, path string
		headers           []string
	}{
		{"authorization", http.MethodPost, "/oauth/token", []string{"content-type"}},
		{"ship", http.MethodPost, "/ship/v1/shipments", []string{"content-type", "authorization"}},
		{"ship", http.MethodPut, "/ship/v1/shipments/cancel", []string{"content-type", "authorization"}},
		{"ship", http.MethodPost, "/ship/v1/shipments/packages/validate", []string{"content-type", "authorization"}},
		{"pickup", http.MethodPost, "/pickup/v1/pickups", []string{"content-type", "authorization"}},
		{"pickup", http.MethodPost, "/pickup/v1/pickups/availabilities", []string{"content-type", "authorization"}},
		{"pickup", http.MethodPut, "/pickup/v1/pickups/cancel", []string{"content-type", "authorization"}},
		{"rate", http.MethodPost, "/rate/v1/rates/quotes", []string{"content-type", "authorization"}},
		{"address", http.MethodPost, "/address/v1/addresses/resolve", []string{"content-type", "authorization"}},
	}
	for _, check := range checks {
		t.Run(check.method+" "+check.path, func(t *testing.T) {
			op, err := operationSpec(docs[check.doc], check.method, check.path)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, name := range requiredHeaders(op) {
				got[name] = true
			}
			for _, want := range check.headers {
				if !got[want] {
					t.Errorf("published %s %s does not require header %s (has %v)", check.method, check.path, want, requiredHeaders(op))
				}
			}
		})
	}
}

func TestCreateLabelVariantsMatchShipSchema(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	ground := createLabelBody("FEDEX_GROUND", "US", true)
	express := createLabelBody("PRIORITY_OVERNIGHT", "US", true)
	germany := createLabelBody("INTERNATIONAL_PRIORITY", "DE", false)
	for _, test := range []struct {
		name string
		body map[string]any
	}{
		{"ground your packaging", ground},
		{"express your packaging", express},
		{"germany without state", germany},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := workflow.ValidateRequest(workflow.ActionCreateLabel, test.body); err != nil {
				t.Fatalf("CLI rejected variant: %v", err)
			}
			if err := validateSchema(docs["ship"], shipSchema, test.body); err != nil {
				t.Fatalf("published Ship schema rejected CLI body: %v", err)
			}
		})
	}

	incomplete := createLabelBody("FEDEX_GROUND", "US", true)
	delete(incomplete["requestedShipment"].(map[string]any), "pickupType")
	if err := workflow.ValidateRequest(workflow.ActionCreateLabel, incomplete); err == nil {
		t.Fatal("CLI accepted a create-label body the published schema rejects for missing pickupType")
	}
}

func TestShipmentCancelAndValidateMatchShipSchema(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	cancel := map[string]any{
		"accountNumber":     map[string]any{"value": "123456789"},
		"trackingNumber":    "794953535000",
		"senderCountryCode": "US",
		"deletionControl":   "DELETE_ALL_PACKAGES",
	}
	if err := workflow.ValidateRequest(workflow.ActionCancelShipment, cancel); err != nil {
		t.Fatal(err)
	}
	if err := validateSchema(docs["ship"], cancelSchema, cancel); err != nil {
		t.Fatalf("cancel schema: %v", err)
	}

	validate := createLabelBody("FEDEX_GROUND", "US", true)
	// Shipment validation's structural schema types totalWeight as an integer.
	// Create-label leaves it off because FedEx's own domestic examples do.
	validate["requestedShipment"].(map[string]any)["totalWeight"] = 2
	if err := mcp.ValidateNarrowReadRequestForTest("validate_shipment", validate); err != nil {
		t.Fatal(err)
	}
	if err := validateSchema(docs["ship"], verifyShipmentSchema, validate); err != nil {
		t.Fatalf("validate schema: %v", err)
	}
}

func TestPickupVariantsMatchPickupSchema(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	ground := schedulePickupBody("FDXG", "")
	express := schedulePickupBody("FDXE", "")
	for _, test := range []struct {
		name string
		body map[string]any
	}{
		{"ground", ground},
		{"express", express},
	} {
		t.Run("schedule "+test.name, func(t *testing.T) {
			if err := workflow.ValidateRequest(workflow.ActionSchedulePickup, test.body); err != nil {
				t.Fatal(err)
			}
			if err := validateSchema(docs["pickup"], "Full_Schema_Create_Pickup", test.body); err != nil {
				t.Fatalf("schema: %v", err)
			}
		})
	}
	missingState := schedulePickupBody("FDXG", "")
	delete(missingState["originDetail"].(map[string]any)["pickupLocation"].(map[string]any)["address"].(map[string]any), "stateOrProvinceCode")
	if err := workflow.ValidateRequest(workflow.ActionSchedulePickup, missingState); err == nil {
		t.Fatal("CLI accepted a pickup location without the state the published schema requires")
	}

	for _, test := range []struct {
		name string
		body map[string]any
	}{
		{"domestic ground", availabilityBody("FDXG", "DOMESTIC", false)},
		{"domestic express", availabilityBody("FDXE", "DOMESTIC", false)},
		{"international express your packaging", availabilityBody("FDXE", "INTERNATIONAL", true)},
	} {
		t.Run("availability "+test.name, func(t *testing.T) {
			if err := mcp.ValidateNarrowReadRequestForTest("pickup_availability", test.body); err != nil {
				t.Fatal(err)
			}
			if err := validateSchema(docs["pickup"], "Full_Schema_Pickup_Availability", test.body); err != nil {
				t.Fatalf("schema: %v", err)
			}
		})
	}
}

func TestPickupCancelFromLedgerMatchesSchema(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	ledger, err := store.Open(store.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, pickup := range []store.Pickup{
		{OperationID: "ground-op", ConfirmationNumber: "CPU16200910", AccountNumber: "123456789", CarrierCode: "FDXG", ScheduledDate: "2020-07-03", Status: "scheduled"},
		{OperationID: "express-op", ConfirmationNumber: "NQAA97", AccountNumber: "123456789", CarrierCode: "FDXE", ScheduledDate: "2020-07-03", LocationCode: "NQAA", Status: "scheduled"},
	} {
		if err := ledger.UpsertPickup(context.Background(), pickup); err != nil {
			t.Fatal(err)
		}
	}
	ledger.Close()

	for _, confirmation := range []string{"CPU16200910", "NQAA97"} {
		t.Run(confirmation, func(t *testing.T) {
			resolved, err := workflow.ResolvePickupCancellation(context.Background(), map[string]any{"pickupConfirmationCode": confirmation}, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := validateSchema(docs["pickup"], "Full_Schema_Cancel_Pickup", resolved.Body); err != nil {
				t.Fatalf("schema: %v body=%v", err, resolved.Body)
			}
		})
	}
}

func TestRateAndAddressVariantsMatchSchemas(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	rate := map[string]any{
		"accountNumber": map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"shipper":       map[string]any{"address": map[string]any{"postalCode": "38116", "countryCode": "US"}},
			"recipient":     map[string]any{"address": map[string]any{"postalCode": "75063", "countryCode": "US"}},
			"pickupType":    "DROPOFF_AT_FEDEX_LOCATION",
			"packagingType": "YOUR_PACKAGING",
			"requestedPackageLineItems": []any{map[string]any{
				"weight":     map[string]any{"units": "LB", "value": 2},
				"dimensions": map[string]any{"length": 8, "width": 6, "height": 4, "units": "IN"},
			}},
		},
	}
	if err := mcp.ValidateNarrowReadRequestForTest("get_rates", rate); err != nil {
		t.Fatal(err)
	}
	if err := validateSchema(docs["rate"], "Full_Schema_Quote_Rate", rate); err != nil {
		t.Fatalf("rate schema: %v", err)
	}

	for _, test := range []struct {
		name    string
		address map[string]any
	}{
		{"US", map[string]any{"streetLines": []any{"7372 PARKRIDGE BLVD"}, "city": "IRVING", "stateOrProvinceCode": "TX", "postalCode": "75063-8659", "countryCode": "US"}},
		{"CA", map[string]any{"streetLines": []any{"100 Queen St W"}, "city": "Toronto", "stateOrProvinceCode": "ON", "postalCode": "M5H2N1", "countryCode": "CA"}},
		{"DE", map[string]any{"streetLines": []any{"Adam-von-Trott-Strasse 1"}, "city": "Berlin", "postalCode": "13627", "countryCode": "DE"}},
		{"GB", map[string]any{"streetLines": []any{"24 Grosvenor Square"}, "city": "London", "postalCode": "W1A 2LQ", "countryCode": "GB"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := map[string]any{"addressesToValidate": []any{map[string]any{"address": test.address}}}
			if err := mcp.ValidateNarrowReadRequestForTest("validate_address", body); err != nil {
				t.Fatal(err)
			}
			if err := validateSchema(docs["address"], "Full_Schema_Validate_Address", body); err != nil {
				t.Fatalf("address schema: %v", err)
			}
		})
	}
}

func TestOAuthClientCredentialsMatchSchema(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"grant_type":    "client_credentials",
		"client_id":     "client-id",
		"client_secret": "client-secret",
	}
	if err := validateSchema(docs["authorization"], "FullSchema", body); err != nil {
		t.Fatal(err)
	}
	example, ok := schemaExample(docs["authorization"], "FullSchema")
	if ok {
		t.Fatalf("FullSchema unexpectedly has a schema example: %#v", example)
	}
	op, err := operationSpec(docs["authorization"], http.MethodPost, "/oauth/token")
	if err != nil {
		t.Fatal(err)
	}
	media := op.RequestBody.Value.Content.Get("application/x-www-form-urlencoded")
	if media == nil || media.Examples == nil || media.Examples["B2B-Proprietary"] == nil {
		t.Fatal("published OAuth document has no B2B-Proprietary example")
	}
}

func createLabelBody(service, country string, withState bool) map[string]any {
	address := func(city, postal string) map[string]any {
		item := map[string]any{
			"streetLines": []any{"10 FedEx Parkway"},
			"city":        city,
			"postalCode":  postal,
			"countryCode": country,
		}
		if withState {
			item["stateOrProvinceCode"] = "TN"
		}
		return item
	}
	return map[string]any{
		"labelResponseOptions": "LABEL",
		"accountNumber":        map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"shipper": map[string]any{
				"contact": map[string]any{"personName": "Ship Er", "phoneNumber": "9015551212"},
				"address": address("Memphis", "38116"),
			},
			"recipients": []any{map[string]any{
				"contact": map[string]any{"companyName": "Re Cipient", "phoneNumber": "9015551213"},
				"address": address("Memphis", "38116"),
			}},
			"pickupType":             "USE_SCHEDULED_PICKUP",
			"serviceType":            service,
			"packagingType":          "YOUR_PACKAGING",
			"shippingChargesPayment": map[string]any{"paymentType": "SENDER"},
			"labelSpecification":     map[string]any{"imageType": "PDF", "labelStockType": "PAPER_4X6"},
			"requestedPackageLineItems": []any{map[string]any{
				"weight":     map[string]any{"units": "LB", "value": 2},
				"dimensions": map[string]any{"length": 8, "width": 6, "height": 4, "units": "IN"},
			}},
		},
	}
}

func schedulePickupBody(carrier, location string) map[string]any {
	_ = location
	return map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             carrier,
		"originDetail": map[string]any{
			"readyDateTimestamp": "2020-07-03T11:00:00Z",
			"customerCloseTime":  "17:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"personName": "Contact Name for Pickup", "phoneNumber": "1234567890"},
				"address": map[string]any{
					"streetLines":         []any{"10 Fedex Pkwy"},
					"city":                "Collierville",
					"stateOrProvinceCode": "TN",
					"postalCode":          "38017",
					"countryCode":         "US",
				},
			},
		},
		"totalWeight":  map[string]any{"units": "LB", "value": 2},
		"packageCount": 1,
	}
}

func availabilityBody(carrier, relationship string, withDimensions bool) map[string]any {
	body := map[string]any{
		"pickupAddress":       map[string]any{"postalCode": "38116", "countryCode": "US"},
		"pickupRequestType":   []any{"SAME_DAY"},
		"carriers":            []any{carrier},
		"countryRelationship": relationship,
	}
	if relationship == "INTERNATIONAL" {
		body["pickupAddress"] = map[string]any{"postalCode": "75008", "countryCode": "FR"}
	}
	if withDimensions {
		body["shipmentAttributes"] = map[string]any{
			"serviceType":   "INTERNATIONAL_PRIORITY",
			"packagingType": "YOUR_PACKAGING",
			"dimensions":    map[string]any{"length": 10, "width": 8, "height": 6, "units": "IN"},
		}
	}
	return body
}
