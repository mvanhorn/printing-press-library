// Copyright 2026 justinwfu. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

// newJournalExportCmd builds the `journal export` subcommand: a one-way
// mirror from the canonical sits table to per-sit Markdown files. SQLite
// remains the source of truth; the Markdown tree is read-friendly and
// version-control-friendly, but never read back in.
//
// Hidden from MCP because writing files to a user-controlled filesystem
// path is a side effect that an agent should not initiate without an
// explicit human request — see AGENTS.md "Side-effect commands".
func newJournalExportCmd(flags *rootFlags) *cobra.Command {
	var (
		exportPath string
		sinceStr   string
		dryRun     bool
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Mirror your sit journal to Markdown files (one per sit)",
		Long: `Mirror the sits journal table to a tree of per-sit Markdown files.

SQLite stays canonical; the Markdown copy is for human reading and for
checking your reflection history into version control. Each sit is
written as <YYYY-MM-DD>-<work-id-slug>.md with YAML frontmatter and a
short body. Re-running overwrites existing files (idempotent).

Default target is $ART_GOAT_JOURNAL_PATH, falling back to ~/.art-goat/journal.`,
		Example: `  # Mirror to the default location
  art-goat-pp-cli journal export

  # Mirror to a chosen directory, only sits from 2026 forward
  art-goat-pp-cli journal export --path ./journal --since 2026-01-01

  # See what would be written without touching disk
  art-goat-pp-cli journal export --dry-run`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return emitJournalExportVerifyEnvelope(cmd, flags)
			}

			targetDir, err := resolveJournalExportDir(exportPath)
			if err != nil {
				return err
			}
			var since time.Time
			if s := strings.TrimSpace(sinceStr); s != "" {
				parsed, err := time.Parse("2006-01-02", s)
				if err != nil {
					return fmt.Errorf("invalid --since (want YYYY-MM-DD): %w", err)
				}
				since = parsed
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			sits, err := db.AllSits(cmd.Context(), since)
			if err != nil {
				return err
			}

			if !dryRun {
				if err := os.MkdirAll(targetDir, 0o755); err != nil {
					return fmt.Errorf("creating journal dir: %w", err)
				}
			}

			written := 0
			for _, sit := range sits {
				body := renderSitMarkdown(cmd.Context(), db, sit)
				filename := sitMarkdownFilename(sit)
				full := filepath.Join(targetDir, filename)
				if dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "would write: %s\n", full)
					written++
					continue
				}
				if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", full, err)
				}
				written++
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"written": written,
					"path":    targetDir,
					"dry_run": dryRun,
				}, flags)
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would write %d file%s to %s.\n", written, pluralS(written), targetDir)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d file%s to %s.\n", written, pluralS(written), targetDir)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&exportPath, "path", "", "Target directory (default: $ART_GOAT_JOURNAL_PATH or ~/.art-goat/journal)")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Only export sits started on or after this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print what would be written without writing")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// resolveJournalExportDir picks the target directory in priority order:
// explicit --path > $ART_GOAT_JOURNAL_PATH > ~/.art-goat/journal.
func resolveJournalExportDir(explicit string) (string, error) {
	if p := strings.TrimSpace(explicit); p != "" {
		return p, nil
	}
	if p := strings.TrimSpace(os.Getenv("ART_GOAT_JOURNAL_PATH")); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".art-goat", "journal"), nil
}

// sitMarkdownFilename builds the per-sit filename. work_id often looks
// like "met:12345" or "aic:24645"; the colon is invalid on some
// filesystems (notably macOS Finder presentation and any FAT-derived
// fs), so it is replaced with a dash to keep the slug portable.
func sitMarkdownFilename(sit store.Sit) string {
	date := sit.StartedAt.UTC().Format("2006-01-02")
	slug := sanitizeWorkIDForFilename(sit.WorkID)
	if slug == "" {
		slug = fmt.Sprintf("sit-%d", sit.ID)
	}
	return fmt.Sprintf("%s-%s.md", date, slug)
}

func sanitizeWorkIDForFilename(workID string) string {
	if workID == "" {
		return ""
	}
	// Replace filesystem-hostile characters with '-'. Colon is the
	// common one (source:id work-id shape); the rest are belt and
	// suspenders for paths that ride through Windows shares or
	// archive tools.
	replacer := strings.NewReplacer(
		":", "-",
		"/", "-",
		"\\", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(workID)
}

// renderSitMarkdown builds the Markdown body for one sit. Work-title
// lookup is best-effort: if the row is missing (sync drift, manually
// inserted sit), fall back to the work_id as the title.
func renderSitMarkdown(ctx context.Context, db *store.Store, sit store.Sit) string {
	title := sit.WorkID
	if sit.WorkID != "" {
		if w, err := db.GetWork(ctx, sit.WorkID); err == nil && w != nil && w.Title != "" {
			title = w.Title
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "sit_id: %d\n", sit.ID)
	fmt.Fprintf(&b, "started_at: %s\n", sit.StartedAt.UTC().Format(time.RFC3339))
	if sit.EndedAt.Valid {
		fmt.Fprintf(&b, "ended_at: %s\n", sit.EndedAt.Time.UTC().Format(time.RFC3339))
	} else {
		b.WriteString("ended_at: \n")
	}
	fmt.Fprintf(&b, "duration_seconds: %d\n", sit.DurationSeconds)
	fmt.Fprintf(&b, "work_id: %s\n", sit.WorkID)
	if sit.Mood.Valid {
		fmt.Fprintf(&b, "mood: %d\n", sit.Mood.Int64)
	} else {
		b.WriteString("mood: \n")
	}
	fmt.Fprintf(&b, "tags: %s\n", yamlTagsList(sit.Tags))
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# Sit on %s\n\n", coalesce(title, "(untitled)"))
	if sit.Prompt != "" {
		fmt.Fprintf(&b, "**Prompt**: %s\n\n", sit.Prompt)
	}
	b.WriteString("## Reflection\n\n")
	if strings.TrimSpace(sit.Reflection) != "" {
		b.WriteString(sit.Reflection)
		if !strings.HasSuffix(sit.Reflection, "\n") {
			b.WriteString("\n")
		}
	} else {
		b.WriteString("_(no reflection recorded)_\n")
	}
	return b.String()
}

// yamlTagsList renders a comma-separated sit.Tags string as a YAML
// inline list (flow form) with each element quoted via %q. Quoting
// every element keeps colons, spaces, and other YAML-special chars
// from corrupting frontmatter parsers (Obsidian / static-site
// generators / yaml-go) regardless of what the user typed.
//
// `oil, repetition`   -> `["oil", "repetition"]`
// `morning: practice` -> `["morning: practice"]`
// empty / whitespace  -> `[]`
func yamlTagsList(raw string) string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%q", t))
	}
	if len(out) == 0 {
		return "[]"
	}
	return "[" + strings.Join(out, ", ") + "]"
}

func emitJournalExportVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal export",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "journal export writes files to the user's filesystem; PRINTING_PRESS_VERIFY=1 short-circuits before any file is created.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
