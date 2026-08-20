// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

func newDelpasoCmd(flags *rootFlags) *cobra.Command {
	var limit, age int
	cmd := &cobra.Command{
		Use:         "delpaso <pickup> <dropoff>",
		Short:       "Quote Delpaso Car Hire directly (Malaga), full coverage included",
		Long:        "Fetch Delpaso's own prices for a Malaga pickup/dropoff window. Delpaso is a single-location Malaga supplier; every quote already includes total coverage / no excess. Dates are dd/mm/yyyy or yyyy-mm-dd.",
		Example:     "  rentalcarspain-pp-cli delpaso 20/08/2026 27/08/2026 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("delpaso needs <pickup> <dropoff>"))
			}
			if err := requireDateArgs("delpaso", args[0], args[1]); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			del := carsource.NewDelpaso(carHTTPClient(flags))
			offers, err := del.Quote(ctx, args[0], args[1], "", "", age) // pp:client-call
			if err != nil {
				return apiErr(err)
			}
			sort.SliceStable(offers, func(i, j int) bool { return offers[i].Total < offers[j].Total })
			if limit > 0 && len(offers) > limit {
				offers = offers[:limit]
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"supplier": "Delpaso", "count": len(offers), "offers": offers})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			return renderOffersTable(cmd, offers, "full-insurance")
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum car groups to return")
	cmd.Flags().IntVar(&age, "driver-age", 35, "Driver age; under 25 adds Delpaso's published young-driver surcharge (€12/day, €36–€100)")
	return cmd
}
