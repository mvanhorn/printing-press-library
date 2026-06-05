// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMethod(t *testing.T) {
	cases := map[string]string{
		"QRIS": "qris", "virtual-account": "va", "VA": "va", "wallet": "ewallet",
		"e-wallet": "ewallet", "internet-banking": "online-banking", "retail": "retail-store",
		"credit-card": "card", "card": "card",
	}
	for in, want := range cases {
		if got := normalizeMethod(in); got != want {
			t.Errorf("normalizeMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveRoutePolicy(t *testing.T) {
	cases := []struct {
		method, override string
		wantSurface      Surface
		wantErr          bool
	}{
		{"qris", "auto", SurfaceSNAP, false},
		{"ewallet", "", SurfaceSNAP, false},
		{"va", "auto", SurfaceSNAP, false},
		{"card", "auto", SurfaceLegacy, false},
		{"bnpl", "auto", SurfaceLegacy, false},
		{"online-banking", "auto", SurfaceLegacy, false},
		{"retail-store", "auto", SurfaceLegacy, false},
		{"qris", "legacy", SurfaceLegacy, false}, // explicit override allowed
		{"card", "snap", "", true},               // cards have no SNAP surface
		{"crypto", "auto", "", true},             // unknown method
		{"qris", "bogus", "", true},              // invalid override
	}
	for _, c := range cases {
		route, err := resolveRoute(paymentRoutes, c.method, c.override)
		if c.wantErr != (err != nil) {
			t.Errorf("resolveRoute(%q,%q) err = %v, wantErr %v", c.method, c.override, err, c.wantErr)
			continue
		}
		if err == nil && route.Surface != c.wantSurface {
			t.Errorf("resolveRoute(%q,%q) surface = %s, want %s", c.method, c.override, route.Surface, c.wantSurface)
		}
	}
}

func TestResolveRoutePayoutLegacyRejected(t *testing.T) {
	// payout routes have no LegacyType, so --surface legacy must error at the
	// resolver level (the friendly 'disbursements submit' guidance is handled
	// separately by the payout command before this is reached).
	if _, err := resolveRoute(payoutRoutes, "bank", "legacy"); err == nil {
		t.Error("resolveRoute(payoutRoutes, bank, legacy) should error: payout has no legacy surface")
	}
	if _, err := resolveRoute(payoutRoutes, "ewallet", "legacy"); err == nil {
		t.Error("resolveRoute(payoutRoutes, ewallet, legacy) should error: payout has no legacy surface")
	}
}

func TestBuildSNAPPayBody(t *testing.T) {
	// qris dispatches to buildQrisGenerateBody: amount.value + auto partnerReferenceNo.
	route := paymentRoutes["qris"]
	body := buildSNAPPayBody(route, "50000.00", "IDR", "", "", "")
	amt, ok := body["amount"].(map[string]any)
	if !ok || amt["value"] != "50000.00" || amt["currency"] != "IDR" {
		t.Errorf("qris body amount wrong: %v", body)
	}
	if body["partnerReferenceNo"] == "" {
		t.Error("partnerReferenceNo not auto-generated")
	}

	// ewallet dispatches to buildEwalletPayBody and must use the SAME field
	// names (additionalInfo.phoneNumber / additionalInfo.channelCode).
	ew := buildSNAPPayBody(paymentRoutes["ewallet"], "75000.00", "IDR", "", "SHOPEEPAY", "081234567890")
	want := buildEwalletPayBody("75000.00", "IDR", "", "081234567890", "SHOPEEPAY")
	ewAI, ok := ew["additionalInfo"].(map[string]any)
	if !ok {
		t.Fatalf("ewallet body missing additionalInfo: %v", ew)
	}
	wantAI := want["additionalInfo"].(map[string]any)
	if ewAI["phoneNumber"] != wantAI["phoneNumber"] || ewAI["channelCode"] != wantAI["channelCode"] {
		t.Errorf("ewallet body field names diverge from buildEwalletPayBody: got %v, want fields %v", ewAI, wantAI)
	}
	if _, ok := ew["amount"].(map[string]any); !ok {
		t.Errorf("ewallet body should carry amount object: %v", ew)
	}

	// va dispatches to buildVaCreateBody (closed-amount) -> totalAmount.
	va := buildSNAPPayBody(paymentRoutes["va"], "150000.00", "IDR", "bca", "", "0812")
	if _, ok := va["totalAmount"]; !ok {
		t.Errorf("va body should use totalAmount: %v", va)
	}
}

func TestCheckSNAPEnvelope(t *testing.T) {
	cases := []struct {
		raw     string
		status  int
		wantErr bool
	}{
		{`{"responseCode":"2001800","responseMessage":"Successful"}`, 200, false},
		{`{"responseCode":"2021800","responseMessage":"In Progress"}`, 202, false},
		{`{"responseCode":"4011800","responseMessage":"Unauthorized"}`, 401, true},
		{`{"responseCode":"4091800","responseMessage":"Conflict"}`, 200, true}, // body-level failure on HTTP 200
		{`{}`, 500, true},
	}
	for _, c := range cases {
		err := checkSNAPEnvelope(json.RawMessage(c.raw), c.status)
		if c.wantErr != (err != nil) {
			t.Errorf("checkSNAPEnvelope(%s, %d) err = %v, wantErr %v", c.raw, c.status, err, c.wantErr)
		}
	}
}
