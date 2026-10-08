package config

import (
	"os"
	"testing"
)

// Guard the generated scope code: OAuth access-token refreshes must not
// select a new local index.
func TestDropboxOAuthStoreScopeCredential(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped while `generate --force` validates an unpatched tree; reapply patches, then run the full suite")
	}
	for _, tc := range []struct {
		name string
		cfg  Config
		want string
	}{
		{"client wins", Config{ClientID: "app-id", RefreshToken: "refresh-one", AccessToken: "access-one"}, "oauth_client=app-id"},
		{"refresh fallback", Config{RefreshToken: "refresh-one", AccessToken: "access-one"}, "oauth_refresh=refresh-one"},
		{"client without refresh", Config{ClientID: "app-id", AccessToken: "access-one"}, "oauth_client=app-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.StoreScopeCredential(); got != tc.want {
				t.Fatalf("StoreScopeCredential() = %q, want %q", got, tc.want)
			}
		})
	}
}
