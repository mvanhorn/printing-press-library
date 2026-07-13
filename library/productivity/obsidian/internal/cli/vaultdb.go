// Copyright 2026 Angelo Pullen and contributors. Licensed under Apache-2.0. See LICENSE.

// Multi-vault mirror support (added 2026-07-13).
//
// The V1 mirror was one shared data.db keyed by note path, and sync followed
// whichever vault Obsidian happened to have ACTIVE. With several vaults on one
// machine that produced two silent failure modes, both observed in the wild
// (Marketing Specialist vault, 2026-07-10):
//
//  1. Wrapper-rooted mirrors. Nested layouts (~/Desktop/<Name>/<Name>/) often
//     leave the OUTER wrapper as Obsidian's active vault. A wrapper-rooted
//     mirror prefixes every note path, so all path-style wikilinks fail to
//     resolve — ~16.7k false "broken" links and a garbage integrity axis,
//     with no error raised anywhere.
//  2. Shared-mirror thrash. Each vault's sync pruned the other vaults' rows
//     (`deleted: 836`) because prune compares the whole `notes` table against
//     one vault's file walk, and the incremental mtime checkpoint was global.
//
// The fix: one mirror DB file per vault (vault-<name>-<hash>.db) plus a
// current.json pointer recording the most recently synced vault. Sync derives
// the DB from the vault it is actually walking; read commands follow the
// pointer by default (see defaultDBPath), so the "sync, then query" flow is
// unchanged — but vaults can no longer clobber each other, and `sync
// --vault-path <abs>` removes the dependency on Obsidian's active-vault state
// entirely.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// vaultDBPath derives the per-vault mirror DB file for an absolute vault path.
// The basename slug keeps the filename human-readable; the path-hash suffix
// disambiguates same-named vaults — including the nested wrapper vs inner
// folder case, where both directories share a basename.
func vaultDBPath(dataDir, vaultPath string) string {
	clean := filepath.Clean(vaultPath)
	sum := sha256.Sum256([]byte(clean))
	slug := sanitizeVaultSlug(filepath.Base(clean))
	if slug == "" {
		slug = "vault"
	}
	return filepath.Join(dataDir, fmt.Sprintf("vault-%s-%s.db", slug, hex.EncodeToString(sum[:])[:8]))
}

// sanitizeVaultSlug lowercases and reduces a vault basename to [a-z0-9-] so it
// is safe as a filename component on every platform.
func sanitizeVaultSlug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// vaultPointer is the on-disk shape of current.json — the "most recently
// synced vault" record that read commands resolve their default DB through.
type vaultPointer struct {
	VaultPath  string `json:"vault_path"`
	DBPath     string `json:"db_path"`
	LastSyncAt string `json:"last_sync_at"`
}

const vaultPointerFile = "current.json"

// writeCurrentVaultPointer atomically records vaultPath/dbPath as the current
// mirror (write-to-temp + rename, so readers never see a torn file).
func writeCurrentVaultPointer(dataDir, vaultPath, dbPath string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(vaultPointer{
		VaultPath:  vaultPath,
		DBPath:     dbPath,
		LastSyncAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dataDir, "."+vaultPointerFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dataDir, vaultPointerFile))
}

// readCurrentVaultPointer loads current.json. ok is false when the pointer is
// missing, malformed, or points at a DB file that no longer exists — callers
// then fall back to the legacy shared data.db.
func readCurrentVaultPointer(dataDir string) (vaultPath, dbPath string, ok bool) {
	b, err := os.ReadFile(filepath.Join(dataDir, vaultPointerFile))
	if err != nil {
		return "", "", false
	}
	var p vaultPointer
	if err := json.Unmarshal(b, &p); err != nil || p.DBPath == "" {
		return "", "", false
	}
	if _, err := os.Stat(p.DBPath); err != nil {
		return "", "", false
	}
	return p.VaultPath, p.DBPath, true
}

// detectNestedWrapper reports whether root looks like the OUTER wrapper of a
// nested vault layout: it contains a child directory with the same basename
// that itself holds markdown content (checked one level deep inside the
// child). Mirrors rooted at the wrapper path-prefix every note, silently
// poisoning link resolution — the 2026-07-10 lint incident signature.
func detectNestedWrapper(root string) (inner string, isWrapper bool) {
	clean := filepath.Clean(root)
	inner = filepath.Join(clean, filepath.Base(clean))
	info, err := os.Stat(inner)
	if err != nil || !info.IsDir() {
		return "", false
	}
	entries, err := os.ReadDir(inner)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				return inner, true
			}
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		subs, err := os.ReadDir(filepath.Join(inner, e.Name()))
		if err != nil {
			continue
		}
		for _, s := range subs {
			if !s.IsDir() && strings.HasSuffix(strings.ToLower(s.Name()), ".md") {
				return inner, true
			}
		}
	}
	return "", false
}

// walkVaultFiles lists every file under vaultPath as vault-relative
// slash-separated paths, skipping dot-directories (.obsidian, .git, .trash…)
// and node_modules. It is the --vault-path replacement for the `obsidian
// files` subprocess, so sync works without Obsidian running at all. folder
// (optional) scopes the result to one vault-relative subtree, matching the
// --folder flag's contract.
func walkVaultFiles(vaultPath, folder string) ([]string, error) {
	root := filepath.Clean(vaultPath)
	var prefix string
	if folder != "" {
		prefix = strings.TrimSuffix(filepath.ToSlash(folder), "/") + "/"
	}
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if p != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if prefix != "" && !strings.HasPrefix(rel, prefix) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}
