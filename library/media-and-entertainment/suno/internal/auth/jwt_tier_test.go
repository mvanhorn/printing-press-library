// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"encoding/base64"
	"testing"
)

func jwtWith(payloadJSON string) string {
	return "hdr." + base64.RawURLEncoding.EncodeToString([]byte(payloadJSON)) + ".sig"
}

func TestPlanTier(t *testing.T) {
	cases := []struct {
		name string
		jwt  string
		want string
	}{
		{"tier:period:", jwtWith(`{"plan":"e1235dd7-9f4d-4738-aeb2-1470466cba27:year:","sub":"u"}`), "e1235dd7-9f4d-4738-aeb2-1470466cba27"},
		{"bearer prefix", "Bearer " + jwtWith(`{"plan":"abc-123:month:"}`), "abc-123"},
		{"plain tier no colon", jwtWith(`{"plan":"plain-tier"}`), "plain-tier"},
		{"no plan claim", jwtWith(`{"sub":"u"}`), ""},
		{"not a jwt", "garbage", ""},
		{"bad base64 payload", "hdr.@@@.sig", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := PlanTier(tc.jwt); got != tc.want {
			t.Errorf("%s: PlanTier = %q, want %q", tc.name, got, tc.want)
		}
	}
}
