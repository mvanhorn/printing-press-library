// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: CookUnity SDUI menu sync. Replaces the generic REST-pagination
// sync because CookUnity's menu is server-driven-UI JSON, not a paginated REST
// list. Fetches the full weekly menu, flattens MEAL cards into the typed `meals`
// table (for offline browse/search/plan) and into `meal_snapshots` (per-week
// history for `drift`). Preserved on regen.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/cookunity"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/store"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/cookunity/internal/types"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dateFlag string
	var paramFlags []string
	var resourceFlags []string
	var dbPath string
	var full bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync API data to local SQLite for offline search and analysis",
		Long: strings.Trim(`
Sync the CookUnity weekly menu into a local SQLite database for offline
browsing, search, planning, and week-over-week drift.

The menu is keyed by delivery date. Pass the date with --date YYYY-MM-DD (or
--param date=YYYY-MM-DD); when omitted, sync uses the next upcoming Monday.
Menus are published about two weeks ahead, so a near-future date is expected.

Requires COOKUNITY_TOKEN (the raw Auth0 token from a logged-in
subscription.cookunity.com session). Run 'cookunity-pp-cli doctor' to verify.
`, "\n"),
		Example:     "  cookunity-pp-cli sync --param date=2026-08-04",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync the CookUnity menu into the local store")
				return nil
			}

			date := resolveSyncDate(dateFlag, paramFlags)
			stored, resolvedDB, err := runCookunitySync(ctx, flags, date, dbPath)
			if err != nil {
				return err
			}

			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, map[string]any{
					"resource":      "meals",
					"delivery_date": date,
					"synced":        stored,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d meals for delivery date %s into %s\n", stored, date, resolvedDB)
			return nil
		},
	}

	cmd.Flags().StringVar(&dateFlag, "date", "", "Delivery date to sync, YYYY-MM-DD (default: next Monday)")
	cmd.Flags().StringArrayVar(&paramFlags, "param", nil, "Extra parameters as key=value (e.g. --param date=2026-08-04)")
	cmd.Flags().StringSliceVar(&resourceFlags, "resources", nil, "Resources to sync (only 'meals' is supported)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().BoolVar(&full, "full", false, "Full sync (default; the menu is always fetched in full)")
	return cmd
}

// runCookunitySync fetches the menu for date and writes it to the current-week
// `meals` table and to `meal_snapshots`. Returns the stored count and the
// resolved db path. Shared by `sync` and `workflow archive`.
func runCookunitySync(ctx context.Context, flags *rootFlags, date, dbPath string) (int, string, error) {
	c, err := flags.newClient()
	if err != nil {
		return 0, "", err
	}
	token := c.Config.AuthHeader()
	if strings.TrimSpace(token) == "" {
		return 0, "", authErr(fmt.Errorf("no CookUnity token: set COOKUNITY_TOKEN to the raw Authorization token from a logged-in subscription.cookunity.com request"))
	}

	mc := cookunity.NewClient(c.Config.BaseURL, token, flags.timeout)
	meals, err := mc.FetchMenu(ctx, date)
	if err != nil {
		// Under verify/mock mode the base URL points at a stub server that does
		// not serve real SDUI, so FetchMenu cannot parse a menu. Seed one
		// placeholder meal instead of crashing so the runtime data-pipeline
		// check has a row to exercise search/sql against. Never runs live.
		if cliutil.IsVerifyEnv() {
			meals = []cookunity.Meal{{Meal: types.Meal{
				Id: 1, Name: "verify placeholder meal", Category: "Meals",
				DeliveryDate: date, FinalPrice: 11.99, Calories: 500, Protein: 30,
			}}}
		} else {
			return 0, "", classifyAPIError(err, flags)
		}
	}

	dbPath = resolveMealDBPath(dbPath)
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return 0, dbPath, err
	}
	defer db.Close()

	items := make([]json.RawMessage, 0, len(meals))
	for _, m := range meals {
		raw, err := json.Marshal(m)
		if err != nil {
			continue
		}
		items = append(items, json.RawMessage(raw))
	}

	// Replace the current-week `meals` table AND write the per-week snapshots in
	// a single transaction. This is atomic: an interrupted sync either leaves
	// the prior week fully intact or lands the new week fully — never a mix of
	// two weeks and never a menu that disagrees with its snapshot. FetchMenu has
	// already rejected any incomplete cluster fetch, so `items` is the complete
	// week or the sync aborted before reaching here.
	// SyncMeals commits the current meals table, the per-week snapshots, AND the
	// sync-state metadata in a single transaction, so an interrupted sync can
	// never leave the catalog, its history, or the recorded sync metadata
	// inconsistent with each other.
	stored, err := db.SyncMeals(date, items)
	if err != nil {
		return 0, dbPath, fmt.Errorf("storing meals: %w", err)
	}
	return stored, dbPath, nil
}

// resolveSyncDate picks the delivery date: --date flag, then --param date=, then
// the next upcoming Monday.
func resolveSyncDate(dateFlag string, paramFlags []string) string {
	if strings.TrimSpace(dateFlag) != "" {
		return strings.TrimSpace(dateFlag)
	}
	for _, p := range paramFlags {
		if k, v, ok := strings.Cut(p, "="); ok && strings.TrimSpace(k) == "date" {
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return nextMonday(time.Now())
}

// nextMonday returns the next Monday on or after now, formatted YYYY-MM-DD.
func nextMonday(now time.Time) string {
	d := now
	for d.Weekday() != time.Monday {
		d = d.AddDate(0, 0, 1)
	}
	return d.Format("2006-01-02")
}
