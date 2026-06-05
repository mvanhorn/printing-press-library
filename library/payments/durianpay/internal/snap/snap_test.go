// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
package snap

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestMinifyJSON(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"object with whitespace", "{ \"amount\" : \"10000.00\",\n \"currency\": \"IDR\" }", `{"amount":"10000.00","currency":"IDR"}`},
		{"already minified", `{"a":1}`, `{"a":1}`},
		{"empty", "", ""},
		{"whitespace only", "  \n ", ""},
		{"non-json passthrough", "plain text", "plain text"},
		{"number precision preserved", `{"n": 10000.50}`, `{"n":10000.50}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(MinifyJSON([]byte(c.in))); got != c.want {
				t.Errorf("MinifyJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestStringToSign(t *testing.T) {
	body := []byte("{ \"amount\" : \"10000.00\",\n \"currency\": \"IDR\" }")
	got := StringToSign("post", "/v1.0/balance-inquiry", "tok123", body, "2026-06-03T17:00:00.000+07:00")
	want := "POST:/v1.0/balance-inquiry:tok123:06e8d2e2e4e926e93b0878fcb4ed14f9c2b596b9603301ff8bd796b61b8d2485:2026-06-03T17:00:00.000+07:00"
	if got != want {
		t.Errorf("StringToSign = %q, want %q", got, want)
	}
}

func TestStringToSignEmptyBody(t *testing.T) {
	got := StringToSign("GET", "/v1.0/transfer/status", "tok", nil, "2026-06-03T17:00:00.000+07:00")
	if !strings.Contains(got, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") {
		t.Errorf("empty-body hash missing from %q", got)
	}
}

func TestSignTransaction(t *testing.T) {
	sts := "POST:/v1.0/balance-inquiry:tok123:06e8d2e2e4e926e93b0878fcb4ed14f9c2b596b9603301ff8bd796b61b8d2485:2026-06-03T17:00:00.000+07:00"
	got := SignTransaction("test-client-secret", sts)
	want := "FQ0sx6aFl+6sY00h7dqgkfBSwjgs/b1EtLtaY+G+NsXxmfUurzTEoefV+ZHkOTUr7n6wFfWstutzkXX75WOBIA=="
	if got != want {
		t.Errorf("SignTransaction = %q, want %q", got, want)
	}
}

func TestTimestampFormat(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	ts := Timestamp(time.Date(2026, 6, 3, 17, 0, 0, 5e6, loc))
	if ts != "2026-06-03T17:00:00.005+07:00" {
		t.Errorf("Timestamp = %q", ts)
	}
}

func TestSignTokenRequestRoundTrip(t *testing.T) {
	privPEM, pubPEM, err := GenerateKeypair()
	if err != nil {
		t.Fatal(err)
	}
	ts := "2026-06-03T17:00:00.000+07:00"
	sigB64, err := SignTokenRequest(privPEM, "client-key-1", ts)
	if err != nil {
		t.Fatal(err)
	}
	// verify with the public half
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		t.Fatal("bad public PEM")
	}
	pub, err := parsePublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("client-key-1|" + ts))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		t.Errorf("signature does not verify: %v", err)
	}
}

func TestSNAPBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://api.durianpay.id/v1":         "https://api.durianpay.id/v1.0",
		"https://api-sandbox.durianpay.id/v1": "https://api-sandbox.durianpay.id/v1.0",
		"https://api.durianpay.id/v1.0":       "https://api.durianpay.id/v1.0",
		"https://api.durianpay.id":            "https://api.durianpay.id/v1.0",
	}
	for in, want := range cases {
		if got := SNAPBaseURL(in); got != want {
			t.Errorf("SNAPBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExternalIDUnique(t *testing.T) {
	a, b := ExternalID(), ExternalID()
	if a == b || a == "" {
		t.Errorf("ExternalID not unique: %q %q", a, b)
	}
}

func TestLoadConfigPartnerDefault(t *testing.T) {
	t.Setenv("DURIANPAY_SNAP_CLIENT_KEY", "ck")
	t.Setenv("DURIANPAY_SNAP_PARTNER_ID", "")
	cfg := LoadConfig("https://api.durianpay.id/v1")
	if cfg.PartnerID != "ck" {
		t.Errorf("PartnerID default = %q, want ck", cfg.PartnerID)
	}
}

func TestPrepareOfflineBodyHashIsHash(t *testing.T) {
	cfg := &Config{ClientKey: "ck", ClientSecret: "cs", PartnerID: "ck", BaseURL: "https://api.durianpay.id/v1.0"}
	c := NewClient(cfg)
	sr := c.PrepareOffline("POST", "/v1.0/balance-inquiry", []byte(`{"a":1}`), "ext-1", "tok")
	if len(sr.BodySHA256) != 64 {
		t.Errorf("BodySHA256 = %q, want 64-char hex (timestamp-colon regression)", sr.BodySHA256)
	}
	if sr.BodySHA256 != BodyHash([]byte(`{"a":1}`)) {
		t.Errorf("BodySHA256 mismatch: %q", sr.BodySHA256)
	}
}
