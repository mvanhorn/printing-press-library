// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/store"

	"github.com/spf13/cobra"
)

type staleRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Online       bool   `json:"online"`
	Mode         string `json:"operation_mode"`
	Installation string `json:"installation"`
	Reason       string `json:"reason"`
}

// newChargersStaleCmd lists chargers that are offline, disconnected, or whose
// locally synced record is older than --minutes. A pure local-store query; the
// Home Assistant integration polls but has no offline alerting. Run `sync` first.
func newChargersStaleCmd(flags *rootFlags) *cobra.Command {
	var minutes int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "List chargers that are offline, disconnected, or not reporting",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Flag chargers that need attention: those reporting offline, those in an
Unknown/Disconnected operation mode, or — when --minutes is set — those whose
locally synced record is older than that many minutes. Reads the local store;
run 'zaptec-pp-cli sync' first.`,
		Example: "  zaptec-pp-cli chargers stale\n  zaptec-pp-cli chargers stale --minutes 30 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("zaptec-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zaptec-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, COALESCE(name,'') AS name, COALESCE(is_online,1) AS is_online,
				        COALESCE(operating_mode,0) AS operating_mode,
				        COALESCE(installation_name,'') AS installation,
				        CAST((julianday('now') - julianday(synced_at)) * 24 * 60 AS INTEGER) AS age_minutes
				 FROM chargers ORDER BY name`)
			if err != nil {
				return fmt.Errorf("querying chargers: %w", err)
			}
			defer rows.Close()

			stale := []staleRow{}
			for rows.Next() {
				var id, name, installation string
				var isOnline, mode int
				var ageMin sql.NullInt64
				if err := rows.Scan(&id, &name, &isOnline, &mode, &installation, &ageMin); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				var reason string
				switch {
				case isOnline == 0:
					reason = "offline"
				case mode == 1:
					reason = "disconnected"
				case mode == 0:
					reason = "unknown operation mode"
				case minutes > 0 && ageMin.Valid && ageMin.Int64 > int64(minutes):
					reason = fmt.Sprintf("no fresh data (%d min old)", ageMin.Int64)
				default:
					continue
				}
				stale = append(stale, staleRow{
					ID: id, Name: name, Online: isOnline != 0,
					Mode: chargerOperationModeName(mode), Installation: installation, Reason: reason,
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), stale, flags)
			}
			if len(stale) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "All chargers are online and healthy.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "CHARGER\tMODE\tINSTALLATION\tREASON")
			for _, s := range stale {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name, s.Mode, s.Installation, s.Reason)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&minutes, "minutes", 0, "Also flag chargers whose synced record is older than N minutes")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
