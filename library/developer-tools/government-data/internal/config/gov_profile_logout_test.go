package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
)

func TestProfileLogoutDoesNotReuseGlobalCredential(t *testing.T) {
	for _, ownCredential := range []bool{false, true} {
		name := "inherited"
		if ownCredential {
			name = "profile"
		}
		t.Run(name, func(t *testing.T) {
			isolateCredentials(t)
			const server = "https://api.gsa.gov/technology/datagov/v4"
			if err := cliutil.SaveCredentials(&cliutil.Credentials{BaseURL: server, DataGovApiKey: "GLOBAL-TEST-KEY"}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "profile.toml")
			if err := os.WriteFile(path, []byte("base_url = \""+server+"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AuthHeader() != "GLOBAL-TEST-KEY" {
				t.Fatal("fresh same-server profile should retain global fallback")
			}
			if ownCredential {
				if err := cfg.SaveCredential("PROFILE-TEST-KEY"); err != nil {
					t.Fatal(err)
				}
			}
			if err := cfg.ClearTokens(); err != nil {
				t.Fatal(err)
			}
			reloaded, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.AuthHeader() != "" {
				t.Fatal("logged-out profile silently authenticated")
			}
			if !reloaded.DisableGlobalCredentials {
				t.Fatal("logout decision not persisted")
			}
			global, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			if global.AuthHeader() != "GLOBAL-TEST-KEY" {
				t.Fatal("profile logout changed global credential")
			}
			if err := reloaded.SaveCredential("NEW-PROFILE-TEST-KEY"); err != nil {
				t.Fatal(err)
			}
			signedIn, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if signedIn.AuthHeader() != "NEW-PROFILE-TEST-KEY" || signedIn.DisableGlobalCredentials {
				t.Fatal("explicit sign-in did not restore profile authentication")
			}
		})
	}
}
