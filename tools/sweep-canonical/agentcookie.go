package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// agentcookieToml is the per-CLI adoption manifest emitted alongside
// .printing-press.json. Mirror of agentcookie v2 adoption-standard
// schema. The sweep tool inlines the TOML render rather than importing
// pkg/agentcookieadoption because the sweep-canonical tool runs in
// GOPATH mode (no go.mod) — see AGENTS.md "Bulk SKILL.md/README.md
// retrofits" for the why.
const agentcookieTomlFilename = "agentcookie.toml"

// agentcookieOverrideMarker, when present in the first kilobyte of an
// existing agentcookie.toml, signals that an author hand-edited the
// file and the sweep tool must not overwrite it.
const agentcookieOverrideMarker = "# agentcookie-manual-override"

// extendedManifest carries the subset of .printing-press.json that the
// agentcookie sweep step needs. The narrower top-level manifest struct
// continues to live in main.go; this is an additive read so existing
// sweep paths are unaffected.
type extendedManifest struct {
	APIName     string   `json:"api_name"`
	CLIName     string   `json:"cli_name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	AuthType    string   `json:"auth_type"`
	AuthEnvVars []string `json:"auth_env_vars"`
}

// agentcookieManifestForSweep computes the next-state body for
// agentcookie.toml at cliDir, returning (body, changed, err).
// Idempotent: a second call against the same inputs returns changed=false.
//
// Skip semantics — all return ("", false, nil):
//   - CLIs with no auth_env_vars (cookie-only or no auth). Logged to stderr.
//   - Existing agentcookie.toml carrying the override marker. Logged to stderr.
//
// The caller owns the os.WriteFile (mirroring the skill/readme pattern in
// sweepCLI) so a partial write failure can't leave a malformed file
// alongside correctly rolled-back peer artifacts.
func agentcookieManifestForSweep(cliDir string) (string, bool, error) {
	mfPath := filepath.Join(cliDir, ".printing-press.json")
	raw, err := os.ReadFile(mfPath)
	if err != nil {
		return "", false, fmt.Errorf("read manifest: %w", err)
	}
	var mf extendedManifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return "", false, fmt.Errorf("parse manifest: %w", err)
	}
	if !hasNonCookieAuth(mf) {
		fmt.Fprintf(os.Stderr, "  agentcookie: skipping %s (cookie-only or no env-var auth)\n", mf.CLIName)
		return "", false, nil
	}
	outPath := filepath.Join(cliDir, agentcookieTomlFilename)
	if hasAgentcookieOverrideMarker(outPath) {
		fmt.Fprintf(os.Stderr, "  agentcookie: skipping %s (manual override marker)\n", mf.CLIName)
		return "", false, nil
	}
	body := renderAgentcookieToml(mf)
	before, _ := os.ReadFile(outPath)
	if string(before) == body {
		return body, false, nil
	}
	return body, true, nil
}

// agentcookieManifestWouldChange returns true when the sweep would
// produce a new or modified file for cliDir. Used by sweepCLI's
// early-return path so a CLI whose only pending change is the
// agentcookie manifest still gets swept. Thin wrapper around
// agentcookieManifestForSweep.
func agentcookieManifestWouldChange(cliDir string) (bool, error) {
	_, changed, err := agentcookieManifestForSweep(cliDir)
	return changed, err
}

// hasNonCookieAuth returns true when the CLI has at least one env-var
// based credential. Mirrors the cli-printing-press helper of the same
// name so sweep and generator agree on which CLIs are eligible.
func hasNonCookieAuth(mf extendedManifest) bool {
	for _, v := range mf.AuthEnvVars {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// hasAgentcookieOverrideMarker returns true when an existing file at
// path begins with the manual-override marker. Missing files and read
// errors return false.
func hasAgentcookieOverrideMarker(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	return strings.Contains(string(head), agentcookieOverrideMarker)
}

// renderAgentcookieToml produces the canonical TOML body for the given
// manifest. Byte-for-byte deterministic. Mirrors the output of
// agentcookie/pkg/agentcookieadoption.Render so the sweep-side emit
// matches generator-side fresh prints exactly.
func renderAgentcookieToml(mf extendedManifest) string {
	cliName := strings.TrimSpace(mf.CLIName)
	if cliName == "" {
		cliName = mf.APIName + "-pp-cli"
	}
	displayName := strings.TrimSpace(mf.DisplayName)
	if displayName == "" {
		displayName = cliName
	}
	description := strings.TrimSpace(mf.Description)

	var b strings.Builder
	b.WriteString("# agentcookie.toml: secrets-bus adoption manifest v2\n")
	b.WriteString("# See docs/spec-agentcookie-secrets-bus-v2-adoption.md\n")
	b.WriteString("schema_version = 2\n")
	fmt.Fprintf(&b, "name = %q\n", cliName)
	fmt.Fprintf(&b, "display_name = %q\n", displayName)
	if description != "" {
		fmt.Fprintf(&b, "description = %q\n", description)
	}
	b.WriteString("project_kind = \"cli\"\n")
	b.WriteString("\n")
	b.WriteString("[secrets.file]\n")
	fmt.Fprintf(&b, "path = %q\n", fmt.Sprintf("~/.config/%s/config.toml", cliName))
	b.WriteString("\n")
	b.WriteString("[sync]\n")
	b.WriteString("default = false\n")
	b.WriteString("\n")
	// Only emit [sync.keys] when there's at least one key. The
	// hasNonCookieAuth guard above ensures we don't reach here with an
	// empty list, but defend in depth so renderAgentcookieToml is safe
	// to call independently from tests.
	keys := make([]string, 0, len(mf.AuthEnvVars))
	for _, k := range mf.AuthEnvVars {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) > 0 {
		sort.Strings(keys)
		b.WriteString("[sync.keys]\n")
		for _, k := range keys {
			// Auth env vars are credentials by definition; flag every
			// one sensitive=true. Per-key sensitivity refinement (e.g.
			// client_id paired with a sensitive client_secret) lands
			// when the generator's EnvVarSpecs path emits the manifest
			// directly, not via this retrofit sweep.
			//
			// %q quotes the key so a future env var containing a dot
			// or other non-bare-key character can't accidentally land
			// as TOML dotted-table notation (nested sub-table under
			// [sync.keys] instead of the intended flat key).
			fmt.Fprintf(&b, "%q = true\n", k)
		}
	}
	return b.String()
}
