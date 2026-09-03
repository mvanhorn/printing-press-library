// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestMinimizeNarrowReadResponseAllowListsOperationalFields(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		response  string
		want      []string
		forbidden []string
	}{
		{
			name:      "rates",
			action:    "get_rates",
			response:  `{"output":{"rateReplyDetails":[{"serviceType":"FEDEX_GROUND","operationalDetail":{"transitTime":"TWO_DAYS","deliveryDay":"FRI"},"ratedShipmentDetails":[{"rateType":"PAYOR_ACCOUNT_PACKAGE","totalNetCharge":12.34,"totalBaseCharge":15,"currency":"USD"}]}]},"messages":[{"message":"unrelated"}],"requestEcho":{"name":"Recipient"}}`,
			want:      []string{`"service_type":"FEDEX_GROUND"`, `"total_net_charge":12.34`, `"transit_time":"TWO_DAYS"`},
			forbidden: []string{"unrelated", "requestEcho", "Recipient"},
		},
		{
			name:      "address",
			action:    "validate_address",
			response:  `{"output":{"resolvedAddresses":[{"classification":"BUSINESS","streetLines":["500 Main St"],"city":"Denver","stateOrProvinceCode":"CO","postalCode":"80202","countryCode":"US","customerName":"Recipient"}]},"messages":[{"message":"unrelated"}]}`,
			want:      []string{`"classification":"BUSINESS"`, `"postal_code":"80202"`, `"street_lines":["500 Main St"]`},
			forbidden: []string{"customerName", "Recipient", "unrelated"},
		},
		{
			name:      "shipment validation",
			action:    "validate_shipment",
			response:  `{"transactionId":"tx-secret","output":{"alerts":[{"code":"SHIPMENT.VALIDATION.SUCCESS","alertType":"NOTE","message":"unrelated"}],"requestEcho":{"name":"Recipient"}}}`,
			want:      []string{`"valid":true`, `"code":"SHIPMENT.VALIDATION.SUCCESS"`, `"alert_type":"NOTE"`},
			forbidden: []string{"tx-secret", "unrelated", "Recipient"},
		},
		{
			name:      "pickup availability",
			action:    "pickup_availability",
			response:  `{"output":{"options":[{"carrier":"FDXG","available":true,"pickupDate":"2026-09-03","cutOffTime":"17:00","accessTime":{"hours":1,"minutes":30},"residentialAvailable":true,"countryRelationship":"DOMESTIC","scheduleDay":"THU","defaultReadyTime":"09:00","defaultLatestTimeOptions":"17:00","pickupAddress":{"streetLines":["500 Main St"]}}]},"messages":[{"message":"unrelated"}]}`,
			want:      []string{`"carrier":"FDXG"`, `"available":true`, `"pickup_date":"2026-09-03"`, `"cutoff_time":"17:00"`, `"access_time":{"hours":1,"minutes":30}`},
			forbidden: []string{"500 Main St", "unrelated", "messages"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := minimizeNarrowReadResponse(test.action, []byte(test.response))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			text := string(encoded)
			for _, want := range test.want {
				if !strings.Contains(text, want) {
					t.Errorf("result %s missing %s", text, want)
				}
			}
			for _, forbidden := range test.forbidden {
				if strings.Contains(text, forbidden) {
					t.Errorf("result %s exposed %s", text, forbidden)
				}
			}
		})
	}
}

