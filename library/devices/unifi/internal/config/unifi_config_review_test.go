package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, contents string) *Config {
	t.Helper()
	for _, name := range []string{"UNIFI_CONSOLE_ID", "UNIFI_GATEWAY", "UNIFI_GATEWAY_HOST", "UNIFI_LOCAL_API_KEY", "UNIFI_TRANSPORT", "UNIFI_API_KEY"} {
		t.Setenv(name, "")
	}
	t.Setenv("UNIFI_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// Saving a key must not forget which console or gateway it is for, or the next command
// silently goes somewhere else.
func TestSavingACredentialKeepsTheRoute(t *testing.T) {
	cfg := loadFixture(t, "console_id = \"C1\"\ngateway = \"192.0.2.1\"\nlocal_api_key = \"local\"\ntransport = \"local\"\n")
	if err := cfg.SaveCredential("new-key"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	want := UniFiTarget{ConsoleID: "C1", Gateway: "192.0.2.1", LocalAPIKey: "local", Transport: "local"}
	if got := reloaded.UniFiTarget(); got != want {
		t.Fatalf("after save: %+v, want %+v", got, want)
	}
	if reloaded.UnifiApiKey != "new-key" && reloaded.AuthHeader() == "" {
		t.Fatal("the new credential was not kept")
	}
}

// Earlier releases documented UNIFI_GATEWAY_HOST with the local key in UNIFI_API_KEY, so
// those installs must keep reaching their gateway rather than sending that key to the cloud.
func TestLegacyGatewayHostStillRoutesLocally(t *testing.T) {
	cfg := loadFixture(t, "")
	t.Setenv("UNIFI_GATEWAY_HOST", "192.0.2.9")
	t.Setenv("UNIFI_API_KEY", "legacy-key")
	if got := cfg.UniFiTarget(); got.Gateway != "192.0.2.9" || got.LocalAPIKey != "legacy-key" {
		t.Fatalf("legacy: %+v", got)
	}
	t.Setenv("UNIFI_GATEWAY", "192.0.2.10")
	t.Setenv("UNIFI_LOCAL_API_KEY", "new-local")
	if got := cfg.UniFiTarget(); got.Gateway != "192.0.2.10" || got.LocalAPIKey != "new-local" {
		t.Fatalf("current settings must win: %+v", got)
	}
	t.Setenv("UNIFI_GATEWAY", "")
	t.Setenv("UNIFI_GATEWAY_HOST", "")
	t.Setenv("UNIFI_LOCAL_API_KEY", "")
	if got := cfg.UniFiTarget(); got.Gateway != "" || got.LocalAPIKey != "" {
		t.Fatalf("the cloud key must not become a local key without the legacy host: %+v", got)
	}
}
