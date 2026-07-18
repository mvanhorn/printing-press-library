// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: a time-windowed digest across every publication in the local
// corpus. Hand-implemented; local-only (no single Substack endpoint aggregates
// across publications). generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/cliutil"
)

// pp:data-source local
func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "A time-windowed digest across every publication in your local corpus — what's new since you last synced",
		Long: `Summarize what's new across every publication in your local corpus within a
time window, grouped by publication and ordered by recency.

Reads only the local corpus built by 'substack-reader-pp-cli archive' — it never hits
the network — so it is a "what did I miss across my newsletters" briefing over
exactly what you have mirrored. Posts without a parseable date are omitted from
the window.`,
		Example:     "  substack-reader-pp-cli digest --since 7d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize recent posts across the local corpus")
				return nil
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			if window < 0 {
				window = -window
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			posts, err := loadCorpusPosts(ctx)
			if err != nil {
				return err
			}

			cutoff := time.Now().Add(-window)
			inWindow := make([]corpusPost, 0, len(posts))
			for _, p := range posts {
				if p.HasDate && !p.Parsed.Before(cutoff) {
					inWindow = append(inWindow, p)
				}
			}
			sort.SliceStable(inWindow, func(i, j int) bool {
				return inWindow[i].Parsed.After(inWindow[j].Parsed)
			})
			if limit > 0 && len(inWindow) > limit {
				inWindow = inWindow[:limit]
			}

			// Group by publication, preserving recency order (a host's first
			// appearance is its most-recent post because inWindow is sorted).
			idx := map[string]int{}
			type group struct {
				Host  string
				Posts []corpusPost
			}
			var groups []*group
			for _, p := range inWindow {
				i, ok := idx[p.Host]
				if !ok {
					i = len(groups)
					idx[p.Host] = i
					groups = append(groups, &group{Host: p.Host})
				}
				groups[i].Posts = append(groups[i].Posts, p)
			}

			// Machine output.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				pubs := make([]map[string]any, 0, len(groups))
				for _, g := range groups {
					items := make([]map[string]any, 0, len(g.Posts))
					for _, p := range g.Posts {
						items = append(items, map[string]any{
							"date":     p.Parsed.Format("2006-01-02"),
							"title":    p.Title,
							"audience": p.Audience,
							"slug":     p.Slug,
						})
					}
					pubs = append(pubs, map[string]any{
						"host":  g.Host,
						"count": len(g.Posts),
						"posts": items,
					})
				}
				envelope := map[string]any{
					"since":        since,
					"window":       window.String(),
					"cutoff":       cutoff.UTC().Format(time.RFC3339),
					"count":        len(inWindow),
					"publications": pubs,
				}
				data, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			// Human output.
			w := cmd.OutOrStdout()
			if len(inWindow) == 0 {
				fmt.Fprintf(w, "No posts in the last %s across the local corpus. Archive more publications or widen --since.\n", since)
				return nil
			}
			fmt.Fprintf(w, "Digest — since %s: %d posts across %d publications\n", since, len(inWindow), len(groups))
			for _, g := range groups {
				fmt.Fprintf(w, "\n%s (%d)\n", orDash(g.Host), len(g.Posts))
				for _, p := range g.Posts {
					fmt.Fprintf(w, "  %s · %s %s\n", p.Parsed.Format("2006-01-02"), audienceTag(p), p.Title)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "time window to include (e.g. 7d, 24h, 2w)")
	cmd.Flags().IntVar(&limit, "limit", 0, "cap the total number of posts shown (0 = no cap)")
	return cmd
}

func audienceTag(p corpusPost) string {
	if p.IsPaid() {
		return "[paid]"
	}
	return "[free]"
}
