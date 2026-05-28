// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/infotbm/internal/store"
	"github.com/spf13/cobra"
)

func newNovelScheduleChangesCmd(flags *rootFlags) *cobra.Command {
	var flagLine string
	var flagSince string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Report sync freshness and current routes within a time window",
		Long: `Check whether the local GTFS data was synced within a given time window
and list the current routes. Use this to verify data freshness before
relying on schedule queries. Requires a prior 'sync --full' run.`,
		Example: strings.Trim(`
  # Show route changes in the last 7 days
  infotbm-pp-cli schedule changes --since 7d

  # Show changes for a specific line in the last 24 hours
  infotbm-pp-cli schedule changes --line A --since 24h

  # Show all changes since last week
  infotbm-pp-cli schedule changes --since 1w
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check sync freshness and list current routes within the requested time window")
				return nil
			}
			if flagSince == "" {
				return usageErr(fmt.Errorf("--since is required (e.g. 7d, 24h, 1w)"))
			}

			sinceDur, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", flagSince, err))
			}
			cutoff := time.Now().Add(-sinceDur)

			// Open local store
			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("infotbm-pp-cli")
			}
			st, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w (run 'infotbm-pp-cli sync' first)", err)
			}
			defer st.Close()

			// Get sync state to see when last synced
			_, lastSynced, syncCount, syncErr := st.GetSyncState("routes")
			if syncErr != nil {
				return fmt.Errorf("reading sync state: %w", syncErr)
			}

			// Get all routes from store
			routes, err := st.List("routes", 0)
			if err != nil {
				return fmt.Errorf("listing routes: %w", err)
			}

			type routeInfo struct {
				ID        string `json:"id"`
				ShortName string `json:"short_name"`
				LongName  string `json:"long_name"`
				Color     string `json:"color,omitempty"`
				Type      string `json:"type,omitempty"`
			}

			// Parse routes and filter by line if requested
			lineUpper := strings.ToUpper(flagLine)
			currentRoutes := make([]routeInfo, 0)
			for _, raw := range routes {
				var route map[string]any
				if json.Unmarshal(raw, &route) != nil {
					continue
				}

				ri := routeInfo{}
				for _, key := range []string{"id", "route_id", "routeId"} {
					if v, ok := route[key].(string); ok {
						ri.ID = v
						break
					}
				}
				for _, key := range []string{"shortName", "short_name", "ShortName", "route_short_name"} {
					if v, ok := route[key].(string); ok {
						ri.ShortName = v
						break
					}
				}
				for _, key := range []string{"longName", "long_name", "LongName", "route_long_name"} {
					if v, ok := route[key].(string); ok {
						ri.LongName = v
						break
					}
				}
				if v, ok := route["color"].(string); ok {
					ri.Color = v
				}
				if v, ok := route["type"].(string); ok {
					ri.Type = v
				} else if v, ok := route["route_type"]; ok {
					ri.Type = fmt.Sprintf("%v", v)
				}

				if flagLine != "" && strings.ToUpper(ri.ShortName) != lineUpper {
					continue
				}
				currentRoutes = append(currentRoutes, ri)
			}

			// Since we can only compare against the sync state metadata (not historical
			// snapshots), report what we know: the current state and sync timing.
			// The sync_state table tracks last sync time, which lets us know if data
			// changed within the requested window.
			type changeReport struct {
				Status string `json:"status"`
				Detail string `json:"detail,omitempty"`
			}
			changes := make([]changeReport, 0)

			if lastSynced.Before(cutoff) {
				changes = append(changes, changeReport{
					Status: "stale",
					Detail: fmt.Sprintf("last sync at %s is older than requested window; no change detection possible without a fresh sync", lastSynced.Format(time.RFC3339)),
				})
			} else {
				changes = append(changes, changeReport{
					Status: "synced",
					Detail: fmt.Sprintf("data was synced at %s (%d routes); within the requested %s window", lastSynced.Format(time.RFC3339), syncCount, flagSince),
				})
			}

			view := struct {
				Line          string         `json:"line,omitempty"`
				Since         string         `json:"since"`
				CutoffTime    string         `json:"cutoff_time"`
				LastSyncedAt  string         `json:"last_synced_at"`
				SyncedCount   int            `json:"synced_route_count"`
				CurrentRoutes []routeInfo    `json:"current_routes"`
				Changes       []changeReport `json:"changes"`
			}{
				Line:          flagLine,
				Since:         flagSince,
				CutoffTime:    cutoff.Format(time.RFC3339),
				LastSyncedAt:  lastSynced.Format(time.RFC3339),
				SyncedCount:   syncCount,
				CurrentRoutes: currentRoutes,
				Changes:       changes,
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagLine, "line", "", "Line short name to filter by")
	cmd.Flags().StringVar(&flagSince, "since", "", "Time window to check for changes (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path override")
	return cmd
}
