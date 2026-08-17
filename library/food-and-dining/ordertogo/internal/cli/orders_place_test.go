// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/store"
)

// The 16 param keys the live web client (order #29, HTTP 200) sends. The CLI's
// built body must carry all of them — the original stub omitted the last 9,
// which is why the server rejected/timed out CLI-built orders.
var workingParamKeys = []string{
	"customerphone", "customername", "addtionalIns", "restname", "orderdetails",
	"restid", "paymentCard", "groupname", "enableGroupRewardpoints",
	"enableRewardpoints", "tax", "context", "isSelfcheckoutOnly", "deviceId",
	"mobileId", "orderType",
}

func TestBuildPostOrderBody_MatchesWorkingShape(t *testing.T) {
	cfg := &config.Config{
		CustomerFirstName: "Example", CustomerLastName: "User", CustomerPhone: "2025550147",
		StripeCustomerID: "cus_example", StripeDefaultCard: "VISA_4242_1230",
		BillingAddress1: "123 Example Street", BillingAddress2: "Unit 4", BillingCity: "Exampleville", BillingState: "wa",
		DeviceID: "dev123", MobileID: "mob123",
		OrderContextJSON: `{"rewards":{"availablePoints":58},"meshuser":{"id":123456}}`,
	}
	items := []cartItem{{ItemID: 19019, Price: 2.99, Togo: "1"}}
	body := buildPostOrderBody(cfg, items, 2.99, 0, 0, "mixsushibarlin", 72)
	if body.Param.PaymentCard.BillingAddress2 != "Unit 4" {
		t.Fatalf("billingAddress2 = %q, want Unit 4", body.Param.PaymentCard.BillingAddress2)
	}

	raw, _ := json.Marshal(body)
	var wrapped map[string]map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	param := wrapped["param"]
	for _, k := range workingParamKeys {
		if _, ok := param[k]; !ok {
			t.Errorf("param missing field %q (server rejects orders without it)", k)
		}
	}
	if len(param) != len(workingParamKeys) {
		got := make([]string, 0, len(param))
		for k := range param {
			got = append(got, k)
		}
		t.Errorf("param has %d keys, want %d. got=%v", len(param), len(workingParamKeys), got)
	}
}

func TestBuildPostOrderBody_ConfiguredTaxRate(t *testing.T) {
	cfg := &config.Config{OrderTaxRate: 0.0825}
	body := buildPostOrderBody(cfg, []cartItem{{ItemID: 1, Price: 10}}, 10, 0, 0, "slug", 72)
	if body.Param.Tax != 0.83 {
		t.Fatalf("tax = %v, want 0.83", body.Param.Tax)
	}
}

func TestBuildPostOrderBody_TaxEstimate(t *testing.T) {
	cfg := &config.Config{StripeCustomerID: "c", StripeDefaultCard: "k"}
	// 2.99 * 0.103 = 0.30797 -> 0.31, matching the captured working order's tax.
	body := buildPostOrderBody(cfg, []cartItem{{ItemID: 1, Price: 2.99}}, 2.99, 0, 0, "slug", 72)
	if body.Param.Tax != 0.31 {
		t.Errorf("tax = %v, want 0.31 (matches captured order)", body.Param.Tax)
	}
}

func TestBuildPostOrderBody_ScalarConstants(t *testing.T) {
	cfg := &config.Config{StripeCustomerID: "c", StripeDefaultCard: "k", DeviceID: "d", MobileID: "m"}
	p := buildPostOrderBody(cfg, []cartItem{{ItemID: 1, Price: 1}}, 1, 0, 0, "myslug", 72).Param
	if p.GroupName != "myslug" || p.OrderType != "1" || !p.EnableRewardpoints || p.EnableGroupRewardpoints || p.IsSelfcheckoutOnly {
		t.Errorf("scalar constants wrong: %+v", p)
	}
	if p.DeviceID != "d" || p.MobileID != "m" {
		t.Errorf("device/mobile not threaded: %q %q", p.DeviceID, p.MobileID)
	}
}

func TestLoadCart_ReuseLastFailsClosed(t *testing.T) {
	_, _, _, _, _, err := loadCart("", true, "slug", 72)
	if err == nil || !strings.Contains(err.Error(), "does not preserve item option strings") {
		t.Fatalf("loadCart(--reuse-last) error = %v", err)
	}
}

func TestBuildOrderContext_VerbatimOrNil(t *testing.T) {
	if buildOrderContext(&config.Config{}) != nil {
		t.Error("empty context config should yield nil (omitted)")
	}
	ctx := buildOrderContext(&config.Config{OrderContextJSON: `{"rewards":{"availablePoints":58}}`})
	if ctx == nil {
		t.Fatal("configured context should parse")
	}
	if _, ok := ctx["rewards"]; !ok {
		t.Error("context should preserve rewards key")
	}
}

