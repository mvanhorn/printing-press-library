// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/store"
)

type dropRow struct {
	PropertyID   int     `json:"property_id"`
	Name         string  `json:"name"`
	CityID       int     `json:"city_id"`
	CheckIn      string  `json:"checkin"`
	Nights       int     `json:"nights"`
	Currency     string  `json:"currency"`
	CurrentAllIn float64 `json:"current_all_in"`
	MedianAllIn  float64 `json:"trailing_median_all_in"`
	DropAmount   float64 `json:"drop_amount"`
	DropPct      float64 `json:"drop_pct"`
	Observations int     `json:"observations"`
}

type watchRunResult struct {
	WatchesChecked    int       `json:"watches_checked"`
	PropertiesPriced  int       `json:"properties_priced"`
	ObservationsAdded int       `json:"observations_added"`
	MinDropPct        float64   `json:"min_drop_pct"`
	Drops             []dropRow `json:"drops"`
	Note              string    `json:"note,omitempty"`
}

func newNovelWatchRunCmd(flags *rootFlags) *cobra.Command {
	var minPct float64
	var dbPath string
	var minObservations int

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Re-price every watch and report only properties whose price actually dropped",
		Long: `Re-price all saved watches and surface meaningful price drops.

Each run appends what it observed to a local price history, then compares the
latest all-in price for each property against its trailing median. Only
properties that fell by at least --min-pct are returned.

This is the one thing a stateless scraper cannot do: without accumulated history
there is no baseline to call a drop against. Expect empty output most days -
that is the command working, not failing. Add watches with 'watch add'.`,
		Example: "  agoda-pp-cli watch run --min-pct 7 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--min-pct=7",
			// No watches yet, or no drops today, are both normal empty states.
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch run")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("agoda-pp-cli")
			}
			out := watchRunResult{MinDropPct: minPct, Drops: make([]dropRow, 0)}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local price history at %s\nrun: agoda-pp-cli watch add <destination> --checkin <date>\n",
					dbPath)
				out.Note = "no local price history yet; add a watch first"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				return nil
			}

			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer st.Close()
			if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
				return err
			}

			watches, err := loadWatches(ctx, st.DB())
			if err != nil {
				return err
			}
			out.WatchesChecked = len(watches)
			if len(watches) == 0 {
				out.Note = "no watches configured; add one with 'agoda-pp-cli watch add <destination> --checkin <date>'"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), out.Note)
				return nil
			}

			// Under live dogfood the matrix enforces a short per-command
			// timeout, so re-price a single watch rather than the full set.
			if cliutil.IsDogfoodEnv() && len(watches) > 1 {
				watches = watches[:1]
			}

			c := newAgodaClient(flags)
			now := time.Now().UTC().Format(time.RFC3339)

			for _, wch := range watches {
				opts := agoda.SearchOptions{
					CityID: wch.CityID, CheckIn: wch.CheckIn, Nights: wch.Nights,
					Rooms: wch.Rooms, Adults: wch.Adults, Currency: wch.Currency,
					Locale: "en-us", Origin: "US", Authenticated: hasAgodaSession(),
				}
				props, err := c.CitySearch(ctx, opts)
				if err != nil {
					// A throttle must stop the run: continuing would record a
					// partial snapshot and corrupt the trailing median that
					// every future drop is measured against.
					if isRateLimited(err) {
						return err
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: watch for city %d on %s failed: %v\n",
						wch.CityID, wch.CheckIn, err)
					continue
				}
				added, err := recordObservations(ctx, st.DB(), wch, props, now)
				if err != nil {
					return err
				}
				out.ObservationsAdded += added
				out.PropertiesPriced += len(props)

				drops, err := detectDrops(ctx, st.DB(), wch, props, minPct, minObservations)
				if err != nil {
					return err
				}
				out.Drops = append(out.Drops, drops...)
			}

			sort.SliceStable(out.Drops, func(i, j int) bool { return out.Drops[i].DropPct > out.Drops[j].DropPct })
			if len(out.Drops) == 0 {
				out.Note = fmt.Sprintf(
					"no property fell at least %.1f%% below its trailing median; history was still recorded",
					minPct)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "computed")
			}
			return renderDrops(cmd, out)
		},
	}
	cmd.Flags().Float64Var(&minPct, "min-pct", 5.0,
		"Minimum percentage below the trailing median to count as a drop")
	cmd.Flags().IntVar(&minObservations, "min-observations", 3,
		"Minimum prior observations required before a drop can be called")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local price-history database")
	return cmd
}

type watchSpec struct {
	CityID      int
	Destination string
	CheckIn     string
	Nights      int
	Adults      int
	Rooms       int
	Currency    string
}

