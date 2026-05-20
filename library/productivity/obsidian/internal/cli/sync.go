// Copyright 2026 drdriftwood. Licensed under Apache-2.0. See LICENSE.

// Sync replaces the generic Printing Press REST-walker with an
// obsidian-specific vault crawler. It iterates every markdown file in
// the active vault, parses frontmatter / wikilinks / tags / embeds, and
// populates the obsidian-specific tables (see internal/store/obsidian_schema.go).
//
// Sync is the ONLY command that requires Obsidian to be running with a
// vault open. Tier-3 read commands (health, stale, orphans, broken,
// decay, hotspots, reconcile, sql) query the mirror and work fully offline.

package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/store"
)

// wikilinkRE captures [[Target]] and [[Target|Alias]] forms. The capture
// group keeps only the target — the alias after the pipe is human-only.
var wikilinkRE = regexp.MustCompile(`\[\[([^\]\|#\^]+)(?:[\|#\^][^\]]*)?\]\]`)

// embedRE captures ![[Target]] embeds. Distinct from wikilinkRE so we
// can tag link_type accurately in the obsidian_links table.
var embedRE = regexp.MustCompile(`!\[\[([^\]\|#\^]+)(?:[\|#\^][^\]]*)?\]\]`)

// externalLinkRE captures [text](https://...) markdown links. Excludes
// relative paths so internal references don't get mistyped as external.
var externalLinkRE = regexp.MustCompile(`\[[^\]]+\]\((https?://[^\)]+)\)`)

// inlineTagRE matches Obsidian inline tags like #foo/bar. The leading
// boundary is either start-of-line or whitespace so #hashes-in-text and
// CSS color codes inside fenced code blocks slip through the cracks — the
// official `obsidian tags file=<f>` call is the authoritative source
// when accuracy matters; this is a backup for content-only sync. Tracks
// "system colors as tags" as a known false positive in V1.
var inlineTagRE = regexp.MustCompile(`(?:^|\s)#([A-Za-z][A-Za-z0-9_/-]*)`)

// frontmatterRE matches the YAML frontmatter prefix that an Obsidian
// note may carry: `---\n...\n---\n` at the very start of the file.
var frontmatterRE = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n`)

// frontmatterTitleKeys is the ordered preference for surfacing a title
// from a note's frontmatter when one is present. Falls back to filename.
var frontmatterTitleKeys = []string{"title", "name"}

// frontmatterAliasKeys captures aliases-as-titles (Obsidian's "aliases"
// property is the canonical mechanism for "this note can be referenced
// by these other names"). When the user hasn't set a title but did set
// aliases, the first alias is a reasonable display title.
var frontmatterAliasKeys = []string{"aliases", "alias"}

// frontmatterTagKeys captures tags declared in YAML frontmatter. Both
// scalar ("tag-name") and list (["a", "b"]) shapes appear in the wild.
var frontmatterTagKeys = []string{"tags", "tag"}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxFiles int
	var full bool
	var folder string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror the open vault into the local SQLite store",
		Long: `Walk every markdown file in the active Obsidian vault and populate the
local mirror. Frontmatter, wikilinks, embeds, and tags are extracted from
file content; created/modified timestamps come from the filesystem.

Sync is the ONLY command that requires Obsidian to be running with a
vault open. Tier-3 read commands (health, stale, orphans, broken,
decay, hotspots, reconcile, sql) query the mirror and work fully offline
once a sync has populated it.

