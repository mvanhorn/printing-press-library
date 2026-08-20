// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `tesla auth export` and the shared cipher / passphrase plumbing used by
// `tesla auth import`. See plan 2026-05-22-001 U2c.
//
// Bundle file layout (on disk):
//
//	magic   : 18 bytes, literal "TESLA-PP-CREDS-v1\n"
//	salt    : 16 bytes, random per-export
//	nonce   : 12 bytes, random per-export (AES-GCM)
//	cipher  : ciphertext || GCM auth tag
//
// Plaintext (pre-encryption) is a tar.gz with this layout:
//
//	manifest.json           // schema_version, bundled_at, tool_version, includes
//	fleet.toml              // serialized [fleet] block (omitted if 'fleet' not in includes)
//	owner-api.toml          // serialized top-level iOS-app tokens (omitted if 'owner-api' not in includes)
//	keys/<original-name>    // contents of cfg.Fleet.PrivateKeyPath (omitted if 'keys' not in includes)
//
// Passphrase handling: read via golang.org/x/term.ReadPassword from /dev/tty
// equivalent (no echo). NEVER accepted via flag or env var, except the explicit
// `--no-prompt` + TESLA_PP_CREDS_PASSPHRASE escape hatch the test suite uses.
//
// Side-effect rule: verify-mode + --dry-run short-circuit before touching disk.

package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/argon2"
	"golang.org/x/term"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// Bundle magic header. Bumping this is a SemVer-breaking change for the bundle
// format itself (distinct from the manifest schema_version which evolves
// inside the tarball).
const credsBundleMagic = "TESLA-PP-CREDS-v1\n"

// credsManifestSchemaVersion is the manifest.json schema version. Encoded as
// "<major>.<minor>". Bumping the minor allows import to warn but proceed;
// bumping the major makes import refuse.
const credsManifestSchemaVersion = "1.0"

// Argon2id parameters. Same constants on import; passphrase-only secret is
// derived deterministically given the per-bundle salt.
const (
	credsArgonTime    uint32 = 1
	credsArgonMemory  uint32 = 64 * 1024
	credsArgonThreads uint8  = 4
	credsArgonKeyLen  uint32 = 32
)

const (
	credsSaltLen  = 16
	credsNonceLen = 12
)

// credsPassphraseEnvVar is honored ONLY when the caller also passed
// --no-prompt. We deliberately do NOT read it in the normal interactive path
// because the entire point of the prompt is to keep the passphrase off
// `ps aux` AND out of the environment of a shared shell.
const credsPassphraseEnvVar = "TESLA_PP_CREDS_PASSPHRASE"

// readPassphraseFn is the indirection tests substitute. The default impl reads
// from /dev/tty (or stdin's fd) with echo off via golang.org/x/term. Tests set
// this to a func returning a known passphrase. The variable is package-level
// so substitution survives across subcommand invocations within one test.
var readPassphraseFn = defaultReadPassphrase

func defaultReadPassphrase(_ string) ([]byte, error) {
	fmt.Fprint(os.Stderr, "Bundle passphrase: ")
	buf, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading passphrase: %w", err)
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("passphrase cannot be empty")
	}
	return buf, nil
}

// readPassphraseOrEnv pulls the passphrase from the prompt unless --no-prompt
// is set AND the env var is present. Returns a usage error if --no-prompt is
// set without the env var (the caller misconfigured CI).
func readPassphraseOrEnv(noPrompt bool, promptLabel string) ([]byte, error) {
	if noPrompt {
		v := os.Getenv(credsPassphraseEnvVar)
		if v == "" {
			return nil, usageErr(fmt.Errorf("--no-prompt requires %s env var", credsPassphraseEnvVar))
		}
		return []byte(v), nil
	}
	return readPassphraseFn(promptLabel)
}

// credsManifest is the manifest.json file stored inside the encrypted tarball.
// Keys are stable across versions: callers MUST NOT rename fields without
// bumping the major version.
type credsManifest struct {
	SchemaVersion  string    `json:"schema_version"`
	BundledAt      time.Time `json:"bundled_at"`
	ToolVersion    string    `json:"tool_version"`
	Includes       []string  `json:"includes"`
	PrivateKeyName string    `json:"private_key_name,omitempty"`
	PrivateKeyPath string    `json:"private_key_path,omitempty"`
}

