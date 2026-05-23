// Tests for agent-context schema v4 (U5 of the 2026-05-22-001 plan).
//
// Schema v4 is purely additive on top of v3: the new fields (Fleet env
// vars, SignedCommandPaths block) sit alongside existing v3 paths
// (commands, auth.env_vars[TESLA_AUTH_TOKEN], available_profiles, etc.).
//
// CRITICAL: this file's most load-bearing test asserts that NO Fleet
// token literal leaks into the agent-context JSON output. The plan calls
// out tokens as the highest-impact leak risk; the surface MUST surface
// only presence, audience (from static constants or non-secret JWT
// claims), scopes, and expiry timestamps.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// fakeRootCmd returns a minimal cobra root that buildAgentContext can walk
// without bringing the full tesla-pp-cli command tree into the test (which
// would require valid client config). The schema fields we care about are
// SchemaVersion, Auth.EnvVars, and SignedCommandPaths.
func fakeRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "tesla-pp-cli",
		Short:   "fake root for agent-context tests",
		Version: "test",
	}
	// A single dummy child so collectAgentCommands returns a non-empty list.
	root.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "fake doctor",
	})
	return root
}

// configWithFleet drops a config.toml at a t.TempDir with the supplied
// Fleet block and sets TESLA_CONFIG + HOME so config.Load("") finds it.
// Also clears TESLA_FLEET_TOKEN so the test controls token source
// explicitly via the FleetConfig.AccessToken arg.
func configWithFleet(t *testing.T, ft config.FleetConfig) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		BaseURL: "https://owner-api.teslamotors.com",
		Fleet:   ft,
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("TESLA_CONFIG", cfgPath)
	t.Setenv("TESLA_FLEET_TOKEN", "")
	t.Setenv("TESLA_FLEET_CLIENT_ID", "")
	t.Setenv("TESLA_FLEET_CLIENT_SECRET", "")
	t.Setenv("TESLA_AUTH_TOKEN", "")
	t.Setenv(commandHermesPortEnv, "")
	return cfgPath
}

func TestAgentContext_SchemaVersionV4(t *testing.T) {
	configWithFleet(t, config.FleetConfig{})
	ctx := buildAgentContext(fakeRootCmd())
	if ctx.SchemaVersion != "4" {
		t.Errorf("schema_version: got %q want %q", ctx.SchemaVersion, "4")
	}
}

func TestAgentContext_EnvVarsIncludeFleetTriple(t *testing.T) {
	configWithFleet(t, config.FleetConfig{})
	ctx := buildAgentContext(fakeRootCmd())
	saw := map[string]agentContextAuthEnvVar{}
	for _, e := range ctx.Auth.EnvVars {
		saw[e.Name] = e
	}
	// v3 surface preserved.
	if _, ok := saw["TESLA_AUTH_TOKEN"]; !ok {
		t.Errorf("v3 TESLA_AUTH_TOKEN entry missing; v4 must be additive")
	}
	// v4 new entries.
	for _, want := range []string{"TESLA_FLEET_TOKEN", "TESLA_FLEET_CLIENT_ID", "TESLA_FLEET_CLIENT_SECRET"} {
		entry, ok := saw[want]
		if !ok {
			t.Errorf("v4 env var %q missing from auth.env_vars", want)
			continue
		}
		if !entry.Sensitive {
			t.Errorf("%s must be marked sensitive=true (carries credentials)", want)
		}
		if entry.Required {
			t.Errorf("%s must be required=false (Fleet is optional)", want)
		}
	}
}

func TestAgentContext_SignedCommandPathsBlockPresent(t *testing.T) {
	configWithFleet(t, config.FleetConfig{})
	ctx := buildAgentContext(fakeRootCmd())
	if ctx.SignedCommandPaths == nil {
		t.Fatalf("signed_command_paths block missing")
	}
	// No Fleet token + no relay -> fleet.available=false, hermes.running=false.
	if ctx.SignedCommandPaths.Fleet.Available {
		t.Errorf("Fleet.Available should be false when no token present")
	}
	if ctx.SignedCommandPaths.Hermes.Running {
		t.Errorf("Hermes.Running should be false when no relay state files present")
	}
}