func loadWatches(ctx context.Context, db *sql.DB) ([]watchSpec, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT city_id, destination, checkin, nights, adults, rooms, currency
        FROM price_watches ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("loading watches: %w", err)
	}
	out := make([]watchSpec, 0)
	for rows.Next() {
		var w watchSpec
		var dest, cur sql.NullString
		if err := rows.Scan(&w.CityID, &dest, &w.CheckIn, &w.Nights, &w.Adults, &w.Rooms, &cur); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning watch: %w", err)
		}
		w.Destination = dest.String
		w.Currency = cur.String
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

func recordObservations(ctx context.Context, db *sql.DB, w watchSpec, props []agoda.Property, at string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning observation transaction: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO price_observations
            (property_id, property_name, city_id, checkin, nights, adults, rooms,
             currency, price_advertised, price_all_in, observed_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("preparing observation insert: %w", err)
	}
	defer stmt.Close()

	added := 0
	for _, p := range props {
		if p.PriceAllIn <= 0 {
			continue
		}
		if _, err := stmt.ExecContext(ctx, p.PropertyID, p.Name, w.CityID, w.CheckIn,
			w.Nights, w.Adults, w.Rooms, w.Currency, p.PriceAdvertised, p.PriceAllIn, at); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("recording observation: %w", err)
		}
		added++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing observations: %w", err)
	}
	return added, nil
}

// detectDrops compares each property's newest price against the median of its
// prior observations. Median is used so a single spike cannot mask a real drop
// or manufacture a fake one.
func detectDrops(ctx context.Context, db *sql.DB, w watchSpec, props []agoda.Property, minPct float64, minObs int) ([]dropRow, error) {
	out := make([]dropRow, 0)
	for _, p := range props {
		if p.PriceAllIn <= 0 {
			continue
		}
		rows, err := db.QueryContext(ctx, `
            SELECT price_all_in FROM price_observations
            WHERE property_id = ? AND checkin = ? AND nights = ? AND adults = ?
              AND rooms = ? AND currency = ?
            ORDER BY observed_at DESC, id DESC LIMIT 30`,
			p.PropertyID, w.CheckIn, w.Nights, w.Adults, w.Rooms, w.Currency)
		if err != nil {
			return nil, fmt.Errorf("reading price history: %w", err)
		}
		hist := make([]float64, 0, 30)
		for rows.Next() {
			var v sql.NullFloat64
			if err := rows.Scan(&v); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scanning price history: %w", err)
			}
			if v.Valid && v.Float64 > 0 {
				hist = append(hist, v.Float64)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		// The newest row is the observation just recorded; exclude it so the
		// baseline is genuinely "trailing". The query orders by (observed_at,
		// id) rather than observed_at alone: observed_at has one-second
		// resolution, so two runs inside the same second would tie and leave
		// the ordering unspecified, which could drop an older row and keep the
		// current price in the baseline it is being compared against. The
		// just-inserted row always holds the highest id, so the composite sort
		// puts it first deterministically.
		if len(hist) > 0 {
			hist = hist[1:]
		}
		if len(hist) < minObs {
			continue
		}
		med := medianFloats(hist)
		if med <= 0 {
			continue
		}
		dropPct := round2Pct((med - p.PriceAllIn) / med * 100)
		if dropPct < minPct {
			continue
		}
		out = append(out, dropRow{
			PropertyID:   p.PropertyID,
			Name:         p.Name,
			CityID:       w.CityID,
			CheckIn:      w.CheckIn,
			Nights:       w.Nights,
			Currency:     w.Currency,
			CurrentAllIn: p.PriceAllIn,
			MedianAllIn:  round2Pct(med),
			DropAmount:   round2Pct(med - p.PriceAllIn),
			DropPct:      dropPct,
			Observations: len(hist),
		})
	}
	return out, nil
}

func medianFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func renderDrops(cmd *cobra.Command, res watchRunResult) error {
	out := cmd.OutOrStdout()
	if len(res.Drops) == 0 {
		fmt.Fprintf(out, "Checked %d watch(es), priced %d properties, recorded %d observations.\n",
			res.WatchesChecked, res.PropertiesPriced, res.ObservationsAdded)
		if res.Note != "" {
			fmt.Fprintf(out, "%s\n", res.Note)
		}
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tCHECK-IN\tNOW\tMEDIAN\tDROP")
	for _, d := range res.Drops {
		name := d.Name
		if len(name) > 38 {
			name = name[:35] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%.2f\t%.2f\t-%.1f%%\n", name, d.CheckIn, d.CurrentAllIn, d.MedianAllIn, d.DropPct)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d drop(s) at or beyond %.1f%% below trailing median.\n", len(res.Drops), res.MinDropPct)
	return nil
}