// credsBundleContents is the in-memory pre-tar shape. Tests pack and unpack
// directly through this struct rather than over the disk roundtrip.
type credsBundleContents struct {
	Manifest      credsManifest
	FleetTOML     []byte // serialized cfg.Fleet block; nil when not included
	OwnerAPITOML  []byte // serialized iOS-app top-level fields; nil when not included
	PrivateKey    []byte // raw key bytes; nil when not included
	PrivateKeyAbs string // absolute path on the source machine (informational, in manifest)
}

func newCredsExportCmd(flags *rootFlags) *cobra.Command {
	var (
		outPath  string
		include  string
		noPrompt bool
	)
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Encrypt and bundle Tesla credentials for transport to another host",
		Long: `Bundle Tesla auth state (Fleet partner-app + tokens, BLE-paired private key,
optional iOS-app owner-api tokens) into a passphrase-encrypted tar.gz.

The output is suitable for transport to a remote agent host (e.g. a cloud
Mac mini) over scp, AirDrop, or any other channel. The receiving host runs
'tesla auth import <bundle>' with the same passphrase.

Security:
  - Passphrase is read via getpass (no echo, no shell history, no ps-visible
    flag). The env var TESLA_PP_CREDS_PASSPHRASE is honored ONLY when
    --no-prompt is set (intended for CI).
  - Encryption: AES-256-GCM with Argon2id passphrase derivation.
  - Bundle is written mode 0600.

Default --include: keys,fleet (owner-api is excluded by default since the
Fleet partner-token path is sufficient for most agent deployments).`,
		Example: `  tesla-pp-cli auth export --out tesla-creds.tgz.enc
  tesla-pp-cli auth export --include keys,fleet,owner-api`,
		Annotations: map[string]string{
			// Writes a file containing sensitive credential material; agents
			// must ask the user to confirm before invoking.
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCredsExport(cmd, flags, outPath, include, noPrompt)
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output bundle path (default: tesla-creds-<utc-timestamp>.tgz.enc)")
	cmd.Flags().StringVar(&include, "include", "keys,fleet", "Comma-separated bundle contents: keys,fleet,owner-api")
	cmd.Flags().BoolVar(&noPrompt, "no-prompt", false, "Read passphrase from TESLA_PP_CREDS_PASSPHRASE env var instead of prompting (for CI)")
	return cmd
}

func runCredsExport(cmd *cobra.Command, flags *rootFlags, outPath, include string, noPrompt bool) error {
	includes, err := parseCredsIncludes(include)
	if err != nil {
		return usageErr(err)
	}
	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil {
		return configErr(err)
	}

	// Build the in-memory bundle BEFORE prompting for a passphrase so that
	// configuration errors (missing key file, no fleet block to export) fail
	// fast without making the user type a passphrase first.
	contents, err := buildCredsBundleContents(cfg, includes)
	if err != nil {
		return err
	}

	// Resolve the output path. Empty -> default in CWD with UTC timestamp.
	if outPath == "" {
		stamp := time.Now().UTC().Format("20060102T150405Z")
		outPath = fmt.Sprintf("tesla-creds-%s.tgz.enc", stamp)
	}

	// Verify-mode and dry-run: print intent without touching disk OR prompting
	// for a passphrase.
	if cliutil.IsVerifyEnv() || dryRunOK(flags) {
		plaintext, packErr := packCredsBundle(contents)
		if packErr != nil {
			return packErr
		}
		envelope := map[string]any{
			"intent":             "export",
			"out_path":           outPath,
			"includes":           includes,
			"estimated_size_b":   len(plaintext) + len(credsBundleMagic) + credsSaltLen + credsNonceLen + 16, // 16 = GCM tag
			"private_key_source": contents.PrivateKeyAbs,
			"schema_version":     credsManifestSchemaVersion,
			"tool_version":       version,
		}
		if cliutil.IsVerifyEnv() {
			envelope["verify_noop"] = true
		}
		if dryRunOK(flags) {
			envelope["dry_run"] = true
		}
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}

	// Pre-flight the output path so an unwritable destination surfaces as a
	// usage error BEFORE prompting for the passphrase. Without this, the user
	// types the passphrase, then hits "permission denied" and has to re-enter.
	if err := checkCredsOutputWritable(outPath); err != nil {
		return usageErr(err)
	}

	passphrase, err := readPassphraseOrEnv(noPrompt, "Bundle passphrase: ")
	if err != nil {
		return err
	}
	// Zero the passphrase buffer when this function returns so the bytes
	// aren't retained in heap-resident garbage.
	defer zeroBytes(passphrase)

	plaintext, err := packCredsBundle(contents)
	if err != nil {
		return err
	}

	salt := make([]byte, credsSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}
	nonce := make([]byte, credsNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}

	key := deriveCredsKey(passphrase, salt)
	defer zeroBytes(key)

	ciphertext, err := encryptCredsBundle(key, nonce, plaintext)
	if err != nil {
		return err
	}

	// Write atomically: temp file in same dir, then rename. This prevents a
	// half-written file on disk if the process is killed mid-write.
	if err := writeCredsBundleAtomic(outPath, salt, nonce, ciphertext); err != nil {
		return err
	}

	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"exported":       true,
		"out_path":       outPath,
		"includes":       includes,
		"schema_version": credsManifestSchemaVersion,
		"tool_version":   version,
		"size_b":         len(credsBundleMagic) + len(salt) + len(nonce) + len(ciphertext),
	}, flags)
}

