package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/carol-bike/internal/config"
)

func TestAuthSetTokenReadsStdinAndRejectsArgv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "data"))
	t.Setenv("CAROL_BIKE_TOKEN", "")

	configPath := filepath.Join(t.TempDir(), "config.json")
	flags := &rootFlags{configPath: configPath, asJSON: true}

	t.Run("rejects positional token", func(t *testing.T) {
		cmd := newAuthSetTokenCmd(flags)
		cmd.SetIn(strings.NewReader(""))
		cmd.SetArgs([]string{"must-not-enter-argv"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected positional token to be rejected")
		}
	})

	t.Run("rejects empty stdin", func(t *testing.T) {
		cmd := newAuthSetTokenCmd(flags)
		cmd.SetIn(strings.NewReader("\n"))
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "token is required on stdin") {
			t.Fatalf("expected empty-stdin error, got %v", err)
		}
	})

	t.Run("saves stdin token without echo", func(t *testing.T) {
		const token = "synthetic-secret-token"
		cmd := newAuthSetTokenCmd(flags)
		var stdout, stderr bytes.Buffer
		cmd.SetIn(strings.NewReader(token + "\n"))
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("set-token failed: %v (stderr=%q)", err, stderr.String())
		}
		if strings.Contains(stdout.String(), token) || strings.Contains(stderr.String(), token) {
			t.Fatalf("token leaked to command output")
		}

		cfg, err := config.Load(configPath)
		if err != nil {
			t.Fatalf("reload config: %v", err)
		}
		if cfg.AccessToken != token {
			t.Fatalf("saved token mismatch: got length %d", len(cfg.AccessToken))
		}
	})
}
