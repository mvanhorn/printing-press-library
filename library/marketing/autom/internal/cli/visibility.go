// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

type domainVisibility struct {
	Domain        string   `json:"domain"`
	KeywordsFound int      `json:"keywords_found"`
	Top3          int      `json:"top_3"`
	InTop         int      `json:"in_top"`        // within the first --top results
	AvgPosition   *float64 `json:"avg_position"`  // over keywords where the domain was found; null when never found
	BestPosition  *int     `json:"best_position"` // null when never found
}

type visibilityKeyword struct {
	Query     string         `json:"query"`
	Engine    string         `json:"engine"`
	Country   string         `json:"country,omitempty"`
	Lang      string         `json:"lang,omitempty"`
	TakenAt   time.Time      `json:"taken_at"`
	Positions map[string]int `json:"positions"` // domain -> position, 0 = absent
}

type visibilityView struct {
	Top             int                 `json:"top"`
	KeywordsTracked int                 `json:"keywords_tracked"`
	Domains         []domainVisibility  `json:"domains"`
	Keywords        []visibilityKeyword `json:"keywords"`
	Note            string              `json:"note,omitempty"`
}

func newNovelVisibilityCmd(flags *rootFlags) *cobra.Command {
	var flagVs, country, engine, dbPath string
	var top int
	cmd := &cobra.Command{
		Use:   "visibility [domain]",
		Short: "See one domain's footprint across every saved keyword: where it appears, average and best position",
		Long: `Use this command for one domain's footprint across EVERY saved keyword, optionally against rival
domains (--vs). It reads the newest saved page of each keyword and counts where each domain
appears, its average and best position, and its top-3 and top-N (--top) placements.
Do NOT use this command for who sits in the top N for one keyword; use 'rank competitors' instead.
Do NOT use it for changes since the last run; use 'rank movers' instead.`,
		Example: strings.Trim(`
  autom-pp-cli visibility autom.dev --top 10 --agent
  autom-pp-cli visibility autom.dev --vs serpapi.com,serper.dev --country fr`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "domain=autom.dev",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "visibility")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain is required, for example: visibility autom.dev"))
			}
			mainDomain, derr := domainArg(args[0], false)
			if derr != nil {
				return derr
			}
			domains := []string{mainDomain}
			for _, d := range splitCSV(flagVs) {
				nd, derr := domainArg(d, false)
				if derr != nil {
					return derr
				}
				if !slices.Contains(domains, nd) {
					domains = append(domains, nd)
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := visibilityView{Top: top, Domains: make([]domainVisibility, 0, len(domains)), Keywords: make([]visibilityKeyword, 0)}
			stats := make([]domainVisibility, len(domains))
			sums := make([]int, len(domains))
			best := make([]int, len(domains))
			for i, d := range domains {
				stats[i].Domain = d
			}
			engineName, err := engineFilter(engine)
			if err != nil {
				return err
			}
			db, err := openSerpStoreIfExists(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				keys, err := db.SerpKeys(ctx, country)
				if err != nil {
					return err
				}
				for _, k := range keys {
					if engineName != "" && k.Engine != engineName {
						continue
					}
					sn, ok, err := db.LatestSerpSnapshot(ctx, k, time.Time{})
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					res, err := db.SerpResults(ctx, sn.ID)
					if err != nil {
						return err
					}
					view.KeywordsTracked++
					kw := visibilityKeyword{Query: k.Query, Engine: k.Engine, Country: k.Country, Lang: k.Lang, TakenAt: sn.TakenAt, Positions: map[string]int{}}
					for i, d := range domains {
						pos, _ := serp.RankOf(res, d)
						kw.Positions[d] = pos
						if pos <= 0 {
							continue
						}
						s := &stats[i]
						s.KeywordsFound++
						sums[i] += pos
						if top > 0 && pos <= top {
							s.InTop++
						}
						if pos <= 3 {
							s.Top3++
						}
						if best[i] == 0 || pos < best[i] {
							best[i] = pos
						}
					}
					view.Keywords = append(view.Keywords, kw)
				}
			}
			for i := range stats {
				if stats[i].KeywordsFound > 0 {
					avg := math.Round(float64(sums[i])/float64(stats[i].KeywordsFound)*10) / 10
					b := best[i]
					stats[i].AvgPosition, stats[i].BestPosition = &avg, &b
				}
				view.Domains = append(view.Domains, stats[i])
			}
			if view.KeywordsTracked == 0 {
				view.Note = "no saved pages match: save some with rank check or batch run first"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d tracked keyword(s), newest saved page of each\n\n", view.KeywordsTracked)
			if view.KeywordsTracked == 0 {
				fmt.Fprintln(w, "No saved pages yet: run rank check or batch run first.")
				return nil
			}
			rows := make([]map[string]any, 0, len(view.Domains))
			for _, d := range view.Domains {
				avg, bestLabel := "-", "-"
				if d.AvgPosition != nil {
					avg = fmt.Sprintf("%.1f", *d.AvgPosition)
				}
				if d.BestPosition != nil {
					bestLabel = posLabel(*d.BestPosition)
				}
				rows = append(rows, map[string]any{"domain": d.Domain, "found": fmt.Sprintf("%d/%d", d.KeywordsFound, view.KeywordsTracked), "top 3": d.Top3, fmt.Sprintf("top %d", view.Top): d.InTop, "avg": avg, "best": bestLabel})
			}
			return printAutoTable(w, rows)
		},
	}
	cmd.Flags().StringVar(&flagVs, "vs", "", "Rival domains to compare, comma-separated")
	cmd.Flags().IntVar(&top, "top", 10, "Count keywords where a domain is within the first N results")
	cmd.Flags().StringVar(&country, "country", "", "Only keywords saved for this country code")
	cmd.Flags().StringVar(&engine, "engine", "", "Only keywords saved for this engine")
	addDBFlag(cmd, &dbPath)
	return cmd
}
