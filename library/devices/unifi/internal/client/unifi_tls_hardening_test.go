package client

import (
	"fmt"
	"strings"
	"testing"
)

func captureTLSWarnings(t *testing.T) *strings.Builder {
	t.Helper()
	var b strings.Builder
	prev := writeTLSWarning
	writeTLSWarning = func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	tlsWarnMu.Lock()
	lastTLSWarn = ""
	tlsWarnMu.Unlock()
	t.Cleanup(func() { writeTLSWarning = prev })
	return &b
}

// The default skip for a private gateway is what lets an on-LAN attacker
// impersonate it, so the operator has to be told it is happening.
func TestPrivateGatewaySkipWarns(t *testing.T) {
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "")
	warned := captureTLSWarnings(t)
	if !gatewaySkipsTLSVerify("192.168.1.1") {
		t.Fatal("private gateway should still skip verification by default")
	}
	if !strings.Contains(warned.String(), "UNIFI_CA_CERT") {
		t.Fatalf("no warning naming the fix: %q", warned.String())
	}
}

// Loopback traffic never crosses the network, so warning there is only noise.
func TestLoopbackGatewaySkipIsSilent(t *testing.T) {
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "")
	warned := captureTLSWarnings(t)
	if !gatewaySkipsTLSVerify("127.0.0.1") {
		t.Fatal("loopback should skip verification")
	}
	if warned.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warned.String())
	}
}

// Before this, a configured CA was overwritten by the private-address default,
// so following the documented fix silently changed nothing.
func TestConfiguredCAKeepsVerificationOnPrivateGateway(t *testing.T) {
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "")
	captureTLSWarnings(t)
	if gatewayTrustSkipsVerify("192.168.1.1", "/tmp/ca.pem") {
		t.Fatal("a configured CA must keep verification on")
	}
	t.Setenv("UNIFI_INSECURE_SKIP_VERIFY", "1")
	if !gatewayTrustSkipsVerify("192.168.1.1", "/tmp/ca.pem") {
		t.Fatal("the explicit operator switch still wins")
	}
}

// Earlier releases told gateway users to save base_url with /proxy/network. Requests now
// carry their product in the path, so that prefix doubled it and missed the gateway.
func TestSavedGatewayBaseURLDoesNotDoubleTheProduct(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"https://192.0.2.1/proxy/network", "/network/integration/v1/sites", "https://192.0.2.1/proxy/network/integration/v1/sites"},
		{"https://192.0.2.1/proxy/network/", "/protect/integration/v1/cameras", "https://192.0.2.1/proxy/protect/integration/v1/cameras"},
		{"http://127.0.0.1:9000", "/network/integration/v1/sites", "http://127.0.0.1:9000/network/integration/v1/sites"},
		{"https://192.0.2.1/proxy/network", "/v1/hosts", "https://192.0.2.1/proxy/network/v1/hosts"},
	}
	for _, tc := range cases {
		if got := passthroughURL(tc.base, tc.path); got != tc.want {
			t.Errorf("passthroughURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}
