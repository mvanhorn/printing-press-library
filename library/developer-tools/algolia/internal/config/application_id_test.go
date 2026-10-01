package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/algolia/internal/cliutil"
)

func TestApplicationIDResolvesEndpointFromEffectiveConfig(t *testing.T) {
	for _, tc := range []struct{ name, config, env, want string }{
		{"saved credentials", "application_id = \"SAVEDAPP\"\napi_key = \"test-key\"\n", "", "SAVEDAPP"},
		{"environment wins", "application_id = \"SAVEDAPP\"\napi_key = \"test-key\"\n", "ENVAPP", "ENVAPP"},
		{"saved template", "[template_vars]\nappId = \"TEMPLATEAPP\"\n", "", "TEMPLATEAPP"},
		{"saved template beats legacy credential placeholder", "application_id = \"ALGOLIA_APPLICATION_ID\"\n[template_vars]\nappId = \"TEMPLATEAPP\"\n", "", "TEMPLATEAPP"},
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

func TestApplicationIDLoadsFromSiblingCredentialsWhenConfigHasAPIKey(t *testing.T) {
	for _, tc := range []struct{ name, config string }{
		{"missing ID", "api_key = \"test-key\"\n"},
		{"legacy placeholder", "api_key = \"test-key\"\n[template_vars]\nappId = \"ALGOLIA_APPLICATION_ID\"\n"},
		{"legacy credential placeholder", "api_key = \"test-key\"\napplication_id = \"ALGOLIA_APPLICATION_ID\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearCredEnv(t)
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			file := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(file, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			lockOwnerOnly(t, file)
			credentialsFile, err := cliutil.CredentialsFilePathForConfig(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(credentialsFile), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(credentialsFile, []byte("application_id = \"SAVEDAPP\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			lockOwnerOnly(t, credentialsFile)
			cfg, err := Load(file)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AlgoliaApplicationId != "SAVEDAPP" || cfg.TemplateVars["appId"] != "SAVEDAPP" || cfg.AlgoliaApiKey != "test-key" {
				t.Fatalf("separate application ID was not paired with the saved API key: app=%q endpoint=%q", cfg.AlgoliaApplicationId, cfg.TemplateVars["appId"])
			}
		})
	}
}

func TestApplicationIDLoadsFromGlobalCredentialsWithDefaultConfig(t *testing.T) {
	clearCredEnv(t)
	t.Setenv("ALGOLIA_CONFIG", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	configDir, err := cliutil.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configFile, []byte("api_key = \"test-key\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lockOwnerOnly(t, configFile)
	credentialsFile, err := cliutil.CredentialsFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(credentialsFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credentialsFile, []byte("application_id = \"GLOBALAPP\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lockOwnerOnly(t, credentialsFile)
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AlgoliaApplicationId != "GLOBALAPP" || cfg.TemplateVars["appId"] != "GLOBALAPP" || cfg.AlgoliaApiKey != "test-key" {
		t.Fatalf("global application ID was not paired with the default config API key: app=%q endpoint=%q", cfg.AlgoliaApplicationId, cfg.TemplateVars["appId"])
	}
}