func TestShipmentValidationRequiresExplicitFedExEvidence(t *testing.T) {
	for name, response := range map[string]string{
		"message only": `{"output":{"alerts":[{"message":"looks fine"}]}}`,
		"warning only": `{"output":{"alerts":[{"code":"SHIPMENT.WARNING","alertType":"WARNING"}]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := minimizeShipmentValidationResponse([]byte(response)); err == nil {
				t.Fatal("response without explicit validation evidence was accepted")
			}
		})
	}
	result, err := minimizeShipmentValidationResponse([]byte(`{"output":{"alerts":[{"code":"SHIPMENT.ERROR","alertType":"ERROR","message":"recipient PII"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	if !strings.Contains(string(encoded), `"valid":false`) || strings.Contains(string(encoded), "recipient PII") {
		t.Fatalf("unexpected minimized rejection: %s", encoded)
	}
}

func TestPickupAvailabilityRetainsDistinctOfficialOptions(t *testing.T) {
	response := []byte(`{"output":{"options":[{"carrier":"FDXG","available":true,"pickupDate":"2026-09-03","cutOffTime":"17:00","accessTime":{"hours":1,"minutes":0}},{"carrier":"FDXE","available":false,"pickupDate":"2026-09-04","cutOffTime":"15:00","accessTime":{"hours":2,"minutes":30}}]}}`)
	result, err := minimizePickupAvailabilityResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	for _, want := range []string{`"carrier":"FDXG"`, `"carrier":"FDXE"`, `"pickup_date":"2026-09-03"`, `"pickup_date":"2026-09-04"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("minimized options %s missing %s", encoded, want)
		}
	}
}

type malformedReadRequest struct {
	name   string
	action string
	body   map[string]any
}

func malformedReadRequests() []malformedReadRequest {
	return []malformedReadRequest{
		{"rate wrong account field", "get_rates", map[string]any{"accountNumber": map[string]any{"number": "123"}, "requestedShipment": map[string]any{}}},
		{"rate invalid carrier", "get_rates", rateRequestWithCarriers([]any{"OTHER"})},
		{"rate invalid controls", "get_rates", rateRequestWithControls(map[string]any{"returnTransitTimes": "yes"})},
		{"rate inconsistent grouped total", "get_rates", rateRequestWithGroupedTotal(2, 1)},
		{"rate pickup option missing detail", "get_rates", rateRequestWithoutPickupDetail()},
		{"rate missing nested address", "get_rates", map[string]any{"accountNumber": map[string]any{"value": "123"}, "requestedShipment": map[string]any{"shipper": map[string]any{}, "recipient": map[string]any{}, "pickupType": "DROPOFF_AT_FEDEX_LOCATION", "rateRequestType": []any{"ACCOUNT"}, "requestedPackageLineItems": []any{map[string]any{"weight": map[string]any{"units": "LB", "value": 1}}}}}},
		{"address missing street", "validate_address", map[string]any{"addressesToValidate": []any{map[string]any{"address": map[string]any{"countryCode": "US"}}}}},
		{"address numeric optional city", "validate_address", map[string]any{"addressesToValidate": []any{map[string]any{"address": map[string]any{"streetLines": []any{"20 Rue de la Paix"}, "city": 1, "countryCode": "FR"}}}}},
		{"address US blank alternatives", "validate_address", map[string]any{"addressesToValidate": []any{map[string]any{"address": map[string]any{"streetLines": []any{"10 FedEx Parkway"}, "city": "", "stateOrProvinceCode": "", "postalCode": "", "countryCode": "US"}}}}},
		{"address invalid controls", "validate_address", map[string]any{"addressesToValidate": []any{map[string]any{"address": map[string]any{"streetLines": []any{"1 Test Way"}, "postalCode": "78701", "countryCode": "US"}}}, "validateAddressControlParameters": map[string]any{"includeResolutionTokens": "yes"}}},
		{"shipment missing parties", "validate_shipment", map[string]any{"accountNumber": map[string]any{"value": "123"}, "requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND", "packagingType": "YOUR_PACKAGING", "requestedPackageLineItems": []any{map[string]any{"weight": map[string]any{"units": "LB", "value": 2}}}}}},
		{"shipment missing pickup type", "validate_shipment", shipmentValidationRequestWithout("pickupType")},
		{"shipment missing total weight", "validate_shipment", shipmentValidationRequestWithout("totalWeight")},
		{"shipment zero total weight", "validate_shipment", shipmentValidationRequestWithTotalWeight(0)},
		{"shipment missing payment", "validate_shipment", shipmentValidationRequestWithout("shippingChargesPayment")},
		{"shipment third party payment missing payor", "validate_shipment", shipmentValidationRequestWithPayment(map[string]any{"paymentType": "THIRD_PARTY"})},
		{"shipment numeric optional state", "validate_shipment", shipmentValidationRequestWithAddressField("stateOrProvinceCode", 1)},
		{"shipment US blank required state", "validate_shipment", shipmentValidationRequestWithAddressField("stateOrProvinceCode", "")},
		{"shipment missing label specification", "validate_shipment", shipmentValidationRequestWithout("labelSpecification")},
		{"shipment unknown label stock", "validate_shipment", shipmentValidationRequestWithLabelStock("UNKNOWN")},
		{"shipment grouped package", "validate_shipment", shipmentValidationRequestWithPackageField("groupPackageCount", 2)},
		{"shipment wrong sequence", "validate_shipment", shipmentValidationRequestWithPackageField("sequenceNumber", 2)},
		{"pickup numeric carrier", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{1}, "countryRelationship": "DOMESTIC"}},
		{"pickup nonobject package detail", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC", "packageDetails": []any{"invalid"}}},
		{"pickup empty package detail", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC", "packageDetails": []any{map[string]any{}}}},
		{"pickup empty shipment attributes", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC", "shipmentAttributes": map[string]any{}}},
		{"pickup numeric city", "pickup_availability", pickupAvailabilityRequestWithAddressField("city", 1)},
		{"pickup invalid street lines", "pickup_availability", pickupAvailabilityRequestWithAddressField("streetLines", "1 Test Way")},
		{"pickup invalid residential", "pickup_availability", pickupAvailabilityRequestWithAddressField("residential", "false")},
		{"pickup invalid classification", "pickup_availability", pickupAvailabilityRequestWithAddressField("addressClassification", "COMMERCIAL")},
		{"pickup invalid urbanization", "pickup_availability", pickupAvailabilityRequestWithAddressField("urbanizationCode", 1)},
		{"pickup incomplete dimensions", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXE"}, "countryRelationship": "INTERNATIONAL", "shipmentAttributes": map[string]any{"serviceType": "INTERNATIONAL_PRIORITY", "dimensions": map[string]any{"length": 1, "units": "IN"}}}},
		{"pickup your packaging without dimensions", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC", "shipmentAttributes": map[string]any{"serviceType": "FEDEX_GROUND", "packagingType": "YOUR_PACKAGING"}}},
		{"pickup wrong account shape", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "pickupRequestType": []any{"SAME_DAY"}, "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC", "associatedAccountNumber": map[string]any{"value": "123"}}},
		{"pickup missing request type", "pickup_availability", map[string]any{"pickupAddress": testPickupAvailabilityAddress(), "carriers": []any{"FDXG"}, "countryRelationship": "DOMESTIC"}},
	}
}

func TestValidateNarrowReadRequestRejectsMalformedNestedRequests(t *testing.T) {
	for _, test := range malformedReadRequests() {
		t.Run(test.name, func(t *testing.T) {
			if err := validateNarrowReadRequest(test.action, test.body); err == nil {
				t.Fatalf("%s accepted malformed request", test.action)
			}
		})
	}
}

func TestValidateNarrowReadRequestAcceptsWellFormedRequests(t *testing.T) {
	for action, body := range testReadRequests() {
		t.Run(action, func(t *testing.T) {
			if err := validateNarrowReadRequest(action, body); err != nil {
				t.Fatalf("well-formed request rejected: %v", err)
			}
		})
	}
}

func TestValidateAddressAcceptsCountrySpecificMinimums(t *testing.T) {
	addresses := map[string]map[string]any{
		"US postal":             {"streetLines": []any{"10 FedEx Parkway"}, "postalCode": "38116", "countryCode": "US"},
		"US city state":         {"streetLines": []any{"10 FedEx Parkway"}, "city": "Memphis", "stateOrProvinceCode": "TN", "countryCode": "US"},
		"non-US common":         {"streetLines": []any{"20 Rue de la Paix"}, "countryCode": "FR"},
		"non-US optional empty": {"streetLines": []any{"20 Rue de la Paix"}, "city": "", "stateOrProvinceCode": "", "postalCode": "", "countryCode": "FR"},
	}
	for name, address := range addresses {
		t.Run(name, func(t *testing.T) {
			request := map[string]any{"addressesToValidate": []any{map[string]any{"address": address}}}
			if err := validateNarrowReadRequest("validate_address", request); err != nil {
				t.Fatalf("official address minimum rejected: %v", err)
			}
		})
	}
}

func TestValidateRatesAcceptsOfficialMinimumAndGroupedPackages(t *testing.T) {
	request := testReadRequests()["get_rates"]
	shipment := request["requestedShipment"].(map[string]any)
	delete(shipment, "rateRequestType")
	shipment["totalPackageCount"] = 2
	shipment["requestedPackageLineItems"].([]any)[0].(map[string]any)["groupPackageCount"] = 2
	request["carrierCodes"] = []any{"FXSP"}
	if err := validateNarrowReadRequest("get_rates", request); err != nil {
		t.Fatalf("official minimum grouped rate request rejected: %v", err)
	}
}

func TestValidateShipmentAcceptsConditionalInternationalAddressAndPayor(t *testing.T) {
	request := shipmentValidationRequestWithPayment(map[string]any{
		"paymentType": "THIRD_PARTY",
		"payor": map[string]any{
			"responsibleParty": map[string]any{"accountNumber": map[string]any{"value": "987654321"}},
		},
	})
	shipment := request["requestedShipment"].(map[string]any)
	address := map[string]any{"streetLines": []any{"20 Rue de la Paix"}, "city": "Paris", "stateOrProvinceCode": "", "postalCode": "", "countryCode": "FR"}
	shipment["shipper"].(map[string]any)["address"] = address
	shipment["recipients"].([]any)[0].(map[string]any)["address"] = address
	if err := validateNarrowReadRequest("validate_shipment", request); err != nil {
		t.Fatalf("conditional international shipment request rejected: %v", err)
	}
}

func TestValidateShipmentAcceptsCollectWithoutPayor(t *testing.T) {
	request := shipmentValidationRequestWithPayment(map[string]any{"paymentType": "COLLECT"})
	if err := validateNarrowReadRequest("validate_shipment", request); err != nil {
		t.Fatalf("COLLECT shipment without payor rejected: %v", err)
	}
}

func TestMalformedNarrowReadRequestsMakeNoHTTPCalls(t *testing.T) {
	for _, test := range malformedReadRequests() {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
			defer api.Close()
			setMCPTestAuth(t, api.URL)
			var operation narrowOperation
			for _, candidate := range narrowOperations {
				if candidate.action == test.action {
					operation = candidate
					break
				}
			}
			result, err := makeNarrowReadHandler(operation)(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"request": test.body}}})
			if err != nil {
				t.Fatal(err)
			}
			if result == nil || !result.IsError {
				t.Fatalf("malformed request did not produce a tool error: %#v", result)
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("malformed request made %d HTTP calls", got)
			}
		})
	}
}

func testReadRequests() map[string]map[string]any {
	return map[string]map[string]any{
		"get_rates": {
			"accountNumber":                map[string]any{"value": "123"},
			"carrierCodes":                 []any{"FDXG"},
			"processingOptions":            []any{"INCLUDE_PICKUPRATES"},
			"rateRequestControlParameters": map[string]any{"returnTransitTimes": true, "rateSortOrder": "COMMITASCENDING"},
			"requestedShipment": map[string]any{
				"shipper":         map[string]any{"address": map[string]any{"postalCode": "78701", "countryCode": "US"}},
				"recipient":       map[string]any{"address": map[string]any{"postalCode": "80202", "countryCode": "US"}},
				"pickupType":      "DROPOFF_AT_FEDEX_LOCATION",
				"pickupDetail":    map[string]any{"requestType": "SAME_DAY", "requestSource": "AUTOMATION"},
				"rateRequestType": []any{"ACCOUNT"},
				"requestedPackageLineItems": []any{map[string]any{
					"weight": map[string]any{"units": "LB", "value": 2},
				}},
			},
		},
		"validate_address":  {"addressesToValidate": []any{map[string]any{"clientReferenceId": "test-address", "address": map[string]any{"streetLines": []any{"10 FedEx Parkway"}, "postalCode": "38116", "countryCode": "US"}}}, "validateAddressControlParameters": map[string]any{"includeResolutionTokens": true}},
		"validate_shipment": testShipmentValidationRequest(),
		"pickup_availability": {
			"pickupAddress":           testPickupAvailabilityAddress(),
			"pickupRequestType":       []any{"SAME_DAY"},
			"carriers":                []any{"FDXG"},
			"countryRelationship":     "DOMESTIC",
			"associatedAccountNumber": "123",
			"dispatchDate":            "2026-09-03",
			"packageReadyTime":        "09:00:00",
			"customerCloseTime":       "17:00:00",
			"shipmentAttributes":      map[string]any{"serviceType": "FEDEX_GROUND", "packagingType": "YOUR_PACKAGING", "weight": map[string]any{"units": "LB", "value": 2}, "dimensions": map[string]any{"length": 10, "width": 8, "height": 6, "units": "IN"}},
			"packageDetails":          []any{map[string]any{"packageSpecialServices": map[string]any{"specialServiceTypes": []any{"SIGNATURE_OPTION"}}}},
		},
	}
}

func testShipmentValidationRequest() map[string]any {
	return map[string]any{
		"accountNumber": map[string]any{"value": "123"},
		"requestedShipment": map[string]any{
			"shipper":                   testReadParty(),
			"recipients":                []any{testReadParty()},
			"pickupType":                "DROPOFF_AT_FEDEX_LOCATION",
			"serviceType":               "FEDEX_GROUND",
			"packagingType":             "YOUR_PACKAGING",
			"totalWeight":               1,
			"shippingChargesPayment":    map[string]any{"paymentType": "SENDER"},
			"labelSpecification":        map[string]any{"imageType": "PDF", "labelStockType": "PAPER_4X6"},
			"totalPackageCount":         1,
			"requestedPackageLineItems": []any{map[string]any{"sequenceNumber": 1, "groupPackageCount": 1, "weight": map[string]any{"units": "LB", "value": 2}}},
		},
	}
}

func shipmentValidationRequestWithout(field string) map[string]any {
	request := testShipmentValidationRequest()
	delete(request["requestedShipment"].(map[string]any), field)
	return request
}

func shipmentValidationRequestWithPackageField(field string, value any) map[string]any {
	request := testShipmentValidationRequest()
	shipment := request["requestedShipment"].(map[string]any)
	shipment["requestedPackageLineItems"].([]any)[0].(map[string]any)[field] = value
	return request
}

func shipmentValidationRequestWithPayment(payment map[string]any) map[string]any {
	request := testShipmentValidationRequest()
	request["requestedShipment"].(map[string]any)["shippingChargesPayment"] = payment
	return request
}

func shipmentValidationRequestWithAddressField(field string, value any) map[string]any {
	request := testShipmentValidationRequest()
	shipment := request["requestedShipment"].(map[string]any)
	shipment["shipper"].(map[string]any)["address"].(map[string]any)[field] = value
	return request
}

func rateRequestWithCarriers(carriers []any) map[string]any {
	request := testReadRequests()["get_rates"]
	request["carrierCodes"] = carriers
	return request
}

func rateRequestWithControls(controls map[string]any) map[string]any {
	request := testReadRequests()["get_rates"]
	request["rateRequestControlParameters"] = controls
	return request
}

func rateRequestWithGroupedTotal(groupCount, totalCount int) map[string]any {
	request := testReadRequests()["get_rates"]
	shipment := request["requestedShipment"].(map[string]any)
	shipment["requestedPackageLineItems"].([]any)[0].(map[string]any)["groupPackageCount"] = groupCount
	shipment["totalPackageCount"] = totalCount
	return request
}

func rateRequestWithoutPickupDetail() map[string]any {
	request := testReadRequests()["get_rates"]
	delete(request["requestedShipment"].(map[string]any), "pickupDetail")
	return request
}

func shipmentValidationRequestWithTotalWeight(totalWeight any) map[string]any {
	request := testShipmentValidationRequest()
	request["requestedShipment"].(map[string]any)["totalWeight"] = totalWeight
	return request
}

func shipmentValidationRequestWithLabelStock(labelStock string) map[string]any {
	request := testShipmentValidationRequest()
	request["requestedShipment"].(map[string]any)["labelSpecification"].(map[string]any)["labelStockType"] = labelStock
	return request
}

func testReadAddress() map[string]any {
	return map[string]any{"streetLines": []any{"1 Test Way"}, "city": "Austin", "stateOrProvinceCode": "TX", "postalCode": "78701", "countryCode": "US"}
}

func testPickupAvailabilityAddress() map[string]any {
	return map[string]any{"postalCode": "78701", "countryCode": "US"}
}

func pickupAvailabilityRequestWithAddressField(field string, value any) map[string]any {
	address := testPickupAvailabilityAddress()
	address[field] = value
	return map[string]any{
		"pickupAddress":       address,
		"pickupRequestType":   []any{"SAME_DAY"},
		"carriers":            []any{"FDXG"},
		"countryRelationship": "DOMESTIC",
	}
}

func testReadParty() map[string]any {
	return map[string]any{"contact": map[string]any{"companyName": "FedEx Test", "phoneNumber": "5555550100"}, "address": testReadAddress()}
}
