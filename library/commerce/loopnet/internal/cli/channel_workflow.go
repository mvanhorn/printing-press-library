// Workflow command. Hand-written (replaces the generated generic archive
// workflow): LoopNet has no syncable JSON resources, so the old `archive`
// subcommand was a no-op. `workflow status` reports what the local store
// holds — listing/property counts, distinct markets, and sync history.
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newWorkflowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Inspect the local LoopNet store",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowStatusCmd(flags))
	return cmd
}

func newWorkflowStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show what the local store holds — listings, properties, and synced markets",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  loopnet-pp-cli workflow status
  loopnet-pp-cli workflow status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := lnOpenStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			listingCount, _ := st.Count(lnResourceListing)
			propertyCount, _ := st.Count(lnResourceProperty)

			type marketRow struct {
				Location     string `json:"location"`
				PropertyType string `json:"property_type"`
				ListingType  string `json:"listing_type"`
				Listings     int    `json:"listings"`
				Syncs        int    `json:"syncs"`
			}
			rows, err := st.DB().Query(
				`SELECT location, property_type, listing_type,
				        COUNT(DISTINCT listing_id), COUNT(DISTINCT observed_at)
				 FROM ln_observations
				 GROUP BY location, property_type, listing_type
				 ORDER BY location, property_type, listing_type`)
			var markets []marketRow
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var m marketRow
					if rows.Scan(&m.Location, &m.PropertyType, &m.ListingType, &m.Listings, &m.Syncs) == nil {
						markets = append(markets, m)
					}
				}
			}
			sort.Slice(markets, func(i, j int) bool { return markets[i].Location < markets[j].Location })

			status := map[string]any{
				"listings":   listingCount,
				"properties": propertyCount,
				"markets":    markets,
				"store_path": st.Path(),
			}
			if len(markets) == 0 && (flags == nil || !flags.asJSON) {
				fmt.Fprintln(cmd.OutOrStdout(), "Local store is empty. Run 'loopnet-pp-cli sync <location>' to populate it.")
				return nil
			}
			return flags.printJSON(cmd, status)
		},
	}
	return cmd
}
