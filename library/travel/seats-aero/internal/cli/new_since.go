// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil"
	"github.com/spf13/cobra"
)

type newSinceCabinCell struct {
	Available     bool     `json:"available"`
	Direct        bool     `json:"direct"`
	Mileage       *float64 `json:"mileage,omitempty"`
	DirectMileage *float64 `json:"direct_mileage,omitempty"`
	Seats         *float64 `json:"seats,omitempty"`
	Airlines      string   `json:"airlines,omitempty"`
}
type newSinceRow struct {
	ID          string                       `json:"id"`
	Date        string                       `json:"date"`
	Source      string                       `json:"source"`
	Origin      string                       `json:"origin"`
	Destination string                       `json:"destination"`
	RouteID     string                       `json:"route_id"`
	Cabins      map[string]newSinceCabinCell `json:"cabins"`
	SyncedAt    string                       `json:"synced_at"`
	FirstSeenAt string                       `json:"first_seen_at"`
}

func newNovelNewSinceCmd(flags *rootFlags) *cobra.Command {
	var origin, destination, cabin, sinceText, source, flagDB string
	var limit int
	cmd := &cobra.Command{
		Use: "new-since", Short: "See which award seats appeared on a route since you last looked, from your synced local data.",
		Long:        "Use this command to see which cached availability rows are newly visible since a past point in time. Do NOT use this to re-verify a specific already-known Availability ID is still bookable before booking; use 'recheck' instead.",
		Example:     "  seats-aero-pp-cli new-since --origin JFK --destination NRT --cabin business --since 24h --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--origin=JFK;--destination=NRT;--cabin=business;--since=24h"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "new-since")
			}
			if flags.dataSource == "live" {
				return novelUsageError(cmd, flags, fmt.Errorf("new-since has no live equivalent; it reads the local store (use --data-source local or auto)"))
			}
			d, err := cliutil.ParseDurationLoose(sinceText)
			if err != nil || d <= 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --since %q: must be a positive duration", sinceText))
			}
			prefix := map[string]string{"economy": "y", "premium": "w", "business": "j", "first": "f"}
			if cabin != "" {
				if _, ok := prefix[cabin]; !ok {
					return novelUsageError(cmd, flags, fmt.Errorf("invalid --cabin %q: use economy, premium, business, or first", cabin))
				}
			}
			if limit <= 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--limit must be greater than zero"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			resolvedDBPath := resolveNovelDBPath(flagDB)
			db, err := openNovelStoreAt(ctx, resolvedDBPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s\n", resolvedDBPath, novelStoreMissingHint(resolvedDBPath))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printNovelJSON(cmd.OutOrStdout(), make([]newSinceRow, 0), flags, nil)
				}
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "availability") {
				hintIfStale(cmd, db, "availability", flags.maxAge)
			}
			cutoff := time.Now().UTC().Add(-d)
			query, params := buildNewSinceQuery(cutoff, origin, destination, source, cabin, limit)
			rs, err := db.DB().QueryContext(ctx, query, params...)
			if err != nil {
				return err
			}
			result := make([]newSinceRow, 0)
			for rs.Next() {
				var v [32]sql.NullString
				scan := make([]any, len(v))
				for i := range v {
					scan[i] = &v[i]
				}
				if err := rs.Scan(scan...); err != nil {
					_ = rs.Close()
					return err
				}
				r := newSinceRow{ID: v[0].String, Date: v[1].String, Source: v[2].String, Origin: v[3].String, Destination: v[4].String, RouteID: v[5].String, Cabins: map[string]newSinceCabinCell{}, SyncedAt: v[30].String, FirstSeenAt: v[31].String}
				for i, name := range []string{"economy", "premium", "business", "first"} {
					if cabin == "" || cabin == name {
						n := 6 + i*6
						r.Cabins[name] = newSinceCell(v[n : n+6])
					}
				}
				result = append(result, r)
			}
			if err := rs.Err(); err != nil {
				_ = rs.Close()
				return err
			}
			if err := rs.Close(); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), result, flags, db)
			}
			if len(result) == 0 {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "No newly seen availability since %s.\n", cutoff.Format(time.RFC3339))
				return err
			}
			items := make([]map[string]any, 0, len(result))
			for _, r := range result {
				items = append(items, map[string]any{"first_seen": r.FirstSeenAt, "date": r.Date, "source": r.Source, "route": r.Origin + "-" + r.Destination, "cabins": r.Cabins})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Filter by origin IATA airport code.")
	cmd.Flags().StringVar(&destination, "destination", "", "Filter by destination IATA airport code.")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Filter by available cabin: economy, premium, business, or first; default is any cabin.")
	cmd.Flags().StringVar(&sinceText, "since", "24h", "Look back by a loose duration such as 24h, 7d, or 1w.")
	cmd.Flags().StringVar(&source, "source", "", "Filter by mileage program source identifier.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite store (default: ~/.local/share/seats-aero-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of newly seen rows to return.")
	return cmd
}

func buildNewSinceQuery(cutoff time.Time, origin, destination, source, cabin string, limit int) (string, []any) {
	prefix := map[string]string{"economy": "y", "premium": "w", "business": "j", "first": "f"}
	query := `SELECT a.id,substr(a.date,1,10),a.source,json_extract(a.data,'$.Route.OriginAirport'),json_extract(a.data,'$.Route.DestinationAirport'),a.route_id,
			 a.y_available,a.y_direct,a.y_mileage_cost_raw,a.y_direct_mileage_cost_raw,a.y_remaining_seats,a.y_airlines,
			 a.w_available,a.w_direct,a.w_mileage_cost_raw,a.w_direct_mileage_cost_raw,a.w_remaining_seats,a.w_airlines,
			 a.j_available,a.j_direct,a.j_mileage_cost_raw,a.j_direct_mileage_cost_raw,a.j_remaining_seats,a.j_airlines,
			 a.f_available,a.f_direct,a.f_mileage_cost_raw,a.f_direct_mileage_cost_raw,a.f_remaining_seats,a.f_airlines,a.synced_at,f.first_seen_at
			 FROM availability_all a JOIN availability_first_seen f ON f.id=a.id WHERE f.first_seen_at >= ? AND a.date IS NOT NULL`
	params := []any{cutoff.UTC().Format("2006-01-02T15:04:05Z")}
	for _, f := range []struct{ v, q string }{{strings.ToUpper(origin), " AND json_extract(a.data,'$.Route.OriginAirport') = ?"}, {strings.ToUpper(destination), " AND json_extract(a.data,'$.Route.DestinationAirport') = ?"}, {source, " AND a.source = ?"}} {
		if f.v != "" {
			query += f.q
			params = append(params, f.v)
		}
	}
	if cabin != "" {
		query += " AND a." + prefix[cabin] + "_available = 1"
	}
	query += " ORDER BY f.first_seen_at DESC, a.date ASC, a.id ASC LIMIT ?"
	return query, append(params, limit)
}

func newSinceCell(v []sql.NullString) newSinceCabinCell {
	num := func(x sql.NullString) *float64 {
		if !x.Valid {
			return nil
		}
		n, err := strconv.ParseFloat(x.String, 64)
		if err != nil {
			return nil
		}
		return &n
	}
	return newSinceCabinCell{Available: v[0].String == "1", Direct: v[1].String == "1", Mileage: num(v[2]), DirectMileage: num(v[3]), Seats: num(v[4]), Airlines: v[5].String}
}
