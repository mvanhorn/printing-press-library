// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: `watch` command (saved search snapshot diff -> new + price drops).

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/store"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Save searches and diff them over time: NEW listings + PRICE DROPS",
		Long: `Saved-search watch. 'add' stores a named search; 'run' re-runs saved searches,
diffs against the last snapshot (by listing id + price), reports new listings and
price drops, then saves a fresh snapshot. State lives in the local SQLite store.`,
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRmCmd(flags))
	return cmd
}

func openWatchStore(cmd *cobra.Command) (*store.Store, error) {
	return store.OpenWithContext(cmd.Context(), defaultDBPath("motohunt-pp-cli"))
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var name, location, mk, style, model, state, q, sort string
	var limit, maxPages int
	cmd := &cobra.Command{
		Use:     "add [search flags...] --name=<name>",
		Short:   "Save a named search query",
		Example: "  motohunt-pp-cli watch add --name harleys --make Harley-Davidson --location 33705",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would save watch %q for site %s\n", name, site.Site)
				return nil
			}
			if name == "" {
				return usageErr(fmt.Errorf("--name is required: motohunt-pp-cli watch add --name <n> [search flags...]"))
			}
			if sort != "" && !validSort(sort) {
				return usageErr(fmt.Errorf("--sort %q invalid: use t, p, a, or c", sort))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would save watch (verify env)")
				return nil
			}
			if limit <= 0 {
				limit = cardsPerPage
			}
			if maxPages <= 0 {
				maxPages = 5
			}
			db, err := openWatchStore(cmd)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			w := store.Watch{
				Name: name, Site: site.Site, Q: q, Location: location, Make: mk, Model: model,
				Style: style, State: state, Sort: sort, Limit: limit, MaxPages: maxPages,
			}
			if err := db.SaveWatch(w); err != nil {
				return fmt.Errorf("saving watch: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "saved", "watch": w}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Watch name (unique key)")
	cmd.Flags().StringVar(&q, "q", "", "Free-text query")
	cmd.Flags().StringVar(&location, "location", "", "US ZIP code")
	cmd.Flags().StringVar(&mk, "make", "", "Make facet")
	cmd.Flags().StringVar(&style, "style", "", "Style facet")
	cmd.Flags().StringVar(&model, "model", "", "Model facet")
	cmd.Flags().StringVar(&state, "state", "", "State facet")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort: t|p|a|c")
	cmd.Flags().IntVar(&limit, "limit", cardsPerPage, "Max cards per run")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Max pages per run")
	return cmd
}

// watchRunResult is the per-watch diff output.
type watchRunResult struct {
	Watch      string             `json:"watch"`
	Scanned    int                `json:"scanned"`
	New        []motohunt.Listing `json:"new"`
	PriceDrops []priceDrop        `json:"price_drops"`
}

type priceDrop struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	OldPrice string `json:"old_price"`
	NewPrice string `json:"new_price"`
	URL      string `json:"url,omitempty"`
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "run [--name <n>]",
		Short: "Re-run saved searches and report new listings + price drops",
		Long: `Re-run one watch (--name) or all saved watches, diff each against its last
snapshot, and report 'new' listings and 'price_drops'. The first run for a watch
reports every match as new. After diffing, the new snapshot is saved.`,
		Example: "  motohunt-pp-cli watch run --agent\n  motohunt-pp-cli watch run --name harleys --agent",
		// No mcp:read-only: this persists a new snapshot per watch, which changes
		// what subsequent runs report as 'new' — a real side effect agents should
		// be prompted on, not a pure read.
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would re-run saved watch(es) and diff snapshots")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), make([]watchRunResult, 0), flags)
			}
			db, err := openWatchStore(cmd)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			var watches []store.Watch
			if name != "" {
				w, ok, gerr := db.GetWatch(name)
				if gerr != nil {
					return gerr
				}
				if !ok {
					return notFoundErr(fmt.Errorf("watch %q not found (run 'watch list')", name))
				}
				watches = []store.Watch{w}
			} else {
				watches, err = db.ListWatches()
				if err != nil {
					return err
				}
				if len(watches) == 0 {
					fmt.Fprintln(os.Stderr, "no saved watches; add one with 'watch add --name <n> [search flags...]'")
					return printDomainJSON(cmd.OutOrStdout(), make([]watchRunResult, 0), flags)
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			client := scrapeClient(flags)

			results := make([]watchRunResult, 0, len(watches))
			for _, w := range watches {
				res, rerr := runWatch(ctx, client, db, w)
				if rerr != nil {
					fmt.Fprintf(os.Stderr, "warning: watch %q run failed: %v\n", w.Name, rerr)
					continue
				}
				results = append(results, res)
			}
			return printDomainJSON(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Run only this watch (default: all)")
	return cmd
}

// runWatch executes a single saved search, diffs vs the prior snapshot, saves
// the new snapshot, and returns the diff.
func runWatch(ctx context.Context, client *motohunt.Client, db *store.Store, w store.Watch) (watchRunResult, error) {
	site, err := motohunt.ResolveSite(w.Site)
	if err != nil {
		return watchRunResult{}, err
	}
	limit := w.Limit
	if limit <= 0 {
		limit = cardsPerPage
	}
	maxPages := w.MaxPages
	if maxPages <= 0 {
		maxPages = 5
	}

	collected := make([]motohunt.Listing, 0, limit)
	curStart := 0
	for pages := 0; pages < maxPages && len(collected) < limit; pages++ {
		url, _, _ := site.BuildSearchURL(motohunt.SearchParams{
			Q: w.Q, Location: w.Location, Make: w.Make, Model: w.Model, Style: w.Style, State: w.State, Sort: w.Sort, Start: curStart,
		})
		doc, ferr := client.Fetch(ctx, url)
		if ferr != nil {
			if len(collected) > 0 {
				break
			}
			return watchRunResult{}, ferr
		}
		cards := motohunt.ParseCards(doc, site)
		if len(cards) == 0 {
			break
		}
		for _, c := range cards {
			collected = append(collected, c)
			if len(collected) >= limit {
				break
			}
		}
		curStart += cardsPerPage
	}

	prev, err := db.GetSnapshot(w.Name)
	if err != nil {
		return watchRunResult{}, err
	}

	res := watchRunResult{
		Watch:      w.Name,
		Scanned:    len(collected),
		New:        make([]motohunt.Listing, 0),
		PriceDrops: make([]priceDrop, 0),
	}
	newSnap := make([]store.SnapshotRow, 0, len(collected))
	seen := map[string]bool{}
	for _, c := range collected {
		if c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		newSnap = append(newSnap, store.SnapshotRow{ListingID: c.ID, Price: c.Price, Title: c.Title, URL: c.URL})
		old, existed := prev[c.ID]
		if !existed {
			res.New = append(res.New, c)
			continue
		}
		oldP, newP := parsePrice(old.Price), parsePrice(c.Price)
		if oldP > 0 && newP > 0 && newP < oldP {
			res.PriceDrops = append(res.PriceDrops, priceDrop{
				ID: c.ID, Title: c.Title, OldPrice: old.Price, NewPrice: c.Price, URL: c.URL,
			})
		}
	}

	// A zero-card result almost always means a transient challenge/maintenance
	// page (HTTP 200, no matching DOM) rather than "every listing vanished".
	// Replacing the snapshot with an empty set would flood the next run with
	// false "new" listings, so preserve the prior snapshot instead.
	if len(collected) == 0 {
		return res, nil
	}
	if err := db.ReplaceSnapshot(w.Name, newSnap); err != nil {
		return watchRunResult{}, fmt.Errorf("saving snapshot: %w", err)
	}
	return res, nil
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved watches with their name, site, search criteria, and last-run snapshot time",
		Example:     "  motohunt-pp-cli watch list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list saved watches")
				return nil
			}
			db, err := openWatchStore(cmd)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			watches, err := db.ListWatches()
			if err != nil {
				return err
			}
			return printDomainJSON(cmd.OutOrStdout(), watches, flags)
		},
	}
	return cmd
}

func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:     "rm --name <n>",
		Short:   "Delete a saved watch (and its snapshot)",
		Example: "  motohunt-pp-cli watch rm --name harleys",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would delete watch %q\n", name)
				return nil
			}
			if name == "" {
				return usageErr(fmt.Errorf("--name is required: motohunt-pp-cli watch rm --name <n>"))
			}
			db, err := openWatchStore(cmd)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			deleted, err := db.DeleteWatch(name)
			if err != nil {
				return err
			}
			if !deleted {
				return notFoundErr(fmt.Errorf("watch %q not found", name))
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"status": "deleted", "name": name}, flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Watch name to delete")
	return cmd
}
