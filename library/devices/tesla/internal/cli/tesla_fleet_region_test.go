package cli

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const (
	testNAFleetBase = "https://fleet-api.prd.na.vn.cloud.tesla.com"
	testEUFleetBase = "https://fleet-api.prd.eu.vn.cloud.tesla.com"
)

func TestFleetAPIBase(t *testing.T) {
	t.Run("env override wins and trims trailing slash", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", testEUFleetBase+"/")
		cfg := &config.Config{Fleet: config.FleetConfig{APIBase: "https://ignored.example.com"}}
		if got := fleetAPIBase(cfg); got != testEUFleetBase {
			t.Errorf("got %q want %q (env override, slash trimmed)", got, testEUFleetBase)
		}
	})
	t.Run("persisted api_base used when no env", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "")
		cfg := &config.Config{Fleet: config.FleetConfig{APIBase: testEUFleetBase}}
		if got := fleetAPIBase(cfg); got != testEUFleetBase {
			t.Errorf("got %q want %q", got, testEUFleetBase)
		}
	})
	t.Run("falls back to north america default", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "")
		if got := fleetAPIBase(&config.Config{}); got != testNAFleetBase {
			t.Errorf("got %q want %q", got, testNAFleetBase)
		}
	})
}

// TestFleetAPIBaseRejectsHostileHost guards the SSRF/token-exfiltration fix: the
// Fleet bearer is sent to this base, so non-https or non-tesla.com hosts (from a
// poisoned env var or a hostile creds bundle) must be ignored, not honored.
func TestFleetAPIBaseRejectsHostileHost(t *testing.T) {
	t.Run("hostile env host ignored, falls back to NA", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "https://evil.example.com")
		if got := fleetAPIBase(&config.Config{}); got != testNAFleetBase {
			t.Errorf("hostile env not rejected: got %q", got)
		}
	})
	t.Run("non-https tesla host ignored", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "http://fleet-api.prd.eu.vn.cloud.tesla.com")
		if got := fleetAPIBase(&config.Config{}); got != testNAFleetBase {
			t.Errorf("non-https not rejected: got %q", got)
		}
	})
	t.Run("lookalike host ignored", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "https://tesla.com.evil.example")
		if got := fleetAPIBase(&config.Config{}); got != testNAFleetBase {
			t.Errorf("lookalike host not rejected: got %q", got)
		}
	})
	t.Run("hostile persisted api_base ignored", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", "")
		cfg := &config.Config{Fleet: config.FleetConfig{APIBase: "https://evil.example.com"}}
		if got := fleetAPIBase(cfg); got != testNAFleetBase {
			t.Errorf("hostile persisted base not rejected: got %q", got)
		}
	})
	t.Run("valid eu host accepted", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_API_URL", testEUFleetBase)
		if got := fleetAPIBase(&config.Config{}); got != testEUFleetBase {
			t.Errorf("valid host rejected: got %q", got)
		}
	})
}

// TestTeslaShouldUseFleetForReadsEnvToken verifies TESLA_FLEET_TOKEN alone
// (no persisted [fleet] token) is enough to route reads through Fleet.
func TestTeslaShouldUseFleetForReadsEnvToken(t *testing.T) {
	t.Setenv("TESLA_PP_FORCE_FLEET_READS", "")
	t.Setenv("TESLA_AUTH_TOKEN", "")
	t.Run("TESLA_FLEET_TOKEN alone enables fleet reads", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "envft")
		if !teslaShouldUseFleetForReads(&config.Config{}) {
			t.Error("expected fleet routing with TESLA_FLEET_TOKEN set")
		}
	})
	t.Run("no token and no env stays on owner-api", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "")
		if teslaShouldUseFleetForReads(&config.Config{}) {
			t.Error("expected no fleet routing")
		}
	})
}

func TestTeslaShouldUseFleetForReads(t *testing.T) {
	t.Setenv("TESLA_FLEET_TOKEN", "")
	t.Setenv("TESLA_AUTH_TOKEN", "")
	future := time.Now().Add(time.Hour)
	fleetOnly := config.FleetConfig{AccessToken: "ft"}
	cases := []struct {
		name  string
		cfg   *config.Config
		force string
		want  bool
	}{
		{"nil cfg", nil, "", false},
		{"no fleet token never routes via fleet", &config.Config{}, "", false},
		{"fleet present, no owner credential", &config.Config{Fleet: fleetOnly}, "", true},
		{"fleet present, stale owner token (zero expiry) is unusable", &config.Config{AccessToken: "stale", Fleet: fleetOnly}, "", true},
		{"fleet present, expired owner token is unusable", &config.Config{AccessToken: "old", TokenExpiry: time.Now().Add(-time.Hour), Fleet: fleetOnly}, "", true},
		{"valid owner token keeps legacy path", &config.Config{AccessToken: "live", TokenExpiry: future, Fleet: fleetOnly}, "", false},
		{"explicit auth header keeps legacy path", &config.Config{AuthHeaderVal: "Bearer x", Fleet: fleetOnly}, "", false},
		{"env auth token keeps legacy path", &config.Config{TeslaAuthToken: "envtok", Fleet: fleetOnly}, "", false},
		{"force=1 overrides a valid owner token", &config.Config{AccessToken: "live", TokenExpiry: future, Fleet: fleetOnly}, "1", true},
		{"force=0 disables even with fleet only", &config.Config{Fleet: fleetOnly}, "0", false},
		{"force=0 with no fleet still false", &config.Config{}, "0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TESLA_PP_FORCE_FLEET_READS", tc.force)
			if got := teslaShouldUseFleetForReads(tc.cfg); got != tc.want {
				t.Errorf("teslaShouldUseFleetForReads = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSaveFleetAPIBase(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	t.Setenv("TESLA_CONFIG", cfgPath)
	t.Setenv("TESLA_FLEET_API_URL", "")
	t.Setenv("TESLA_AUTH_TOKEN", "")

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Fleet.AccessToken = "ft"

	if err := cfg.SaveFleetAPIBase(testEUFleetBase); err != nil {
		t.Fatalf("SaveFleetAPIBase: %v", err)
	}
	reloaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Fleet.APIBase != testEUFleetBase {
		t.Errorf("persisted api_base: got %q want %q", reloaded.Fleet.APIBase, testEUFleetBase)
	}
	if reloaded.Fleet.AccessToken != "ft" {
		t.Errorf("SaveFleetAPIBase clobbered access_token: got %q", reloaded.Fleet.AccessToken)
	}

	// Empty input is a no-op and must not clobber the stored value.
	if err := reloaded.SaveFleetAPIBase(""); err != nil {
		t.Fatalf("SaveFleetAPIBase(empty): %v", err)
	}
	final, _ := config.Load(cfgPath)
	if final.Fleet.APIBase != testEUFleetBase {
		t.Errorf("empty input clobbered api_base: got %q", final.Fleet.APIBase)
	}
}
