package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationIDResolvesEndpointFromEffectiveConfig(t *testing.T) {
	for _, tc := range []struct{ name, config, env, want string }{
		{"saved credentials", "application_id = \"SAVEDAPP\"\napi_key = \"test-key\"\n", "", "SAVEDAPP"},
		{"environment wins", "application_id = \"SAVEDAPP\"\napi_key = \"test-key\"\n", "ENVAPP", "ENVAPP"},
		{"saved template", "[template_vars]\nappId = \"TEMPLATEAPP\"\n", "", "TEMPLATEAPP"},
		{"legacy placeholder", "[template_vars]\nappId = \"ALGOLIA_APPLICATION_ID\"\n", "", ""},
		{"missing remains unresolved", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearCredEnv(t)
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			t.Setenv("ALGOLIA_APPLICATION_ID", tc.env)
			file := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(file, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(file)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.TemplateVars["appId"]; got != tc.want {
				t.Fatalf("appId = %q, want %q", got, tc.want)
			}
			if cfg.AlgoliaApplicationId != tc.want {
				t.Fatalf("header application ID = %q, want %q", cfg.AlgoliaApplicationId, tc.want)
			}
		})
	}
}
