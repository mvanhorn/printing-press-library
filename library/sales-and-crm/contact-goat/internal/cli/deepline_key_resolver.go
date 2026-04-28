// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// resolveDeeplineKey centralizes Deepline API key resolution across every
// paid path in the CLI (waterfall, dossier, prospect, deepline subcommands,
// doctor). The resolver checks three sources in priority order:
//
//   1. flag      — value passed on the command line via --deepline-key
//   2. env       — DEEPLINE_API_KEY in the process environment
//   3. file:<P>  — the Deepline CLI's persisted key at $HOME/.local/deepline/<host>/.env
//
// The file source closes the UX gap reported on 2026-04-28: a user with the
// official Deepline CLI installed and authenticated has the key on disk at
// mode 600, but contact-goat used to fail with "API key required" because it
// only checked env + flag. The user already trusts the file (their own home
// dir, mode 600 by default); reading it is a sibling-tool integration, not a
// new trust boundary.
//
// Security guards on file discovery:
//
//   - File must be under $HOME (no path traversal escape, no symlinks pointing
//     outside $HOME).
//   - File must NOT be group- or world-writable. The threat model is: a
//     non-owner process substituting a malicious key while we read it. Any
//     write bit outside the owner triple disqualifies the file. Read-only
//     looseness (e.g., mode 0644 — what the official Deepline CLI actually
//     writes) is accepted; "anyone can read but only owner can write" is the
//     upstream sibling tool's choice and refusing it would defeat the whole
//     auto-discovery feature.
//   - Value must start with "dlp_" (the Deepline key prefix) — otherwise we
//     skip the file rather than emit a malformed key.
//   - Empty values are skipped, not returned with an empty source.
//
// The resolver never logs or echoes the key value. Callers that want to
// surface the discovery state (e.g., doctor) use the returned source string,
// which is safe — it names the bucket and (for files) the path, never the
// secret.

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// deeplineKeyPrefix is the canonical Deepline API key prefix. Mirrors
// internal/deepline.KeyPrefix; duplicated here to avoid an import cycle from
// internal/cli into internal/deepline (which imports internal/cli for nothing
// today, but the resolver is dependency-free on purpose).
const deeplineKeyPrefix = "dlp_"

// deeplineHomeFunc is the home-directory accessor used by resolveDeeplineKey.
// Tests override this via a setter so they can sandbox file discovery to a
// t.TempDir() without touching the real $HOME.
var deeplineHomeFunc = os.UserHomeDir

// preferredDeeplineHostSlug is the host-slug subdirectory the resolver
// prefers when multiple sibling-CLI key files exist. The Deepline CLI today
// writes to ~/.local/deepline/code-deepline-com/.env; we prefer that path
// when both it and another slug are present, then fall back to the first
// lexically-sorted slug whose key parses successfully.
const preferredDeeplineHostSlug = "code-deepline-com"

// resolveDeeplineKey returns the Deepline API key and a label for where it
// came from. The label values are "flag", "env", "file:<absolute-path>", or
// the empty string when nothing resolved.
//
// Precedence: a non-empty flagValue wins over env, env wins over file. Any
// value found at a higher-priority source short-circuits the chain — we do
// not validate flag/env shape here (callers that validate the prefix run
// after the resolver returns).
func resolveDeeplineKey(flagValue string) (key, source string) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, "flag"
	}
	if v := strings.TrimSpace(os.Getenv("DEEPLINE_API_KEY")); v != "" {
		return v, "env"
	}
	if v, path := discoverDeeplineKeyFromSiblingCLI(); v != "" {
		return v, "file:" + path
	}
	return "", ""
}

// resolveDeeplineKeyWithSkips returns the same primary result as
// resolveDeeplineKey, plus a list of human-readable reasons describing any
// candidate sibling-CLI files that were rejected during file discovery.
// The doctor command surfaces these so users can fix a permissions or
// prefix-shape issue without having to re-discover what the resolver did.
//
// Skip reasons are file-shaped strings ("<path>: <reason>") suitable for
// display. They never include the key value.
func resolveDeeplineKeyWithSkips(flagValue string) (key, source string, skips []string) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, "flag", nil
	}
	if v := strings.TrimSpace(os.Getenv("DEEPLINE_API_KEY")); v != "" {
		return v, "env", nil
	}
	v, path, fileSkips := discoverDeeplineKeyFromSiblingCLIWithSkips()
	if v != "" {
		return v, "file:" + path, fileSkips
	}
	return "", "", fileSkips
}

// discoverDeeplineKeyFromSiblingCLI walks $HOME/.local/deepline/*/.env and
// returns the first acceptable key with its absolute path. It prefers the
// canonical host slug ("code-deepline-com") before falling back to lexical
// order across all matching directories.
func discoverDeeplineKeyFromSiblingCLI() (key, path string) {
	v, p, _ := discoverDeeplineKeyFromSiblingCLIWithSkips()
	return v, p
}

