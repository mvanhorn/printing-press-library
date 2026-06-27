// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-source signal report (hand-authored implementation).
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/source"
)

type reportSignal struct {
	Source      string `json:"source"`
	Title       string `json:"title"`
	URL         string `json:"url,omitempty"`
	Author      string `json:"author,omitempty"`
	Points      int    `json:"points,omitempty"`
	Comments    int    `json:"comments,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type reportView struct {
	Query       string          `json:"query"`
	Window      string          `json:"window"`
	GeneratedAt string          `json:"generated_at"`
	RunID       string          `json:"run_id"`
	Coverage    []coverageEntry `json:"coverage"`
	Keywords    []keywordCount  `json:"keywords"`
	Signals     []reportSignal  `json:"signals"`
	Note        string          `json:"note,omitempty"`
}

func newNovelReportCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagLimit int
	var flagSource string

	cmd := &cobra.Command{
		Use:   "report [topic]",
		Short: "Cross-source signal report for a topic, with per-source coverage",
		Long: strings.Trim(`
Ask one question across the wired sources and get a recency-aware signal report:
a per-source coverage table, the cited items backing the topic, and a mechanical
keyword frequency tally. Observed evidence is kept separate from any synthesis —
the keyword tally is a raw word count, not an interpreted theme.

v1 sources are Hacker News (topic-searchable) and Techmeme (headline river,
filtered locally by your query). The snapshot is written to the local store so
'evidence' can replay the cited items.`, "\n"),
		Example: strings.Trim(`
  vibe-signal-pp-cli report "AI browser agents" --window 30d
  vibe-signal-pp-cli report "local-first software" --window 14d --json
  vibe-signal-pp-cli report "Postgres" --source hackernews --limit 30`, "\n"),
		// A topic is free-form text: any string is a valid query, so there is
		// no "invalid argument" that should error — an unmatched topic returns
		// an honest empty result with exit 0.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a cross-source signal report")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic is required, e.g. report \"AI agents\""))
			}
			topic := strings.TrimSpace(strings.Join(args, " "))
			since, windowDays, err := parseWindow(flagWindow)
			if err != nil {
				return err
			}
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}
			if cliutil.IsDogfoodEnv() && limit > 5 {
				limit = 5 // curtail live fan-out to fit the dogfood timeout
			}
			sources, err := selectedSources(flagSource)
			if err != nil {
				return err
			}

			signals, coverage := syncSources(ctx, sources, source.SyncOptions{
				Query: topic, Since: since, Limit: limit,
			})

			// Persist the snapshot so `evidence` can replay it.
			runID := newRunID(topic)
			dbPath := defaultDBPath("vibe-signal-pp-cli")
			if db, derr := openSignalStore(ctx, dbPath); derr == nil {
				defer db.Close()
				coverageJSON, _ := json.Marshal(coverage)
				_ = db.RecordRun(ctx, runID, topic, windowDays, string(coverageJSON))
				_ = db.UpsertSignals(ctx, runID, signalsToRows(topic, signals))
			}

			view := buildReportView(topic, flagWindow, runID, coverage, signals)

			// Surface partial failures on stderr (never silently drop a source).
			failed := 0
			for _, c := range coverage {
				if c.Status == "failed" {
					failed++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: source %q failed: %s\n", c.Source, c.Error)
				}
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			renderReport(cmd, view, failed)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Recency window (e.g. 7d, 30d, 48h)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max items to pull per source")
	cmd.Flags().StringVar(&flagSource, "source", "", "Restrict to one source (default: all)")
	return cmd
}

func buildReportView(topic, window, runID string, coverage []coverageEntry, signals []source.Signal) reportView {
	// Order signals by points desc, then recency, for representative ranking.
	sorted := make([]source.Signal, len(signals))
	copy(sorted, signals)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Points != sorted[j].Points {
			return sorted[i].Points > sorted[j].Points
		}
		return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
	})
	rs := make([]reportSignal, 0, len(sorted))
	for _, s := range sorted {
		published := ""
		if !s.PublishedAt.IsZero() {
			published = s.PublishedAt.Format(time.RFC3339)
		}
		rs = append(rs, reportSignal{
			Source: s.Source, Title: s.Title, URL: s.URL, Author: s.Author,
			Points: s.Points, Comments: s.Comments, PublishedAt: published,
		})
	}
	view := reportView{
		Query:       topic,
		Window:      window,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		RunID:       runID,
		Coverage:    coverage,
		Keywords:    keywordTally(signals, 12),
		Signals:     rs,
	}
	if len(rs) == 0 {
		view.Note = "no signals found across sources for this topic and window; widen --window or try a broader topic"
	}
	return view
}

func renderReport(cmd *cobra.Command, view reportView, failed int) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Topic: %s\nWindow: %s\nGenerated: %s\n\n", view.Query, view.Window, view.GeneratedAt)

	fmt.Fprintln(out, "Coverage:")
	for _, c := range view.Coverage {
		line := fmt.Sprintf("  - %-12s %s, %d items", c.Source, c.Status, c.Count)
		if c.Error != "" {
			line += " (" + c.Error + ")"
		}
		fmt.Fprintln(out, line)
	}

	if len(view.Keywords) > 0 {
		fmt.Fprintln(out, "\nTop keywords (mechanical title-word frequency, not synthesis):")
		parts := make([]string, 0, len(view.Keywords))
		for _, k := range view.Keywords {
			parts = append(parts, fmt.Sprintf("%s(%d)", k.Term, k.Count))
		}
		fmt.Fprintln(out, "  "+strings.Join(parts, "  "))
	}

	fmt.Fprintln(out, "\nSignals (observed evidence):")
	if len(view.Signals) == 0 {
		fmt.Fprintln(out, "  (none)")
	}
	for _, s := range view.Signals {
		meta := s.Source
		if s.Points > 0 || s.Comments > 0 {
			meta += fmt.Sprintf(" · %d pts · %d comments", s.Points, s.Comments)
		}
		if s.Author != "" {
			meta += " · " + s.Author
		}
		fmt.Fprintf(out, "  • %s\n    %s [%s]\n", s.Title, s.URL, meta)
	}
	if view.Note != "" {
		fmt.Fprintln(out, "\n"+view.Note)
	}
	if failed > 0 {
		fmt.Fprintf(out, "\n%d source(s) failed — see warnings above; results are partial.\n", failed)
	}
}
