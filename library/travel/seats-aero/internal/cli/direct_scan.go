// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
	"github.com/spf13/cobra"
)

type directScanRow struct {
	ID            string  `json:"id"`
	Date          string  `json:"date"`
	Source        string  `json:"source"`
	Origin        string  `json:"origin"`
	Destination   string  `json:"destination"`
	Cabin         string  `json:"cabin"`
	Miles         float64 `json:"miles"`
	Seats         float64 `json:"seats"`
	Airlines      string  `json:"airlines"`
	Taxes         float64 `json:"taxes"`
	TaxesCurrency string  `json:"taxes_currency"`
	SyncedAt      string  `json:"synced_at"`
}

func newNovelDirectScanCmd(flags *rootFlags) *cobra.Command {
	var origin, destination, cabin, sources, start, end, flagDB string
	var maxMileage, limit int
	cmd := &cobra.Command{
		Use: "direct-scan", Short: "Find direct-flight award seats under a mileage ceiling across every synced program at once.",
		Long:        "Use this command to filter already-synced availability across ALL routes and programs for direct-only flights under a mileage ceiling. Do NOT use this to view a single route's full date-by-cabin matrix (use 'calendar') or to discover new destinations from an origin with no fixed route in mind (use 'reach').",
		Example:     "  seats-aero-pp-cli direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--origin=JFK;--destination=NRT;--cabin=business;--max-mileage=90000"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "direct-scan")
			}
			if flags.dataSource == "live" {
				return novelUsageError(cmd, flags, fmt.Errorf("direct-scan has no live equivalent; use --data-source local or auto"))
			}
			prefixes := map[string]string{"economy": "y", "premium": "w", "business": "j", "first": "f"}
			cabin = strings.ToLower(strings.TrimSpace(cabin))
			p, ok := prefixes[cabin]
			if !ok {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --cabin %q: use economy, premium, business, or first", cabin))
			}
			if maxMileage < 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--max-mileage must be zero or greater"))
			}
			if limit <= 0 {
				return novelUsageError(cmd, flags, fmt.Errorf("--limit must be greater than zero"))
			}
			for name, value := range map[string]string{"start": start, "end": end} {
				if value != "" {
					if _, err := time.Parse("2006-01-02", value); err != nil {
						return novelUsageError(cmd, flags, fmt.Errorf("invalid --%s %q: use YYYY-MM-DD", name, value))
					}
				}
			}
			if start != "" && end != "" && start > end {
				return novelUsageError(cmd, flags, fmt.Errorf("--end must not be before --start"))
			}
			origin, destination = strings.ToUpper(strings.TrimSpace(origin)), strings.ToUpper(strings.TrimSpace(destination))
			programs, err := cliutil.ParseStringList(sources)
			if err != nil {
				return novelUsageError(cmd, flags, fmt.Errorf("invalid --sources: %w", err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path := resolveNovelDBPath(flagDB)
			db, err := openNovelStoreAt(ctx, path)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s\n", path, novelStoreMissingHint(path))
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printNovelJSON(cmd.OutOrStdout(), make([]directScanRow, 0), flags, nil)
				}
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "availability") {
				hintIfStale(cmd, db, "availability", flags.maxAge)
			}
			warnUnknownSources(ctx, cmd, db, programs)
			query, params := buildDirectScanQuery(p, origin, destination, programs, start, end, maxMileage, limit)
			rows, err := db.DB().QueryContext(ctx, query, params...)
			if err != nil {
				return err
			}
			result := make([]directScanRow, 0)
			for rows.Next() {
				var id, date, source, o, d, airlines, currency, synced sql.NullString
				var miles, seats, taxes sql.NullFloat64
				if err := rows.Scan(&id, &date, &source, &o, &d, &miles, &seats, &airlines, &taxes, &currency, &synced); err != nil {
					_ = rows.Close()
					return err
				}
				result = append(result, directScanRow{id.String, date.String, source.String, o.String, d.String, cabin, miles.Float64, seats.Float64, airlines.String, taxes.Float64, currency.String, synced.String})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printNovelJSON(cmd.OutOrStdout(), result, flags, db)
			}
			if len(result) == 0 {
				message := fmt.Sprintf("No direct %s awards in the local store.\n", cabin)
				if maxMileage > 0 {
					message = fmt.Sprintf("No direct %s awards under %d miles in the local store.\n", cabin, maxMileage)
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), message)
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "DATE\tSOURCE\tROUTE\tMILES\tSEATS\tAIRLINES")
			for _, r := range result {
				fmt.Fprintf(w, "%s\t%s\t%s→%s\t%.0f\t%.0f\t%s\n", r.Date, r.Source, r.Origin, r.Destination, r.Miles, r.Seats, r.Airlines)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Filter by origin IATA airport code.")
	cmd.Flags().StringVar(&destination, "destination", "", "Filter by destination IATA airport code.")
	cmd.Flags().StringVar(&cabin, "cabin", "business", "Cabin class: economy, premium, business, or first.")
	cmd.Flags().IntVar(&maxMileage, "max-mileage", 0, "Maximum one-way mileage cost (0 means no ceiling).")
	cmd.Flags().StringVar(&sources, "sources", "", "Comma-separated mileage program source identifiers.")
	cmd.Flags().StringVar(&start, "start", "", "Earliest travel date in YYYY-MM-DD.")
	cmd.Flags().StringVar(&end, "end", "", "Latest travel date in YYYY-MM-DD.")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of awards to return.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite store (default: ~/.local/share/seats-aero-pp-cli/data.db)")
	return cmd
}

