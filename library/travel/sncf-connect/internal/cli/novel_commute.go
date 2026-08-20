// Copyright 2026 jmbernabotto and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/travel/sncf-connect/internal/store"
	"github.com/spf13/cobra"
)

func newCommuteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commute",
		Short: "Manage saved commute routes",
		Long: `Save and check your daily commute route.

Save once with 'commute save', then run 'commute check' each morning from cron
to get upcoming departures and live disruption status. Exits 2 when a disruption
is active on your route — scriptable for notifications.`,
	}
	cmd.AddCommand(newCommuteSaveCmd(flags))
	cmd.AddCommand(newCommuteCheckCmd(flags))
	cmd.AddCommand(newCommuteListCmd(flags))
	return cmd
}

func openCommuteStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("sncf-connect-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	_, err = s.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS commute_routes (
		name TEXT PRIMARY KEY,
		coverage TEXT NOT NULL,
		from_id TEXT NOT NULL,
		to_id TEXT NOT NULL,
		created_at DATETIME DEFAULT (datetime('now'))
	)`)
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("ensuring commute_routes table: %w", err)
	}
	return s, nil
}

func newCommuteSaveCmd(flags *rootFlags) *cobra.Command {
	var from, to, name, coverage, dbPath string

	cmd := &cobra.Command{
		Use:         "save",
		Short:       "Save a commute route for daily checking",
		Annotations: map[string]string{"mcp:read-only": "false"},
		Example: `  sncf-connect-pp-cli commute save --from "stop_area:SNCF:87686006" --to "stop_area:SNCF:87723197" --name morning
  sncf-connect-pp-cli commute save --from "stop_area:SNCF:87686006" --to "stop_area:SNCF:87723197"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" || to == "" {
				return fmt.Errorf("--from and --to are required")
			}
			s, err := openCommuteStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()

			_, err = s.DB().ExecContext(cmd.Context(),
				`INSERT OR REPLACE INTO commute_routes (name, coverage, from_id, to_id) VALUES (?, ?, ?, ?)`,
				name, coverage, from, to)
			if err != nil {
				return fmt.Errorf("saving commute route: %w", err)
			}

			if flags.asJSON {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"status":   "saved",
					"name":     name,
					"coverage": coverage,
					"from":     from,
					"to":       to,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved commute route %q (%s → %s).\nRun: sncf-connect-pp-cli commute check --name %s\n",
				name, from, to, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Departure stop area or stop point ID")
	cmd.Flags().StringVar(&to, "to", "", "Arrival stop area or stop point ID")
	cmd.Flags().StringVar(&name, "name", "default", "Route nickname")
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/sncf-connect-pp-cli/data.db)")
	return cmd
}

