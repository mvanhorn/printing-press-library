// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/store"
)

type rankCheckView struct {
	Domain     string        `json:"domain"`
	Query      string        `json:"query"`
	Engine     string        `json:"engine"`
	Country    string        `json:"country,omitempty"`
	Lang       string        `json:"lang,omitempty"`
	Found      bool          `json:"found"`
	Position   int           `json:"position"`
	URL        string        `json:"url,omitempty"`
	Previous   *int          `json:"previous_position,omitempty"`
	Change     *int          `json:"change,omitempty"` // positive = moved up
	Checked    time.Time     `json:"checked_at"`
	SnapshotID int64         `json:"snapshot_id,omitempty"`
	Top        []serp.Result `json:"top"`
}

func newRankCheckCmd(flags *rootFlags) *cobra.Command {
	var query, engine, country, lang, location, dbPath string
	var top int
	var noSave bool
	cmd := &cobra.Command{
		Use:   "check [domain]",
		Short: "Check where a domain ranks for a keyword on one engine, and save the results page",
		Long: `Run one live search and report the position of a domain (subdomains count),
plus the top results. The results page is saved locally (unless --no-save),
which feeds rank history, rank competitors, rank movers, serp diff and visibility.
Costs 1 Autom credit.`,
		Example: strings.Trim(`
  autom-pp-cli rank check autom.dev --query "serp api pricing" --country fr
  autom-pp-cli rank check autom.dev --query "google search scraper" --engine bing --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write": "true", // saves the results page unless --no-save
			"pp:data-source":  "live",
			"pp:happy-args":   "domain=autom.dev;--query=serp api pricing;--no-save",
			// With --no-save the check only reads: live dogfood runs it for real.
			"pp:preview-happy-path": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank check")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain is required, for example: rank check autom.dev --query \"serp api pricing\""))
			}
			if strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			e, err := serp.LookupEngine(engine)
			if err != nil {
				return usageErr(err)
			}
			domain, err := domainArg(args[0], false)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := searchOpts{Country: country, Lang: lang, Location: location}
			results, err := fetchResults(ctx, c, e, query, opts)
			if err != nil {
				return err
			}
			view := rankCheckView{Domain: domain, Query: query, Engine: e.Name, Country: strings.ToLower(country), Lang: strings.ToLower(lang), Checked: time.Now().UTC(), Top: serp.TopN(results, top)}
			view.Position, view.URL = serp.RankOf(results, domain)
			view.Found = view.Position > 0
			if !noSave {
				db, err := openSerpStore(ctx, dbPath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
				} else {
					defer db.Close()
					// Previous position comes from the last saved page before this one.
					key := store.SerpKey{Query: query, Engine: e.Name, Country: country, Lang: lang}
					if prev, ok, err := db.LatestSerpSnapshot(ctx, key, time.Time{}); err == nil && ok {
						if prevRes, err := db.SerpResults(ctx, prev.ID); err == nil {
							p, _ := serp.RankOf(prevRes, domain)
							view.Previous = &p
							if p > 0 && view.Position > 0 {
								d := p - view.Position
								view.Change = &d
							}
						}
					}
					view.SnapshotID = saveSnapshot(ctx, cmd, db, query, e.Name, opts, results)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.Found {
				fmt.Fprintf(w, "%s ranks #%d for %q on %s%s\n  %s\n", domain, view.Position, query, e.Name, localeSuffix(country, lang), view.URL)
			} else {
				fmt.Fprintf(w, "%s is not in the %d results for %q on %s%s\n", domain, len(results), query, e.Name, localeSuffix(country, lang))
			}
			if view.Previous != nil {
				switch {
				case view.Change != nil && *view.Change > 0:
					fmt.Fprintf(w, "  up %d since the last check (was #%d)\n", *view.Change, *view.Previous)
				case view.Change != nil && *view.Change < 0:
					fmt.Fprintf(w, "  down %d since the last check (was #%d)\n", -*view.Change, *view.Previous)
				case *view.Previous == 0 && view.Found:
					fmt.Fprintln(w, "  new: not ranked at the last check")
				case *view.Previous > 0 && !view.Found:
					fmt.Fprintf(w, "  dropped out (was #%d)\n", *view.Previous)
				}
			}
			fmt.Fprintln(w)
			rows := make([]map[string]any, 0, len(view.Top))
			for _, r := range view.Top {
				mark := ""
				if serp.DomainMatches(r.Domain, domain) {
					mark = "<-"
				}
				rows = append(rows, map[string]any{"pos": r.Position, "domain": r.Domain, "title": truncateText(r.Title, 60), "": mark})
			}
			return printAutoTable(w, rows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Keyword to search, for example \"serp api pricing\"")
	cmd.Flags().StringVar(&engine, "engine", "google", "Engine: google, bing, brave, duckduckgo or yahoo")
	cmd.Flags().StringVar(&country, "country", "", "Two-letter country code, for example fr")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code, for example fr")
	cmd.Flags().StringVar(&location, "location", "", "Location to search from (Google and Bing only), for example \"Paris, France\". Not part of the keyword's history key: pages with and without --location share one history")
	cmd.Flags().IntVar(&top, "top", 10, "How many top results to show")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Do not save the results page locally")
	addDBFlag(cmd, &dbPath)
	return cmd
}

func localeSuffix(country, lang string) string {
	parts := make([]string, 0, 2)
	if country != "" {
		parts = append(parts, strings.ToUpper(country))
	}
	if lang != "" {
		parts = append(parts, strings.ToLower(lang))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if rankCmd, _, err := root.Find([]string{"rank"}); err == nil && rankCmd != root {
			addNovelCommandIfAbsent(rankCmd, newRankCheckCmd(flags))
		}
	})
}