// parseCredsIncludes normalizes the --include flag value. Recognized tokens:
// "keys", "fleet", "owner-api". Empty input is a usage error.
func parseCredsIncludes(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("--include cannot be empty")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(strings.ToLower(tok))
		if tok == "" {
			continue
		}
		switch tok {
		case "keys", "fleet", "owner-api":
		default:
			return nil, fmt.Errorf("unknown --include token %q (valid: keys, fleet, owner-api)", tok)
		}
		if !seen[tok] {
			seen[tok] = true
			out = append(out, tok)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--include must list at least one of: keys, fleet, owner-api")
	}
	return out, nil
}

func includesHas(includes []string, target string) bool {
	for _, i := range includes {
		if i == target {
			return true
		}
	}
	return false
}

// buildCredsBundleContents reads the persisted config and key file to construct
// the in-memory bundle. This is the function the verify-mode short-circuit
// also calls so the intent envelope reflects real on-disk state.
func buildCredsBundleContents(cfg *config.Config, includes []string) (*credsBundleContents, error) {
	c := &credsBundleContents{
		Manifest: credsManifest{
			SchemaVersion: credsManifestSchemaVersion,
			BundledAt:     time.Now().UTC(),
			ToolVersion:   version,
			Includes:      includes,
		},
	}

	if includesHas(includes, "fleet") {
		fleetTOML, err := marshalFleetBlock(cfg.Fleet)
		if err != nil {
			return nil, fmt.Errorf("serializing fleet block: %w", err)
		}
		c.FleetTOML = fleetTOML
	}

	if includesHas(includes, "owner-api") {
		ownerTOML, err := marshalOwnerAPIBlock(cfg)
		if err != nil {
			return nil, fmt.Errorf("serializing owner-api block: %w", err)
		}
		c.OwnerAPITOML = ownerTOML
	}

	if includesHas(includes, "keys") {
		keyPath := strings.TrimSpace(cfg.Fleet.PrivateKeyPath)
		if keyPath == "" {
			return nil, usageErr(fmt.Errorf("--include keys requested but cfg.Fleet.PrivateKeyPath is empty (run 'tesla auth fleet-register' first)"))
		}
		expanded := expandUserHome(keyPath)
		raw, err := os.ReadFile(expanded)
		if err != nil {
			return nil, fmt.Errorf("reading private key %s: %w", expanded, err)
		}
		c.PrivateKey = raw
		c.PrivateKeyAbs = expanded
		c.Manifest.PrivateKeyName = filepath.Base(expanded)
		c.Manifest.PrivateKeyPath = keyPath
	}

	return c, nil
}

