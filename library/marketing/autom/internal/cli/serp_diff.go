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

type serpDiffView struct {
	Query    string        `json:"query"`
	Engine   string        `json:"engine"`
	Country  string        `json:"country,omitempty"`
	FromID   int64         `json:"from_snapshot,omitempty"`
	ToID     int64         `json:"to_snapshot,omitempty"`
	FromTime *time.Time    `json:"from_at,omitempty"`
	ToTime   *time.Time    `json:"to_at,omitempty"`
	Changes  []serp.Change `json:"changes"`
	Note     string        `json:"note,omitempty"`
}

func newSerpDiffCmd(flags *rootFlags) *cobra.Command {
	var query, engine, country, lang, dbPath string
	var from, to int64
	var top int
	var all bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show which URLs entered, left or moved between two saved results pages",
		Long: `Compare two saved pages of the same keyword: by default the two newest, or any two
pages with --from and --to (ids from 'serp list'). Unchanged URLs are hidden unless --all.`,
		Example: strings.Trim(`
  autom-pp-cli serp diff --query "serp api pricing"
  autom-pp-cli serp diff --from 12 --to 31 --top 20 --json`, "\n"),
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
				return writeDryRun(cmd.OutOrStdout(), flags, "serp diff")
			}
			if (from == 0) != (to == 0) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to go together; give both snapshot ids, or neither and --query"))
			}
			if strings.TrimSpace(query) == "" && from == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --query, or both --from and --to snapshot ids"))
			}
			engineName, err := engineFilter(engine)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			view := serpDiffView{Query: query, Engine: engineName, Country: strings.ToLower(country), Changes: make([]serp.Change, 0)}
			db, err := openSerpStoreIfExists(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				var a, b store.SerpSnapshot
				var okA, okB bool
				if from != 0 && to != 0 {
					if a, okA, err = db.SerpSnapshotByID(ctx, from); err != nil {
						return err
					}
					if b, okB, err = db.SerpSnapshotByID(ctx, to); err != nil {
						return err
					}
					if !okA || !okB {
						return notFoundErr(fmt.Errorf("snapshot %d or %d not found; list them with: autom-pp-cli serp list", from, to))
					}
					if a.Key() != b.Key() {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: snapshots %d and %d are for different keywords (%s/%s/%s vs %s/%s/%s)\n", a.ID, b.ID, a.Query, a.Engine, a.Country, b.Query, b.Engine, b.Country)
					}
				} else {
					snaps, err := db.SerpSnapshots(ctx, store.SerpKey{Query: query, Engine: engineName, Country: country, Lang: lang}, 1)
					if err != nil {
						return err
					}
					if len(snaps) == 1 {
						// Both pages must share one exact key (engine, country, lang).
						if snaps, err = db.SerpSnapshotsExact(ctx, snaps[0].Key(), 2); err != nil {
							return err
						}
					}
					if len(snaps) == 2 {
						a, b, okA, okB = snaps[1], snaps[0], true, true
					}
				}
				if okA && okB {
					before, err := db.SerpResults(ctx, a.ID)
					if err != nil {
						return err
					}
					after, err := db.SerpResults(ctx, b.ID)
					if err != nil {
						return err
					}
					view.Query, view.Engine, view.Country = b.Query, b.Engine, b.Country
					view.FromID, view.ToID = a.ID, b.ID
					ta, tb := a.TakenAt, b.TakenAt
					view.FromTime, view.ToTime = &ta, &tb
					for _, ch := range serp.Diff(before, after, top) {
						if ch.Kind == "same" && !all {
							continue
						}
						view.Changes = append(view.Changes, ch)
					}
					if len(view.Changes) == 0 {
						view.Note = "no change between the two pages"
					}
				} else {
					view.Note = "fewer than two saved pages match; save another page with rank check first"
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if view.FromTime != nil {
				fmt.Fprintf(w, "%q on %s: page %d (%s) -> page %d (%s)\n", view.Query, view.Engine, view.FromID, view.FromTime.Local().Format("2006-01-02 15:04"), view.ToID, view.ToTime.Local().Format("2006-01-02 15:04"))
			}
			if view.Note != "" {
				fmt.Fprintln(w, view.Note)
			}
			for _, ch := range view.Changes {
				fmt.Fprintf(w, "  %-8s %-4s -> %-4s %s\n", ch.Kind, posLabel(ch.From), posLabel(ch.To), ch.URL)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Keyword whose two newest pages are compared")
	cmd.Flags().StringVar(&engine, "engine", "", "Engine the pages were saved for (default: any)")
	cmd.Flags().StringVar(&country, "country", "", "Country code the pages were saved for")
	cmd.Flags().StringVar(&lang, "lang", "", "Language code the pages were saved for")
	cmd.Flags().Int64Var(&from, "from", 0, "Older snapshot id")
	cmd.Flags().Int64Var(&to, "to", 0, "Newer snapshot id")
	cmd.Flags().IntVar(&top, "top", 10, "Compare only the first N positions (0 = all)")
	cmd.Flags().BoolVar(&all, "all", false, "Also list URLs that did not move")
	addDBFlag(cmd, &dbPath)
	return cmd
}

func newSerpListCmd(flags *rootFlags) *cobra.Command {
	var query, engine, country, lang, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved results pages (snapshot ids for serp diff)",
		Example: strings.Trim(`
  autom-pp-cli serp list --query "serp api pricing"
  autom-pp-cli serp list --limit 20 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "serp list")
			}
			engineName, err := engineFilter(engine)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			snaps := make([]store.SerpSnapshot, 0)
			db, err := openSerpStoreIfExists(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
				if snaps, err = db.SerpSnapshots(ctx, store.SerpKey{Query: query, Engine: engineName, Country: country, Lang: lang}, limit); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), snaps, flags)
			}
			if len(snaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved results pages.")
				return nil
			}
			rows := make([]map[string]any, 0, len(snaps))
			for _, s := range snaps {
				rows = append(rows, map[string]any{"id": s.ID, "date": s.TakenAt.Local().Format("2006-01-02 15:04"), "query": s.Query, "engine": s.Engine, "country": s.Country, "results": s.ResultCount})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Only this keyword")
	cmd.Flags().StringVar(&engine, "engine", "", "Only this engine")
	cmd.Flags().StringVar(&country, "country", "", "Only this country code")
	cmd.Flags().StringVar(&lang, "lang", "", "Only this language code")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum pages to list (0 = all)")
	addDBFlag(cmd, &dbPath)
	return cmd
}

func newSerpCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "serp",
		Short:       "Saved results pages: list them and diff two of them",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newSerpDiffCmd(flags))
	addNovelCommandIfAbsent(cmd, newSerpListCmd(flags))
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSerpCmd(flags))
	})
}
