// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
//
// `tesla auth import` — reverses `tesla auth export`. See plan 2026-05-22-001 U2c.
//
// Refusal policy (the agent should ask the user to set --force only when
// they understand what they're overwriting):
//   - Non-expired Fleet AccessToken already on disk -> refuse without --force.
//   - Private key file already at the destination path -> refuse without --force.
//   - Major manifest schema_version mismatch -> refuse regardless of --force.
//
// Same passphrase prompt mechanism as export (see tesla_creds_export.go).
// Wrong passphrase and bundle corruption both surface as a single generic
// "decryption failed" error — distinguishing them would leak passphrase
// validity to anyone who steals a bundle off the wire.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

func newCredsImportCmd(flags *rootFlags) *cobra.Command {
	var (
		force    bool
		noPrompt bool
	)
	cmd := &cobra.Command{
		Use:   "import <bundle-path>",
		Short: "Decrypt and install a credentials bundle from 'tesla auth export'",
		Long: `Restore Tesla auth state from a passphrase-encrypted bundle produced by
'tesla auth export' on another host.

Refusal policy:
  - Refuses to overwrite a non-expired Fleet AccessToken unless --force.
  - Refuses to overwrite an existing private key file unless --force.
  - Refuses outright on major manifest schema_version mismatch (re-export
    from a same-major-version source CLI).

Wrong passphrase and a corrupted bundle both surface as the same generic
"decryption failed" error.`,
		Example: `  tesla-pp-cli auth import tesla-creds-20260522T120000Z.tgz.enc
  tesla-pp-cli auth import bundle.tgz.enc --force`,
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			// Mutates config.toml AND writes a private key file. The agent
			// must ask the user before firing this.
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCredsImport(cmd, flags, args[0], force, noPrompt)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing non-expired Fleet token or private key file")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Read passphrase from TESLA_PP_CREDS_PASSPHRASE env var instead of prompting (for CI)")
	return cmd
}

