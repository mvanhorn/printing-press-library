// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// rdStackRow is one tool that co-occurs with the queried tool.
type rdStackRow struct {
	Slug     string  `json:"slug"`
	Together int     `json:"together"`
	SharePct float64 `json:"sharePct"`
	Upvotes  int     `json:"upvotes"`
}

type rdStackResult struct {
	Tool       string       `json:"tool"`
	Window     string       `json:"window"`
	Workflows  int          `json:"workflowsUsingTool"`
	SoloCount  int          `json:"workflowsUsingToolAlone"`
	PairsWith  []rdStackRow `json:"pairsWith"`
	Suggestion []string     `json:"didYouMean,omitempty"`
	Note       string       `json:"note,omitempty"`
}

func newNovelStackCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit  int
		flagSince  string
		flagDBPath string
	)

	cmd := &cobra.Command{
		Use:   "stack <tool-slug>",
		Short: "Show which other tools appear alongside a given tool in real workflows",
		Long: strings.Trim(`
Show the stacks people actually run around a tool.

Self-joins the mirrored post-to-tool mapping to count which other tools appear
in the same workflows, with each pairing's share of that tool's workflows.
The API exposes no tool-relationship surface at all, so this only works from
the local mirror.

Use this when picking complementary tooling, or to answer "what do people pair
with X". Run 'rundown-pp-cli tools rank' first to see which slugs are common.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli stack claude-code
  rundown-pp-cli stack n8n --limit 10
  rundown-pp-cli stack chatgpt --since 30d --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "tool-slug=claude-code;--limit=5",
			// 3 = unknown tool slug (typed not-found), not a transport failure.
			"pp:typed-exit-codes": "0,3",
			"pp:data-source":      "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stack")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a tool slug is required, e.g. rundown-pp-cli stack claude-code"))
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			target := strings.ToLower(strings.TrimSpace(args[0]))
			if target == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("the tool slug must not be blank"))
			}
			cutoff, err := rdWindowStart(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := rdResolveDBPath(flagDBPath)
			if stop, err := rdMirrorMissing(cmd, flags, dbPath, rdStackResult{
				Tool:      target,
				PairsWith: make([]rdStackRow, 0),
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
				together int
				upvotes  int
			}
			pairs := map[string]*agg{}
			allSlugs := map[string]bool{}
			usingTool, solo := 0, 0

			for _, p := range posts {
				slugs := p.toolSlugs()
				for _, s := range slugs {
					allSlugs[s] = true
				}
				if !rdInWindow(p, cutoff) {
					continue
				}
				uses := false
				for _, s := range slugs {
					if s == target {
						uses = true
						break
					}
				}
				if !uses {
					continue
				}
				usingTool++

				others := map[string]bool{}
				for _, s := range slugs {
					if s != target {
						others[s] = true
					}
				}
				if len(others) == 0 {
					solo++
				}
				for s := range others {
					if a, ok := pairs[s]; ok {
						a.together++
						a.upvotes += p.UpvoteCount
						continue
					}
					pairs[s] = &agg{together: 1, upvotes: p.UpvoteCount}
				}
			}

			rows := make([]rdStackRow, 0, len(pairs))
			for slug, a := range pairs {
				share := 0.0
				if usingTool > 0 {
					share = float64(a.together) / float64(usingTool) * 100
				}
				rows = append(rows, rdStackRow{
					Slug:     slug,
					Together: a.together,
					SharePct: share,
					Upvotes:  a.upvotes,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Together != rows[j].Together {
					return rows[i].Together > rows[j].Together
				}
				if rows[i].Upvotes != rows[j].Upvotes {
					return rows[i].Upvotes > rows[j].Upvotes
				}
				return rows[i].Slug < rows[j].Slug
			})
			if len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			label := flagSince
			if cutoff.IsZero() {
				label = "all time"
			}
			result := rdStackResult{
				Tool:      target,
				Window:    label,
				Workflows: usingTool,
				SoloCount: solo,
				PairsWith: rows,
			}
			if usingTool == 0 {
				switch {
				case len(posts) == 0:
					result.Note = "the local mirror holds no workflows; run 'rundown-pp-cli sync' before using stack."
				case !allSlugs[target]:
					result.Suggestion = rdNearestSlugs(target, allSlugs, 5)
					result.Note = fmt.Sprintf("no mirrored workflow uses the tool slug %q. Run 'rundown-pp-cli tools rank' to see valid slugs.", target)
				default:
					result.Note = fmt.Sprintf("%q is used in the mirror but not within the last %s; widen --since.", target, label)
				}
			}

			// A slug absent from the whole catalogue is bad input and earns a
			// typed not-found exit. A known slug with no workflows inside the
			// window is a legitimate empty result and stays exit 0.
			//
			// An empty mirror cannot distinguish the two, so never claim the
			// slug is unknown when there is simply nothing synced yet — that
			// would turn "you haven't run sync" into a spurious input error.
			unknownSlug := usingTool == 0 && len(posts) > 0 && !allSlugs[target]
			notFound := func() error {
				return notFoundErr(fmt.Errorf(
					"unknown tool slug %q; run 'rundown-pp-cli tools rank' to see the slugs in use", target))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
				if unknownSlug {
					return notFound()
				}
				return nil
			}

			out := cmd.OutOrStdout()
			if usingTool == 0 {
				if unknownSlug {
					if len(result.Suggestion) > 0 {
						fmt.Fprintf(out, "Did you mean: %s\n", strings.Join(result.Suggestion, ", "))
					}
					return notFound()
				}
				fmt.Fprintf(out, "%s\n", result.Note)
				return nil
			}
			fmt.Fprintf(out, "%s — %d workflows (last %s)\n\n", bold(target), usingTool, label)
			if len(rows) == 0 {
				fmt.Fprintf(out, "Always used alone; no co-occurring tools in this window.\n")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "PAIRED WITH\tTOGETHER\tSHARE\tUPVOTES")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%.0f%%\t%d\n", r.Slug, r.Together, r.SharePct, r.Upvotes)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(out, "\nUsed alone in %d of %d workflows.\n", solo, usingTool)
			fmt.Fprintf(out, "See those workflows: rundown-pp-cli top --tool %s --since all\n", target)
			return nil
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum co-occurring tools to return")
	cmd.Flags().StringVar(&flagSince, "since", "all", "Time window to analyse, e.g. 30d (default: all mirrored workflows)")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// rdNearestSlugs suggests catalogue slugs closest to an unrecognised input.
func rdNearestSlugs(target string, known map[string]bool, limit int) []string {
	type scored struct {
		slug string
		dist int
	}
	all := make([]scored, 0, len(known))
	for slug := range known {
		all = append(all, scored{slug: slug, dist: levenshteinDistance(target, slug)})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].dist != all[j].dist {
			return all[i].dist < all[j].dist
		}
		return all[i].slug < all[j].slug
	})
	out := make([]string, 0, limit)
	for _, s := range all {
		// Only suggest genuinely close matches.
		if s.dist > len(target)/2+2 {
			break
		}
		out = append(out, s.slug)
		if len(out) >= limit {
			break
		}
	}
	return out
}
