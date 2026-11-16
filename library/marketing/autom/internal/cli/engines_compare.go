// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

// sideResult is one engine's (or country's) answer in a comparison.
type sideResult struct {
	Key            string        `json:"key"`
	ResultCount    int           `json:"result_count"`
	DomainPosition int           `json:"domain_position,omitempty"`
	DomainURL      string        `json:"domain_url,omitempty"`
	SnapshotID     int64         `json:"snapshot_id,omitempty"`
	Top            []serp.Result `json:"top"`
}

type compareView struct {
	Query         string         `json:"query"`
	Engine        string         `json:"engine,omitempty"`
	Country       string         `json:"country,omitempty"`
	Lang          string         `json:"lang,omitempty"`
	Domain        string         `json:"domain,omitempty"`
	Top           int            `json:"top"`
	Sides         []sideResult   `json:"results"`
	Overlap       serp.Overlap   `json:"overlap"`
	FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
	CreditsUsed   int            `json:"credits_used"`
}

func newNovelEnginesCompareCmd(flags *rootFlags) *cobra.Command {
	var flagEngines, flagDomain, country, lang, dbPath string
	var top int
	var save bool
	cmd := &cobra.Command{
		Use:   "compare [query]",
		Short: "See how one query ranks across Google, Bing, Brave, DuckDuckGo and Yahoo: shared domains, domains unique to each engine",
		Long: `Use this command to run ONE query on SEVERAL engines and compare overlap and per-engine positions.
It prints the domains every engine shows, the domains only one engine shows, a pairwise
similarity score, and your --domain's position on each engine. Costs 1 credit per engine.
Do NOT use this command for one engine across several countries; use 'geo compare' instead.
Do NOT use it for a cited source list to answer a question; use 'ground' instead.`,
		Example: strings.Trim(`
  autom-pp-cli engines compare "serp api pricing" --engines google,bing,brave --domain autom.dev --agent
  autom-pp-cli engines compare "serp api pricing" --country fr --save`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":              "true",
			"mcp:read-only-unless-flags": "save",
			"pp:data-source":             "live",
			"pp:happy-args":              "query=serp api pricing;--engines=google,bing",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "engines compare")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a query is required, for example: engines compare \"serp api pricing\""))
			}
			engines, err := serp.ParseEngineList(flagEngines)
			if err != nil {
				return usageErr(err)
			}
			if len(engines) < 2 {
				return usageErr(fmt.Errorf("compare needs at least two engines; for one engine use 'rank check' or the engine's search command"))
			}
			if _, err := domainArg(flagDomain, true); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := searchOpts{Country: country, Lang: lang}
			results, failures := cliutil.FanoutRun(ctx, engines,
				func(e serp.Engine) string { return e.Name },
				func(ctx context.Context, e serp.Engine) ([]serp.Result, error) {
					return fetchResults(ctx, c, e, query, opts)
				}, cliutil.WithConcurrency(min(len(engines), searchConcurrency)))
			view := compareView{Query: query, Country: strings.ToLower(country), Lang: strings.ToLower(lang), Domain: serp.NormalizeDomain(flagDomain), Top: top, Sides: make([]sideResult, 0)}
			full := map[string][]serp.Result{}
			keys := make([]string, 0, len(results))
			for _, r := range results {
				full[r.Source] = r.Value
				keys = append(keys, r.Source)
			}
			fails, errs := fanoutFailures(failures)
			view.FetchFailures = fails
			if len(keys) == 0 {
				return printThenFail(cmd, flags, view, allFailedErr("engine", fails, errs))
			}
			view.CreditsUsed = len(keys)
			fillSides(ctx, cmd, &view, keys, full, save, dbPath, func(key string) (string, searchOpts) { return key, opts })
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d engines failed; the comparison covers the remaining %d\n", len(failures), len(engines), len(keys))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printCompareHuman(cmd, view, "engine")
			return nil
		},
	}
	cmd.Flags().StringVar(&flagEngines, "engines", "google,bing,brave,duckduckgo,yahoo", "Engines to compare, comma-separated")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Domain whose position is reported on each engine, for example autom.dev")
	cmd.Flags().StringVar(&country, "country", "", "Two-letter country code, for example fr")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code, for example fr")
	cmd.Flags().IntVar(&top, "top", 10, "Compare the first N results of each engine")
	cmd.Flags().BoolVar(&save, "save", false, "Save each engine's results page for rank history and movers")
	addDBFlag(cmd, &dbPath)
	return cmd
}

// fillSides builds the per-side rows and the overlap, and saves pages when asked.
// engineFor maps a side key to the engine and locale used to fetch it.
func fillSides(ctx context.Context, cmd *cobra.Command, view *compareView, keys []string, full map[string][]serp.Result, save bool, dbPath string, engineFor func(key string) (string, searchOpts)) {
	topSets := map[string][]serp.Result{}
	for _, k := range keys {
		topSets[k] = serp.TopN(full[k], view.Top)
	}
	view.Overlap = serp.ComputeOverlap(keys, topSets)
	var saver func(key string) int64
	if save {
		db, err := openSerpStore(ctx, dbPath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
		} else {
			defer db.Close()
			saver = func(key string) int64 {
				engine, o := engineFor(key)
				return saveSnapshot(ctx, cmd, db, view.Query, engine, o, full[key])
			}
		}
	}
	for _, k := range keys {
		side := sideResult{Key: k, ResultCount: len(full[k]), Top: topSets[k]}
		if view.Domain != "" {
			side.DomainPosition, side.DomainURL = serp.RankOf(full[k], view.Domain)
		}
		if saver != nil {
			side.SnapshotID = saver(k)
		}
		view.Sides = append(view.Sides, side)
	}
}

func printCompareHuman(cmd *cobra.Command, view compareView, label string) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%q, top %d per %s\n\n", view.Query, view.Top, label)
	for _, s := range view.Sides {
		line := fmt.Sprintf("%-11s %2d results", s.Key, s.ResultCount)
		if view.Domain != "" {
			if s.DomainPosition > 0 {
				line += fmt.Sprintf("   %s #%d", view.Domain, s.DomainPosition)
			} else {
				line += fmt.Sprintf("   %s not found", view.Domain)
			}
		}
		fmt.Fprintln(w, line)
	}
	fmt.Fprintf(w, "\nShown by every %s (%d): %s\n", label, len(view.Overlap.Shared), joinOrDash(view.Overlap.Shared))
	for _, s := range view.Sides {
		fmt.Fprintf(w, "Only on %s (%d): %s\n", s.Key, len(view.Overlap.Unique[s.Key]), joinOrDash(view.Overlap.Unique[s.Key]))
	}
	for _, f := range view.FetchFailures {
		fmt.Fprintf(w, "failed: %s: %s\n", f.Source, f.Error)
	}
}