func newCommuteCheckCmd(flags *rootFlags) *cobra.Command {
	var name, route, dbPath string
	var count int

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check next departures and disruptions for your saved commute",
		Long: `Fetches live departures and disruption status for your saved commute route.

Exit codes:
  0  No active disruptions, departures listed.
  1  No saved route found — run commute save first.
  2  Active disruption found on your route.`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1,2",
		},
		Example: `  sncf-connect-pp-cli commute check
  sncf-connect-pp-cli commute check --name morning --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if route != "" {
				name = route
			}
			s, err := openCommuteStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()

			var coverage, fromID, toID string
			err = s.DB().QueryRowContext(cmd.Context(),
				`SELECT coverage, from_id, to_id FROM commute_routes WHERE name = ?`, name).
				Scan(&coverage, &fromID, &toID)
			if err == sql.ErrNoRows {
				return fmt.Errorf("no saved route named %q — run 'sncf-connect-pp-cli commute save --name %s --from ... --to ...' first", name, name)
			}
			if err != nil {
				return fmt.Errorf("reading commute route: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			depsPath := fmt.Sprintf("/coverage/%s/stop_areas/%s/departures", coverage, fromID)
			depsParams := map[string]string{
				"count": fmt.Sprintf("%d", count),
			}
			depsData, _, depsErr := resolveRead(cmd.Context(), c, flags, "departures", true, depsPath, depsParams, nil)

			disrPath := fmt.Sprintf("/coverage/%s/stop_areas/%s/disruptions", coverage, fromID)
			disrData, _, disrErr := resolveRead(cmd.Context(), c, flags, "disruptions", true, disrPath, nil, nil)
			if disrErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch disruptions: %v\n", disrErr)
			}

			hasDisruption := false
			var disruptions []map[string]any
			if disrData != nil {
				disruptions = navitiaItems(disrData, "disruptions")
				for _, d := range disruptions {
					if status, _ := d["status"].(string); status == "active" {
						hasDisruption = true
						break
					}
				}
			}

			if flags.asJSON {
				var deps []map[string]any
				if depsErr == nil && depsData != nil {
					deps = navitiaItems(depsData, "departures")
				}
				out := map[string]any{
					"route":          name,
					"from":           fromID,
					"to":             toID,
					"coverage":       coverage,
					"departures":     deps,
					"disruptions":    disruptions,
					"has_disruption": hasDisruption,
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(out)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Commute: %s → %s (%s)\n\n", fromID, toID, coverage)

				if hasDisruption {
					fmt.Fprintf(cmd.OutOrStdout(), "DISRUPTION ACTIVE on your route:\n")
					for _, d := range disruptions {
						if status, _ := d["status"].(string); status == "active" {
							fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", extractDisruptionMessage(d))
						}
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}

				if depsErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch departures: %v\n", depsErr)
				} else if depsData != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "Next departures from %s:\n", fromID)
					for i, d := range navitiaItems(depsData, "departures") {
						if i >= count {
							break
						}
						fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s → %s\n",
							extractDepartureTime(d), extractDepartureLine(d), extractDepartureDirection(d))
					}
				}
			}

			if hasDisruption {
				return &cliError{code: 2, err: fmt.Errorf("active disruption on commute route %q", name)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "default", "Route nickname to check")
	cmd.Flags().StringVar(&route, "route", "", "Route nickname (alias for --name)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/sncf-connect-pp-cli/data.db)")
	cmd.Flags().IntVar(&count, "count", 5, "Number of next departures to show")
	return cmd
}

func newCommuteListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved commute routes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  sncf-connect-pp-cli commute list\n  sncf-connect-pp-cli commute list --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openCommuteStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()

			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT name, coverage, from_id, to_id, created_at FROM commute_routes ORDER BY name`)
			if err != nil {
				return fmt.Errorf("listing commute routes: %w", err)
			}
			defer rows.Close()

			type route struct {
				Name      string `json:"name"`
				Coverage  string `json:"coverage"`
				From      string `json:"from"`
				To        string `json:"to"`
				CreatedAt string `json:"created_at"`
			}
			var routes []route
			for rows.Next() {
				var r route
				if err := rows.Scan(&r.Name, &r.Coverage, &r.From, &r.To, &r.CreatedAt); err != nil {
					return err
				}
				routes = append(routes, r)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(routes)
			}
			if len(routes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No saved commute routes. Use 'commute save' to add one.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %-40s %s\n", "NAME", "COVERAGE", "FROM", "TO")
			for _, r := range routes {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-12s %-40s %s\n", r.Name, r.Coverage, r.From, r.To)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/sncf-connect-pp-cli/data.db)")
	return cmd
}

func extractDisruptionMessage(d map[string]any) string {
	if msgs, ok := d["messages"].([]any); ok && len(msgs) > 0 {
		if m, ok := msgs[0].(map[string]any); ok {
			if text, ok := m["text"].(string); ok {
				return text
			}
		}
	}
	if cause, ok := d["cause"].(string); ok && cause != "" {
		return cause
	}
	if id, ok := d["id"].(string); ok {
		return id
	}
	return "disruption"
}

func extractDepartureTime(d map[string]any) string {
	if sdt, ok := d["stop_date_time"].(map[string]any); ok {
		if dt, ok := sdt["departure_date_time"].(string); ok && len(dt) >= 13 {
			return dt[9:11] + ":" + dt[11:13]
		}
	}
	return "--:--"
}

func extractDepartureLine(d map[string]any) string {
	if di, ok := d["display_informations"].(map[string]any); ok {
		if code, ok := di["code"].(string); ok && code != "" {
			return code
		}
		if label, ok := di["label"].(string); ok {
			return label
		}
	}
	return "?"
}

func extractDepartureDirection(d map[string]any) string {
	if di, ok := d["display_informations"].(map[string]any); ok {
		if dir, ok := di["direction"].(string); ok {
			return dir
		}
	}
	return "?"
}
