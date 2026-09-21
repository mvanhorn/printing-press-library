package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/config"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeTestCommand(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestAuthSetupAndStatusPreferCanonicalEnvName(t *testing.T) {
	testenv.Isolate(t)
	var flags rootFlags
	root := newRootCmd(&flags)
	out, err := executeTestCommand(root, "auth", "setup")
	if err != nil {
		t.Fatalf("auth setup: %v", err)
	}
	if !strings.Contains(out, "SENDFOX_API_TOKEN") {
		t.Fatalf("auth setup should mention SENDFOX_API_TOKEN, got %q", out)
	}
	if strings.Contains(out, "SENDFOX_BEARER_AUTH") {
		t.Fatalf("auth setup should not steer users to compatibility alias, got %q", out)
	}
}

func TestAuthLogoutWarnsWhenCanonicalEnvStillSet(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("SENDFOX_API_TOKEN", "test-token")
	t.Setenv("SENDFOX_BEARER_AUTH", "")
	var flags rootFlags
	root := newRootCmd(&flags)
	cfg := t.TempDir() + "/config.toml"
	out, err := executeTestCommand(root, "--config", cfg, "auth", "logout")
	if err != nil {
		t.Fatalf("auth logout: %v", err)
	}
	if !strings.Contains(out, "SENDFOX_API_TOKEN env var is still set") {
		t.Fatalf("logout should warn about SENDFOX_API_TOKEN, got %q", out)
	}
}

func TestAuthSetTokenUsesStdinAndDoesNotEcho(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("SENDFOX_API_TOKEN", "")
	t.Setenv("SENDFOX_BEARER_AUTH", "")
	var flags rootFlags
	root := newRootCmd(&flags)
	cfgPath := t.TempDir() + "/config.toml"
	root.SetIn(strings.NewReader("synthetic-unit-token\n"))
	out, err := executeTestCommand(root, "--config", cfgPath, "auth", "set-token", "--stdin")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "synthetic-unit-token") {
		t.Fatal("token echoed")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AccessToken != "synthetic-unit-token" {
		t.Fatal("token did not round-trip through isolated config")
	}
	root = newRootCmd(&rootFlags{})
	if _, err := executeTestCommand(root, "auth", "set-token", "synthetic-positional-token"); err == nil {
		t.Fatal("positional token accepted")
	}
}
