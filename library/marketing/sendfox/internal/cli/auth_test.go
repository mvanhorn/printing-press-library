package cli

import (
	"bytes"
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
