// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel features (not generator output).
// PATCH: local quote ledger (SQLite) powering watch / history / log / search —
// every quote snapshot persists so prices can be tracked over time offline.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

func ledgerPath() string {
	return filepath.Join(filepath.Dir(defaultDBPath("blacklane-pp-cli")), "quote-ledger.db")
}

func openLedger() (*sql.DB, error) {
	path := ledgerPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating ledger directory: %w", err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS quotes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts TEXT NOT NULL,
		route_key TEXT NOT NULL,
		service_type TEXT NOT NULL,
		depart_at TEXT NOT NULL,
		pickup TEXT NOT NULL,
		dropoff TEXT,
		class TEXT NOT NULL,
		gross REAL NOT NULL,
		currency TEXT NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func routeKey(serviceType, pickup, dropoff string) string {
	return strings.ToLower(strings.Join([]string{serviceType, pickup, dropoff}, "|"))
}

// recordQuote persists every priced class of a quote to the ledger.
func recordQuote(db *sql.DB, r *quoteResult) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	drop := ""
	if r.Dropoff != nil {
		drop = r.Dropoff.Address
	}
	rk := routeKey(r.ServiceType, r.Pickup.Address, drop)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, p := range r.Packages {
		if _, err := tx.Exec(
			`INSERT INTO quotes(ts,route_key,service_type,depart_at,pickup,dropoff,class,gross,currency) VALUES(?,?,?,?,?,?,?,?,?)`,
			ts, rk, r.ServiceType, r.DepartAt, r.Pickup.Address, drop, p.Title, amountFloat(p.GrossAmount), p.Currency,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ---- watch: take a price snapshot and report change vs the prior one ----

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var at string
	var hourly int

	cmd := &cobra.Command{
		Use:   "watch <pickup> [dropoff]",
		Short: "Snapshot a route's price into the local ledger and report any change",
		Long:  "Quote a route, save every class to the local SQLite ledger, and report the cheapest-class delta\nsince the last snapshot of the same route. Run on a schedule (cron) to track fares over time.",
		Example: strings.Trim(`
  blacklane-pp-cli watch "JFK" "Times Square NYC" --at 2026-06-20T15:00
  blacklane-pp-cli watch "Union Square SF" --hourly 3 --at 2026-06-20T09:00 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			departAt, err := normalizeDepartAt(at)
			if err != nil {
				return err
			}
			pickup, err := resolveLocation(args[0], 0, 0, false, flags.timeout)
			if err != nil {
				return err
			}
			var dropoff *geoPoint
			st, secs := "transfer", 0
			if hourly > 0 {
				s, err := hourlySeconds(hourly)
				if err != nil {
					return err
				}
				st, secs = "hourly", s
			} else {
				if len(args) < 2 {
					return fmt.Errorf("transfer watch needs a dropoff (or use --hourly <hours>)")
				}
				d, err := resolveLocation(args[1], 0, 0, false, flags.timeout)
				if err != nil {
					return err
				}
				dropoff = &d
			}
			if dryRunOK(flags) {
				return nil
			}
			r, err := doQuote(flags, st, departAt, secs, pickup, dropoff)
			if err != nil {
				return err
			}
			db, err := openLedger()
			if err != nil {
				return err
			}
			defer db.Close()

			drop := ""
			if dropoff != nil {
				drop = dropoff.Address
			}
			rk := routeKey(st, pickup.Address, drop)
			var prev float64
			var prevTS string
			_ = db.QueryRow(
				`SELECT gross, ts FROM quotes WHERE route_key=? ORDER BY ts DESC, gross ASC LIMIT 1`, rk,
			).Scan(&prev, &prevTS)

			if err := recordQuote(db, r); err != nil {
				return err
			}
			cheapest := r.Packages[0]
			cur := amountFloat(cheapest.GrossAmount)
			out := map[string]any{
				"departAt":      departAt,
				"cheapestClass": cheapest.Title,
				"grossAmount":   cheapest.GrossAmount,
				"currency":      cheapest.Currency,
			}
			if prevTS != "" {
				out["previousGross"] = fmt.Sprintf("%.2f", prev)
				out["delta"] = fmt.Sprintf("%+.2f", cur-prev)
				out["previousAt"] = prevTS
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s %s", cheapest.Title, cheapest.GrossAmount, cheapest.Currency)
			if prevTS != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  (%+.2f vs last snapshot)", cur-prev)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "Pickup datetime (required)")
	cmd.Flags().IntVar(&hourly, "hourly", 0, "Hourly booking of N hours instead of a transfer")
	return cmd
}

// ---- log / history / search over the ledger ----

type ledgerRow struct {
	TS       string  `json:"ts"`
	Service  string  `json:"serviceType"`
	DepartAt string  `json:"departAt"`
	Pickup   string  `json:"pickup"`
	Dropoff  string  `json:"dropoff"`
	Class    string  `json:"class"`
	Gross    float64 `json:"gross"`
	Currency string  `json:"currency"`
}

func queryLedger(where string, arg any, limit int) ([]ledgerRow, error) {
	db, err := openLedger()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	q := `SELECT ts,service_type,depart_at,pickup,dropoff,class,gross,currency FROM quotes`
	var rows *sql.Rows
	if where != "" {
		q += " WHERE " + where + " ORDER BY id DESC LIMIT ?"
		rows, err = db.Query(q, arg, limit)
	} else {
		q += " ORDER BY id DESC LIMIT ?"
		rows, err = db.Query(q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		var drop sql.NullString
		if err := rows.Scan(&r.TS, &r.Service, &r.DepartAt, &r.Pickup, &drop, &r.Class, &r.Gross, &r.Currency); err != nil {
			return nil, err
		}
		r.Dropoff = drop.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func renderLedger(cmd *cobra.Command, flags *rootFlags, rows []ledgerRow) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return emitDomainList(cmd, flags, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No quotes recorded yet. Run 'blacklane-pp-cli watch ...' to start a ledger.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tROUTE\tCLASS\tPRICE")
	for _, r := range rows {
		route := r.Pickup
		if r.Dropoff != "" {
			route += " → " + r.Dropoff
		}
		if len(route) > 44 {
			route = route[:43] + "…"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f %s\n", r.TS, route, r.Class, r.Gross, r.Currency)
	}
	w.Flush()
	return nil
}

func newNovelLogCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "log",
		Short:       "List recent quotes recorded in the local ledger",
		Example:     "  blacklane-pp-cli log --limit 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := queryLedger("", nil, limit)
			if err != nil {
				return err
			}
			return renderLedger(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "search <term>",
		Short:       "Search recorded quotes by pickup, dropoff, or class",
		Example:     "  blacklane-pp-cli search JFK --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			like := "%" + strings.ToLower(args[0]) + "%"
			// 3-way LIKE over pickup/dropoff/class needs its own query (queryLedger
			// is single-arg), so run it directly here.
			db, derr := openLedger()
			if derr != nil {
				return derr
			}
			defer db.Close()
			rs, qerr := db.Query(`SELECT ts,service_type,depart_at,pickup,dropoff,class,gross,currency FROM quotes
				WHERE lower(pickup) LIKE ? OR lower(dropoff) LIKE ? OR lower(class) LIKE ?
				ORDER BY id DESC LIMIT ?`, like, like, like, limit)
			if qerr != nil {
				return qerr
			}
			defer rs.Close()
			var rows []ledgerRow
			for rs.Next() {
				var r ledgerRow
				var drop sql.NullString
				if err := rs.Scan(&r.TS, &r.Service, &r.DepartAt, &r.Pickup, &drop, &r.Class, &r.Gross, &r.Currency); err != nil {
					return err
				}
				r.Dropoff = drop.String
				rows = append(rows, r)
			}
			if err := rs.Err(); err != nil {
				return err
			}
			return renderLedger(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to return")
	return cmd
}