// discoverDeeplineKeyFromSiblingCLIWithSkips is the same walk but also
// returns rejection reasons for candidate files that were found but failed
// one of the security guards (mode wrong, symlink escape, prefix wrong,
// empty value). The doctor command displays these so the user can correct
// a permissions issue without having to re-discover the resolver's logic.
func discoverDeeplineKeyFromSiblingCLIWithSkips() (key, path string, skips []string) {
	home, err := deeplineHomeFunc()
	if err != nil || home == "" {
		return "", "", nil
	}

	root := filepath.Join(home, ".local", "deepline")
	entries, err := os.ReadDir(root)
	if err != nil {
		// Most users won't have the Deepline CLI installed. ENOENT here is
		// the common, silent path — not a skip.
		return "", "", nil
	}

	// Build the candidate list: every immediate subdirectory's .env file.
	// Sort lexically so the fallback "first match" is deterministic when
	// the preferred slug is absent.
	var candidates []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidates = append(candidates, e.Name())
	}
	sort.Strings(candidates)

	// Reorder so the preferred host slug is tried first when present.
	if preferredIdx := indexOfSlug(candidates, preferredDeeplineHostSlug); preferredIdx > 0 {
		candidates = append([]string{preferredDeeplineHostSlug}, removeSlugAt(candidates, preferredIdx)...)
	}

	for _, slug := range candidates {
		envPath := filepath.Join(root, slug, ".env")
		v, reason := readDeeplineEnvFile(envPath, home)
		if v != "" {
			return v, envPath, skips
		}
		if reason != "" {
			skips = append(skips, envPath+": "+reason)
		}
	}
	return "", "", skips
}

// readDeeplineEnvFile parses a sibling-CLI .env file and returns either a
// validated key or a skip reason describing why the file was rejected. An
// empty key with an empty reason means the file simply did not contain a
// DEEPLINE_API_KEY entry — that's not a security skip, just a non-match.
//
// Guards (in order):
//
//   - Lstat — symlink whose target leaves $HOME is rejected.
//   - Mode  — group or world WRITE access is rejected. Read access is
//             accepted because the upstream sibling CLI writes mode 0644.
//   - Parse — non-quoted KEY=VALUE per line, comments and blanks ignored.
//   - Prefix — value must start with "dlp_".
//   - Empty — DEEPLINE_API_KEY="" falls through (no error, no key).
func readDeeplineEnvFile(envPath, home string) (key, skipReason string) {
	info, err := os.Lstat(envPath)
	if err != nil {
		// File doesn't exist — silent miss, not a skip.
		return "", ""
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		target, terr := filepath.EvalSymlinks(envPath)
		if terr != nil {
			return "", "skipped: broken symlink"
		}
		// Refuse symlinks whose resolved target lies outside $HOME. A symlink
		// from inside $HOME to /etc/passwd or to a shared scratch dir is
		// exactly the trust-boundary escape this guard exists to refuse.
		absHome, herr := filepath.Abs(home)
		if herr != nil {
			return "", "skipped: cannot resolve home directory"
		}
		absTarget, terr := filepath.Abs(target)
		if terr != nil {
			return "", "skipped: cannot resolve symlink target"
		}
		if !strings.HasPrefix(absTarget+string(filepath.Separator), absHome+string(filepath.Separator)) &&
			absTarget != absHome {
			return "", "skipped: symlink escapes home directory"
		}
		// Re-stat through the symlink so the mode check uses the real file.
		info, err = os.Stat(envPath)
		if err != nil {
			return "", "skipped: cannot stat symlink target"
		}
	}

	mode := info.Mode().Perm()
	// Reject only group/world WRITE bits. World/group READ is acceptable —
	// the official Deepline CLI writes the .env file at mode 0644, and
	// rejecting that would defeat auto-discovery. The trust property is
	// "no other principal can substitute the key"; read access by other
	// users on a single-user workstation isn't a credential-substitution
	// vector.
	if mode&0o022 != 0 {
		return "", "skipped: mode " + modeOctal(mode) + " is group- or world-writable (chmod g-w,o-w to fix)"
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", "skipped: read failed (" + err.Error() + ")"
	}

	rawValue, found := parseDotenvDeeplineKey(string(data))
	if !found {
		return "", ""
	}
	if rawValue == "" {
		return "", "skipped: empty value"
	}
	if !strings.HasPrefix(rawValue, deeplineKeyPrefix) {
		return "", "skipped: value missing dlp_ prefix"
	}
	return rawValue, ""
}

// parseDotenvDeeplineKey is a tiny dotenv parser: KEY=VALUE per line, blank
// lines and comments ignored, surrounding double or single quotes stripped.
// Returns the value of the first DEEPLINE_API_KEY assignment encountered,
// and a bool indicating whether the key appeared at all (so callers can
// distinguish "not in file" from "in file but empty").
func parseDotenvDeeplineKey(data string) (value string, found bool) {
	for _, rawLine := range strings.Split(data, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip an optional leading "export " so files written with the
		// shell-export form parse the same way.
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key != "DEEPLINE_API_KEY" {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		// Strip a single matching pair of surrounding quotes — the dotenv
		// convention. Both single and double quotes accepted.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		return val, true
	}
	return "", false
}

// modeOctal renders a Unix mode as the conventional octal string ("0644")
// for use in skip-reason messages.
func modeOctal(m fs.FileMode) string {
	return "0" + octal3(uint32(m.Perm()))
}

// octal3 formats a 9-bit Unix mode as a 3-digit octal string. Hand-rolled
// to avoid importing fmt for one call in a hot path that runs on every
// doctor invocation.
func octal3(n uint32) string {
	digits := []byte{'0', '0', '0'}
	digits[0] = byte('0' + ((n >> 6) & 0o7))
	digits[1] = byte('0' + ((n >> 3) & 0o7))
	digits[2] = byte('0' + (n & 0o7))
	return string(digits)
}

func indexOfSlug(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func removeSlugAt(s []string, i int) []string {
	out := make([]string, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}