// packCredsBundle serializes the bundle into a gzipped tarball. Same shape as
// the import side's unpackCredsBundle reads.
func packCredsBundle(c *credsBundleContents) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	manifestBytes, err := json.MarshalIndent(c.Manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarEntry(tw, "manifest.json", manifestBytes, 0o600); err != nil {
		return nil, err
	}
	if c.FleetTOML != nil {
		if err := writeTarEntry(tw, "fleet.toml", c.FleetTOML, 0o600); err != nil {
			return nil, err
		}
	}
	if c.OwnerAPITOML != nil {
		if err := writeTarEntry(tw, "owner-api.toml", c.OwnerAPITOML, 0o600); err != nil {
			return nil, err
		}
	}
	if c.PrivateKey != nil {
		name := c.Manifest.PrivateKeyName
		if name == "" {
			name = "private-key.pem"
		}
		if err := writeTarEntry(tw, "keys/"+name, c.PrivateKey, 0o600); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar close: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte, mode int64) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    mode,
		Size:    int64(len(data)),
		ModTime: time.Unix(0, 0).UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("tar header %s: %w", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write %s: %w", name, err)
	}
	return nil
}

// deriveCredsKey runs Argon2id with the documented parameters.
func deriveCredsKey(passphrase, salt []byte) []byte {
	return argon2.IDKey(passphrase, salt, credsArgonTime, credsArgonMemory, credsArgonThreads, credsArgonKeyLen)
}

func encryptCredsBundle(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	// Bind the magic header bytes into the GCM tag as additionalData. Without
	// this, an attacker who obtains a bundle could flip header bytes (e.g.
	// downgrade the version sentinel) without the auth tag detecting the
	// change; only the plaintext bytes.Equal check on import would catch it.
	// With this binding, any header tamper makes Open fail with a generic
	// decryption error - the same failure mode as wrong-passphrase.
	return gcm.Seal(nil, nonce, plaintext, []byte(credsBundleMagic)), nil
}

func writeCredsBundleAtomic(outPath string, salt, nonce, ciphertext []byte) error {
	dir := filepath.Dir(outPath)
	tmp, err := os.CreateTemp(dir, ".tesla-creds-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we don't successfully rename.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := tmp.Write([]byte(credsBundleMagic)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write magic: %w", err)
	}
	if _, err := tmp.Write(salt); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write salt: %w", err)
	}
	if _, err := tmp.Write(nonce); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write nonce: %w", err)
	}
	if _, err := tmp.Write(ciphertext); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write ciphertext: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, outPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	cleanup = false
	// Re-assert mode 0600 on the final path; some filesystems don't preserve
	// the temp's mode through rename.
	if err := os.Chmod(outPath, 0o600); err != nil {
		return fmt.Errorf("chmod final: %w", err)
	}
	return nil
}

// checkCredsOutputWritable validates the destination directory exists and is
// writable. We use a temp-file probe rather than a stat-only check because a
// stat-writable directory can still fail to accept new files (full disk,
// quota, immutable bit, etc.).
func checkCredsOutputWritable(outPath string) error {
	dir := filepath.Dir(outPath)
	if dir == "" {
		dir = "."
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("output directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("output directory %s is not a directory", dir)
	}
	probe, err := os.CreateTemp(dir, ".tesla-creds-probe-*")
	if err != nil {
		return fmt.Errorf("output directory %s is not writable: %w", dir, err)
	}
	probeName := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probeName)
	return nil
}

// marshalFleetBlock serializes ONLY the [fleet] sub-struct as a top-level
// "fleet" table. We round-trip through a wrapper struct so the on-wire shape
// matches `[fleet]` in config.toml, which keeps the import-side parsing
// symmetric.
func marshalFleetBlock(f config.FleetConfig) ([]byte, error) {
	wrapper := struct {
		Fleet config.FleetConfig `toml:"fleet"`
	}{Fleet: f}
	return tomlMarshalCreds(wrapper)
}

