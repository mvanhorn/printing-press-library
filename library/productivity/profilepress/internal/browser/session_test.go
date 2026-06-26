package browser

import "testing"

func TestSessionConfigValidateRequiresCDPURL(t *testing.T) {
	if err := (SessionConfig{}).Validate(); err == nil {
		t.Fatal("empty CDP URL should fail")
	}
}

func TestSessionConfigValidateAllowsCDPURL(t *testing.T) {
	if err := (SessionConfig{CDPURL: "http://127.0.0.1:9222"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