func runCredsImport(cmd *cobra.Command, flags *rootFlags, bundlePath string, force, noPrompt bool) error {
	salt, nonce, ciphertext, err := readBundleFile(bundlePath)
	if err != nil {
		return usageErr(err)
	}

	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil {
		return configErr(err)
	}

	// Verify-mode and dry-run BOTH short-circuit BEFORE prompting for a
	// passphrase. The intent envelope reports what would happen without
	// performing any disk writes. Use _ for unused locals to keep the verify
	// path independent of the decryption side.
	_ = salt
	_ = nonce
	_ = ciphertext
	if cliutil.IsVerifyEnv() || dryRunOK(flags) {
		envelope := map[string]any{
			"intent":      "import",
			"bundle_path": bundlePath,
			"force":       force,
		}
		if cliutil.IsVerifyEnv() {
			envelope["verify_noop"] = true
		}
		if dryRunOK(flags) {
			envelope["dry_run"] = true
		}
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}

	passphrase, err := readPassphraseOrEnv(noPrompt, "Bundle passphrase: ")
	if err != nil {
		return err
	}
	defer zeroBytes(passphrase)

	key := deriveCredsKey(passphrase, salt)
	defer zeroBytes(key)

	plaintext, err := decryptCredsBundle(key, nonce, ciphertext)
	if err != nil {
		return usageErr(err)
	}

	contents, err := unpackCredsBundle(plaintext)
	if err != nil {
		return usageErr(fmt.Errorf("parsing bundle contents: %w", err))
	}

	// Manifest version check BEFORE any disk write.
	if err := checkCredsSchemaVersion(contents.Manifest.SchemaVersion); err != nil {
		return usageErr(err)
	}

	// Pre-flight refusal checks. We do these BEFORE writing anything so that
	// a refusal is total and atomic (no partial restore).
	conflicts, err := credsImportConflicts(cfg, contents, force)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 && !force {
		return usageErr(fmt.Errorf("refusing to overwrite existing state: %s (pass --force to override)", strings.Join(conflicts, ", ")))
	}

	// Apply each section. Order matters: write the key file FIRST so a
	// downstream config that references it has a real file to point at.
	written := map[string]string{}
	if contents.PrivateKey != nil {
		keyPath, err := writeImportedPrivateKey(contents, force)
		if err != nil {
			return err
		}
		written["private_key_path"] = keyPath
	}
	if contents.FleetTOML != nil {
		fleetBlock, err := parseFleetBlock(contents.FleetTOML)
		if err != nil {
			return fmt.Errorf("parsing fleet block: %w", err)
		}
		// If the bundle carried keys but the manifest's PrivateKeyPath is now
		// stale (because we rehomed the file to ~/.tesla/<name>), override
		// the [fleet] block's private_key_path with the actual landing zone.
		if p, ok := written["private_key_path"]; ok && p != "" {
			fleetBlock.PrivateKeyPath = p
		}
		if err := saveImportedFleetBlock(cfg, fleetBlock); err != nil {
			return err
		}
		written["fleet_block"] = "applied"
	}
	if contents.OwnerAPITOML != nil {
		oa, err := parseOwnerAPIBlock(contents.OwnerAPITOML)
		if err != nil {
			return fmt.Errorf("parsing owner-api block: %w", err)
		}
		// SaveTokens overwrites client_id + client_secret too; pass empty so
		// we don't clobber install-scoped values. Reload to pick up any
		// fleet-block changes from the previous step.
		cfg2, err := config.Load(flagsConfigPath(flags))
		if err != nil {
			return configErr(err)
		}
		if err := cfg2.SaveTokens(cfg2.ClientID, cfg2.ClientSecret, oa.AccessToken, oa.RefreshToken, oa.TokenExpiry); err != nil {
			return configErr(fmt.Errorf("saving owner-api tokens: %w", err))
		}
		written["owner_api"] = "applied"
	}

	out := map[string]any{
		"imported":       true,
		"bundle_path":    bundlePath,
		"includes":       contents.Manifest.Includes,
		"schema_version": contents.Manifest.SchemaVersion,
		"applied":        written,
	}
	// Surface a manifest version mismatch as a warning in the envelope so
	// scripted callers see it without parsing stderr.
	if warning := schemaVersionWarning(contents.Manifest.SchemaVersion); warning != "" {
		out["schema_version_warning"] = warning
		fmt.Fprintln(cmd.ErrOrStderr(), "warning:", warning)
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}

// checkCredsSchemaVersion refuses on major mismatch, allows minor mismatch
// (the caller surfaces a warning in the JSON envelope).
func checkCredsSchemaVersion(got string) error {
	if strings.TrimSpace(got) == "" {
		return fmt.Errorf("bundle missing manifest.schema_version")
	}
	gotMaj, _, err := parseSemverPair(got)
	if err != nil {
		return fmt.Errorf("invalid manifest.schema_version %q: %w", got, err)
	}
	wantMaj, _, err := parseSemverPair(credsManifestSchemaVersion)
	if err != nil {
		// Programmer error — the constant should parse.
		return fmt.Errorf("internal: parsing local schema version: %w", err)
	}
	if gotMaj != wantMaj {
		return fmt.Errorf("bundle schema_version %q is incompatible with this CLI (supports %q)", got, credsManifestSchemaVersion)
	}
	return nil
}

func schemaVersionWarning(got string) string {
	gotMaj, gotMin, err := parseSemverPair(got)
	if err != nil {
		return ""
	}
	wantMaj, wantMin, err := parseSemverPair(credsManifestSchemaVersion)
	if err != nil {
		return ""
	}
	if gotMaj == wantMaj && gotMin != wantMin {
		return fmt.Sprintf("bundle schema_version %q differs from CLI %q (same major; proceeding)", got, credsManifestSchemaVersion)
	}
	return ""
}

func parseSemverPair(s string) (major, minor int, err error) {
	parts := strings.SplitN(strings.TrimSpace(s), ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("expected major.minor format")
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("major: %w", err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("minor: %w", err)
	}
	return major, minor, nil
}

// credsImportConflicts lists the on-disk artifacts that would be overwritten.
// Returning a non-empty slice with force=false is a refusal; force=true
// suppresses the refusal but the list is still surfaced for the audit log.
func credsImportConflicts(cfg *config.Config, contents *credsBundleContents, force bool) ([]string, error) {
	var conflicts []string

	// Non-expired Fleet AccessToken on disk?
	if contents.FleetTOML != nil {
		existing := cfg.FleetTokens()
		// "non-expired" = has an access token AND (no expiry recorded OR
		// expiry is in the future). A missing expiry is conservatively
		// treated as non-expired so we don't silently overwrite a still-valid
		// credential. fleet-login should always record an expiry, but partial
		// configs (e.g. test fixtures, hand-edits) can omit it.
		if existing.AccessToken != "" && (existing.TokenExpiry.IsZero() || existing.TokenExpiry.After(time.Now())) {
			conflicts = append(conflicts, "fleet.access_token (non-expired)")
		}
	}

	// Private key file already at the bundle's destination path?
	if contents.PrivateKey != nil {
		dest := plannedKeyDestPath(contents)
		if _, err := os.Stat(dest); err == nil {
			conflicts = append(conflicts, "private key at "+dest)
		}
	}
	_ = force // returning the list to the caller; force handling is at the caller
	return conflicts, nil
}

// plannedKeyDestPath returns where writeImportedPrivateKey will land the key.
// Per spec the destination is ~/.tesla/<original-name>; if the home dir can't
// be resolved we fall back to /tmp so the destination is stable and visible
// in conflict messages.
func plannedKeyDestPath(contents *credsBundleContents) string {
	name := contents.Manifest.PrivateKeyName
	if name == "" {
		name = "private-key.pem"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), ".tesla", name)
	}
	return filepath.Join(home, ".tesla", name)
}

