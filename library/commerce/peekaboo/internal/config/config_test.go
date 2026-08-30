package config

import "testing"

func TestAuthHeaderTokenOverridesLegacyAuthHeader(t *testing.T) {
	cfg := &Config{AuthHeaderVal: "Bearer legacy", PeekabooToken: "guest-token"}
	if got, want := cfg.AuthHeader(), "Bearer guest-token"; got != want {
		t.Fatalf("AuthHeader() = %q, want %q", got, want)
	}
}
