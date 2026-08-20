// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newJournalCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Read and write the art-goat reflection journal",
		Long: `The journal is the durable record of your practice. art-goat stores it
as SQLite at ~/.local/share/art-goat-pp-cli/data.db (or wherever --db
points). Subcommands:

  journal write    — write a standalone reflection (without a sit)
  journal stats    — reframed practice metrics (breadth, variety, region)
  journal search   — FTS5 over your reflection history
  journal revisit  — surface the sit closest to a past anchor (e.g. 1y)
  journal compare  — render two past sits side-by-side
  journal export   — mirror your sits to Markdown files (one per sit)
  journal opt-in   — opt into per-sit streak greeting`,
	}
	cmd.AddCommand(newJournalWriteCmd(flags))
	cmd.AddCommand(newJournalStatsCmd(flags))
	cmd.AddCommand(newJournalSearchCmd(flags))
	cmd.AddCommand(newJournalRevisitCmd(flags))
	cmd.AddCommand(newJournalCompareCmd(flags))
	cmd.AddCommand(newJournalExportCmd(flags))
	cmd.AddCommand(newJournalOptInCmd(flags))
	cmd.AddCommand(newJournalCompactCmd(flags))
	return cmd
}

func newJournalWriteCmd(flags *rootFlags) *cobra.Command {
	var workID string
	var reflectionFlag string
	var mood int
	var tags string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "write",
		Short: "Write a reflection entry to the journal (without a sit)",
		Long: `Capture a free-form reflection in the journal without going through the
sit flow. Useful when you want to note something later — a recurring
image you keep thinking about, a connection between pieces, a question.

Pass --reflection to skip the interactive prompt entirely (agent-friendly).
Without --reflection, art-goat reads one line from stdin.`,
		Example: `  # Interactive
  art-goat-pp-cli journal write

  # Non-interactive
  art-goat-pp-cli journal write --reflection "kept thinking about waves all week"

  # With a work id and mood
  art-goat-pp-cli journal write --work-id aic:24645 --mood 4 --tags water,repetition`,
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return emitJournalWriteVerifyEnvelope(cmd, flags)
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

			reflection := strings.TrimSpace(reflectionFlag)
			if reflection == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Reflection (single line):")
				fmt.Fprint(cmd.OutOrStdout(), "> ")
				in := bufio.NewReader(os.Stdin)
				line, _ := in.ReadString('\n')
				reflection = strings.TrimSpace(line)
			}
			if reflection == "" {
				return fmt.Errorf("reflection is required (pass --reflection or type one in)")
			}

			sit := store.Sit{
				StartedAt:       time.Now().UTC(),
				EndedAt:         sql.NullTime{Time: time.Now().UTC(), Valid: true},
				WorkID:          workID,
				DurationSeconds: 0,
				Prompt:          "",
				Reflection:      reflection,
				Tags:            tags,
				Mode:            "bare",
			}
			if mood > 0 && mood <= 5 {
				sit.Mood = sql.NullInt64{Int64: int64(mood), Valid: true}
			}
			id, err := db.InsertSit(cmd.Context(), sit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"id":         id,
					"work_id":    workID,
					"reflection": reflection,
					"mood":       mood,
					"tags":       tags,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved as journal entry #%d.\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&workID, "work-id", "", "Associate this reflection with a work (e.g. aic:24645)")
	cmd.Flags().StringVar(&reflectionFlag, "reflection", "", "Reflection text (skips interactive prompt)")
	cmd.Flags().IntVar(&mood, "mood", 0, "Optional mood rating 1-5")
	cmd.Flags().StringVar(&tags, "tags", "", "Optional comma-separated tags")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newJournalStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Reframed practice metrics: breadth, variety, region, mood drift",
		Long: `Show your practice stats. Headlines are breadth (sources visited), variety
(mediums, periods, regions), and mood drift. Streak appears at the bottom
labeled 'if you want to know' — deliberately not the headline metric.

Pass --json for structured output.`,
		Example: `  art-goat-pp-cli journal stats
  art-goat-pp-cli journal stats --json --select total_sits,by_source`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			stats, err := db.CollectJournalStats(cmd.Context())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), statsToEnvelope(stats), flags)
			}
			renderJournalStats(cmd, stats)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newJournalSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "FTS5 search across your reflection history",
		Long: `Search your reflections by token. Empty query lists the most recent sits.
Uses SQLite FTS5; supports phrase queries (\"water cycle\"), prefix
(water*), and column filters (reflection:water).`,
		Example: `  art-goat-pp-cli journal search "solitude"
  art-goat-pp-cli journal search --limit 5
  art-goat-pp-cli journal search "tags:stillness" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = strings.Join(args, " ")
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
			sits, err := db.SearchSits(cmd.Context(), query, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out := make([]map[string]any, 0, len(sits))
				for _, sit := range sits {
					out = append(out, sitToEnvelope(sit))
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"query": query, "results": out, "count": len(out)}, flags)
			}
			renderJournalSearch(cmd, query, sits)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

func renderJournalStats(cmd *cobra.Command, stats *store.JournalStats) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Practice")
	if stats.TotalSits == 0 {
		fmt.Fprintln(out, "  No sits yet — try `art-goat-pp-cli sit` to start.")
		return
	}
	fmt.Fprintf(out, "  %d sits · %s of attention\n", stats.TotalSits, formatDuration(stats.TotalSeconds))
	if stats.LastSitAt.Valid && !stats.LastSitAt.Time.IsZero() {
		fmt.Fprintf(out, "  Last sit: %s\n", humanAgo(stats.LastSitAt.Time))
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Breadth")
	fmt.Fprintf(out, "  %d source%s visited\n", len(stats.BySource), pluralS(len(stats.BySource)))
	for src, n := range stats.BySource {
		fmt.Fprintf(out, "    %s: %d\n", src, n)
	}

	if len(stats.ByMedium) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Variety")
		fmt.Fprintf(out, "  Mediums: %d\n", len(stats.ByMedium))
		for medium, n := range topMap(stats.ByMedium, 5) {
			fmt.Fprintf(out, "    %s: %d\n", medium, n)
		}
	}

	if len(stats.ByRegion) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Regions visited")
		for region, n := range topMap(stats.ByRegion, 8) {
			fmt.Fprintf(out, "  %s: %d\n", region, n)
		}
	}

	if len(stats.ByPeriodCentury) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Centuries visited")
		for century, n := range stats.ByPeriodCentury {
			fmt.Fprintf(out, "  %s: %d\n", century, n)
		}
	}

	if stats.AvgMoodOverall > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Mood drift")
		fmt.Fprintf(out, "  Average mood: %.1f/5\n", stats.AvgMoodOverall)
		for src, mood := range stats.MoodBySource {
			fmt.Fprintf(out, "    %s: %.1f\n", src, mood)
		}
	}

	if len(stats.TopTags) > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Recent tags")
		for _, tc := range stats.TopTags {
			fmt.Fprintf(out, "  %s: %d\n", tc.Tag, tc.Count)
		}
	}

	if stats.CurrentStreak > 0 {
		fmt.Fprintln(out, "")
		fmt.Fprintf(out, "If you want to know: %d day%s consecutive.\n", stats.CurrentStreak, pluralS(stats.CurrentStreak))
	}
	fmt.Fprintln(out, "")
}

func renderJournalSearch(cmd *cobra.Command, query string, sits []store.Sit) {
	out := cmd.OutOrStdout()
	if len(sits) == 0 {
		if query != "" {
			fmt.Fprintf(out, "No reflections matched %q.\n", query)
		} else {
			fmt.Fprintln(out, "Journal is empty.")
		}
		return
	}
	for _, sit := range sits {
		when := sit.StartedAt.Format("2006-01-02")
		fmt.Fprintf(out, "%s  #%d  %s\n", when, sit.ID, sit.WorkID)
		if sit.Prompt != "" {
			fmt.Fprintf(out, "  Prompt: %s\n", sit.Prompt)
		}
		if sit.Reflection != "" {
			fmt.Fprintf(out, "  %s\n", sit.Reflection)
		}
		if sit.Tags != "" {
			fmt.Fprintf(out, "  Tags: %s\n", sit.Tags)
		}
		fmt.Fprintln(out, "")
	}
}

func statsToEnvelope(stats *store.JournalStats) map[string]any {
	envelope := map[string]any{
		"total_sits":        stats.TotalSits,
		"total_seconds":     stats.TotalSeconds,
		"by_source":         stats.BySource,
		"by_medium":         stats.ByMedium,
		"by_region":         stats.ByRegion,
		"by_period_century": stats.ByPeriodCentury,
		"avg_mood_overall":  stats.AvgMoodOverall,
		"mood_by_source":    stats.MoodBySource,
		"top_tags":          stats.TopTags,
		"current_streak":    stats.CurrentStreak,
	}
	if stats.LastSitAt.Valid {
		envelope["last_sit_at"] = stats.LastSitAt.Time.Format(time.RFC3339)
	}
	return envelope
}

func sitToEnvelope(sit store.Sit) map[string]any {
	out := map[string]any{
		"id":               sit.ID,
		"started_at":       sit.StartedAt.Format(time.RFC3339),
		"work_id":          sit.WorkID,
		"duration_seconds": sit.DurationSeconds,
		"prompt":           sit.Prompt,
		"reflection":       sit.Reflection,
		"tags":             sit.Tags,
		"mode":             sit.Mode,
	}
	if sit.Mood.Valid {
		out["mood"] = sit.Mood.Int64
	}
	if sit.EndedAt.Valid {
		out["ended_at"] = sit.EndedAt.Time.Format(time.RFC3339)
	}
	return out
}

func emitJournalWriteVerifyEnvelope(cmd *cobra.Command, flags *rootFlags) error {
	envelope := map[string]any{
		"command":                 "journal write",
		"verify_noop":             true,
		"success":                 false,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "journal write reads stdin; PRINTING_PRESS_VERIFY=1 short-circuits before reading or writing.",
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}

func formatDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	if d >= time.Hour {
		hours := int(d.Hours())
		mins := int((d - time.Duration(hours)*time.Hour).Minutes())
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

func humanAgo(t time.Time) string {
	delta := time.Since(t)
	switch {
	case delta < 24*time.Hour:
		return "today"
	case delta < 48*time.Hour:
		return "yesterday"
	case delta < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(delta.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func topMap(m map[string]int, n int) map[string]int {
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].v > all[j-1].v; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	if len(all) > n {
		all = all[:n]
	}
	out := make(map[string]int, len(all))
	for _, kv := range all {
		out[kv.k] = kv.v
	}
	return out
}

// avoid unused import if context isn't used elsewhere in this file
var _ = context.Background
