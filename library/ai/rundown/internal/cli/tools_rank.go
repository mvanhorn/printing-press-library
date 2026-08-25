// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/store"
	"github.com/spf13/cobra"
)

// rdToolRankRow is one tool's adoption record.
type rdToolRankRow struct {
	Rank       int     `json:"rank"`
	Slug       string  `json:"slug"`
	Workflows  int     `json:"workflows"`
	Upvotes    int     `json:"upvotes"`
	AvgUpvotes float64 `json:"avgUpvotes"`
	// Share is the percentage of in-window workflows that use this tool.
	Share float64 `json:"sharePct"`
}

type rdToolRankResult struct {
	Window    string          `json:"window"`
	Tools     []rdToolRankRow `json:"tools"`
	Workflows int             `json:"workflowsInWindow"`
	Catalogue int             `json:"catalogueSize"`
	Unused    int             `json:"catalogueToolsUnused"`
	Note      string          `json:"note,omitempty"`
}

func newNovelToolsRankCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince  string
		flagLimit  int
		flagDBPath string
	)

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Rank AI tools by how often they appear in workflows and the upvotes those workflows earned",
		Long: strings.Trim(`
Rank the tools the community actually builds with.

'tools' returns the flat catalogue the site offers, with no usage data of
any kind. This command joins every mirrored workflow against its tool tags to
produce real adoption counts, upvote totals, and each tool's share of the
workflows in the window.

Reads the local mirror, so run 'rundown-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli tools rank
  rundown-pp-cli tools rank --since 30d --limit 15
  rundown-pp-cli tools rank --agent --select slug,workflows,avgUpvotes
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--limit=10",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tools rank")
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
			if stop, err := rdMirrorMissing(cmd, flags, dbPath, rdToolRankResult{
				Window: flagSince,
				Tools:  make([]rdToolRankRow, 0),
			}); stop {
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

			type agg struct {
				workflows int
				upvotes   int
			}
			counts := map[string]*agg{}
			inWindow := 0
			for _, p := range posts {
				if !rdInWindow(p, cutoff) {
					continue
				}
				inWindow++
				// De-duplicate: a post tagging the same tool twice counts once.
				seen := map[string]bool{}
				for _, slug := range p.toolSlugs() {
					if seen[slug] {
						continue
					}
					seen[slug] = true
					if a, ok := counts[slug]; ok {
						a.workflows++
						a.upvotes += p.UpvoteCount
						continue
					}
					counts[slug] = &agg{workflows: 1, upvotes: p.UpvoteCount}
				}
			}

			rows := make([]rdToolRankRow, 0, len(counts))
			for slug, a := range counts {
				avg := 0.0
				share := 0.0
				if a.workflows > 0 {
					avg = float64(a.upvotes) / float64(a.workflows)
				}
				if inWindow > 0 {
					share = float64(a.workflows) / float64(inWindow) * 100
				}
				rows = append(rows, rdToolRankRow{
					Slug:       slug,
					Workflows:  a.workflows,
					Upvotes:    a.upvotes,
					AvgUpvotes: avg,
					Share:      share,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Workflows != rows[j].Workflows {
					return rows[i].Workflows > rows[j].Workflows
				}
				if rows[i].Upvotes != rows[j].Upvotes {
					return rows[i].Upvotes > rows[j].Upvotes
				}
				return rows[i].Slug < rows[j].Slug
			})
			if len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}
			for i := range rows {
				rows[i].Rank = i + 1
			}

			catalogue := rdCatalogueSize(ctx, db)
			label := flagSince
			if cutoff.IsZero() {
				label = "all time"
			}
			result := rdToolRankResult{
				Window:    label,
				Tools:     rows,
				Workflows: inWindow,
				Catalogue: catalogue,
			}
			if catalogue > 0 {
				result.Unused = catalogue - len(counts)
				if result.Unused < 0 {
					result.Unused = 0
				}
			}
			switch {
			case len(posts) == 0:
				result.Note = "the local mirror holds no workflows, so no tool usage can be measured — run 'rundown-pp-cli sync' first."
			case inWindow == 0:
				result.Note = fmt.Sprintf("no workflows in the last %s; widen --since or re-sync.", label)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintf(out, "No tool usage found for the last %s.\n", label)
				if result.Note != "" {
					fmt.Fprintf(out, "%s\n", result.Note)
				}
				return nil
			}
			fmt.Fprintf(out, "Tool adoption — last %s (%d workflows)\n\n", label, inWindow)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "#\tTOOL\tWORKFLOWS\tSHARE\tUPVOTES\tAVG")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s\t%d\t%.0f%%\t%d\t%.1f\n",
					r.Rank, r.Slug, r.Workflows, r.Share, r.Upvotes, r.AvgUpvotes)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if result.Catalogue > 0 {
				fmt.Fprintf(out, "\n%d of %d catalogued tools are unused in this window.\n", result.Unused, result.Catalogue)
			}
			fmt.Fprintf(out, "See what pairs with one: rundown-pp-cli stack %s\n", rows[0].Slug)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "all", "Time window to rank inside, e.g. 7d, 30d (default: all mirrored workflows)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum tools to return")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// rdCatalogueSize counts the synced tool catalogue, returning 0 when the
// tools resource has not been synced.
func rdCatalogueSize(ctx context.Context, db *store.Store) int {
	var n int
	row := db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM resources WHERE resource_type IN ('tools')`)
	if err := row.Scan(&n); err != nil {
		return 0
	}
	return n
}
