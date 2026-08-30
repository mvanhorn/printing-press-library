// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for the service-account auth helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestJWTClaimsJSON(t *testing.T) {
	t.Parallel()
	raw, err := jwtClaimsJSON("svc@project.iam.gserviceaccount.com", "scopeA scopeB",
		"https://oauth2.googleapis.com/token", "admin@example.com", 1000, 4600)
	if err != nil {
		t.Fatalf("jwtClaimsJSON error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("claims not valid JSON: %v", err)
	}
	if got["iss"] != "svc@project.iam.gserviceaccount.com" {
		t.Errorf("iss = %v", got["iss"])
	}
	if got["sub"] != "admin@example.com" {
		t.Errorf("sub (impersonation subject) = %v", got["sub"])
	}
	if got["scope"] != "scopeA scopeB" {
		t.Errorf("scope = %v", got["scope"])
	}
	if got["aud"] != "https://oauth2.googleapis.com/token" {
		t.Errorf("aud = %v", got["aud"])
	}
	// iat/exp are numbers in JSON.
	if int64(got["iat"].(float64)) != 1000 || int64(got["exp"].(float64)) != 4600 {
		t.Errorf("iat/exp = %v/%v", got["iat"], got["exp"])
	}
}

func TestSplitCSV(t *testing.T) {
	t.Parallel()
	got := splitCSV(" a , b ,, c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
