package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/config"
)

func TestApplicationIDMatchesEndpointAndHeader(t *testing.T) {
	for _, tc := range []struct{ name, saved, environment, want string }{
		{"saved credential", "application_id = \"SAVEDAPP\"\n", "", "SAVEDAPP"},
		{"saved template", "[template_vars]\nappId = \"TEMPLATEAPP\"\n", "", "TEMPLATEAPP"},
		{"environment wins", "application_id = \"SAVEDAPP\"\n", "ENVAPP", "ENVAPP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ALGOLIA_API_KEY", "")
			t.Setenv("ALGOLIA_APPLICATION_ID", tc.environment)
			t.Setenv("ALGOLIA_BASE_URL", "")
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			file := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(file, []byte("api_key = \"test-key\"\n"+tc.saved), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(file)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/"+tc.want+"/1/indexes" || r.Header.Get("x-algolia-application-id") != tc.want || r.Header.Get("x-algolia-api-key") != "test-key" {
					t.Errorf("wrong endpoint/auth pairing: path=%q application-id=%q", r.URL.Path, r.Header.Get("x-algolia-application-id"))
				}
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
			defer server.Close()
			cfg.BaseURL = server.URL + "/{appId}"
			cfg.Headers = map[string]string{"x-algolia-application-id": "WRONGAPP"}
			c := New(cfg, time.Second, 0)
			c.NoCache = true
			if _, err := c.GetWithHeaders(context.Background(), "/1/indexes", nil, map[string]string{"x-algolia-application-id": "WRONGENDPOINT"}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("requests = %d", calls)
			}
		})
	}
}

func TestMissingApplicationIDNamesSupportedEnvironmentVariable(t *testing.T) {
	_, err := buildURL("https://{appId}.algolia.net", "/1/indexes", nil)
	if err == nil || !strings.Contains(err.Error(), "export ALGOLIA_APPLICATION_ID=") {
		t.Fatalf("error = %v", err)
	}
}
