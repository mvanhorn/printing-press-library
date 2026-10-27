// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

const shipResponse = `{"output":{"transactionShipments":[{"masterTrackingNumber":"794600000001","serviceType":"FEDEX_GROUND","pieceResponses":[{"trackingNumber":"794600000001","netCharge":12.34,"currency":"USD","packageDocuments":[{"contentType":"LABEL","docType":"PDF","encodedLabel":"JVBERi0xLjcKJSVFT0YK"}]}]}]}}`

func shipRequest(ref string) map[string]any {
	return map[string]any{
		"labelResponseOptions": "LABEL",
		"accountNumber":        map[string]any{"value": "123456789"},
		"requestedShipment": map[string]any{
			"pickupType":             "USE_SCHEDULED_PICKUP",
			"serviceType":            "FEDEX_GROUND",
			"packagingType":          "YOUR_PACKAGING",
			"shippingChargesPayment": map[string]any{"paymentType": "SENDER"},
			"totalPackageCount":      1,
			"shipper": map[string]any{
				"contact": map[string]any{"personName": "Ship Er", "phoneNumber": "5551112222"},
				"address": map[string]any{"streetLines": []any{"1 Main St"}, "city": "Seattle", "stateOrProvinceCode": "WA", "postalCode": "98101", "countryCode": "US"},
			},
			"recipients": []any{map[string]any{
				"contact": map[string]any{"personName": "Re Cipient", "phoneNumber": "5553334444"},
				"address": map[string]any{"streetLines": []any{"1 Main St"}, "city": "Portland", "stateOrProvinceCode": "OR", "postalCode": "97201", "countryCode": "US"},
			}},
			"requestedPackageLineItems": []any{map[string]any{
				"sequenceNumber":     1,
				"weight":             map[string]any{"units": "LB", "value": 2.5},
				"customerReferences": []any{map[string]any{"customerReferenceType": "CUSTOMER_REFERENCE", "value": ref}},
			}},
			"labelSpecification": map[string]any{"imageType": "PDF", "labelStockType": "PAPER_4X6"},
		},
	}
}

func archivedShipments(t *testing.T) []store.Shipment {
	t.Helper()
	st, err := store.Open("")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rows, err := st.DB().Query(`SELECT tracking_number, account, recipient_city, weight_value, net_charge_amount, net_charge_currency, COALESCE(raw_response, '') FROM shipments`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []store.Shipment
	for rows.Next() {
		var sp store.Shipment
		if err := rows.Scan(&sp.TrackingNumber, &sp.Account, &sp.RecipientCity, &sp.WeightValue, &sp.NetChargeAmount, &sp.NetChargeCurrency, &sp.RawResponse); err != nil {
			t.Fatal(err)
		}
		out = append(out, sp)
	}
	return out
}

func checkArchivedRow(t *testing.T) {
	t.Helper()
	rows := archivedShipments(t)
	if len(rows) != 1 {
		t.Fatalf("archived %d shipments, want 1", len(rows))
	}
	row := rows[0]
	if row.TrackingNumber != "794600000001" || row.NetChargeAmount != 12.34 || row.NetChargeCurrency != "USD" ||
		row.RecipientCity != "Portland" || row.WeightValue != 2.5 || row.Account != "123456789" {
		t.Fatalf("archived row missing operational fields: %+v", row)
	}
	if row.RawResponse != "" {
		t.Fatalf("archive stored the raw response: %q", row.RawResponse)
	}
}

func TestShipmentCreateArchivesShipment(t *testing.T) {
	newFakeAPI(t, http.StatusOK, shipResponse)
	result, envelope := callTool(t, registeredTools(t), "shipments_create", map[string]any{"request": shipRequest("A")})
	if result.IsError {
		t.Fatalf("shipments_create=%v", envelope)
	}
	if _, ok := envelope["archive_warning"]; ok {
		t.Fatalf("archive warning: %v", envelope)
	}
	checkArchivedRow(t)
}

func TestShipBulkArchivesShipments(t *testing.T) {
	newFakeAPI(t, http.StatusOK, shipResponse)
	result, envelope := callTool(t, registeredTools(t), "ship_bulk", map[string]any{"shipments": []any{shipRequest("B")}})
	if result.IsError {
		t.Fatalf("ship_bulk=%v", envelope)
	}
	checkArchivedRow(t)
}

func TestWriteSummaryKeepsPieceNetCharge(t *testing.T) {
	var decoded any
	if err := json.Unmarshal([]byte(shipResponse), &decoded); err != nil {
		t.Fatal(err)
	}
	summary, _ := json.Marshal(summarizeWriteResponse("shipments_create", decoded))
	text := string(summary)
	for _, want := range []string{`"netCharge":12.34`, `"currency":"USD"`, `"trackingNumber":"794600000001"`} {
		if !contains(text, want) {
			t.Fatalf("summary %s missing %s", text, want)
		}
	}
	if contains(text, "JVBERi0xLjcKJSVFT0YK") {
		t.Fatalf("summary kept encoded label: %s", text)
	}
}

func TestShipBulkHonorsCancellation(t *testing.T) {
	fake := newFakeAPI(t, http.StatusOK, shipResponse)
	tools := registeredTools(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := tools["ship_bulk"].Handler(ctx, toolRequest(map[string]any{"shipments": []any{shipRequest("C"), shipRequest("D")}}))
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(toolResultText(t, result)), &envelope); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 0 || envelope["status"] != "stopped" {
		t.Fatalf("canceled ship_bulk calls=%d result=%v, want nothing sent", fake.calls, envelope)
	}
	counts, _ := envelope["counts"].(map[string]any)
	if counts["not_attempted"] != float64(2) {
		t.Fatalf("counts=%v, want 2 not_attempted", counts)
	}
}

func TestWritePacerSpacesCallsAndNeverRampsUp(t *testing.T) {
	p := &pacer{interval: 40 * time.Millisecond}
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := p.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 110*time.Millisecond {
		t.Fatalf("4 paced writes took %v, want >= 3 intervals", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.wait(ctx); err == nil {
		t.Fatal("pacer ignored a canceled context")
	}
	// Canceled waiters must not reserve slots for later writes.
	p2 := &pacer{interval: 50 * time.Millisecond}
	if err := p2.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := p2.wait(ctx); err == nil {
			t.Fatal("canceled wait returned nil")
		}
	}
	start = time.Now()
	if err := p2.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("next write waited %v after canceled callers, want about one interval", elapsed)
	}
	if writeInterval < 1001*time.Millisecond {
		t.Fatalf("writeInterval=%v must stay below 1 req/s", writeInterval)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
