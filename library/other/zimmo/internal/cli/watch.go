// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

var watchName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`)

type watchChange struct {
	Kind      string   `json:"kind"` // new | cheaper | gone
	Code      string   `json:"zimmo_code"`
	Address   string   `json:"address"`
	Price     *float64 `json:"price"`
	OldPrice  *float64 `json:"old_price,omitempty"`
	URL       string   `json:"url,omitempty"`
	Surface   *float64 `json:"surface_m2,omitempty"`
	EPC       string   `json:"epc,omitempty"`
	Type      string   `json:"type,omitempty"`
	FirstTime bool     `json:"-"`
}

type watchRunView struct {
	Search   string        `json:"search"`
	Total    int           `json:"total_matching"`
	Scanned  int           `json:"scanned"`
	FirstRun bool          `json:"first_run"`
	New      int           `json:"new"`
	Cheaper  int           `json:"cheaper"`
	Gone     int           `json:"gone"`
	Changes  []watchChange `json:"changes"`
	Note     string        `json:"note,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Saved searches with alerts: new listings, price cuts and listings gone since the last run",
		Long: `Save a search once, then run it daily (cron) to get only what changed:
new listings, cheaper listings and listings that left the search (sold,
rented or withdrawn). No Zimmo account; state lives in the local store.`,
		Example: strings.Trim(`
  zimmo-pp-cli watch save schaerbeek-houses --commune schaerbeek --type house --max-price 450000
  zimmo-pp-cli watch run schaerbeek-houses --json
  zimmo-pp-cli watch run --all
  zimmo-pp-cli watch list`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newWatchSaveCmd(flags), newWatchRunCmd(flags), newWatchListCmd(flags), newWatchRmCmd(flags))
	return cmd
}

func newWatchSaveCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var dbPath string
	cmd := &cobra.Command{
		Use:   "save [name]",
		Short: "Save (or replace) a named search with the same flags as find",
		Example: strings.Trim(`
  zimmo-pp-cli watch save ixelles-flats --commune ixelles --type apartment --max-price 350000
  zimmo-pp-cli watch save peb --postcode 1030,1210 --epc F,G`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "local", "pp:happy-args": "name=ixelles-flats;--postcode=1050;--type=apartment"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "save a named search")
			}
			if len(args) != 1 || !watchName.MatchString(args[0]) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give one search name (lowercase letters, digits, - or _)"))
			}
			crit, err := cf.build()
			if err != nil {
				return err
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if len(crit.Communes) > 0 {
				crit, err = resolveCriteria(ctx, zimmoClient(flags), db, crit)
				if err != nil {
					return err
				}
			}
			if err := db.SaveZimmoSearch(ctx, args[0], crit, time.Now()); err != nil {
				return err
			}
			return printZimmo(cmd.OutOrStdout(), map[string]any{"saved": args[0], "criteria": crit, "next": "zimmo-pp-cli watch run " + args[0]}, flags)
		},
	}
	addCritFlags(cmd, &cf)
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var pages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "run [name]",
		Short: "Run saved searches and report new, cheaper and gone listings since the previous run",
		Long: `Runs one saved search (or --all), stores every result, and reports changes
against the previous run. The first run reports every listing as new. A
listing that left the results is reported once as gone.`,
		Example: strings.Trim(`
  zimmo-pp-cli watch run ixelles-flats
  zimmo-pp-cli watch run --all --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--all"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "run saved searches")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a saved search name or --all"))
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			searches, err := db.ZimmoSavedSearches(ctx, name)
			if err != nil {
				return err
			}
			if name != "" && len(searches) == 0 {
				return notFoundErr(fmt.Errorf("no saved search %q; create it with 'watch save %s ...'", name, name))
			}
			views := make([]watchRunView, 0, len(searches))
			zc := zimmoClient(flags)
			for _, ss := range searches {
				res, err := walkSearch(ctx, zc, ss.Criteria, harnessPages(pages), 0)
				if err != nil {
					return fmt.Errorf("running %s: %w", ss.Name, err)
				}
				seen, err := db.ZimmoSearchSeen(ctx, ss.Name)
				if err != nil {
					return err
				}
				v := diffWatch(ss.Name, seen, res.Listings, ss.LastRunAt == "")
				v.Total, v.Scanned = res.Total, res.Scanned
				complete := res.Truncated == nil && res.Scanned >= res.Total
				if !complete {
					v.Note = fmt.Sprintf("scanned %d of %d results; gone-detection skipped (raise --pages)", res.Scanned, res.Total)
				}
				if err := db.UpsertZimmoListings(ctx, res.Listings, time.Now(), false); err != nil {
					return err
				}
				// Only a complete scan may declare listings gone; keep the old
				// seen-set otherwise so a truncated run does not fake departures.
				if complete {
					if err := db.ReplaceZimmoSearchSeen(ctx, ss.Name, res.Listings, time.Now()); err != nil {
						return err
					}
				} else {
					v.Gone = 0
					kept := v.Changes[:0]
					for _, c := range v.Changes {
						if c.Kind != "gone" {
							kept = append(kept, c)
						}
					}
					v.Changes = kept
					if err := db.MergeZimmoSearchSeen(ctx, ss.Name, res.Listings, time.Now()); err != nil {
						return err
					}
				}
				views = append(views, v)
			}
			_ = db.SaveSyncState("listings", "", len(views))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(views) == 1 && name != "" {
					return printZimmo(cmd.OutOrStdout(), views[0], flags)
				}
				return printZimmo(cmd.OutOrStdout(), views, flags)
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved searches. Create one with: zimmo-pp-cli watch save <name> --commune ... ")
				return nil
			}
			w := cmd.OutOrStdout()
			for _, v := range views {
				fmt.Fprintf(w, "%s: %d new, %d cheaper, %d gone (%d matching)\n", v.Search, v.New, v.Cheaper, v.Gone, v.Total)
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				for _, c := range v.Changes {
					old := ""
					if c.OldPrice != nil {
						old = " (was " + fmtEUR(*c.OldPrice) + ")"
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s%s\t%s\n", c.Kind, c.Code, fmtPtrEUR(c.Price), old, termSafe(c.Address))
				}
				_ = tw.Flush()
				if v.Note != "" {
					fmt.Fprintln(w, "  note: "+v.Note)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Run every saved search")
	cmd.Flags().IntVar(&pages, "pages", 10, "Maximum result pages (100 listings each) per search")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// diffWatch compares the previous seen-set with the current results.
func diffWatch(name string, seen map[string]store.SeenEntry, current []zimmo.Listing, firstRun bool) watchRunView {
	v := watchRunView{Search: name, FirstRun: firstRun, Changes: make([]watchChange, 0)}
	cur := map[string]bool{}
	for _, l := range current {
		if cur[l.Code] {
			continue
		}
		cur[l.Code] = true
		prev, known := seen[l.Code]
		ch := watchChange{Code: l.Code, Address: l.Address, Price: l.Price, URL: l.URL, Surface: l.Surface, EPC: l.EPC, Type: l.Type}
		switch {
		case !known:
			ch.Kind = "new"
			v.New++
			v.Changes = append(v.Changes, ch)
		case prev.LastPrice != nil && l.Price != nil && *l.Price < *prev.LastPrice:
			ch.Kind, ch.OldPrice = "cheaper", prev.LastPrice
			v.Cheaper++
			v.Changes = append(v.Changes, ch)
		}
	}
	for code, prev := range seen {
		if !cur[code] {
			v.Gone++
			v.Changes = append(v.Changes, watchChange{Kind: "gone", Code: code, OldPrice: prev.LastPrice})
		}
	}
	return v
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved searches with their criteria and last run",
		Example:     "  zimmo-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list saved searches")
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			rows := make([]store.SavedSearch, 0)
			if path, ok := localStoreExists(dbPath); ok {
				db, err := openZimmoStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				if rows, err = db.ZimmoSavedSearches(ctx, ""); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved searches.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tLAST RUN\tCRITERIA")
			for _, r := range rows {
				last := r.LastRunAt
				if last == "" {
					last = "never"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, dateOnly(last), termSafe(r.Criteria.Key()))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "rm [name]",
		Short:       "Delete a saved search and its seen-set",
		Example:     "  zimmo-pp-cli watch rm ixelles-flats",
		Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "delete a saved search")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("give one saved search name"))
			}
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			ok, err := db.DeleteZimmoSearch(ctx, args[0])
			if err != nil {
				return err
			}
			if !ok {
				return notFoundErr(fmt.Errorf("no saved search %q", args[0]))
			}
			return printZimmo(cmd.OutOrStdout(), map[string]any{"deleted": args[0]}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newWatchCmd(flags))
	})
}
