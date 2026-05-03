package cli

import (
	"github.com/spf13/cobra"
)

// pp:novel-static-reference
func newCheapestCmd(flags *rootFlags) *cobra.Command {
	var checkin, checkout, backend string
	var guests, maxDirect int
	cmd := &cobra.Command{
		Use:         "cheapest <listing-url>",
		Short:       "Find the host direct booking site and cheapest option for a listing",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := stripURLArg(args[0])
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), dryRunCheapest(target), flags)
			}
			out, err := computeCheapest(cmd.Context(), target, cheapestParams{Checkin: checkin, Checkout: checkout, Guests: guests, SearchBackend: backend, MaxDirectResults: maxDirect})
			if err != nil {
				return classifyAPIError(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&checkin, "checkin", "", "Arrival date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Departure date YYYY-MM-DD")
	cmd.Flags().IntVar(&guests, "guests", 1, "Guest count")
	cmd.Flags().StringVar(&backend, "search-backend", "", "Search backend: parallel, ddg, brave, tavily")
	cmd.Flags().IntVar(&maxDirect, "max-direct-results", 5, "Maximum direct-site search results")
	return cmd
}
