package cli

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List local seller listings with no recent engagement",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)
			rows, err := db.Query(`SELECT id, title, price_cents, distance_miles, url, seller_name, public_location, listed_at, updated_at, engagement_count, raw_json
				FROM listings
				WHERE COALESCE(NULLIF(listed_at, ''), updated_at) <= ? AND engagement_count = 0
				ORDER BY COALESCE(NULLIF(listed_at, ''), updated_at) ASC`, cutoff)
			if err != nil {
				return err
			}
			defer rows.Close()
			listings := []listingRow{}
			for rows.Next() {
				var l listingRow
				if err := rows.Scan(&l.ID, &l.Title, &l.PriceCents, &l.DistanceMiles, &l.URL, &l.SellerName, &l.PublicLocation, &l.ListedAt, &l.UpdatedAt, &l.EngagementCount, &l.RawJSON); err != nil {
					return err
				}
				listings = append(listings, l)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(listings)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Minimum listing age in days")
	return cmd
}
