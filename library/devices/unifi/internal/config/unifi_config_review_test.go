package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, contents string) *Config {
	t.Helper()
	return loadWithEnv(t, contents, nil)
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

// loadWithEnv loads contents with the given UniFi env vars set (the rest cleared) before
// Load runs, since Load is where UNIFI_API_KEY becomes the cloud credential.
func loadWithEnv(t *testing.T, contents string, env map[string]string) *Config {
	t.Helper()
	for _, name := range []string{"UNIFI_CONSOLE_ID", "UNIFI_GATEWAY", "UNIFI_GATEWAY_HOST", "UNIFI_LOCAL_API_KEY", "UNIFI_TRANSPORT", "UNIFI_API_KEY"} {
		t.Setenv(name, env[name])
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

// Under the legacy names UNIFI_API_KEY is the gateway's key. It becomes the local key,
// so it must not also be the cloud key, or Site Manager requests carry it to api.ui.com.
func TestLegacyGatewayKeyIsNotSentToTheCloud(t *testing.T) {
	cfg := loadWithEnv(t, "", map[string]string{"UNIFI_GATEWAY_HOST": "192.0.2.9", "UNIFI_API_KEY": "legacy-key"})
	if got := cfg.UniFiTarget(); got.Gateway != "192.0.2.9" || got.LocalAPIKey != "legacy-key" {
		t.Fatalf("legacy install must still route locally: %+v", got)
	}
	if cfg.UnifiApiKey != "" || cfg.AuthHeader() != "" {
		t.Fatalf("the gateway key became the cloud key: UnifiApiKey=%q AuthHeader=%q", cfg.UnifiApiKey, cfg.AuthHeader())
	}
	if cfg.AuthSource == "env:UNIFI_API_KEY" {
		t.Fatalf("auth source still claims the env cloud key: %q", cfg.AuthSource)
	}
	// The key used to scope the local store as the cloud credential; it has to keep
	// doing so, or an upgrade strands the existing data-<hash>.db.
	if got := cfg.StoreScopeCredential(); got != "legacy-key" {
		t.Fatalf("store scope = %q, want the legacy key", got)
	}
}

// A saved cloud key next to the legacy pair has its own store. The legacy key's store
// must not adopt it as an older name, or the next cloud-only run loses that history.
func TestLegacyStoreScopeDoesNotAdoptTheSavedCloudStore(t *testing.T) {
	cfg := loadWithEnv(t, "", nil)
	if err := cfg.SaveCredential("saved-cloud"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UNIFI_GATEWAY_HOST", "192.0.2.9")
	t.Setenv("UNIFI_API_KEY", "legacy-key")
	reloaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.UnifiApiKey != "saved-cloud" {
		t.Fatalf("setup: saved cloud key = %q", reloaded.UnifiApiKey)
	}
	if got := reloaded.StoreScopeCredential(); got != "legacy-key" {
		t.Fatalf("store scope = %q, want the legacy key", got)
	}
	if got := reloaded.StoreScopeLegacyCredential(); got != "legacy-key" {
		t.Fatalf("migration source = %q, want the legacy key (nothing to adopt)", got)
	}
}

// Only the legacy pair overrides the store scope; every other setup keeps its own.
func TestStoreScopeUnchangedOutsideTheLegacyPair(t *testing.T) {
	cloud := loadWithEnv(t, "", map[string]string{"UNIFI_API_KEY": "cloud-key"})
	if got := cloud.StoreScopeCredential(); got != "cloud-key" {
		t.Fatalf("cloud store scope = %q, want cloud-key", got)
	}
	if got := cloud.StoreScopeLegacyCredential(); got != "cloud-key" {
		t.Fatalf("cloud migration source = %q, want cloud-key", got)
	}
	local := loadWithEnv(t, "", map[string]string{"UNIFI_GATEWAY": "192.0.2.10", "UNIFI_LOCAL_API_KEY": "local"})
	if got := local.StoreScopeCredential(); got != "" {
		t.Fatalf("local-only store scope = %q, want unscoped", got)
	}
}

// UNIFI_API_KEY stays the cloud key whenever the legacy host did not move it.
func TestCloudKeyKeptWhenTheLegacyHostDoesNotMoveIt(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		env      map[string]string
	}{
		{"no gateway", "", map[string]string{"UNIFI_API_KEY": "cloud-key"}},
		{"current gateway names", "", map[string]string{"UNIFI_GATEWAY": "192.0.2.10", "UNIFI_LOCAL_API_KEY": "local", "UNIFI_API_KEY": "cloud-key"}},
		{"legacy host with a separate local key", "", map[string]string{"UNIFI_GATEWAY_HOST": "192.0.2.9", "UNIFI_LOCAL_API_KEY": "local", "UNIFI_API_KEY": "cloud-key"}},
		{"gateway from the config file", "gateway = \"192.0.2.1\"\n", map[string]string{"UNIFI_GATEWAY_HOST": "192.0.2.9", "UNIFI_API_KEY": "cloud-key"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := loadWithEnv(t, tc.contents, tc.env)
			if cfg.UnifiApiKey != "cloud-key" {
				t.Fatalf("cloud key = %q, want cloud-key", cfg.UnifiApiKey)
			}
			if got := cfg.UniFiTarget().LocalAPIKey; got == "cloud-key" {
				t.Fatalf("the cloud key became the local key: %+v", cfg.UniFiTarget())
			}
		})
	}
}

// A file others can edit is not trusted to choose where a key goes, and a save must not
// launder it: rewriting at 0600 would make the next load trust what this one rejected.
func TestSavingDoesNotTrustALooseConfigsGateway(t *testing.T) {
	cfg := loadFixture(t, "console_id = \"C1\"\ngateway = \"192.0.2.66\"\nlocal_api_key = \"planted\"\n")
	if err := os.Chmod(cfg.Path, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveCredential("new-key"); err != nil {
		t.Skipf("a loose config refuses to save here: %v", err)
	}
	reloaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.UniFiTarget()
	if got.Gateway != "" || got.LocalAPIKey != "" {
		t.Fatalf("a loose file's route became trusted: %+v", got)
	}
	if got.ConsoleID != "C1" {
		t.Fatalf("the console id is safe to keep: %+v", got)
	}
}

// Logout says credentials are cleared, so the gateway key has to go with them. The route
// itself is not a secret and stays.
func TestLogoutRemovesTheStoredGatewayKey(t *testing.T) {
	cfg := loadFixture(t, "console_id = \"C1\"\ngateway = \"192.0.2.1\"\nlocal_api_key = \"local\"\n")
	if err := cfg.ClearTokens(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.UniFiTarget()
	if got.LocalAPIKey != "" {
		t.Fatal("logout left the gateway key on disk")
	}
	if got.ConsoleID != "C1" || got.Gateway != "192.0.2.1" {
		t.Fatalf("logout should keep the route: %+v", got)
	}
}
