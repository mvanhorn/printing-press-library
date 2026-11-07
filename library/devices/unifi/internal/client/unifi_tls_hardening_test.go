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
