// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/store"
)

type rankHistoryRow struct {
	TakenAt  time.Time `json:"taken_at"`
	Query    string    `json:"query"`
	Engine   string    `json:"engine"`
	Country  string    `json:"country,omitempty"`
	Lang     string    `json:"lang,omitempty"`
	Position int       `json:"position"` // 0 = not in the saved results
	URL      string    `json:"url,omitempty"`
	Snapshot int64     `json:"snapshot_id"`
}

func newRankHistoryCmd(flags *rootFlags) *cobra.Command {
	var query, engine, country, lang, dbPath, since string
	var limit int
	cmd := &cobra.Command{
		Use:   "history [domain]",
		Short: "Show a domain's saved positions over time, per keyword",
		Long: `Use this command for the full position trend of a domain, keyword by keyword,
from the results pages saved by rank check and batch run. Export it with --csv or --json.
Do NOT use this command for what moved across all keywords since a date; use 'rank movers' instead.`,
		Example: strings.Trim(`
  autom-pp-cli rank history autom.dev --query "serp api pricing"
  autom-pp-cli rank history autom.dev --since 30d --csv`, "\n"),
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
				return writeDryRun(cmd.OutOrStdout(), flags, "rank history")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain is required, for example: rank history autom.dev"))
			}
			cutoff, err := sinceCutoff(since)
			if err != nil {
				return err
			}
			engineName, err := engineFilter(engine)
			if err != nil {
				return err
			}
			domain, err := domainArg(args[0], false)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows := make([]rankHistoryRow, 0)
			db, err := openSerpStoreIfExists(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				snaps, err := db.SerpSnapshots(ctx, store.SerpKey{Query: query, Engine: engineName, Country: country, Lang: lang}, 0)
				if err != nil {
					return err
				}
				for _, sn := range snaps { // rows of snaps are already drained
					if !cutoff.IsZero() && sn.TakenAt.Before(cutoff) {
						continue
					}
					res, err := db.SerpResults(ctx, sn.ID)
					if err != nil {
						return err
					}
					pos, u := serp.RankOf(res, domain)
					rows = append(rows, rankHistoryRow{TakenAt: sn.TakenAt, Query: sn.Query, Engine: sn.Engine, Country: sn.Country, Lang: sn.Lang, Position: pos, URL: u, Snapshot: sn.ID})
					if limit > 0 && len(rows) >= limit {
						break
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No saved results for %s match these filters.\n", domain)
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{"date": r.TakenAt.Local().Format("2006-01-02 15:04"), "query": r.Query, "engine": r.Engine, "country": r.Country, "position": posLabel(r.Position)})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Only this keyword")
	cmd.Flags().StringVar(&engine, "engine", "", "Only this engine")
	cmd.Flags().StringVar(&country, "country", "", "Only this country code")
	cmd.Flags().StringVar(&lang, "lang", "", "Only this language code")
	cmd.Flags().StringVar(&since, "since", "", "Only results saved within this window, for example 30d")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows (0 = all)")
	addDBFlag(cmd, &dbPath)
	return cmd
}

type competitorRow struct {
	Position int    `json:"position"`
	Domain   string `json:"domain"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Previous int    `json:"previous_position"` // 0 = not in the previous saved page
	Kind     string `json:"kind"`              // entered, up, down, same (empty with a single saved page)
}

type competitorsView struct {
	Query    string          `json:"query"`
	Engine   string          `json:"engine"`
	Country  string          `json:"country,omitempty"`
	Lang     string          `json:"lang,omitempty"`
	Latest   *time.Time      `json:"latest_at,omitempty"`
	Previous *time.Time      `json:"previous_at,omitempty"`
	Top      []competitorRow `json:"top"`
	Left     []serp.Change   `json:"left_top"`
	Note     string          `json:"note,omitempty"`
}

func newRankCompetitorsCmd(flags *rootFlags) *cobra.Command {
	var query, engine, country, lang, dbPath string
	var top int
	cmd := &cobra.Command{
		Use:   "competitors",
		Short: "Who holds the top results for one keyword, and what changed since the previous saved page",
		Long: `Use this command for who sits in the top N for ONE keyword, with entries, exits and moves
since the previous saved page. Reads saved pages only; run 'rank check' first.
Do NOT use this command for one domain's footprint across every keyword; use 'visibility' instead.`,
		Example: strings.Trim(`
  autom-pp-cli rank competitors --query "serp api pricing" --top 10
  autom-pp-cli rank competitors --query "serp api pricing" --engine bing --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--query=serp api pricing",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank competitors")
			}
			if strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--query is required"))
			}
			e, err := serp.LookupEngine(engine)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := competitorsView{Query: query, Engine: e.Name, Country: strings.ToLower(country), Top: make([]competitorRow, 0), Left: make([]serp.Change, 0)}
			db, err := openSerpStoreIfExists(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				snaps, err := db.SerpSnapshots(ctx, store.SerpKey{Query: query, Engine: e.Name, Country: country, Lang: lang}, 1)
				if err != nil {
					return err
				}
				if len(snaps) == 1 {
					// Compare like with like: the previous page must share the newest page's
					// exact key, not just match the optional filters.
					if snaps, err = db.SerpSnapshotsExact(ctx, snaps[0].Key(), 2); err != nil {
						return err
					}
				}
				switch len(snaps) {
				case 0:
					view.Note = fmt.Sprintf("no saved page for %q on %s; run: autom-pp-cli rank check <domain> --query %q --engine %s", query, e.Name, query, e.Name)
				default:
					latest, err := db.SerpResults(ctx, snaps[0].ID)
					if err != nil {
						return err
					}
					t0 := snaps[0].TakenAt
					view.Latest = &t0
					view.Country, view.Lang = snaps[0].Country, snaps[0].Lang
					var before []serp.Result
					if len(snaps) == 2 {
						if before, err = db.SerpResults(ctx, snaps[1].ID); err != nil {
							return err
						}
						t1 := snaps[1].TakenAt
						view.Previous = &t1
					} else {
						view.Note = "only one saved page for this keyword: changes appear after the next rank check"
					}
					changes := serp.Diff(before, latest, top)
					byURL := map[string]serp.Change{}
					for _, ch := range changes {
						if ch.Kind == "left" {
							if len(snaps) == 2 {
								view.Left = append(view.Left, ch)
							}
							continue
						}
						byURL[ch.URL] = ch
					}
					for _, r := range serp.TopN(latest, top) {
						ch := byURL[r.URL]
						kind := ch.Kind
						if len(snaps) < 2 {
							kind = ""
						}
						view.Top = append(view.Top, competitorRow{Position: r.Position, Domain: r.Domain, Title: r.Title, URL: r.URL, Previous: ch.From, Kind: kind})
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.Note != "" {
				fmt.Fprintln(w, view.Note)
			}
			if len(view.Top) == 0 {
				return nil
			}
			table := make([]map[string]any, 0, len(view.Top))
			for _, r := range view.Top {
				table = append(table, map[string]any{"pos": r.Position, "domain": r.Domain, "was": posLabel(r.Previous), "change": r.Kind})
			}
			if err := printAutoTable(w, table); err != nil {
				return err
			}
			for _, l := range view.Left {
				fmt.Fprintf(w, "left the top %d: %s (was #%d)\n", top, l.Domain, l.From)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Keyword to inspect")
	cmd.Flags().StringVar(&engine, "engine", "google", "Engine: google, bing, brave, duckduckgo or yahoo")
	cmd.Flags().StringVar(&country, "country", "", "Country code used when the page was saved")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code used when the page was saved")
	cmd.Flags().IntVar(&top, "top", 10, "Size of the top to compare")
	addDBFlag(cmd, &dbPath)
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if rankCmd, _, err := root.Find([]string{"rank"}); err == nil && rankCmd != root {
			addNovelCommandIfAbsent(rankCmd, newRankHistoryCmd(flags))
			addNovelCommandIfAbsent(rankCmd, newRankCompetitorsCmd(flags))
		}
	})
}
