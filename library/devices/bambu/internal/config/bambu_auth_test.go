package config

import "testing"

func TestBambuCredentialsAreNotGenericHTTPAuth(t *testing.T) {
	cfg := Config{BambuSerial: "PRINTERSERIAL001", BambuAccessCode: "LOCALCODE123"}
	if got := cfg.WithoutHTTPAuth().AuthHeader(); got != "" {
		t.Fatalf("generic HTTP auth exposed Bambu credential: %q", got)
	}
}
