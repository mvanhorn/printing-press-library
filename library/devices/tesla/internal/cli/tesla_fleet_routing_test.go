package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// TestNewClientFleetRoutingDoesNotPersist verifies that newClient routes reads
// through the regional Fleet API (FleetMode + client BaseURL + fleet bearer)
// without writing anything to the on-disk config — the transient routing state
// must stay in memory.
func TestNewClientFleetRoutingDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	t.Setenv("TESLA_CONFIG", path)
	t.Setenv("TESLA_AUTH_TOKEN", "")
	t.Setenv("TESLA_FLEET_API_URL", "")
	t.Setenv("TESLA_PP_FORCE_FLEET_READS", "")

	seed, err := config.Load(path)
	if err != nil {
		t.Fatalf("seed load: %v", err)
	}
	if err := seed.SaveFleetTokens("cid", "secret", "ft", "r", time.Now().Add(time.Hour), "host.example", ""); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	if err := seed.SaveFleetAPIBase(testEUFleetBase); err != nil {
		t.Fatalf("seed api_base: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	f := &rootFlags{configPath: path, rateLimit: 2}
	c, err := f.newClient()
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if !c.FleetMode {
		t.Error("expected FleetMode=true")
	}
	if c.BaseURL != testEUFleetBase {
		t.Errorf("client BaseURL = %q, want %q", c.BaseURL, testEUFleetBase)
	}
	if got := c.Config.AuthHeader(); got != "Bearer ft" {
		t.Errorf("client auth = %q, want Bearer ft", got)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("newClient must not mutate the on-disk config")
	}
}
