// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCreateLabelRequestValidation(t *testing.T) {
	request := validCreateLabelRequest()
	if err := request.Validate(); err != nil {
		t.Fatalf("valid request: %v", err)
	}

	tests := []struct {
		name   string
		change func(*CreateLabelRequest)
		want   string
	}{
		{"account", func(r *CreateLabelRequest) { r.AccountNumber.Value = "" }, "accountNumber.value"},
		{"label disposition", func(r *CreateLabelRequest) { r.LabelResponseOptions = "URL_ONLY" }, "must be LABEL"},
		{"shipper", func(r *CreateLabelRequest) { r.RequestedShipment.Shipper = Party{} }, "shipper.contact"},
		{"recipient", func(r *CreateLabelRequest) { r.RequestedShipment.Recipients = nil }, "exactly one recipient"},
		{"multiple recipients", func(r *CreateLabelRequest) {
			r.RequestedShipment.Recipients = append(r.RequestedShipment.Recipients, r.RequestedShipment.Recipients[0])
		}, "exactly one recipient"},
		{"service", func(r *CreateLabelRequest) { r.RequestedShipment.ServiceType = "" }, "serviceType"},
		{"packaging", func(r *CreateLabelRequest) { r.RequestedShipment.PackagingType = "" }, "packagingType"},
		{"package", func(r *CreateLabelRequest) {
			r.RequestedShipment.RequestedPackageLineItems = nil
			r.RequestedShipment.TotalPackageCount = 0
		}, "exactly one package"},
		{"multiple packages", func(r *CreateLabelRequest) {
			r.RequestedShipment.RequestedPackageLineItems = append(r.RequestedShipment.RequestedPackageLineItems, r.RequestedShipment.RequestedPackageLineItems[0])
			r.RequestedShipment.TotalPackageCount = 2
		}, "exactly one package"},
		{"grouped packages", func(r *CreateLabelRequest) {
			r.RequestedShipment.RequestedPackageLineItems[0].GroupPackageCount = 2
		}, "groupPackageCount must be omitted or 1"},
		{"weight", func(r *CreateLabelRequest) { r.RequestedShipment.RequestedPackageLineItems[0].Weight.Value = 0 }, "greater than zero"},
		{"package count", func(r *CreateLabelRequest) { r.RequestedShipment.TotalPackageCount = 2 }, "must match"},
		{"format", func(r *CreateLabelRequest) { r.RequestedShipment.LabelSpecification.ImageType = "ZPLII" }, "must be PDF"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := validCreateLabelRequest()
			test.change(&candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want substring %q", err, test.want)
			}
		})
	}
}

func TestOtherRequestValidation(t *testing.T) {
	cancelShipment := CancelShipmentRequest{
		AccountNumber: AccountNumber{Value: "123456789"}, TrackingNumber: "794953535000",
		SenderCountryCode: "US", DeletionControl: "DELETE_ALL_PACKAGES",
	}
	if err := cancelShipment.Validate(); err != nil {
		t.Fatalf("valid shipment cancellation: %v", err)
	}
	cancelShipment.DeletionControl = ""
	if err := cancelShipment.Validate(); err == nil {
		t.Fatal("missing deletion mode accepted")
	}

	schedule := validSchedulePickupRequest()
	if err := schedule.Validate(); err != nil {
		t.Fatalf("valid pickup schedule: %v", err)
	}
	schedule.PackageCount = 0
	schedule.TotalWeight.Value = -1
	if err := schedule.Validate(); err == nil || !strings.Contains(err.Error(), "packageCount") || !strings.Contains(err.Error(), "totalWeight") {
		t.Fatalf("invalid pickup error=%v", err)
	}

	groundCancel := CancelPickupRequest{
		AssociatedAccountNumber: AccountNumber{Value: "123456789"},
		PickupConfirmationCode:  "GR123", CarrierCode: "FDXG", ScheduledDate: "2026-09-03",
	}
	if err := groundCancel.Validate(); err != nil {
		t.Fatalf("valid Ground cancellation: %v", err)
	}
	expressCancel := groundCancel
	expressCancel.CarrierCode = "FDXE"
	if err := expressCancel.Validate(); err == nil || !strings.Contains(err.Error(), "location") {
		t.Fatalf("Express cancellation without location error=%v", err)
	}
	expressCancel.Location = "NQAA"
	if err := expressCancel.Validate(); err != nil {
		t.Fatalf("valid Express cancellation: %v", err)
	}
	expressCancel.ScheduledDate = "09/03/2026"
	if err := expressCancel.Validate(); err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Fatalf("bad date error=%v", err)
	}
}

