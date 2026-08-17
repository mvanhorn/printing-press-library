// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: find the soonest opening for a practitioner + treatment,
// paging past Jane's 7-day availability window cap.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// resolveLocationID returns the location id to use: the explicit flag if set,
// otherwise the clinic's sole location (auto-resolved), else an error asking
// the user to pick.
func resolveLocationID(cmd *cobra.Command, flags *rootFlags, explicit int) (int, error) {
	if explicit > 0 {
		return explicit, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return 0, err
	}
	data, err := c.Get(cmd.Context(), "/api/v2/locations", nil)
	if err != nil {
		return 0, classifyAPIError(err, flags)
	}
	var locs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &locs); err != nil {
		return 0, fmt.Errorf("parsing locations: %w", err)
	}
	if len(locs) == 1 {
		return locs[0].ID, nil
	}
	if len(locs) == 0 {
		return 0, fmt.Errorf("no locations found for this clinic")
	}
	msg := "multiple locations; pass --location <id>:\n"
	for _, l := range locs {
		msg += fmt.Sprintf("  %d  %s\n", l.ID, l.Name)
	}
	return 0, usageErr(fmt.Errorf("%s", msg))
}

func newNovelNextOpeningCmd(flags *rootFlags) *cobra.Command {
	var treatment, staff, location, horizon int
	var fromStr string

	cmd := &cobra.Command{
		Use:   "next-opening",
		Short: "Find the soonest available slot for a practitioner + treatment, paging past Jane's 7-day availability cap.",
		Long: `Scan availability for the soonest bookable opening with a given
practitioner and treatment. Jane's availability API only answers 7 days at a
time; this pages consecutive windows up to --horizon-days so you can ask "when
is the earliest I can get in" in one command.

Find treatment and staff IDs with 'treatments' and 'staff'.`,
		Example:     "  janeapp-pp-cli next-opening --clinic embophysio --treatment 1 --staff 1\n  janeapp-pp-cli next-opening --treatment 1 --staff 1 --horizon-days 60 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if flags.dryRun {
				return nil
			}
			if treatment <= 0 {
				return usageErr(fmt.Errorf("required flag --treatment not set (see 'treatments')"))
			}
			if staff <= 0 {
				return usageErr(fmt.Errorf("required flag --staff not set (see 'staff')"))
			}
			from := time.Now()
			if fromStr != "" {
				t, err := time.Parse("2006-01-02", fromStr)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --from %q: use YYYY-MM-DD", fromStr))
				}
				from = t
			}
			locID, err := resolveLocationID(cmd, flags, location)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			op, err := findNextOpening(cmd.Context(), c, locID, treatment, staff, from, horizon)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if op == nil {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"found": false, "horizon_days": horizon}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No openings found in the next %d days.\n", horizon)
				return nil
			}
			result := map[string]any{
				"found":           true,
				"start_at":        op.StartAt,
				"end_at":          op.EndAt,
				"duration":        op.Duration,
				"staff_member_id": op.StaffMemberID,
				"treatment_id":    op.TreatmentID,
				"location_id":     op.LocationID,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				b, _ := json.Marshal(result)
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Next opening: %s (%d min)\n", op.StartAt, op.Duration/60)
			fmt.Fprintf(cmd.OutOrStdout(), "Book it:\n  janeapp-pp-cli book --treatment %d --staff %d --location %d --at %s\n",
				op.TreatmentID, op.StaffMemberID, op.LocationID, strconv.Quote(op.StartAt))
			return nil
		},
	}
	cmd.Flags().IntVar(&treatment, "treatment", 0, "Treatment ID (see 'treatments')")
	cmd.Flags().IntVar(&staff, "staff", 0, "Practitioner ID (see 'staff')")
	cmd.Flags().IntVar(&location, "location", 0, "Location ID (auto-resolved when the clinic has one location)")
	cmd.Flags().IntVar(&horizon, "horizon-days", 30, "How many days ahead to scan (pages 7-day windows)")
	cmd.Flags().StringVar(&fromStr, "from", "", "Start scanning from this date (YYYY-MM-DD; default today)")
	return cmd
}
