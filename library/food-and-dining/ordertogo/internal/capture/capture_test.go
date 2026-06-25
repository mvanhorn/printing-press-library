// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package capture

import (
	"os"
	"path/filepath"
	"testing"
)

const realBody = `{"param":{"customerphone":"5209076052","customername":"Matt","addtionalIns":"","restname":"mixsushibarlin","orderdetails":{"items":[{"item_id":19019,"price":0}],"subtotal":2.99},"restid":72,"deviceId":"dev123","mobileId":"mob123","context":{"rewards":{"availablePoints":58},"meshuser":{"id":961227}},"paymentCard":{"cardType":"StripeElement","st_cus_id":"cus_T7CWmLOtr5RBLw","tip":0.15,"defaultCardMap":{"key":"MASTERCARD_2126_931"},"lastname":"Van Horn","firstname":"Matt","phonenum":"5209076052","billingAddress1":""}}}`

func wantFields(t *testing.T, pc *PaymentConfig) {
	t.Helper()
	if pc.StripeCustomerID != "cus_T7CWmLOtr5RBLw" {
		t.Errorf("st_cus_id = %q", pc.StripeCustomerID)
	}
	if pc.StripeDefaultCard != "MASTERCARD_2126_931" {
		t.Errorf("card = %q", pc.StripeDefaultCard)
	}
	if pc.CustomerFirstName != "Matt" || pc.CustomerLastName != "Van Horn" {
		t.Errorf("name = %q %q", pc.CustomerFirstName, pc.CustomerLastName)
	}
	if pc.CustomerPhone != "5209076052" {
		t.Errorf("phone = %q", pc.CustomerPhone)
	}
}

func TestExtractPaymentConfig_FullBody(t *testing.T) {
	pc, err := ExtractPaymentConfig([]byte(realBody))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantFields(t, pc)
}

func TestExtractPaymentConfig_RequestShapeFields(t *testing.T) {
	pc, err := ExtractPaymentConfig([]byte(realBody))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pc.DeviceID != "dev123" || pc.MobileID != "mob123" {
		t.Errorf("device/mobile = %q %q", pc.DeviceID, pc.MobileID)
	}
	if pc.OrderContextJSON == "" || !contains(pc.OrderContextJSON, "availablePoints") {
		t.Errorf("context not extracted: %q", pc.OrderContextJSON)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestExtractPaymentConfig_Truncated(t *testing.T) {
	// Cut mid-billingAddress to simulate the prefix_2000 artifact.
	truncated := realBody[:len(realBody)-15]
	pc, err := ExtractPaymentConfig([]byte(truncated))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantFields(t, pc)
}

func TestExtractPaymentConfig_NoFields(t *testing.T) {
	if _, err := ExtractPaymentConfig([]byte(`{"unrelated":true}`)); err == nil {
		t.Fatal("expected error when no payment fields present")
	}
}

func TestLoadPaymentConfig_HAR(t *testing.T) {
	har := `{"log":{"entries":[
		{"request":{"method":"GET","url":"https://www.ordertogo.com/m/api/restaurants/mixsushibarlin/menus/full"}},
		{"request":{"method":"POST","url":"https://www.ordertogo.com/m/api/postmicmeshorder","postData":{"text":` + quote(realBody) + `}}}
	]}}`
	dir := t.TempDir()
	path := filepath.Join(dir, "order.har")
	if err := os.WriteFile(path, []byte(har), 0o600); err != nil {
		t.Fatal(err)
	}
	pc, err := LoadPaymentConfig(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantFields(t, pc)
}

func TestLoadPaymentConfig_Artifact(t *testing.T) {
	artifact := `{"endpoint":{"path":"/m/api/postmicmeshorder"},"actual_observed_body_prefix_2000":` + quote(realBody) + `}`
	dir := t.TempDir()
	path := filepath.Join(dir, "captured.json")
	if err := os.WriteFile(path, []byte(artifact), 0o600); err != nil {
		t.Fatal(err)
	}
	pc, err := LoadPaymentConfig(path)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantFields(t, pc)
}

func TestLoadPaymentConfig_HARNoOrder(t *testing.T) {
	har := `{"log":{"entries":[{"request":{"method":"GET","url":"https://www.ordertogo.com/m/api/restaurants/x"}}]}}`
	dir := t.TempDir()
	path := filepath.Join(dir, "noorder.har")
	if err := os.WriteFile(path, []byte(har), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPaymentConfig(path); err == nil {
		t.Fatal("expected error when HAR has no order POST")
	}
}

// quote JSON-encodes s as a string literal (with surrounding quotes).
func quote(s string) string {
	var out []byte
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"', '\\':
			out = append(out, '\\', s[i])
		default:
			out = append(out, s[i])
		}
	}
	out = append(out, '"')
	return string(out)
}
