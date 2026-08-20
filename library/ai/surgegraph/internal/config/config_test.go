package config

import (
	"testing"
)

// TestAuthHeaderDoesNotMutateAuthSource pins PATCH(greptile-3): AuthHeader()
// must not overwrite the AuthSource label set by Load(). The previous body
// unconditionally relabeled AuthSource as "env:SURGEGRAPH_TOKEN" whenever
// SurgegraphToken was populated — silently mislabeling tokens that were
// actually loaded from disk and breaking `doctor`'s auth-source report.
func TestAuthHeaderDoesNotMutateAuthSource(t *testing.T) {
	c := &Config{
		SurgegraphToken: "tok-from-disk",
		AuthSource:      "config",
	}
	if got := c.AuthHeader(); got != "Bearer tok-from-disk" {
		t.Errorf("header: want %q, got %q", "Bearer tok-from-disk", got)
	}
	if c.AuthSource != "config" {
		t.Errorf("AuthSource mutated by AuthHeader(): want %q, got %q", "config", c.AuthSource)
	}
	// Repeat call must be equally non-mutating.
	_ = c.AuthHeader()
	if c.AuthSource != "config" {
		t.Errorf("AuthSource mutated on second AuthHeader() call: got %q", c.AuthSource)
	}
}

// TestAuthHeaderPrecedence covers the precedence order documented in
// AuthHeader: AuthHeaderVal > SurgegraphToken > AccessToken. PATCH(greptile-3)
// also relied on this ordering being stable, so it is pinned alongside the
// no-mutation guarantee.
func TestAuthHeaderPrecedence(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "empty", cfg: Config{}, want: ""},
		{name: "raw-auth-header-wins", cfg: Config{AuthHeaderVal: "X-Custom abc"}, want: "X-Custom abc"},
		{name: "static-over-access-token", cfg: Config{SurgegraphToken: "static", AccessToken: "oauth"}, want: "Bearer static"},
		{name: "access-token-when-no-static", cfg: Config{AccessToken: "oauth"}, want: "Bearer oauth"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.AuthHeader()
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}
