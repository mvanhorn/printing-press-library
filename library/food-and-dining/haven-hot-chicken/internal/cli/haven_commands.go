// Haven public-menu commands. Hand-authored and preserved on regeneration.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/haven-hot-chicken/internal/haven"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/haven-hot-chicken/internal/store"
	"github.com/spf13/cobra"
)

type havenOptions struct {
	db, locations, item, query string
	location                   int64
	quantities                 []string
	lat, lon                   float64
	limit                      int
}

func newHavenCommand(flags *rootFlags, action string) *cobra.Command {
	switch action {
	case "refresh":
		return newNovelHavenRefreshCmd(flags)
	case "menu":
		return newNovelHavenMenuCmd(flags)
	case "locations":
		return newNovelHavenLocationsCmd(flags)
	case "compare":
		return newNovelHavenCompareCmd(flags)
	case "subtotal":
		return newNovelHavenSubtotalCmd(flags)
	case "changes":
		return newNovelHavenChangesCmd(flags)
	case "common":
		return newNovelHavenCommonCmd(flags)
	case "nearby":
		return newNovelHavenNearbyCmd(flags)
	}
	panic("unknown Haven action")
}

func configureHavenCommand(flags *rootFlags, action string, cmd *cobra.Command, o *havenOptions) *cobra.Command {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "haven "+action)
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("haven %s accepts flags only; see --help", action))
		}
		if o.limit < 1 || o.limit > 1000 {
			return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
		}
		if action == "refresh" && flags.dataSource == "local" {
			return usageErr(fmt.Errorf("haven refresh requires network access; use --data-source live or auto"))
		}
		if action != "refresh" && flags.dataSource == "live" {
			return usageErr(fmt.Errorf("haven %s reads saved menus; run haven refresh first and use --data-source local or auto", action))
		}
		if action == "nearby" && (!cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon")) {
			return usageErr(fmt.Errorf("--lat and --lon are required, for example --lat 41.3 --lon -72.9"))
		}
		if action == "compare" && strings.TrimSpace(o.item) == "" {
			return usageErr(fmt.Errorf("--item requires an exact item name, for example --item 'The Sandwich'"))
		}
		if (action == "menu" || action == "subtotal" || action == "changes") && o.location <= 0 {
			return usageErr(fmt.Errorf("--location requires a positive location ID; run locations list"))
		}
		quantities := map[int64]int64{}
		if action == "subtotal" {
			if len(o.quantities) == 0 {
				return usageErr(fmt.Errorf("--item requires ID=quantity, for example --item 9656289=2"))
			}
			for _, entry := range o.quantities {
				parts := strings.Split(entry, "=")
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("--item %q must be ID=quantity", entry))
				}
				id, e1 := strconv.ParseInt(parts[0], 10, 64)
				n, e2 := strconv.ParseInt(parts[1], 10, 64)
				if e1 != nil || e2 != nil || id <= 0 || n < 1 || n > 10000 || quantities[id]+n > 10000 {
					return usageErr(fmt.Errorf("--item %q requires a positive ID and quantity 1..10000", entry))
				}
				quantities[id] += n
			}
		}
		var ids []int64
		if action == "refresh" || action == "compare" || action == "common" {
			var err error
			ids, err = parseHavenIDs(o.locations)
			if err != nil {
				return usageErr(err)
			}
			if (action == "compare" || action == "common") && len(ids) < 2 {
				return usageErr(fmt.Errorf("--locations requires at least two different IDs"))
			}
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		dbPath := o.db
		if dbPath == "" {
			dbPath = defaultDBPath("haven-hot-chicken-pp-cli")
		}
		var db *store.Store
		var err error
		if action == "nearby" {
			if _, err = haven.Nearby([]haven.Location{}, o.lat, o.lon, o.limit); err != nil {
				return usageErr(err)
			}
		}
		if action == "refresh" {
			db, err = store.OpenWithContext(ctx, dbPath)
		} else {
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				field := "rows"
				if action == "menu" || action == "subtotal" {
					field = "items"
				}
				if action == "locations" {
					field = "locations"
				}
				return flags.printJSON(cmd, map[string]any{field: []any{}, "note": "No saved observations. Run haven refresh first, using the same --db if specified."})
			} else if statErr != nil {
				return fmt.Errorf("inspect saved menus: %w", statErr)
			}
			db, err = store.OpenReadOnlyContext(ctx, dbPath)
		}
		if err != nil {
			return fmt.Errorf("open saved menus: %w", err)
		}
		defer db.Close()
		if action == "refresh" {
			return refreshHaven(ctx, cmd, flags, db, ids)
		}
		if action == "locations" || action == "nearby" {
			locations, err := haven.LoadLocations(ctx, db.DB())
			if err != nil {
				return err
			}
			if action == "locations" {
				total := len(locations)
				if total > o.limit {
					locations = locations[:o.limit]
				}
				return flags.printJSON(cmd, map[string]any{"locations": locations, "total": total, "note": "Saved locations. Run haven refresh to update."})
			}
			result, err := haven.Nearby(locations, o.lat, o.lon, o.limit)
			if err != nil {
				return usageErr(err)
			}
			return flags.printJSON(cmd, result)
		}
		if len(ids) == 0 {
			ids = []int64{o.location}
		}
		snapshots := make([]haven.Snapshot, 0, len(ids))
		missing := make([]int64, 0)
		for _, id := range ids {
			rows, err := haven.Latest(ctx, db.DB(), id, 1)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				missing = append(missing, id)
			} else {
				snapshots = append(snapshots, rows[0])
			}
		}
		if len(missing) > 0 {
			return flags.printJSON(cmd, map[string]any{"items": []any{}, "missing_locations": missing, "note": "No complete saved menu for these locations. Run haven refresh --locations with these IDs first."})
		}
		for _, s := range snapshots {
			if flags.maxAge > 0 && time.Since(s.FetchedAt) > flags.maxAge {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: location %d observation exceeds --max-age %s (%s); run haven refresh\n", s.LocationID, flags.maxAge, s.FetchedAt.Format(time.RFC3339))
			}
		}
		var result any
		switch action {
		case "menu":
			items := make([]haven.Item, 0)
			q := strings.ToLower(strings.TrimSpace(o.query))
			for _, i := range snapshots[0].Items {
				if !i.Hidden && strings.Contains(strings.ToLower(i.Name), q) {
					items = append(items, i)
				}
			}
			total := len(items)
			if total > o.limit {
				items = items[:o.limit]
			}
			result = map[string]any{"location_id": o.location, "fetched_at": snapshots[0].FetchedAt, "items": items, "total": total, "currency": "USD", "note": "Prices are base item prices in cents; availability is an observation, not a reservation."}
		case "compare":
			result, err = haven.Compare(snapshots, o.item)
		case "common":
			result, err = haven.Common(snapshots)
		case "subtotal":
			result, err = haven.Subtotal(snapshots[0], quantities)
		case "changes":
			rows, e := haven.Latest(ctx, db.DB(), o.location, 2)
			if e != nil {
				return e
			}
			if len(rows) < 2 {
				return flags.printJSON(cmd, map[string]any{"changes": []any{}, "location_id": o.location, "note": "Two complete observations are required. Refresh this location again later; no history has been inferred."})
			}
			result, err = haven.Changes(rows[1], rows[0])
		}
		if err != nil {
			return usageErr(err)
		}
		result, err = limitHavenResult(result, o.limit)
		if err != nil {
			return err
		}
		return flags.printJSON(cmd, result)
	}
	return cmd
}

