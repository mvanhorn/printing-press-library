// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestParseLabelResponsePieceVariant(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7\n%%EOF\n"))
	response := []byte(`{
		"transactionId":"tx-ground","customerTransactionId":"customer-1",
		"output":{"transactionShipments":[{
			"masterTrackingNumber":"MASTER1","serviceType":"FEDEX_GROUND","serviceName":"FedEx Ground",
			"pieceResponses":[{"trackingNumber":"TRACK1","netChargeAmount":28.06,"baseCharge":31.25,"currency":"USD",
				"packageDocuments":[{"contentType":"LABEL","docType":"PDF","encodedLabel":"` + encoded + `"}]}],
			"completedShipmentDetail":{"carrierCode":"FDXG","packagingDescription":"YOUR_PACKAGING"}
		}]}}
	`)
	result, pdf, err := ParseAndDecodeLabelResponse(response, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.TransactionID != "tx-ground" || result.TrackingNumber != "TRACK1" || result.MasterTrackingNumber != "MASTER1" {
		t.Fatalf("identifiers=%+v", result)
	}
	if result.ServiceType != "FEDEX_GROUND" || result.CarrierCode != "FDXG" || result.PackagingType != "YOUR_PACKAGING" {
		t.Fatalf("service fields=%+v", result)
	}
	if result.NetChargeAmount != 28.06 || result.ListChargeAmount != 31.25 || result.Currency != "USD" {
		t.Fatalf("charges=%+v", result)
	}
	if string(pdf) != "%PDF-1.7\n%%EOF\n" {
		t.Fatalf("pdf=%q", pdf)
	}
}

func TestParseLabelResponseShipmentDocumentAndRatingVariant(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n%%EOF\n"))
	response := []byte(`{
		"transactionId":"tx-express",
		"output":{"transactionShipments":[{
			"serviceType":"PRIORITY_OVERNIGHT","pieceResponses":[{"trackingNumber":"TRACK2"}],"shipmentDocuments":[
				{"contentType":"COMMERCIAL_INVOICE","docType":"PDF","encodedLabel":"ignored-sensitive-document"},
				{"contentType":"LABEL","docType":"PDF","encodedLabel":"` + encoded + `"}],
			"completedShipmentDetail":{"carrierCode":"FDXE","masterTrackingId":{"trackingNumber":"TRACK2"},
				"shipmentRating":{"shipmentRateDetails":[
					{"rateType":"PAYOR_LIST_PACKAGE","totalNetCharge":"45.00","currency":"USD"},
					{"rateType":"PAYOR_ACCOUNT_PACKAGE","totalNetCharge":"32.40","totalBaseCharge":"40.00","currency":"USD"}
				]}}
		}]}}
	`)
	result, err := ParseLabelResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	if result.TrackingNumber != "TRACK2" || result.MasterTrackingNumber != "TRACK2" || result.CarrierCode != "FDXE" {
		t.Fatalf("identifiers=%+v", result)
	}
	if result.NetChargeAmount != 32.4 || result.ListChargeAmount != 40 || result.Currency != "USD" {
		t.Fatalf("account rating not selected: %+v", result)
	}
	if result.EncodedLabel != encoded || len(result.Labels) != 1 {
		t.Fatalf("label selection=%+v", result.Labels)
	}
}

