// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package capture

import (
	"os"
	"path/filepath"
	"testing"
)

const realBody = `{"param":{"customerphone":"2025550147","customername":"Example","addtionalIns":"","restname":"mixsushibarlin","orderdetails":{"items":[{"item_id":19019,"price":0}],"subtotal":2.99},"restid":72,"tax":0.31,"deviceId":"device-example","mobileId":"mobile-example","context":{"rewards":{"availablePoints":58},"meshuser":{"id":123456}},"paymentCard":{"cardType":"StripeElement","st_cus_id":"cus_example_123","tip":0.15,"defaultCardMap":{"key":"VISA_4242_1230"},"lastname":"User","firstname":"Example","phonenum":"2025550147","billingAddress1":"123 Example Street","billingAddress2":"Unit 4","billingCity":"Exampleville","billingState":"WA"}}}`

func wantFields(t *testing.T, pc *PaymentConfig) {
	t.Helper()
	if pc.StripeCustomerID != "cus_example_123" {
		t.Errorf("st_cus_id = %q", pc.StripeCustomerID)
	}
	if pc.StripeDefaultCard != "VISA_4242_1230" {
		t.Errorf("card = %q", pc.StripeDefaultCard)
	}
	if pc.CustomerFirstName != "Example" || pc.CustomerLastName != "User" {
		t.Errorf("name = %q %q", pc.CustomerFirstName, pc.CustomerLastName)
	}
	if pc.CustomerPhone != "2025550147" {
		t.Errorf("phone = %q", pc.CustomerPhone)
	}
}

func TestExtractPaymentConfig_FullBody(t *testing.T) {
	pc, err := ExtractPaymentConfig([]byte(realBody))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantFields(t, pc)
	if pc.BillingAddress1 != "123 Example Street" || pc.BillingAddress2 != "Unit 4" || pc.BillingCity != "Exampleville" || pc.BillingState != "WA" {
		t.Errorf("billing address not extracted: %+v", pc)
	}
}

func TestExtractPaymentConfig_RequestShapeFields(t *testing.T) {
	pc, err := ExtractPaymentConfig([]byte(realBody))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pc.DeviceID != "device-example" || pc.MobileID != "mobile-example" {
		t.Errorf("device/mobile = %q %q", pc.DeviceID, pc.MobileID)
	}
	if pc.OrderContextJSON == "" || !contains(pc.OrderContextJSON, "availablePoints") {
		t.Errorf("context not extracted: %q", pc.OrderContextJSON)
	}
	if pc.OrderTaxRate < 0.1036 || pc.OrderTaxRate > 0.1037 {
		t.Errorf("tax rate = %v", pc.OrderTaxRate)
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
	if pc.BillingAddress2 != "Unit 4" || pc.BillingCity != "Exampleville" {
		t.Errorf("complete billing fields before truncation were not extracted: %+v", pc)
	}
}

func TestExtractPaymentConfig_NoFields(t *testing.T) {
	if _, err := ExtractPaymentConfig([]byte(`{"unrelated":true}`)); err == nil {
		t.Fatal("expected error when no payment fields present")
	}
}

func TestLoadPaymentConfig_HAR(t *testing.T) {
	har := `{"log":{"entries":[
		{"request":{"method":"GET","url":"https://www.ordertogo.com/m/api/restaurants/mixsushibarlin/menus/full"}},
		{"request":{"method":"POST","url":"https://www.ordertogo.com/m/api/postmicmeshorder","headers":[{"name":"User-Agent","value":"ExampleBrowser/1.0"},{"name":"Sec-Ch-Ua","value":"ExampleBrand"},{"name":"Sec-Ch-Ua-Mobile","value":"?0"},{"name":"Sec-Ch-Ua-Platform","value":"ExampleOS"}],"postData":{"text":` + quote(realBody) + `}}}
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
	if pc.UserAgent != "ExampleBrowser/1.0" || pc.SecCHUA != "ExampleBrand" || pc.SecCHUAMobile != "?0" || pc.SecCHUAPlatform != "ExampleOS" {
		t.Errorf("browser headers not extracted: %+v", pc)
	}
}

func TestJSONStringFieldDecodesEscapes(t *testing.T) {
	body := []byte(`{"field":"line\nslash\\quote\"unicode \u263a"}`)
	if got, want := jsonStringField(body, "field"), "line\nslash\\quote\"unicode ☺"; got != want {
		t.Fatalf("jsonStringField() = %q, want %q", got, want)
	}
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
