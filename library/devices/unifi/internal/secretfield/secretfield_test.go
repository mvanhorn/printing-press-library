// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package secretfield

import "testing"

// Names taken from the Network, Protect and Mobility schemas in the research docs. RADIUS
// and VPN secrets are not in those schemas today (they expose only ids and names), so the
// classic-controller names are listed to keep the pattern honest if a console returns them.
func TestClassification(t *testing.T) {
	for _, name := range []string{"passphrase", "password", "presharedKeys", "preSharedKey", "pre_shared_key", "psk", "x_passphrase", "x_password", "x_shared_secret", "sharedSecret", "radiusSecret", "x_ipsec_pre_shared_key", "macAuthenticationConfiguration", "authorization"} {
		if !IsSecretField(name) {
			t.Errorf("%s is not secret", name)
		}
	}
	for _, name := range []string{"modelKey", "poe_passthrough", "nextToken", "inputToken", "authorized", "authorizedGuestLimit", "presharedKeyNetworkIds", "name", "code", "hasKeypad", "keypadSettings", "groupRekeyIntervalSeconds"} {
		if IsSecretField(name) {
			t.Errorf("%s is secret", name)
		}
	}
}

func TestHoldsDistinguishesRealValuesFromPlaceholders(t *testing.T) {
	if !HoldsSecret([]byte(`{"a":{"passphrase":"x"}}`)) {
		t.Fatal("missed a passphrase")
	}
	if HoldsSecret([]byte(`{"a":{"passphrase":"[secret:v1:ab]"}}`)) || HoldsSecret([]byte(`{"passphrase":"","secret":null}`)) {
		t.Fatal("a placeholder or empty value is not a secret")
	}
	if !HoldsPlaceholder([]byte(`{"x":["[scrubbed]"]}`)) || HoldsPlaceholder([]byte(`{"name":"a [scrubbed] sign"}`)) {
		t.Fatal("placeholder detection is wrong")
	}
}

// Numbers under broad names are counters; numbers under strong names are still secrets.
func TestNumbersOnlySealedUnderStrongNames(t *testing.T) {
	sealed := func(obj map[string]any) bool {
		_, changed := Walk(obj, func(any) any { return ScrubbedValue })
		return changed
	}
	if sealed(map[string]any{"total_tokens": 12.0, "authPort": 1812.0}) {
		t.Fatal("a counter was treated as a secret")
	}
	if !sealed(map[string]any{"password": 123456.0}) {
		t.Fatal("a numeric password was not sealed")
	}
}
