// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/store"

	"github.com/spf13/cobra"
)

// newSessionsCmd is a parent for session-history analytics over the local store.
func newSessionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sessions",
		Short:       "Analyze charging sessions from local history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long:        "Queries over locally synced charge history. Run 'zaptec-pp-cli sync' first.",
	}
	cmd.AddCommand(newSessionsAnomaliesCmd(flags))
	return cmd
}

type anomalyRow struct {
	ID      string  `json:"id"`
	Charger string  `json:"charger"`
	KWh     float64 `json:"kwh"`
	Hours   float64 `json:"hours"`
	Start   string  `json:"start"`
	Reason  string  `json:"reason"`
}

// newSessionsAnomaliesCmd flags charge sessions that look wrong: near-zero
// energy, abnormally long, or with no recorded end. Pure threshold SQL over the
// local chargehistory table — no incumbent tool surfaces this.
func newSessionsAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minKWh float64
	var maxHours float64
	var dbPath string

	cmd := &cobra.Command{
		Use:         "anomalies",
		Short:       "Flag sessions with near-zero energy, abnormal duration, or no end",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  zaptec-pp-cli sessions anomalies --since 2026-04-01\n  zaptec-pp-cli sessions anomalies --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("zaptec-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zaptec-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `SELECT id,
			                 COALESCE(NULLIF(device_name,''), charger_id) AS charger,
			                 COALESCE(energy,0) AS kwh,
			                 start_date_time,
			                 end_date_time,
			                 (julianday(end_date_time) - julianday(start_date_time)) * 24.0 AS hours
			          FROM chargehistory WHERE start_date_time IS NOT NULL`
			var qargs []any
			if since != "" {
				query += " AND start_date_time >= ?"
				qargs = append(qargs, since)
			}
			query += " ORDER BY start_date_time DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying sessions: %w", err)
			}
			defer rows.Close()

			anomalies := []anomalyRow{}
			for rows.Next() {
				var id, charger string
				var kwh float64
				var start sql.NullString
				var end sql.NullString
				var hours sql.NullFloat64
				if err := rows.Scan(&id, &charger, &kwh, &start, &end, &hours); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				var reason string
				switch {
				case !end.Valid || end.String == "":
					reason = "no end recorded (possibly stuck)"
				case kwh < minKWh:
					reason = "near-zero energy delivered"
				case hours.Valid && hours.Float64 > maxHours:
					reason = fmt.Sprintf("very long session (%.1fh)", hours.Float64)
				default:
					continue
				}
				anomalies = append(anomalies, anomalyRow{
					ID: id, Charger: charger, KWh: round2(kwh),
					Hours: round2(hours.Float64), Start: start.String, Reason: reason,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), anomalies, flags)
			}
			if len(anomalies) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No anomalous sessions found.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "SESSION\tCHARGER\tKWH\tSTART\tREASON")
			for _, a := range anomalies {
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%s\t%s\n", a.ID, a.Charger, a.KWh, a.Start, a.Reason)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only sessions on/after this date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&minKWh, "min-kwh", 0.05, "Sessions below this energy are flagged near-zero")
	cmd.Flags().Float64Var(&maxHours, "max-hours", 24, "Sessions longer than this many hours are flagged")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
