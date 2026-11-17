// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/store"
)

type moverRow struct {
	Query   string `json:"query"`
	Engine  string `json:"engine"`
	Country string `json:"country,omitempty"`
	Lang    string `json:"lang,omitempty"`
	Domain  string `json:"domain"`
	URL     string `json:"url,omitempty"`
	From    int    `json:"from"` // 0 = absent from the top at the baseline
	To      int    `json:"to"`   // 0 = absent from the top now
	Delta   int    `json:"delta"`
}

type moversView struct {
	Since              string     `json:"since"`
	Cutoff             time.Time  `json:"cutoff"`
	Domain             string     `json:"domain,omitempty"`
	Top                int        `json:"top"`
	KeywordsCompared   int        `json:"keywords_compared"`
	KeywordsNoBaseline int        `json:"keywords_without_baseline"` // no saved page on both sides of the cutoff
	Gains              []moverRow `json:"gains"`
	Losses             []moverRow `json:"losses"`
	Entered            []moverRow `json:"entered_top"`
	Left               []moverRow `json:"left_top"`
	Note               string     `json:"note,omitempty"`
}

func newNovelRankMoversCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagDomain, country, engine, dbPath string
	var top, minDelta int
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "List what moved across every tracked keyword since a cutoff: biggest gains, biggest drops",
		Long: `Use this command to see what moved across ALL tracked keywords since a cutoff: biggest gains,
biggest drops, and entries into or exits from the top results. It compares the newest saved page
of each keyword with the newest page saved before the cutoff. Reads saved pages only.
Do NOT use this command for the full trend of one keyword; use 'rank history' instead.
Do NOT use it for a domain's overall footprint; use 'visibility' instead.`,
		Example: strings.Trim(`
  autom-pp-cli rank movers --since 7d --domain autom.dev --agent
  autom-pp-cli rank movers --since 30d --country fr --min-delta 3`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank movers")
			}
			if minDelta < 1 {
				minDelta = 1
			}
			cutoff, err := sinceCutoff(flagSince)
			if err != nil {
				return err
			}
			engineName, err := engineFilter(engine)
			if err != nil {
				return err
			}
			domain, err := domainArg(flagDomain, true)
			if err != nil {
				return err
			}
			view := moversView{Since: flagSince, Cutoff: cutoff.UTC(), Domain: domain, Top: top,
				Gains: make([]moverRow, 0), Losses: make([]moverRow, 0), Entered: make([]moverRow, 0), Left: make([]moverRow, 0)}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
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
					latest, ok, err := db.LatestSerpSnapshot(ctx, k, time.Time{})
					if err != nil {
						return err
					}
					if !ok {
						continue
					}
					base, ok, err := db.LatestSerpSnapshot(ctx, k, cutoff)
					if err != nil {
						return err
					}
					if !ok || base.ID == latest.ID {
						view.KeywordsNoBaseline++
						continue
					}
					view.KeywordsCompared++
					now, err := db.SerpResults(ctx, latest.ID)
					if err != nil {
						return err
					}
					then, err := db.SerpResults(ctx, base.ID)
					if err != nil {
						return err
					}
					if domain != "" {
						appendDomainMove(&view, k, domain, then, now, top, minDelta)
						continue
					}
					appendURLMoves(&view, k, then, now, top, minDelta)
				}
			}
			sort.SliceStable(view.Gains, func(i, j int) bool { return view.Gains[i].Delta > view.Gains[j].Delta })
			sort.SliceStable(view.Losses, func(i, j int) bool { return view.Losses[i].Delta < view.Losses[j].Delta })
			if view.KeywordsCompared == 0 {
				view.Note = "no keyword has a saved page both before the cutoff and after it; save pages over time with rank check or batch run, or widen --since"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Compared %d keyword(s) against their page from before %s", view.KeywordsCompared, cutoff.Local().Format("2006-01-02"))
			if view.KeywordsNoBaseline > 0 {
				fmt.Fprintf(w, " (%d without pages on both sides of the cutoff)", view.KeywordsNoBaseline)
			}
			fmt.Fprintln(w)
			if view.Note != "" {
				fmt.Fprintln(w, view.Note)
				return nil
			}
			printMovers := func(title string, rows []moverRow) {
				if len(rows) == 0 {
					return
				}
				fmt.Fprintf(w, "\n%s\n", title)
				for _, r := range rows {
					fmt.Fprintf(w, "  %-28s %-10s %-3s %-28s %s -> %s\n", truncateText(r.Query, 28), r.Engine, r.Country, truncateText(r.Domain, 28), posLabel(r.From), posLabel(r.To))
				}
			}
			printMovers("Gains", view.Gains)
			printMovers("Losses", view.Losses)
			printMovers(fmt.Sprintf("Entered the top %d", top), view.Entered)
			printMovers(fmt.Sprintf("Left the top %d", top), view.Left)
			if len(view.Gains)+len(view.Losses)+len(view.Entered)+len(view.Left) == 0 {
				fmt.Fprintln(w, "No position changed by the minimum delta.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Compare with the newest page saved before this window, for example 7d or 30d")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Only report this domain's moves (default: every URL in the top)")
	cmd.Flags().StringVar(&country, "country", "", "Only keywords saved for this country code")
	cmd.Flags().StringVar(&engine, "engine", "", "Only keywords saved for this engine")
	cmd.Flags().IntVar(&top, "top", 10, "Size of the top used for entries and exits")
	cmd.Flags().IntVar(&minDelta, "min-delta", 1, "Smallest position change to report")
	addDBFlag(cmd, &dbPath)
	return cmd
}

// appendDomainMove classifies one keyword's change for a single domain.
func appendDomainMove(view *moversView, k store.SerpKey, domain string, then, now []serp.Result, top, minDelta int) {
	to, u := serp.RankOf(now, domain)
	from, _ := serp.RankOf(then, domain)
	if top > 0 && to > top {
		to = 0
	}
	if top > 0 && from > top {
		from = 0
	}
	row := moverRow{Query: k.Query, Engine: k.Engine, Country: k.Country, Lang: k.Lang, Domain: domain, URL: u, From: from, To: to, Delta: from - to}
	switch {
	case from == 0 && to > 0:
		row.Delta = 0
		view.Entered = append(view.Entered, row)
	case from > 0 && to == 0:
		row.Delta = 0
		view.Left = append(view.Left, row)
	case from > 0 && to > 0 && from-to >= minDelta:
		view.Gains = append(view.Gains, row)
	case from > 0 && to > 0 && to-from >= minDelta:
		view.Losses = append(view.Losses, row)
	}
}

// appendURLMoves classifies every URL change in one keyword's top.
func appendURLMoves(view *moversView, k store.SerpKey, then, now []serp.Result, top, minDelta int) {
	for _, ch := range serp.Diff(then, now, top) {
		row := moverRow{Query: k.Query, Engine: k.Engine, Country: k.Country, Lang: k.Lang, Domain: ch.Domain, URL: ch.URL, From: ch.From, To: ch.To, Delta: ch.Delta}
		switch {
		case ch.Kind == "entered":
			view.Entered = append(view.Entered, row)
		case ch.Kind == "left":
			view.Left = append(view.Left, row)
		case ch.Kind == "up" && ch.Delta >= minDelta:
			view.Gains = append(view.Gains, row)
		case ch.Kind == "down" && -ch.Delta >= minDelta:
			view.Losses = append(view.Losses, row)
		}
	}
}
