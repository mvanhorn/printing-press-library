// tesla snap — fetch vehicle_data for one vehicle and persist to the local
// tesla_vehicle_states table. Run on a schedule (cron / launchd) to build the
// history the analytics features (timeline, vampire, cost) operate over.
//
// This is the documented Tesla-specific population path because the generator's
// default sync is empty (per-vehicle endpoints don't fit the flat-resource
// enumeration model).
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/store"
)

func newSnapCmd(flags *rootFlags) *cobra.Command {
	var (
		all      bool
		failFast bool
	)
	cmd := &cobra.Command{
		Use:   "snap [vehicle_id]",
		Short: "Capture a vehicle_data snapshot into the local store (for analytics)",
		Long: `Fetches /api/1/vehicles/{vehicle_id}/vehicle_data and writes a row to
tesla_vehicle_states. Run on a cron (e.g. every 30 minutes) so the timeline,
vampire, and cost analytics have data to stitch.

With --all, fetches /api/1/products first and snaps every vehicle on the account.`,
		Example:     "  tesla-pp-cli snap 5YJ3E1EA6XXXXXXXX --json\n  tesla-pp-cli snap --all --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "snapped": 0}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "args": args}, flags)
			}

			ctx := cmd.Context()
			s, err := store.OpenWithContext(ctx, defaultDBPath("tesla-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := store.EnsureTeslaSchema(ctx, s); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var vehicleIDs []string
			if all {
				raw, err := c.Get("/api/1/products", nil)
				if err != nil {
					return fmt.Errorf("products: %w", err)
				}
				var env struct {
					Response []struct {
						VIN       string `json:"vin"`
						VehicleID int64  `json:"vehicle_id"`
					} `json:"response"`
				}
				if err := json.Unmarshal(raw, &env); err != nil {
					return fmt.Errorf("parse products: %w", err)
				}
				for _, p := range env.Response {
					if p.VIN == "" {
						continue
					}
					vehicleIDs = append(vehicleIDs, fmt.Sprintf("%d", p.VehicleID))
				}
				if len(vehicleIDs) == 0 {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"snapped": 0, "note": "no vehicles on account"}, flags)
				}
			} else if len(args) == 0 {
				return cmd.Help()
			} else {
				vehicleIDs = args
			}

			type result struct {
				VehicleID string `json:"vehicle_id"`
				Status    string `json:"status"`
				Err       string `json:"error,omitempty"`
			}
			var results []result
			ok := 0
			for _, vid := range vehicleIDs {
				path := strings.ReplaceAll("/api/1/vehicles/{vehicle_id}/vehicle_data", "{vehicle_id}", vid)
				raw, gerr := c.Get(path, nil)
				if gerr != nil {
					results = append(results, result{VehicleID: vid, Status: "error", Err: gerr.Error()})
					if failFast {
						break
					}
					continue
				}
				if err := store.InsertTeslaVehicleState(ctx, s, vid, vid, raw); err != nil {
					results = append(results, result{VehicleID: vid, Status: "stored_error", Err: err.Error()})
					if failFast {
						break
					}
					continue
				}
				results = append(results, result{VehicleID: vid, Status: "ok"})
				ok++
			}
			out := map[string]any{
				"snapped":   ok,
				"attempted": len(vehicleIDs),
				"results":   results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Fetch /api/1/products and snap every vehicle on the account")
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop on the first error instead of continuing")
	return cmd
}