func TestValidateRequestFromMapAndRawJSON(t *testing.T) {
	request := validCreateLabelRequest()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var mapped map[string]any
	if err := json.Unmarshal(data, &mapped); err != nil {
		t.Fatal(err)
	}
	mapped["optionalFutureFedExField"] = map[string]any{"keptByCaller": true}
	if err := ValidateRequest(ActionCreateLabel, mapped); err != nil {
		t.Fatalf("map request: %v", err)
	}
	if err := ValidateRequest(ActionCreateLabel, json.RawMessage(data)); err != nil {
		t.Fatalf("raw request: %v", err)
	}
	if err := ValidateRequest(ActionCreateLabel, []byte(data)); err != nil {
		t.Fatalf("byte request: %v", err)
	}
	if err := ValidateRequest("unknown", mapped); !errors.Is(err, ErrUnsupportedAction) {
		t.Fatalf("unsupported action error=%v", err)
	}
	if err := ValidateRequest(ActionCancelPickup, request); err == nil {
		t.Fatal("typed request accepted for wrong action")
	}
}

func TestCanonicalBodyHash(t *testing.T) {
	left := json.RawMessage(` { "weight": 3, "account": {"value":"123"} } `)
	right := map[string]any{"account": map[string]any{"value": "123"}, "weight": 3}
	leftHash, err := CanonicalBodySHA256(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := RequestHash(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("canonical hashes differ: %s != %s", leftHash, rightHash)
	}
	if len(leftHash) != 64 {
		t.Fatalf("hash length=%d", len(leftHash))
	}
	changedHash, err := RequestHash(map[string]any{"account": map[string]any{"value": "123"}, "weight": 4})
	if err != nil {
		t.Fatal(err)
	}
	if changedHash == leftHash {
		t.Fatal("changed request retained same hash")
	}
	if _, err := CanonicalBody(json.RawMessage(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("multiple JSON values accepted")
	}
}

func TestExtractOperationalFields(t *testing.T) {
	request := validCreateLabelRequest()
	fields, err := ExtractOperationalFields(ActionCreateLabel, request)
	if err != nil {
		t.Fatal(err)
	}
	if fields.AccountNumber != "123456789" || fields.ServiceType != "FEDEX_GROUND" || fields.PackagingType != "YOUR_PACKAGING" {
		t.Fatalf("shipment identity fields=%+v", fields)
	}
	if fields.ShipperName != "Origin LLC" || fields.RecipientName != "Destination Inc" || fields.RecipientAddress != "2 Market St, Dock 3" {
		t.Fatalf("party fields=%+v", fields)
	}
	if fields.WeightValue != 3.5 || fields.WeightUnits != "LB" || fields.Reference != "ORDER-42" || fields.PackageCount != 1 {
		t.Fatalf("package fields=%+v", fields)
	}
	if len(fields.RequestHash) != 64 {
		t.Fatalf("request hash=%q", fields.RequestHash)
	}

	pickup, err := ExtractOperationalFields(ActionSchedulePickup, validSchedulePickupRequest())
	if err != nil {
		t.Fatal(err)
	}
	if pickup.CarrierCode != "FDXE" || pickup.ScheduledDate != "2026-09-03" || pickup.ReadyDateTimestamp != "2026-09-03T14:00:00-05:00" || pickup.CustomerCloseTime != "18:00:00" {
		t.Fatalf("pickup fields=%+v", pickup)
	}
}

func validCreateLabelRequest() CreateLabelRequest {
	return CreateLabelRequest{
		LabelResponseOptions: "LABEL",
		AccountNumber:        AccountNumber{Value: "123456789"},
		RequestedShipment: RequestedShipment{
			Shipper:     Party{Contact: Contact{CompanyName: "Origin LLC", PhoneNumber: "5551112222"}, Address: Address{StreetLines: []string{"1 Main St"}, City: "Chicago", StateOrProvinceCode: "IL", PostalCode: "60601", CountryCode: "US"}},
			Recipients:  []Party{{Contact: Contact{CompanyName: "Destination Inc", PhoneNumber: "5553334444"}, Address: Address{StreetLines: []string{"2 Market St", "Dock 3"}, City: "Detroit", StateOrProvinceCode: "MI", PostalCode: "48201", CountryCode: "US"}}},
			ServiceType: "FEDEX_GROUND", PackagingType: "YOUR_PACKAGING", TotalPackageCount: 1,
			RequestedPackageLineItems: []PackageLineItem{{SequenceNumber: 1, Weight: Weight{Units: "LB", Value: 3.5}, CustomerReferences: []CustomerReference{{CustomerReferenceType: "CUSTOMER_REFERENCE", Value: "ORDER-42"}}}},
			LabelSpecification:        LabelSpecification{ImageType: "PDF", LabelStockType: "PAPER_4X6"},
		},
	}
}

func validSchedulePickupRequest() SchedulePickupRequest {
	return SchedulePickupRequest{
		AssociatedAccountNumber: AccountNumber{Value: "123456789"},
		OriginDetail: PickupOriginDetail{
			PickupLocation:     PickupLocation{Contact: Contact{CompanyName: "Origin LLC", PhoneNumber: "5551112222"}, Address: Address{StreetLines: []string{"1 Main St"}, City: "Chicago", StateOrProvinceCode: "IL", PostalCode: "60601", CountryCode: "US"}},
			ReadyDateTimestamp: "2026-09-03T14:00:00-05:00", CustomerCloseTime: "18:00:00",
		},
		TotalWeight: Weight{Units: "LB", Value: 3.5}, PackageCount: 1, CarrierCode: "FDXE",
	}
}
