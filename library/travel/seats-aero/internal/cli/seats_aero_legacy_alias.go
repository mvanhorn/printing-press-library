package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSeatsAeroLegacyAliasCmd(flags))
	})
}

// pp:data-source auto
func newSeatsAeroLegacyAliasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "seats-aero-partner-search",
		Short:  "Deprecated alias for awards",
		Hidden: true,
		Example: `  # Deprecated spelling of the awards search (use 'awards --cabins business' instead)
  seats-aero-pp-cli seats-aero-partner-search --origin-airport JFK --destination-airport NRT --cabin business --json`,
		Deprecated:  "",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.Root().ErrOrStderr(), "warning: 'seats-aero-partner-search' is deprecated; use 'awards' (--cabin is now --cabins)")
			flags.freshnessMeta = autoRefreshIfStale(cmd.Context(), flags, []string{"awards"})
			awardsCmd, _, err := cmd.Root().Find([]string{"awards"})
			if err != nil || awardsCmd == nil || awardsCmd.Name() != "awards" {
				awardsCmd = nil
				for _, candidate := range cmd.Root().Commands() {
					if candidate.Name() == "awards" {
						awardsCmd = candidate
						break
					}
				}
			}
			if awardsCmd == nil || awardsCmd.RunE == nil {
				return fmt.Errorf("awards command is unavailable")
			}
			if cmd.Flags().Changed("departure-date") {
				fmt.Fprintln(cmd.Root().ErrOrStderr(), "warning: --departure-date is ignored (it was never a Seats.aero parameter)")
			}
			mapping := map[string]string{"cabin": "cabins"}
			for _, oldName := range []string{"origin-airport", "destination-airport", "cabin", "start-date", "end-date", "cursor", "skip", "take", "order-by", "include-trips", "only-direct-flights", "carriers", "all"} {
				if !cmd.Flags().Changed(oldName) {
					continue
				}
				newName := oldName
				if mapped, ok := mapping[oldName]; ok {
					newName = mapped
				}
				value, valueErr := cmd.Flags().GetString(oldName)
				if valueErr != nil {
					value = cmd.Flags().Lookup(oldName).Value.String()
				}
				if err := awardsCmd.Flags().Set(newName, value); err != nil {
					return err
				}
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			awardsCmd.SetContext(ctx)
			return awardsCmd.RunE(awardsCmd, args)
		},
	}

	cmd.Flags().String("origin-airport", "", "A list of origin airports.")
	cmd.Flags().String("destination-airport", "", "A list of destination airports.")
	cmd.Flags().String("cabin", "", "Cabin class (renamed to --cabins).")
	cmd.Flags().String("start-date", "", "Start date in YYYY-MM-DD format.")
	cmd.Flags().String("end-date", "", "End date in YYYY-MM-DD format.")
	cmd.Flags().String("departure-date", "", "Deprecated and ignored.")
	cmd.Flags().String("cursor", "", "Pagination cursor.")
	cmd.Flags().Int("skip", 0, "How many results to skip.")
	cmd.Flags().Int("take", 500, "Maximum results to return.")
	cmd.Flags().String("order-by", "", "Result ordering.")
	cmd.Flags().Bool("include-trips", false, "Include trip-level details.")
	cmd.Flags().Bool("only-direct-flights", false, "Only return direct flights.")
	cmd.Flags().String("carriers", "", "Comma-separated carriers.")
	cmd.Flags().Bool("all", false, "Fetch all pages.")
	return cmd
}