func buildDirectScanQuery(p, origin, destination string, programs []string, start, end string, maxMileage, limit int) (string, []any) {
	effective := fmt.Sprintf("CASE WHEN %s_direct_mileage_cost_raw > 0 THEN %s_direct_mileage_cost_raw ELSE %s_mileage_cost_raw END", p, p, p)
	query := fmt.Sprintf(`SELECT id,substr(date,1,10),source,json_extract(data,'$.Route.OriginAirport'),json_extract(data,'$.Route.DestinationAirport'),%s,CASE WHEN %s_direct_remaining_seats > 0 THEN %s_direct_remaining_seats ELSE %s_remaining_seats END,CASE WHEN COALESCE(%s_direct_airlines,'') <> '' THEN %s_direct_airlines ELSE %s_airlines END,%s_direct_total_taxes,taxes_currency,synced_at FROM availability_all WHERE %s_available=1 AND %s_direct=1`, effective, p, p, p, p, p, p, p, p, p)
	params := make([]any, 0)
	if maxMileage > 0 {
		query += " AND " + effective + " <= ?"
		params = append(params, maxMileage)
	}
	if origin != "" {
		query += " AND json_extract(data,'$.Route.OriginAirport')=?"
		params = append(params, origin)
	}
	if destination != "" {
		query += " AND json_extract(data,'$.Route.DestinationAirport')=?"
		params = append(params, destination)
	}
	if len(programs) > 0 {
		query += " AND source IN (" + strings.TrimRight(strings.Repeat("?,", len(programs)), ",") + ")"
		for _, v := range programs {
			params = append(params, strings.TrimSpace(v))
		}
	}
	if start != "" {
		query += " AND date >= ?"
		params = append(params, start)
	}
	if end != "" {
		query += " AND date < date(?, '+1 day')"
		params = append(params, end)
	}
	query += " ORDER BY (" + effective + ") IS NULL, " + effective + " ASC, date ASC, id ASC LIMIT ?"
	return query, append(params, limit)
}

// warnUnknownSources names the --sources entries that have no rows in the local
// availability mirror, so a typo'd or not-yet-synced program identifier is not
// silently dropped into an empty result set. It only warns on stderr; the
// query, the JSON envelope, and the exit code are unchanged. It stays quiet
// when the mirror holds no sources at all (the unsynced hint already fires).
func warnUnknownSources(ctx context.Context, cmd *cobra.Command, db *store.Store, requested []string) {
	if db == nil || len(requested) == 0 {
		return
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT DISTINCT source FROM availability_all WHERE source IS NOT NULL AND source<>''`)
	if err != nil {
		return
	}
	defer func() { _ = rows.Close() }()
	known := make(map[string]bool)
	synced := make([]string, 0)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return
		}
		known[strings.ToLower(s)] = true
		synced = append(synced, s)
	}
	if rows.Err() != nil || len(synced) == 0 {
		return
	}
	missing := make([]string, 0)
	for _, r := range requested {
		if r = strings.TrimSpace(r); r != "" && !known[strings.ToLower(r)] {
			missing = append(missing, r)
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(synced)
	fmt.Fprintf(cmd.ErrOrStderr(), "warning: --sources %s not present in the local store (synced sources: %s); check the program identifier or sync it first\n", strings.Join(missing, ","), strings.Join(synced, ","))
}
