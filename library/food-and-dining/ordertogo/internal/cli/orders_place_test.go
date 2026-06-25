// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
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
		CustomerFirstName: "Matt", CustomerLastName: "Van Horn", CustomerPhone: "5209076052",
		StripeCustomerID: "cus_x", StripeDefaultCard: "MASTERCARD_4623_1131",
		BillingAddress1: "6650 e mercer way", BillingCity: "mercer island", BillingState: "wa",
		DeviceID: "dev123", MobileID: "mob123",
		OrderContextJSON: `{"rewards":{"availablePoints":58},"meshuser":{"id":961227}}`,
	}
	items := []cartItem{{ItemID: 19019, Price: 2.99, Togo: "1"}}
	body := buildPostOrderBody(cfg, items, 2.99, 0, 0, "mixsushibarlin", 72)

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
