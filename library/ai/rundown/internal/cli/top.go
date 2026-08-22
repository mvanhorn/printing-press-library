// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// rdTopResult wraps the ranking so an empty result can explain itself.
//
// A bare array cannot distinguish "nothing was posted this week" from "you have
// not synced yet" — an agent would report the former when the latter is true.
type rdTopResult struct {
	Window        string     `json:"window"`
	Workflows     []rdTopRow `json:"workflows"`
	MirroredTotal int        `json:"mirroredTotal"`
	Note          string     `json:"note,omitempty"`
}

// rdTopRow is one ranked workflow in `top` output.
type rdTopRow struct {
	Rank        int      `json:"rank"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	UpvoteCount int      `json:"upvoteCount"`
	Comments    int      `json:"commentCount"`
	Author      string   `json:"author"`
	Tools       []string `json:"tools"`
	CreatedAt   string   `json:"createdAt"`
	Age         string   `json:"age"`
	Featured    bool     `json:"newsletterFeatured"`
	URL         string   `json:"url"`
}

func newNovelTopCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince  string
		flagLimit  int
		flagTool   string
		flagDBPath string
	)

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank the highest-upvoted workflows inside a time window like 7d or 30d",
		Long: strings.Trim(`
Rank mirrored community workflows by upvotes inside a time window.

The community API exposes sort=top but has no date filter of any kind, so
"best rated this week" cannot be expressed against the live endpoint. This
command windows the local mirror on createdAt and then ranks, which is why it
needs 'rundown-pp-cli sync' first.

Use this command for time-bounded rankings. For an all-time ranking straight
from the API, use 'rundown-pp-cli posts --sort top' instead.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli top --since 7d --limit 5
  rundown-pp-cli top --since 30d --tool claude-code
  rundown-pp-cli top --since 7d --agent --select title,upvoteCount,url
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--since=30d;--limit=5",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "top")
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}

			cutoff, err := rdWindowStart(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := rdResolveDBPath(flagDBPath)
			empty := make([]rdTopRow, 0)
			if stop, err := rdMirrorMissing(cmd, flags, dbPath, empty); stop {
				return err
			}

			db, err := rdOpenMirrorStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			posts, err := rdLoadPosts(ctx, db)
			if err != nil {
				return err
			}

			wantTool := strings.ToLower(strings.TrimSpace(flagTool))
			matched := make([]rdPost, 0, len(posts))
			for _, p := range posts {
				if !rdInWindow(p, cutoff) {
					continue
				}
				if wantTool != "" {
					found := false
					for _, s := range p.toolSlugs() {
						if s == wantTool {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				}
				matched = append(matched, p)
			}
			rdSortByUpvotes(matched)
			if len(matched) > flagLimit {
				matched = matched[:flagLimit]
			}

			rows := make([]rdTopRow, 0, len(matched))
			for i, p := range matched {
				rows = append(rows, rdTopRow{
					Rank:        i + 1,
					ID:          p.ID,
					Title:       p.Title,
					UpvoteCount: p.UpvoteCount,
					Comments:    p.CommentCount,
					Author:      p.authorName(),
					Tools:       p.toolSlugs(),
					CreatedAt:   p.CreatedAt,
					Age:         rdAgo(p.CreatedAt),
					Featured:    p.featured(),
					URL:         rdPostURL(p.ID),
				})
			}

			window := flagSince
			if cutoff.IsZero() {
				window = "all time"
			}

			result := rdTopResult{
				Window:        window,
				Workflows:     rows,
				MirroredTotal: len(posts),
			}
			switch {
			case len(posts) == 0:
				result.Note = "the local mirror holds no workflows, so this is not an empty week — run 'rundown-pp-cli sync' first."
			case len(rows) == 0 && wantTool != "":
				result.Note = fmt.Sprintf("no workflow in the last %s uses %q; widen --since or drop --tool.", window, wantTool)
			case len(rows) == 0:
				result.Note = fmt.Sprintf("no workflows in the last %s across %d mirrored; widen --since or re-sync.", window, len(posts))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			out := cmd.OutOrStdout()
			if len(posts) == 0 {
				fmt.Fprintf(out, "%s\n", result.Note)
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintf(out, "No workflows found in the last %s", window)
				if wantTool != "" {
					fmt.Fprintf(out, " using %s", wantTool)
				}
				fmt.Fprintf(out, ".\nScanned %d mirrored workflows. Widen --since, or run 'rundown-pp-cli sync' if the mirror is stale.\n", len(posts))
				return nil
			}

			fmt.Fprintf(out, "Top %d workflows — last %s (from %d mirrored)\n\n", len(rows), window, len(posts))
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "#\tUPVOTES\tAGE\tTITLE\tTOOLS")
			for _, r := range rows {
				tools := "-"
				if len(r.Tools) > 0 {
					tools = strings.Join(r.Tools, ", ")
					if len(r.Tools) > 3 {
						tools = strings.Join(r.Tools[:3], ", ") + fmt.Sprintf(" +%d", len(r.Tools)-3)
					}
				}
				star := ""
				if r.Featured {
					star = " *"
				}
				fmt.Fprintf(tw, "%d\t%d\t%s\t%s%s\t%s\n",
					r.Rank, r.UpvoteCount, r.Age, truncate(r.Title, 62), star, tools)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(out, "\n* featured in The Rundown newsletter\nRead one: rundown-pp-cli show %s\n", rows[0].ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "7d", "Time window to rank inside, e.g. 7d, 2w, 30d (use 'all' for no cutoff)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum workflows to return")
	cmd.Flags().StringVar(&flagTool, "tool", "", "Only rank workflows using this tool slug, e.g. claude-code")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
