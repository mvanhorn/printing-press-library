package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
)

type govOriginTransport func(*http.Request) (*http.Response, error)

func (f govOriginTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGovFederalKeyRespectsProfileOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, base, env, want string
		demo                  bool
	}{
		{name: "private stored key", base: "https://private.example/api"},
		{name: "plaintext federal route", base: "http://api.gsa.gov/technology/datagov/v4"},
		{name: "different federal port", base: "https://api.gsa.gov:8443/technology/datagov/v4"},
		{name: "federal lookalike", base: "https://api.gsa.gov.private.example"},
		{name: "private demo", base: "https://private.example/api", demo: true, want: "DEMO_KEY"},
		{name: "federal stored key", base: "https://api.gsa.gov/technology/datagov/v4", want: "PROFILE-TEST-KEY"},
		{name: "federal demo retains key", base: "https://api.gsa.gov:443/other-path", demo: true, want: "PROFILE-TEST-KEY"},
		{name: "private environment key", base: "https://private.example/api", env: "ENV-TEST-KEY"},
		{name: "private environment demo", base: "https://private.example/api", env: "ENV-TEST-KEY", demo: true, want: "DEMO_KEY"},
		{name: "federal environment key", base: "https://api.gsa.gov", env: "ENV-TEST-KEY", want: "ENV-TEST-KEY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GOVERNMENT_DATA_HOME", t.TempDir())
			t.Setenv("GOVERNMENT_DATA_BASE_URL", "")
			t.Setenv("DATA_GOV_API_KEY", "")
			path := filepath.Join(t.TempDir(), "profile.toml")
			if err := os.WriteFile(path, []byte("base_url = \""+tc.base+"\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.SaveCredential("PROFILE-TEST-KEY"); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DATA_GOV_API_KEY", tc.env)
			cfg, err = config.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			key := govFederalKey(cfg, tc.demo)
			if key != tc.want {
				t.Fatal("credential selection did not honor the federal origin")
			}
			requests := 0
			c := govdata.New(key)
			c.ConfigureHTTP(&http.Client{Transport: govOriginTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.Host == "api.gsa.gov" {
					if tc.want == "" || r.Header.Get("X-Api-Key") != tc.want {
						t.Error("incorrect federal credential header")
					}
				} else if r.Header.Get("X-Api-Key") != "" {
					t.Error("California received a credential")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"success":true}`)), Header: make(http.Header)}, nil
			})}, 0)
			_, err = c.Fetch(context.Background(), "federal", "/search", nil)
			if tc.want == "" {
				if err == nil || requests != 0 {
					t.Fatal("foreign credential caused a federal request")
				}
			} else if err != nil || requests != 1 {
				t.Fatal("expected federal request was not made")
			}
			if _, err := c.Fetch(context.Background(), "california", "/package_search", nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
