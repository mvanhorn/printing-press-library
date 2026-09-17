// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestNormalizePhoneIdentity(t *testing.T) {
	for _, value := range []string{"1", "12", "123", "1234", "12345", "123456", "( )-+", "5551212"} {
		t.Run(value, func(t *testing.T) {
			if got, ok := normalizePhoneIdentity(value); ok {
				t.Fatalf("normalizePhoneIdentity(%q)=(%q, true), want unmappable", value, got)
			}
		})
	}
	bare, bareOK := normalizePhoneIdentity("2125551212")
	e164, e164OK := normalizePhoneIdentity("+1 (212) 555-1212")
	if !bareOK || !e164OK || bare != e164 || bare != "2125551212" {
		t.Fatalf("bare=(%q,%t) e164=(%q,%t)", bare, bareOK, e164, e164OK)
	}
}