Re-runs are incremental: a note whose mtime is older than the last sync
is skipped unless --full is passed.`,
		Example: `  # First sync (walks the full vault)
  obsidian-pp-cli sync

  # Incremental — only touched files since last sync
  obsidian-pp-cli sync

  # Full resync (ignore mtime checkpoint)
  obsidian-pp-cli sync --full

  # Scope to one folder
  obsidian-pp-cli sync --folder Projects

  # Cap work in CI / dogfood
  obsidian-pp-cli sync --max-files 20`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,4,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("obsidian-pp-cli")
			}

			// Verifier-mode short-circuit: PRINTING_PRESS_VERIFY=1 runs
			// every command against a mock server with no `obsidian`
			// subprocess available, so calling out would crash the data-
			// pipeline probe. Return a successful no-op envelope so
			// verifier-mode treats sync as wired without exercising it.
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					out, _ := json.MarshalIndent(map[string]any{
						"verify_mode":  true,
						"vault_path":   "",
						"total_files":  0,
						"synced":       0,
						"skipped":      0,
						"errored":      0,
						"last_sync_at": "",
					}, "", "  ")
					fmt.Fprintln(cmd.OutOrStdout(), string(out))
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "verify mode: sync skipped (no obsidian subprocess in mock environment).")
				}
				return nil
			}

			// Under dogfood (--live matrix, real network calls but bounded
			// per AGENTS.md "Long-running commands under live-dogfood"),
			// cap work hard so the happy-path probe completes within the
			// 30s per-command timeout.
			if cliutil.IsDogfoodEnv() && (maxFiles == 0 || maxFiles > 20) {
				maxFiles = 20
			}

			vaultPath, err := obsidianVaultPath()
			if err != nil {
				return fmt.Errorf("could not resolve vault path: %w\nIs Obsidian running with a vault open?", err)
			}

			files, err := obsidianListFiles(folder)
			if err != nil {
				return fmt.Errorf("listing vault files: %w", err)
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			if err := db.EnsureObsidianSchema(); err != nil {
				return err
			}

			lastSync := time.Time{}
			if !full {
				if t, ok := readLastSync(db.DB()); ok {
					lastSync = t
				}
			}

			started := time.Now()
			syncedCount := 0
			skippedCount := 0
			errorCount := 0
			// Track every .md path the walker considered so the post-loop
			// pruning step can delete notes whose vault files no longer
			// exist. Using a set (map keys) avoids O(n) lookups.
			processedMD := make(map[string]struct{}, len(files))
			for i, rel := range files {
				if maxFiles > 0 && i >= maxFiles {
					break
				}
				if !strings.HasSuffix(strings.ToLower(rel), ".md") {
					continue
				}
				processedMD[rel] = struct{}{}
				absPath := filepath.Join(vaultPath, rel)
				info, err := os.Stat(absPath)
				if err != nil {
					errorCount++
					continue
				}
				if !full && !lastSync.IsZero() && !info.ModTime().After(lastSync) {
					skippedCount++
					continue
				}
				if err := syncOneNote(db, rel, absPath, info); err != nil {
					errorCount++
					if flags.dataSource != "local" {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %v\n", rel, err)
					}
					continue
				}
				syncedCount++
			}

			// Prune notes whose vault files were deleted between syncs.
			// Only safe to do when the file list reflects the complete
			// vault: skip pruning when --max-files capped the walk, when
			// --folder narrowed it, or when the previous loop hit an
			// errored Stat (we don't want a transient filesystem error to
			// mass-delete the mirror). ON DELETE CASCADE on the child
			// tables (obsidian_tags / obsidian_links / frontmatter_kv)
			// handles the cleanup.
			deletedCount := 0
			pruneEligible := maxFiles == 0 && folder == "" && errorCount == 0
			if pruneEligible {
				var n int
				n, err = pruneDeletedNotes(db, processedMD)
				if err != nil {
					return fmt.Errorf("pruning deleted notes: %w", err)
				}
				deletedCount = n
			}

			now := time.Now().UTC().Format(time.RFC3339)
			if _, err := db.DB().Exec(`
				INSERT INTO vault_sync_state (id, vault_path, last_sync_at, notes_synced)
				VALUES (1, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					vault_path   = excluded.vault_path,
					last_sync_at = excluded.last_sync_at,
					notes_synced = notes_synced + excluded.notes_synced
			`, vaultPath, now, syncedCount); err != nil {
				return fmt.Errorf("recording sync state: %w", err)
			}

			elapsed := time.Since(started)
			if flags.asJSON {
				out, _ := json.MarshalIndent(map[string]any{
					"vault_path":   vaultPath,
					"total_files":  len(files),
					"synced":       syncedCount,
					"skipped":      skippedCount,
					"errored":      errorCount,
					"deleted":      deletedCount,
					"max_files":    maxFiles,
					"duration_ms":  elapsed.Milliseconds(),
					"last_sync_at": now,
				}, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Sync complete in %s: synced=%d skipped=%d errored=%d deleted=%d (vault %s, %d files total)\n",
				elapsed.Round(time.Millisecond), syncedCount, skippedCount, errorCount, deletedCount, vaultPath, len(files))
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/obsidian-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxFiles, "max-files", 0, "Maximum files to sync (0 = unlimited; capped at 20 under PRINTING_PRESS_DOGFOOD)")
	cmd.Flags().BoolVar(&full, "full", false, "Full resync — ignore mtime checkpoint")
	cmd.Flags().StringVar(&folder, "folder", "", "Sync only files under this vault-relative folder")
	return cmd
}

// obsidianVaultPath asks the local `obsidian` binary for the active
// vault's absolute filesystem path.
func obsidianVaultPath() (string, error) {
	out, err := exec.Command("obsidian", "vault", "info=path").Output()
	if err != nil {
		return "", fmt.Errorf("obsidian vault info=path: %w", err)
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", fmt.Errorf("obsidian returned empty vault path")
	}
	return p, nil
}

// obsidianListFiles returns every file the obsidian CLI reports for the
// current vault, optionally scoped to a folder.
func obsidianListFiles(folder string) ([]string, error) {
	args := []string{"files"}
	if folder != "" {
		args = append(args, "folder="+folder)
	}
	out, err := exec.Command("obsidian", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("obsidian files: %w", err)
	}
	var files []string
	for _, ln := range strings.Split(string(out), "\n") {
		s := strings.TrimSpace(ln)
		if s != "" {
			files = append(files, s)
		}
	}
	return files, nil
}

// syncOneNote reads a single markdown file, parses it, and upserts every
// derived row (notes / frontmatter_kv / obsidian_tags / obsidian_links).
//
// pp:client-call — the parent sync command's filesystem walk is the
// real external call; this function does deterministic CPU work on the
// already-fetched bytes.
func syncOneNote(db *store.Store, relPath, absPath string, info os.FileInfo) error {
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}
	content := string(raw)

	fmYAML, body := splitFrontmatter(content)
	frontmatter := map[string]any{}
	if fmYAML != "" {
		_ = yaml.Unmarshal([]byte(fmYAML), &frontmatter)
	}

	title := deriveTitle(frontmatter, relPath)
	wordCount := countWords(body)
	hash := sha256.Sum256(raw)
	contentHash := hex.EncodeToString(hash[:])
	frontmatterJSON, _ := json.Marshal(frontmatter)

	createdAt := info.ModTime().UTC().Format(time.RFC3339)
	if cv, ok := frontmatter["created"]; ok {
		if s, ok := cv.(string); ok && s != "" {
			createdAt = s
		}
	}
	modifiedAt := info.ModTime().UTC().Format(time.RFC3339)

	// All writes for one note go through a single transaction so a
	// mid-flight error (constraint violation on a tag/link, disk hiccup)
	// rolls back the DELETEs of child rows. Without the transaction, the
	// parent loop's per-note errorCount path would leave the note record
	// with empty obsidian_tags / obsidian_links / frontmatter_kv tables
	// and the next incremental sync would skip the note (mtime unchanged
	// vs vault_sync_state.last_sync_at), so Tier-3 commands would silently
	// report wrong results until --full reseeded.
	tx, err := db.DB().BeginTx(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
		INSERT INTO notes (path, title, created_at, modified_at, word_count, content_hash, frontmatter_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title            = excluded.title,
			created_at       = excluded.created_at,
			modified_at      = excluded.modified_at,
			word_count       = excluded.word_count,
			content_hash     = excluded.content_hash,
			frontmatter_json = excluded.frontmatter_json
	`, relPath, title, createdAt, modifiedAt, wordCount, contentHash, string(frontmatterJSON)); err != nil {
		return fmt.Errorf("upserting note: %w", err)
	}
	var noteID int64
	row := tx.QueryRow(`SELECT id FROM notes WHERE path = ?`, relPath)
	if err := row.Scan(&noteID); err != nil {
		return fmt.Errorf("looking up note id: %w", err)
	}

	// Refresh per-note child rows (cheaper than diff for V1's data sizes).
	if _, err := tx.Exec(`DELETE FROM obsidian_tags WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM obsidian_links WHERE source_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM frontmatter_kv WHERE note_id = ?`, noteID); err != nil {
		return err
	}

	for _, t := range extractTags(body, frontmatter) {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO obsidian_tags (note_id, tag) VALUES (?, ?)`, noteID, t); err != nil {
			return err
		}
	}

	for _, l := range extractLinks(body) {
		if _, err := tx.Exec(
			`INSERT INTO obsidian_links (source_id, target_path, link_type, resolved) VALUES (?, ?, ?, ?)`,
			noteID, l.target, l.linkType, l.resolved,
		); err != nil {
			return err
		}
	}

	for k, v := range frontmatter {
		valueStr := frontmatterValueToString(v)
		if _, err := tx.Exec(
			`INSERT INTO frontmatter_kv (note_id, key, value) VALUES (?, ?, ?)`,
			noteID, k, valueStr,
		); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	return nil
}

// splitFrontmatter returns (yamlBody, restOfDocument). If no frontmatter
// prefix is present, the first return value is empty.
func splitFrontmatter(content string) (string, string) {
	m := frontmatterRE.FindStringSubmatchIndex(content)
	if m == nil {
		return "", content
	}
	return content[m[2]:m[3]], content[m[1]:]
}

func deriveTitle(frontmatter map[string]any, relPath string) string {
	for _, k := range frontmatterTitleKeys {
		if v, ok := frontmatter[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	for _, k := range frontmatterAliasKeys {
		if v, ok := frontmatter[k]; ok {
			switch x := v.(type) {
			case string:
				if strings.TrimSpace(x) != "" {
					return x
				}
			case []any:
				if len(x) > 0 {
					if s, ok := x[0].(string); ok && strings.TrimSpace(s) != "" {
						return s
					}
				}
			}
		}
	}
	base := filepath.Base(relPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

type extractedLink struct {
	target   string
	linkType string
	resolved int
}

func extractLinks(body string) []extractedLink {
	var out []extractedLink
	seen := map[string]bool{}
	for _, m := range embedRE.FindAllStringSubmatch(body, -1) {
		t := strings.TrimSpace(m[1])
		k := "embed|" + t
		if t != "" && !seen[k] {
			out = append(out, extractedLink{target: t, linkType: "embed", resolved: 0})
			seen[k] = true
		}
	}
	for _, m := range wikilinkRE.FindAllStringSubmatch(body, -1) {
		t := strings.TrimSpace(m[1])
		k := "wikilink|" + t
		// Skip wikilinks that were already captured as embeds (the regex
		// alternatives overlap because every embed is also a wikilink).
		// embedRE consumes the leading `!` so check with the embed key.
		if t == "" || seen["embed|"+t] || seen[k] {
			continue
		}
		out = append(out, extractedLink{target: t, linkType: "wikilink", resolved: 0})
		seen[k] = true
	}
	for _, m := range externalLinkRE.FindAllStringSubmatch(body, -1) {
		u := strings.TrimSpace(m[1])
		k := "external|" + u
		if u != "" && !seen[k] {
			out = append(out, extractedLink{target: u, linkType: "external", resolved: 1})
			seen[k] = true
		}
	}
	return out
}

func extractTags(body string, frontmatter map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	add := func(t string) {
		t = strings.TrimPrefix(strings.TrimSpace(t), "#")
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
	}
	for _, m := range inlineTagRE.FindAllStringSubmatch(body, -1) {
		add(m[1])
	}
	for _, k := range frontmatterTagKeys {
		if v, ok := frontmatter[k]; ok {
			switch x := v.(type) {
			case string:
				for _, t := range strings.Fields(strings.ReplaceAll(x, ",", " ")) {
					add(t)
				}
			case []any:
				for _, item := range x {
					if s, ok := item.(string); ok {
						add(s)
					}
				}
			}
		}
	}
	return out
}

func frontmatterValueToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case int, int64, float64, bool:
		return fmt.Sprintf("%v", x)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}

// readLastSync returns the recorded last_sync_at timestamp (UTC), if any.
func readLastSync(db *sql.DB) (time.Time, bool) {
	row := db.QueryRow(`SELECT last_sync_at FROM vault_sync_state WHERE id = 1`)
	var s string
	if err := row.Scan(&s); err != nil {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// pruneDeletedNotes removes rows from `notes` whose `path` is not in
// the set of paths the sync walker just processed. Vault deletions
// would otherwise leave ghost notes that every Tier-3 command silently
// includes in its calculations.
//
// Callers must guarantee `keep` reflects the COMPLETE current vault
// (no --max-files cap, no --folder scope, no errored Stat). The
// schema's ON DELETE CASCADE on `obsidian_tags`, `obsidian_links`, and
// `frontmatter_kv` handles the child rows.
//
// Returns the number of notes deleted.
func pruneDeletedNotes(db *store.Store, keep map[string]struct{}) (int, error) {
	dbi := db.DB()
	rows, err := dbi.Query(`SELECT path FROM notes`)
	if err != nil {
		return 0, fmt.Errorf("listing mirror notes: %w", err)
	}
	defer rows.Close()
	var stale []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return 0, err
		}
		if _, ok := keep[p]; !ok {
			stale = append(stale, p)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(stale) == 0 {
		return 0, nil
	}
	tx, err := dbi.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, p := range stale {
		if _, err := tx.Exec(`DELETE FROM notes WHERE path = ?`, p); err != nil {
			return 0, fmt.Errorf("deleting %s: %w", p, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit prune tx: %w", err)
	}
	committed = true
	return len(stale), nil
}
