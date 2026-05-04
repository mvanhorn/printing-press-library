// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"
	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/store"

	"github.com/spf13/cobra"
)

// compareOutput is the wide-table layout: rows are field names, columns
// are listings.
type compareOutput struct {
	Fields   []string      `json:"fields"`
	Listings []apt.Listing `json:"listings"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "compare <url-or-id> <url-or-id>...",
		Short:       "Pivot 2-8 listings into a wide table — one column per listing.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli compare the-domain-austin-tx the-grove-austin-tx --json
  apartments-pp-cli compare https://www.apartments.com/foo/abc123/ the-grove-austin-tx
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if len(args) > 8 {
				return usageErr(fmt.Errorf("at most 8 listings can be compared at once"))
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var listings []apt.Listing
			for _, arg := range args {
				li, fetchErr := loadOrFetchListing(db, c, arg)
				if fetchErr != nil {
					return fetchErr
				}
				listings = append(listings, li)
			}

			out := compareOutput{
				Fields: []string{
					"property_id", "title", "address.city", "address.state",
					"beds", "baths", "max_rent", "sqft", "price_per_sqft",
					"amenities_count", "phone",
				},
				Listings: listings,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// loadOrFetchListing reads a listing from the local cache by property
// ID; on miss, fetches via the client and caches the result.
func loadOrFetchListing(db *store.Store, c *client.Client, arg string) (apt.Listing, error) {
	propertyID := arg
	listingURL := arg
	relPath := "/" + strings.Trim(arg, "/") + "/"
	if strings.HasPrefix(arg, "http") {
		propertyID = apt.ListingURLToPropertyID(arg)
		idx := strings.Index(arg, "://")
		if idx >= 0 {
			rest := arg[idx+3:]
			slash := strings.Index(rest, "/")
			if slash >= 0 {
				relPath = rest[slash:]
			}
		}
	} else {
		listingURL = "https://www.apartments.com" + relPath
	}

	if propertyID != "" {
		var data string
		err := db.DB().QueryRow(`SELECT data FROM listing WHERE id = ?`, propertyID).Scan(&data)
		if err == nil && data != "" {
			var li apt.Listing
			if json.Unmarshal([]byte(data), &li) == nil && li.URL != "" {
				return li, nil
			}
		}
	}

	body, gerr := c.Get(relPath, nil)
	if gerr != nil {
		return apt.Listing{}, classifyAPIError(gerr)
	}
	li, perr := apt.ParseListing([]byte(body), listingURL)
	if perr != nil {
		return apt.Listing{}, apiErr(perr)
	}
	if li.PropertyID != "" {
		if raw, mErr := json.Marshal(li); mErr == nil {
			_ = db.UpsertListing(raw)
		}
	}
	return li, nil
}