// marshalOwnerAPIBlock serializes the top-level iOS-app credential fields
// (AccessToken, RefreshToken, TokenExpiry) into a small TOML doc. We do NOT
// bundle ClientID/ClientSecret/AuthHeaderVal/TeslaAuthToken — those are
// install-scoped and would leak more than the agent needs.
func marshalOwnerAPIBlock(cfg *config.Config) ([]byte, error) {
	type ownerAPI struct {
		AccessToken  string    `toml:"access_token"`
		RefreshToken string    `toml:"refresh_token"`
		TokenExpiry  time.Time `toml:"token_expiry"`
	}
	return tomlMarshalCreds(ownerAPI{
		AccessToken:  cfg.AccessToken,
		RefreshToken: cfg.RefreshToken,
		TokenExpiry:  cfg.TokenExpiry,
	})
}

// expandUserHome resolves a leading "~/" or "~" to the current user's home.
// Anything else is returned unchanged. We intentionally do NOT support
// "~otheruser/" because the only path we expand is one the user owns.
func expandUserHome(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// zeroBytes overwrites b with zeroes. Best-effort: Go's GC can copy slices so
// older copies may still linger, but this is cheap defense-in-depth.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// readBundleFile reads a bundle file from disk and returns its three sections.
// Shared between import and any future verify subcommand.
func readBundleFile(path string) (salt, nonce, ciphertext []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("reading bundle %s: %w", path, err)
	}
	return parseBundleBytes(data)
}

func parseBundleBytes(data []byte) (salt, nonce, ciphertext []byte, err error) {
	magicLen := len(credsBundleMagic)
	minLen := magicLen + credsSaltLen + credsNonceLen + 16 // 16 = GCM tag
	if len(data) < minLen {
		return nil, nil, nil, fmt.Errorf("bundle too short (%d bytes; need at least %d)", len(data), minLen)
	}
	if !bytes.Equal(data[:magicLen], []byte(credsBundleMagic)) {
		return nil, nil, nil, fmt.Errorf("bundle magic header mismatch (not a tesla-pp creds bundle)")
	}
	off := magicLen
	salt = data[off : off+credsSaltLen]
	off += credsSaltLen
	nonce = data[off : off+credsNonceLen]
	off += credsNonceLen
	ciphertext = data[off:]
	return salt, nonce, ciphertext, nil
}

// decryptCredsBundle reverses encryptCredsBundle. GCM verifies the auth tag;
// a corrupted bundle or wrong passphrase both surface as the same generic
// "decryption failed" error — distinguishing the two would leak passphrase
// validity to an attacker who has stolen the bundle.
func decryptCredsBundle(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		// Block-size failure here would only fire on a programmer error
		// (wrong key length). Pass through verbatim — not a passphrase issue.
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	// additionalData must match the seal side (binds the magic header bytes).
	// See encryptCredsBundle for the rationale.
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(credsBundleMagic))
	if err != nil {
		return nil, fmt.Errorf("decryption failed")
	}
	return plaintext, nil
}

// tomlMarshalCreds wraps the toml.Marshal call so the import/export pair has a
// single seam: if the project ever switches TOML libraries, this is the only
// spot that has to change for the bundle format.
func tomlMarshalCreds(v any) ([]byte, error) {
	return toml.Marshal(v)
}

// unpackCredsBundle parses the in-memory tar.gz back into a credsBundleContents.
func unpackCredsBundle(plaintext []byte) (*credsBundleContents, error) {
	gz, err := gzip.NewReader(bytes.NewReader(plaintext))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	c := &credsBundleContents{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}
		buf, readErr := io.ReadAll(tr)
		if readErr != nil {
			return nil, fmt.Errorf("tar read %s: %w", hdr.Name, readErr)
		}
		switch {
		case hdr.Name == "manifest.json":
			if err := json.Unmarshal(buf, &c.Manifest); err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
		case hdr.Name == "fleet.toml":
			c.FleetTOML = buf
		case hdr.Name == "owner-api.toml":
			c.OwnerAPITOML = buf
		case strings.HasPrefix(hdr.Name, "keys/"):
			c.PrivateKey = buf
		}
	}
	return c, nil
}