func TestAgentContext_SignedCommandPaths_FleetPresentSurfaceMetadata(t *testing.T) {
	// Build a minimal JWT (header.payload.signature) whose payload carries
	// aud + scope so decodeJWTClaims can extract them. The token bytes
	// themselves are obviously not a real secret; we still want to assert
	// they don't leak into the JSON output below.
	const fakeJWT = "ZmFrZQ.eyJhdWQiOiJodHRwczovL2ZsZWV0LWFwaS5wcmQubmEudm4uY2xvdWQudGVzbGEuY29tIiwic2NvcGUiOiJ2ZWhpY2xlX2NtZHMgdmVoaWNsZV9kZXZpY2VfZGF0YSJ9.c2ln"
	expiry := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	configWithFleet(t, config.FleetConfig{
		AccessToken: fakeJWT,
		TokenExpiry: expiry,
	})
	ctx := buildAgentContext(fakeRootCmd())
	if ctx.SignedCommandPaths == nil {
		t.Fatalf("signed_command_paths block missing")
	}
	if !ctx.SignedCommandPaths.Fleet.Available {
		t.Errorf("Fleet.Available should be true when access token present")
	}
	if ctx.SignedCommandPaths.Fleet.Audience == "" {
		t.Errorf("Fleet.Audience should be populated from JWT claim or static fallback")
	}
	if len(ctx.SignedCommandPaths.Fleet.Scopes) == 0 {
		t.Errorf("Fleet.Scopes should be populated from JWT claim or static fallback")
	}
	if ctx.SignedCommandPaths.Fleet.Expiry == "" {
		t.Errorf("Fleet.Expiry should reflect the persisted expiry timestamp")
	}
}

// TestAgentContext_NoFleetTokenLiteralInJSON is the load-bearing security
// guard called out in U5's CRITICAL bullet. Encode the full agent-context
// payload with a long Fleet token literal in config and assert the token
// bytes do not appear anywhere in the JSON output. Only presence + static
// audience/scope + expiry timestamp should leak.
func TestAgentContext_NoFleetTokenLiteralInJSON(t *testing.T) {
	// A unique, easy-to-grep token literal. The middle segment is a base64
	// JSON payload so decodeJWTClaims succeeds (otherwise the audience/
	// scope fields stay empty); the header/signature segments are
	// uniquely-identifiable sentinels we then assert are absent in JSON.
	const tokenSentinel = "TOK_SENTINEL_HEADER_BYTES_001"
	const refreshSentinel = "REFRESH_SENTINEL_002"
	const secretSentinel = "CLIENT_SECRET_SENTINEL_003"
	const clientIDSentinel = "CLIENT_ID_SENTINEL_004"
	jwt := tokenSentinel + ".eyJhdWQiOiJodHRwczovL2ZsZWV0LWFwaS5wcmQubmEudm4uY2xvdWQudGVzbGEuY29tIn0.sig"

	configWithFleet(t, config.FleetConfig{
		AccessToken:  jwt,
		RefreshToken: refreshSentinel,
		ClientSecret: secretSentinel,
		ClientID:     clientIDSentinel,
		TokenExpiry:  time.Now().Add(time.Hour),
	})
	ctx := buildAgentContext(fakeRootCmd())
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal agent-context: %v", err)
	}
	s := string(raw)

	leaks := []string{tokenSentinel, refreshSentinel, secretSentinel, clientIDSentinel, jwt}
	for _, sentinel := range leaks {
		if strings.Contains(s, sentinel) {
			t.Errorf("agent-context JSON leaks credential bytes (%q): %s", sentinel, s)
		}
	}
}

// TestAgentContext_V3ShapePreserved is the backcompat guard: every v3
// field path is reachable at the same JSON key in v4 output. Downstream
// MCP consumers parsing on `cli.name`, `auth.env_vars`, `commands`,
// `available_profiles`, `feedback_endpoint_configured`, etc. must still
// find them.
func TestAgentContext_V3ShapePreserved(t *testing.T) {
	configWithFleet(t, config.FleetConfig{})
	ctx := buildAgentContext(fakeRootCmd())
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{
		"schema_version",
		"cli",
		"auth",
		"commands",
		"available_profiles",
		"feedback_endpoint_configured",
	} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("v3 key %q missing from v4 output (must be additive)", key)
		}
	}
	auth, _ := decoded["auth"].(map[string]any)
	if auth == nil {
		t.Fatalf("auth block missing")
	}
	envVars, _ := auth["env_vars"].([]any)
	if len(envVars) == 0 {
		t.Fatalf("auth.env_vars empty; v3 entry must remain")
	}
	// First entry should still describe TESLA_AUTH_TOKEN (insertion order
	// matters because tests check the slice).
	first, _ := envVars[0].(map[string]any)
	if first == nil || first["name"] != "TESLA_AUTH_TOKEN" {
		t.Errorf("auth.env_vars[0] must still be TESLA_AUTH_TOKEN; got %v", envVars[0])
	}
}
