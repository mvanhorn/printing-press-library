// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.
// PATCH(retro #marianatek-bearer-prefix): regression test for AuthHeader()
// auto-prefixing "Bearer " when the source is `oauth_authorization`.

package config

import "testing"

func TestAuthHeaderBearerPrefix(t *testing.T) {
	cases := []struct {
		name        string
		authHeader  string
		oauthRaw    string
		wantHeader  string
	}{
		{"auth_header verbatim", "Bearer abc", "ignored", "Bearer abc"},
		{"auth_header verbatim non-bearer", "Token xyz", "ignored", "Token xyz"},
		{"oauth raw token auto-prefixes Bearer", "", "testtok1", "Bearer testtok1"},
		{"oauth already prefixed Bearer passes through", "", "Bearer testtok1", "Bearer testtok1"},
		{"oauth already prefixed lowercase bearer passes through", "", "bearer testtok1", "bearer testtok1"},
		{"oauth already prefixed Token passes through", "", "Token testtok1", "Token testtok1"},
		{"empty both returns empty", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := &Config{
				AuthHeaderVal:              c.authHeader,
				CustomerOauthAuthorization: c.oauthRaw,
			}
			got := cfg.AuthHeader()
			if got != c.wantHeader {
				t.Fatalf("AuthHeader() = %q, want %q", got, c.wantHeader)
			}
		})
	}
}
