// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"

	"github.com/spf13/cobra"
)

func newLocationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "locations [query]",
		Short:       "List supported Spanish airports, or resolve a place name to a DoYouSpain code",
		Long:        "With no argument, list the built-in Spanish airports the tool supports (IATA + DoYouSpain code). With a query, resolve any place name to its DoYouSpain location code via live autocomplete.",
		Example:     "  rentalcarspain-pp-cli locations\n  rentalcarspain-pp-cli locations Malaga",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// No query: list the built-in Spain airport table.
			if len(args) < 1 {
				airports := carsource.SpainAirports()
				if wantsMachineOutput(flags) || flags.asJSON {
					b, _ := json.Marshal(map[string]any{"count": len(airports), "airports": airports})
					return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
				}
				w := cmd.OutOrStdout()
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "IATA\tAIRPORT\tDOYOUSPAIN")
				for _, a := range airports {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", a.IATA, a.Name, a.DoYouSpainCode)
				}
				tw.Flush()
				fmt.Fprintln(w, "\nUse an IATA code, an airport name, or a DoYouSpain code as the location in 'search'/'best'.")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dys := carsource.NewDoYouSpain(carHTTPClient(flags))
			locs, err := dys.ResolveLocation(ctx, args[0]) // pp:client-call
			if err != nil {
				return apiErr(err)
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"count": len(locs), "locations": locs})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			if len(locs) == 0 {
				fmt.Fprintln(w, "No locations found.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "CODE\tLOCATION\tIATA")
			for _, l := range locs {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", l.Code, l.Description, l.IATA)
			}
			return tw.Flush()
		},
	}
	return cmd
}