func writeImportedPrivateKey(contents *credsBundleContents, force bool) (string, error) {
	dest := plannedKeyDestPath(contents)
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	if _, err := os.Stat(dest); err == nil && !force {
		// Should have been caught by credsImportConflicts; this is
		// belt-and-suspenders against a TOCTOU between conflicts check and
		// write. With --force we just overwrite.
		return "", usageErr(fmt.Errorf("private key already exists at %s (pass --force to overwrite)", dest))
	}
	if err := os.WriteFile(dest, contents.PrivateKey, 0o600); err != nil {
		return "", fmt.Errorf("writing private key %s: %w", dest, err)
	}
	// Re-assert 0600 in case the file already existed with looser bits.
	if err := os.Chmod(dest, 0o600); err != nil {
		return "", fmt.Errorf("chmod %s: %w", dest, err)
	}
	return dest, nil
}

// parseFleetBlock unmarshals a fleet.toml entry produced by marshalFleetBlock.
// The on-wire shape is `[fleet]` so we round-trip through a wrapper struct.
func parseFleetBlock(data []byte) (config.FleetConfig, error) {
	var wrapper struct {
		Fleet config.FleetConfig `toml:"fleet"`
	}
	if err := toml.Unmarshal(data, &wrapper); err != nil {
		return config.FleetConfig{}, err
	}
	return wrapper.Fleet, nil
}

func parseOwnerAPIBlock(data []byte) (struct {
	AccessToken  string
	RefreshToken string
	TokenExpiry  time.Time
}, error) {
	var raw struct {
		AccessToken  string    `toml:"access_token"`
		RefreshToken string    `toml:"refresh_token"`
		TokenExpiry  time.Time `toml:"token_expiry"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return struct {
			AccessToken  string
			RefreshToken string
			TokenExpiry  time.Time
		}{}, err
	}
	return struct {
		AccessToken  string
		RefreshToken string
		TokenExpiry  time.Time
	}{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		TokenExpiry:  raw.TokenExpiry,
	}, nil
}

// saveImportedFleetBlock writes the imported FleetConfig to the on-disk
// config.toml. Uses config.SaveFleetTokens which preserves top-level iOS-app
// fields. Note that SaveFleetTokens applies only non-empty fields, but for an
// import we want the full block (including potentially empty tokens after
// fleet-register). We accomplish that by mutating cfg.Fleet directly here,
// then calling SaveFleetTokens with a single non-empty value to trigger the
// save() call.
func saveImportedFleetBlock(cfg *config.Config, fleet config.FleetConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	// Direct assignment is intentional: we want the bundle's [fleet] block
	// applied verbatim, not field-merged.
	cfg.Fleet = fleet
	// SaveFleetTokens(..., publicKeyDomain=cfg.Fleet.PublicKeyDomain, ...) is
	// safe even when our cfg.Fleet already has the value — the method
	// only refuses to assign EMPTY strings, never refuses non-empty.
	return cfg.SaveFleetTokens(
		fleet.ClientID,
		fleet.ClientSecret,
		fleet.AccessToken,
		fleet.RefreshToken,
		fleet.TokenExpiry,
		fleet.PublicKeyDomain,
		fleet.PrivateKeyPath,
	)
}
