// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newNovelSeatsCmd(flags *rootFlags) *cobra.Command {
	var cap int
	var paywall, dbPath string
	cmd := &cobra.Command{
		Use:   "seats",
		Short: "Count paying members against a seat cap and list who paid.",
		Long:  "Use this command for current paid-seat counts and remaining capacity. Do NOT use this command for a yes/no launch decision against a target; use 'founding' instead.",
		Example: strings.Trim(`
  circle-pp-cli seats --cap 30 --json
  circle-pp-cli seats --paywall 1000969 --data-source live
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "seats")
			}
			if cap < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--cap must be non-negative"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("circle-pp-cli")
			}
			payers, source, err := loadCirclePayers(ctx, cmd, flags, dbPath, paywall)
			if err != nil {
				return err
			}
			result := struct {
				Paid      int           `json:"paid"`
				Cap       int           `json:"cap"`
				Remaining int           `json:"remaining"`
				Source    string        `json:"source"`
				Payers    []circlePayer `json:"payers"`
			}{len(payers), cap, max(cap-len(payers), 0), source, payers}
			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, result, flags)
			}
			if _, err := fmt.Fprintf(out, "%d paid / %d seats; %d remaining (%s)\n", result.Paid, cap, result.Remaining, source); err != nil {
				return err
			}
			tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "MEMBER ID\tEMAIL\tNAME\tPAYWALL\tFIRST PAID AT\tSOURCE"); err != nil {
				return err
			}
			for _, p := range payers {
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", p.MemberID, p.Email, p.Name, p.PaywallName, p.FirstPaidAt, p.Source); err != nil {
					return err
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&cap, "cap", 30, "Seat cap for the community.")
	cmd.Flags().StringVar(&paywall, "paywall", "", "Limit payers to this paywall ID.")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local synced store (default: the CLI's store)")
	return cmd
}
