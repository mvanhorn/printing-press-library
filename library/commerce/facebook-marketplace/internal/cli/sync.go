package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var listingsPath string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Populate the local Marketplace SQLite mirror",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			imported := 0
			if listingsPath != "" {
				listings, err := loadListingsFile(listingsPath)
				if err != nil {
					return err
				}
				for _, listing := range listings {
					if err := upsertListing(db, listing); err != nil {
						return err
					}
					imported++
				}
			}
			var listingCount, watchCount, matchCount int
			_ = db.QueryRow(`SELECT COUNT(*) FROM listings`).Scan(&listingCount)
			_ = db.QueryRow(`SELECT COUNT(*) FROM watches`).Scan(&watchCount)
			_ = db.QueryRow(`SELECT COUNT(*) FROM matches`).Scan(&matchCount)
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"imported": imported,
				"store": map[string]any{
					"listings": listingCount,
					"watches":  watchCount,
					"matches":  matchCount,
				},
			})
		},
	}
	cmd.Flags().StringVar(&listingsPath, "listings", "", "Optional JSON file of listings to import into the local mirror")
	return cmd
}
