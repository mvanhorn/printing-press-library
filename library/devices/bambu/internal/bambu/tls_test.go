package bambu

import (
	"strings"
	"testing"
)

func TestIdentityMismatchDoesNotExposeSerials(t *testing.T) {
	actual, wanted := "PEERSERIAL12345", "WANTEDSERIAL123"
	err := verifyPrinterIdentity(actual, wanted)
	if err == nil {
		t.Fatal("expected identity mismatch")
	}
	if strings.Contains(err.Error(), actual) || strings.Contains(err.Error(), wanted) {
		t.Fatalf("identity error exposed a serial: %q", err)
	}
}
