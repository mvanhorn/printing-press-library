// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: raw cited evidence from the local store (hand-authored).
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/internal/cliutil"
)

type evidenceItem struct {
	Source      string          `json:"source"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	URL         string          `json:"url,omitempty"`
	Author      string          `json:"author,omitempty"`
	Points      int             `json:"points,omitempty"`
	Comments    int             `json:"comments,omitempty"`
	PublishedAt string          `json:"published_at,omitempty"`
	Excerpt     string          `json:"excerpt,omitempty"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}

func newNovelEvidenceCmd(flags *rootFlags) *cobra.Command {
	var flagSource string
	var flagLimit int
	var flagRaw bool

	cmd := &cobra.Command{
		Use:   "evidence [topic]",
		Short: "List the raw, cited items backing a topic from the local store",
		Long: strings.Trim(`
List the citable evidence (post URL, author, timestamp, points, comments) backing
a topic. Reads from the local snapshot; if nothing is stored for the topic yet, it
syncs the sources live and caches the result, so no prior 'report' is required.
Use --raw to include each item's verbatim source payload, or --no-cache to force a
fresh sync.`, "\n"),
		Example: strings.Trim(`
  vibe-signal-pp-cli evidence "AI browser agents" --source hackernews --limit 20
  vibe-signal-pp-cli evidence "local-first software" --json --raw`, "\n"),
		// A topic is free-form text: any string is a valid query, so an
		// unmatched topic returns an empty result with exit 0, not a usage error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list stored evidence for a topic")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic is required, e.g. evidence \"AI agents\""))
			}
			topic := strings.TrimSpace(strings.Join(args, " "))
			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}

			dbPath := defaultDBPath("vibe-signal-pp-cli")
			db, err := openSignalStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			rows, err := db.QuerySignals(ctx, topic, flagSource, limit)
			if err != nil {
				return err
			}

			// Fetch-on-miss: if nothing is stored for this topic yet, sync the
			// sources live and persist a snapshot, so evidence works without a
			// prior 'report'. --no-cache forces a fresh sync even if rows exist.
			if len(rows) == 0 || flags.noCache {
				if _, _, _, serr := liveSyncAndStore(ctx, db, topic, flagSource, time.Time{}, 0, limit); serr != nil {
					var rle *cliutil.RateLimitError
					if errors.As(serr, &rle) {
						return serr // never mask throttling as "no data"
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live sync failed: %v\n", serr)
				}
				rows, err = db.QuerySignals(ctx, topic, flagSource, limit)
				if err != nil {
					return err
				}
			}

			items := make([]evidenceItem, 0, len(rows))
			for _, r := range rows {
				published := ""
				if !r.PublishedAt.IsZero() {
					published = r.PublishedAt.Format(time.RFC3339)
				}
				it := evidenceItem{
					Source: r.Source, ID: r.SourceID, Title: r.Title, URL: r.URL,
					Author: r.Author, Points: r.Points, Comments: r.Comments,
					PublishedAt: published, Excerpt: r.Excerpt,
				}
				if flagRaw && r.RawJSON != "" {
					it.Raw = json.RawMessage(r.RawJSON)
				}
				items = append(items, it)
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			}
			out := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintf(out, "No evidence found for %q across the wired sources.\n", topic)
				return nil
			}
			for _, it := range items {
				meta := it.Source
				if it.Points > 0 || it.Comments > 0 {
					meta += fmt.Sprintf(" · %d pts · %d comments", it.Points, it.Comments)
				}
				if it.Author != "" {
					meta += " · " + it.Author
				}
				if it.PublishedAt != "" {
					meta += " · " + it.PublishedAt
				}
				fmt.Fprintf(out, "• %s\n  %s [%s]\n", it.Title, it.URL, meta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSource, "source", "", "Restrict to one source (e.g. hackernews, techmeme)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max items to return")
	cmd.Flags().BoolVar(&flagRaw, "raw", false, "Include each item's verbatim source payload")
	return cmd
}
