package client

import "testing"

func TestAmbiguousRetryRequiresWriteProtection(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if safeToRetryAmbiguous(method, false, "") {
			t.Fatalf("%s without idempotency key can replay", method)
		}
		if !safeToRetryAmbiguous(method, false, "request-key") {
			t.Fatalf("%s with idempotency key cannot retry", method)
		}
	}
	if !safeToRetryAmbiguous("GET", false, "") {
		t.Fatal("read-only GET cannot retry")
	}
	if safeToRetryAmbiguous("GET", true, "") {
		t.Fatal("mutating GET without idempotency key can replay")
	}
}