func TestParseLabelResponseRejectsIncompleteAndMalformed(t *testing.T) {
	if _, err := ParseLabelResponse([]byte(`{"output":{"transactionShipments":[{"masterTrackingNumber":"TRACK","pieceResponses":[{"trackingNumber":"TRACK"}]}]}}`)); !errors.Is(err, ErrLabelMissing) {
		t.Fatalf("missing label error=%v", err)
	}
	if _, err := ParseLabelResponse([]byte(`{"output":{"transactionShipments":[]}}`)); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("missing shipment error=%v", err)
	}
	if _, err := ParseLabelResponse([]byte(`not json`)); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("malformed response error=%v", err)
	}
	if _, err := ParseLabelResponse([]byte(`{"output":{"transactionShipments":[{},{}]}}`)); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("multiple shipment error=%v", err)
	}
	if _, err := ParseLabelResponse([]byte(`{"output":{"transactionShipments":[{"masterTrackingNumber":"TRACK","pieceResponses":[{"trackingNumber":"TRACK","packageDocuments":[{"encodedLabel":"one","contentType":"LABEL","docType":"PDF"},{"encodedLabel":"two","contentType":"LABEL","docType":"PDF"}]}]}]}}`)); !errors.Is(err, ErrInvalidResponse) {
		t.Fatalf("multiple label error=%v", err)
	}
	if _, err := ParseLabelResponse([]byte(`{"output":{"transactionShipments":[{"masterTrackingNumber":"TRACK","pieceResponses":[{"trackingNumber":"TRACK","packageDocuments":[{"encodedLabel":"invoice","contentType":"application/pdf","docType":"COMMERCIAL_INVOICE"}]}]}]}}`)); !errors.Is(err, ErrLabelMissing) {
		t.Fatalf("non-label document error=%v", err)
	}
}

func TestParsePickupGroundAndExpressVariants(t *testing.T) {
	ground, err := ParsePickupResponse([]byte(`{
		"transactionId":"pickup-ground","output":{"pickupConfirmationCode":"GR123","location":"GND1","message":"Pickup scheduled"}}
	`))
	if err != nil {
		t.Fatal(err)
	}
	if ground.ConfirmationNumber != "GR123" || ground.LocationCode != "GND1" || ground.TransactionID != "pickup-ground" {
		t.Fatalf("Ground result=%+v", ground)
	}

	express, err := ParsePickupResponse([]byte(`{
		"transactionId":"pickup-express","output":{"pickupConfirmationDetail":{"confirmationNumber":"EX456","locationCode":"NQAA","carrierCode":"FDXE","scheduledDate":"2026-09-03"}}}
	`))
	if err != nil {
		t.Fatal(err)
	}
	if express.ConfirmationNumber != "EX456" || express.LocationCode != "NQAA" || express.CarrierCode != "FDXE" || express.ScheduledDate != "2026-09-03" {
		t.Fatalf("Express result=%+v", express)
	}
	if _, err := ParsePickupResponse([]byte(`{"output":{"message":"no confirmation"}}`)); !errors.Is(err, ErrConfirmationMissing) {
		t.Fatalf("missing confirmation error=%v", err)
	}
}

func TestParseCancellationVariants(t *testing.T) {
	shipment, err := ParseCancelShipmentResponse([]byte(`{"transactionId":"cancel-ship","output":{"cancelledShipment":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !shipment.SuccessKnown || !shipment.Successful || shipment.TransactionID != "cancel-ship" {
		t.Fatalf("shipment cancel=%+v", shipment)
	}

	pickup, err := ParseCancelPickupResponse([]byte(`{
		"transactionId":"cancel-pickup","output":{"pickupConfirmationCode":"EX456","location":"NQAA","cancelConfirmationMessage":"Pickup request has been cancelled"}}
	`))
	if err != nil {
		t.Fatal(err)
	}
	if !pickup.SuccessKnown || !pickup.Successful || pickup.ConfirmationNumber != "EX456" || pickup.LocationCode != "NQAA" {
		t.Fatalf("pickup cancel=%+v", pickup)
	}
	rejected, err := ParseCancelShipmentResponse([]byte(`{"output":{"cancelledShipment":false},"unrelated":{"success":true,"message":"recipient data"}}`))
	if err != nil || !rejected.SuccessKnown || rejected.Successful {
		t.Fatalf("shipment rejection=%+v err=%v", rejected, err)
	}
	ambiguous, err := ParseCancelPickupResponse([]byte(`{"output":{"message":"SUCCESS"}}`))
	if err != nil || ambiguous.SuccessKnown || ambiguous.Successful {
		t.Fatalf("ambiguous pickup cancellation=%+v err=%v", ambiguous, err)
	}
}

func TestParseResponseDispatch(t *testing.T) {
	if _, err := ParseResponse("unknown", []byte(`{}`)); !errors.Is(err, ErrUnsupportedAction) {
		t.Fatalf("unsupported parse error=%v", err)
	}
}