func parseHavenIDs(csv string) ([]int64, error) {
	ids := make([]int64, 0)
	seen := map[int64]bool{}
	for _, part := range strings.Split(csv, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("--locations requires comma-separated positive IDs, for example 14208,14205")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) > 10 {
		return nil, fmt.Errorf("--locations accepts at most ten locations per refresh")
	}
	return ids, nil
}

func refreshHaven(ctx context.Context, cmd *cobra.Command, flags *rootFlags, db *store.Store, ids []int64) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	headers := map[string]string{"Accept-Version": "v3.5", "Thanx-Merchant": "havenhotchicken"}
	raw, err := c.GetWithHeadersNoCache(ctx, "/locations", nil, headers)
	if err != nil {
		return classifyAPIErrorOnly(err)
	}
	locations, err := haven.ParseLocations(raw)
	if err != nil {
		return fmt.Errorf("parse locations: %w", err)
	}
	known := map[int64]bool{}
	for _, l := range locations {
		known[l.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			return usageErr(fmt.Errorf("location %d is not in Haven's current location list; run locations list", id))
		}
	}
	snapshots := make([]haven.Snapshot, 0, len(ids))
	for _, id := range ids {
		raw, err := c.GetWithHeadersNoCache(ctx, "/menu_categories", map[string]string{"location_id": strconv.FormatInt(id, 10)}, headers)
		if err != nil {
			return fmt.Errorf("refresh location %d; no observations saved: %w", id, classifyAPIErrorOnly(err))
		}
		s, err := haven.ParseMenu(raw, id, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("parse location %d; no observations saved: %w", id, err)
		}
		snapshots = append(snapshots, s)
	}
	if err := haven.Save(ctx, db.DB(), locations, snapshots); err != nil {
		return fmt.Errorf("save observations: %w", err)
	}
	rows := make([]map[string]any, 0, len(snapshots))
	for _, s := range snapshots {
		rows = append(rows, map[string]any{"location_id": s.LocationID, "fetched_at": s.FetchedAt, "items": len(s.Items), "complete": s.Complete})
	}
	return flags.printJSON(cmd, map[string]any{"observations": rows, "locations": len(locations), "note": "Saved complete menu observations. Prices and availability can change before checkout."})
}

// Bound list outputs without discarding the total number of matching records.
func limitHavenResult(v any, limit int) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err = decoder.Decode(&obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"rows", "items", "excluded", "changes"} {
		if rows, ok := obj[key].([]any); ok && len(rows) > limit {
			obj[key] = rows[:limit]
			obj[key+"_total"] = len(rows)
			obj["truncated"] = true
		}
	}
	return obj, nil
}