func TestPlaceCooldown(t *testing.T) {
	t.Setenv("ORDERTOGO_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	path := placeAttemptPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)

	// No prior attempt -> no cooldown.
	_ = os.Remove(path)
	if placeCooldownRemaining() != 0 {
		t.Error("no recorded attempt should yield zero cooldown")
	}
	// Fresh attempt -> cooldown active.
	recordPlaceAttempt()
	if placeCooldownRemaining() <= 0 {
		t.Error("a just-recorded attempt should yield a positive cooldown")
	}
	// Old attempt -> window passed.
	_ = os.WriteFile(path, []byte(time.Now().Add(-placeCooldownWindow-time.Minute).Format(time.RFC3339)), 0o600)
	if placeCooldownRemaining() != 0 {
		t.Error("an attempt older than the window should yield zero cooldown")
	}
}

func TestNewRequestID_Format(t *testing.T) {
	// epoch-millis "_" 4-digit suffix, e.g. 1782343972960_7398
	re := regexp.MustCompile(`^\d{13}_\d{4}$`)
	id := newRequestID()
	if !re.MatchString(id) {
		t.Errorf("requestid %q does not match <13-digit-ms>_<4-digit>", id)
	}
}

func TestRedactPostOrderBody_RemovesAccountData(t *testing.T) {
	body := buildPostOrderBody(&config.Config{
		CustomerFirstName: "Example", CustomerLastName: "User", CustomerPhone: "2025550147",
		StripeCustomerID: "cus_secret", StripeDefaultCard: "card_secret",
		BillingAddress1: "123 Example Street", BillingCity: "Exampleville", BillingState: "WA",
		DeviceID: "device-secret", MobileID: "mobile-secret",
		OrderContextJSON: `{"meshuser":{"email":"private@example.com"}}`,
	}, []cartItem{{ItemID: 1, Price: 1}}, 1, 0, 0, "slug", 72)
	raw, err := json.Marshal(redactPostOrderBody(body))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"Example", "User", "2025550147", "cus_secret", "card_secret", "123 Example Street", "private@example.com", "device-secret", "mobile-secret"} {
		if regexp.MustCompile(regexp.QuoteMeta(secret)).Match(raw) {
			t.Errorf("redacted body still contains %q: %s", secret, raw)
		}
	}
}

func TestParsePostOrderResponse_RejectsInvalidSuccessBodies(t *testing.T) {
	for _, body := range []string{`not-json`, `{"transaction":{"amount":12.34}}`} {
		if _, err := parsePostOrderResponse([]byte(body), "slug"); err == nil {
			t.Errorf("parsePostOrderResponse(%q) returned nil error", body)
		}
	}
}

func TestParsePostOrderResponse_BuildsTrackURL(t *testing.T) {
	result, err := parsePostOrderResponse([]byte(`{"transaction":{"orderid":42,"amount":12.34},"order":{"orderToken":"restaurant_slug_42"}}`), "fallback")
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderID != 42 || result.Total != 12.34 || result.TrackURL != "/trackorder/restaurant_slug/42" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestPersistPlacedOrderWritesLocalHistory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "orders.db")
	result := postOrderResult{OrderID: 42, Total: 12.34, Tax: 0.84, Tip: 1.50, CardType: "Visa", OrderedAt: time.Now()}
	items := []cartItem{{ItemID: 19001, Price: 9.99}}
	if err := persistPlacedOrder(context.Background(), dbPath, result, items, 9.99, 0, 0, "example-restaurant", 72); err != nil {
		t.Fatal(err)
	}

	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	order, err := db.LastOrder("example-restaurant")
	if err != nil {
		t.Fatal(err)
	}
	if order == nil || order.OrderID != "42" || order.RestID != "72" || len(order.Items) != 1 || order.Items[0].ID != "19001" {
		t.Fatalf("unexpected persisted order: %+v", order)
	}
	if order.Total != 12.34 || order.Tax != 0.84 || order.Tip != 1.50 {
		t.Fatalf("unexpected persisted totals: %+v", order)
	}
}

func TestConfiguredHeader_UsesCapturedValue(t *testing.T) {
	cfg := &config.Config{Headers: map[string]string{"sec-ch-ua": "captured"}}
	if got := configuredHeader(cfg, "Sec-Ch-Ua", "fallback"); got != "captured" {
		t.Fatalf("configuredHeader() = %q", got)
	}
}
