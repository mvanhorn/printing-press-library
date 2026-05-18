// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Bulk export to Obsidian, plain Markdown, or JSONL",
		Long: "Exports recordings + transcripts + summaries from the local store.\n" +
			"Run `plaud-pp-cli sync && plaud-pp-cli sync-transcripts --all` first to\n" +
			"populate the store. Each subcommand writes one file per recording in\n" +
			"the chosen shape.",
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newExportObsidianCmd(flags))
	cmd.AddCommand(newExportMarkdownCmd(flags))
	cmd.AddCommand(newExportJSONLCmd(flags))
	return cmd
}

func newExportObsidianCmd(flags *rootFlags) *cobra.Command {
	var flagOut, flagSince string
	cmd := &cobra.Command{
		Use:   "obsidian",
		Short: "Export recordings to an Obsidian vault directory (YAML frontmatter + body)",
		Example: `  plaud-pp-cli export obsidian --out ~/Documents/ObsidianVault/Plaud
  plaud-pp-cli export obsidian --out ./vault --since 90d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagOut == "" {
				return usageErr(fmt.Errorf("--out <directory> is required"))
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}
			s, err := openPlaudStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := os.MkdirAll(flagOut, 0o755); err != nil {
				return apiErr(fmt.Errorf("creating output dir: %w", err))
			}
			recs, err := loadExportRows(cmd.Context(), s, since)
			if err != nil {
				return apiErr(err)
			}
			count := 0
			for _, r := range recs {
				body := obsidianFrontmatter(r) + "\n\n" + r.markdownBody()
				fname := filepath.Join(flagOut, sanitizeFilename(r.Filename)+".md")
				if err := os.WriteFile(fname, []byte(body), 0o644); err != nil {
					return apiErr(fmt.Errorf("writing %s: %w", fname, err))
				}
				count++
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"exported": count, "out_dir": flagOut, "format": "obsidian",
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagOut, "out", "", "Output directory (will be created)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only recordings within this window (e.g. 90d)")
	return cmd
}

func newExportMarkdownCmd(flags *rootFlags) *cobra.Command {
	var flagOut, flagSince string
	cmd := &cobra.Command{
		Use:   "markdown",
		Short: "Export recordings as plain Markdown files (no YAML frontmatter)",
		Example: `  plaud-pp-cli export markdown --out ./md
  plaud-pp-cli export markdown --out ./md --since 30d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagOut == "" {
				return usageErr(fmt.Errorf("--out <directory> is required"))
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}
			s, err := openPlaudStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := os.MkdirAll(flagOut, 0o755); err != nil {
				return apiErr(fmt.Errorf("creating output dir: %w", err))
			}
			recs, err := loadExportRows(cmd.Context(), s, since)
			if err != nil {
				return apiErr(err)
			}
			count := 0
			for _, r := range recs {
				body := "# " + r.Filename + "\n\n" + r.markdownBody()
				fname := filepath.Join(flagOut, sanitizeFilename(r.Filename)+".md")
				if err := os.WriteFile(fname, []byte(body), 0o644); err != nil {
					return apiErr(fmt.Errorf("writing %s: %w", fname, err))
				}
				count++
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"exported": count, "out_dir": flagOut, "format": "markdown",
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagOut, "out", "", "Output directory")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only recordings within this window")
	return cmd
}

func newExportJSONLCmd(flags *rootFlags) *cobra.Command {
	var flagOut, flagSince string
	cmd := &cobra.Command{
		Use:   "jsonl",
		Short: "Export recordings as a single JSONL file (one recording per line)",
		Example: `  plaud-pp-cli export jsonl --out ./plaud.jsonl
  plaud-pp-cli export jsonl --out - --since 30d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagOut == "" {
				return usageErr(fmt.Errorf("--out <path> is required (use - for stdout)"))
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}
			s, err := openPlaudStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			recs, err := loadExportRows(cmd.Context(), s, since)
			if err != nil {
				return apiErr(err)
			}
			var w *os.File
			if flagOut == "-" {
				w = os.Stdout
			} else {
				if err := os.MkdirAll(filepath.Dir(flagOut), 0o755); err != nil {
					return apiErr(fmt.Errorf("creating output dir: %w", err))
				}
				f, ferr := os.Create(flagOut)
				if ferr != nil {
					return apiErr(fmt.Errorf("creating file: %w", ferr))
				}
				defer f.Close()
				w = f
			}
			enc := json.NewEncoder(w)
			count := 0
			for _, r := range recs {
				if err := enc.Encode(r); err != nil {
					return apiErr(fmt.Errorf("encoding row: %w", err))
				}
				count++
			}
			if flagOut != "-" {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"exported": count, "out_path": flagOut, "format": "jsonl",
				}, flags)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOut, "out", "", "Output path (use - for stdout)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only recordings within this window")
	return cmd
}

type exportRow struct {
	ID                 string           `json:"id"`
	Filename           string           `json:"filename"`
	StartTime          int64            `json:"start_time"`
	Duration           int64            `json:"duration"`
	Scene              string           `json:"scene"`
	Transcripts        []map[string]any `json:"transcripts"`
	SummaryMarkdown    string           `json:"summary_markdown"`
	SummaryDecisions   []string         `json:"summary_decisions,omitempty"`
	SummaryActionItems []string         `json:"summary_action_items,omitempty"`
	SummaryTopics      []string         `json:"summary_topics,omitempty"`
}

func (r exportRow) markdownBody() string {
	var b strings.Builder
	if r.SummaryMarkdown != "" {
		b.WriteString("## Summary\n\n")
		b.WriteString(r.SummaryMarkdown)
		b.WriteString("\n\n")
	}
	if len(r.Transcripts) > 0 {
		b.WriteString("## Transcript\n\n")
		for _, seg := range r.Transcripts {
			speaker, _ := seg["speaker"].(string)
			content, _ := seg["content"].(string)
			if speaker != "" {
				b.WriteString("**")
				b.WriteString(speaker)
				b.WriteString(":** ")
			}
			b.WriteString(content)
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func obsidianFrontmatter(r exportRow) string {
	return fmt.Sprintf("---\nfile_id: %s\nfilename: %q\nstart_time: %d\nduration: %d\nscene: %q\n---", r.ID, r.Filename, r.StartTime, r.Duration, r.Scene)
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "-", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	cleaned := replacer.Replace(strings.TrimSpace(name))
	if cleaned == "" {
		return "untitled"
	}
	if len(cleaned) > 200 {
		cleaned = cleaned[:200]
	}
	return cleaned
}

// loadExportRows pulls recordings + transcripts + summaries from the typed
// tables and joins them in-memory into exportRow shape.
func loadExportRows(ctx context.Context, s *store.Store, since int64) ([]exportRow, error) {
	q := `
		SELECT r.id, r.filename, r.start_time, r.duration, r.scene,
		       COALESCE(sum.markdown, '') AS summary_markdown,
		       COALESCE(sum.decisions, '[]') AS decisions,
		       COALESCE(sum.action_items, '[]') AS action_items,
		       COALESCE(sum.topics, '[]') AS topics
		FROM recordings_typed r
		LEFT JOIN summaries sum ON sum.recording_id = r.id
		WHERE r.is_trash = 0
	`
	args := []any{}
	if since > 0 {
		q += " AND r.start_time >= ?"
		args = append(args, since)
	}
	q += " ORDER BY r.start_time DESC"

	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("loading recordings: %w", err)
	}
	defer rows.Close()

	out := []exportRow{}
	for rows.Next() {
		var r exportRow
		var decisions, actionItems, topics string
		if err := rows.Scan(&r.ID, &r.Filename, &r.StartTime, &r.Duration, &r.Scene, &r.SummaryMarkdown, &decisions, &actionItems, &topics); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(decisions), &r.SummaryDecisions)
		_ = json.Unmarshal([]byte(actionItems), &r.SummaryActionItems)
		_ = json.Unmarshal([]byte(topics), &r.SummaryTopics)
		r.Transcripts = loadTranscriptSegments(ctx, s.DB(), r.ID)
		out = append(out, r)
	}
	return out, rows.Err()
}

func loadTranscriptSegments(ctx context.Context, db *sql.DB, recordingID string) []map[string]any {
	rows, err := db.QueryContext(ctx, `
		SELECT idx, start_time, end_time, speaker, content
		FROM transcripts
		WHERE recording_id = ?
		ORDER BY idx ASC
	`, recordingID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var idx int
		var start, end float64
		var speaker, content sql.NullString
		if err := rows.Scan(&idx, &start, &end, &speaker, &content); err != nil {
			return out
		}
		out = append(out, map[string]any{
			"idx":        idx,
			"start_time": start,
			"end_time":   end,
			"speaker":    speaker.String,
			"content":    content.String,
		})
	}
	return out
}
