// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// extractPathFromArg extracts a Stingray-friendly path from a Redfin listing
// URL. Accepts:
//   - Full URL:  https://www.redfin.com/TX/Austin/123-Main-St-78704/home/12345
//   - Path:      /TX/Austin/123-Main-St-78704/home/12345
//   - Bare path: TX/Austin/123-Main-St-78704/home/12345
func extractPathFromArg(arg string) string {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return ""
	}
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		if u, err := url.Parse(arg); err == nil {
			return u.Path
		}
	}
	if !strings.HasPrefix(arg, "/") {
		arg = "/" + arg
	}
	return arg
}

func newListingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listing [property-url-or-path]",
		Short: "Fetch and merge full Stingray detail for one listing.",
		Long: `Resolve a Redfin listing URL through three Stingray detail endpoints
(initialInfo, aboveTheFold, belowTheFold), strip each {}&& CSRF prefix,
and merge the responses into a single Listing record. Falls back
gracefully if individual endpoints fail (e.g., belowTheFold 403s).

The merged Listing is also cached in the local homes table by URL so
later transcendence commands (compare, comps, rank) can read it offline.`,
		Example: `  redfin-pp-cli listing https://www.redfin.com/TX/Austin/123-Main-St-78704/home/12345
  redfin-pp-cli listing /TX/Austin/123-Main-St-78704/home/12345 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			path := extractPathFromArg(args[0])
			if path == "" {
				return usageErr(fmt.Errorf("empty listing path"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			initial, ierr := c.Get("/stingray/api/home/details/initialInfo", map[string]string{"path": path})
			if ierr != nil {
				return classifyAPIError(ierr)
			}
			// Pull propertyId / listingId for the next two calls.
			var env struct {
				Payload struct {
					PropertyID int64 `json:"propertyId"`
					ListingID  int64 `json:"listingId"`
				} `json:"payload"`
			}
			_ = json.Unmarshal(redfin.StripStingrayPrefix(initial), &env)
			pid := strconv.FormatInt(env.Payload.PropertyID, 10)
			lid := strconv.FormatInt(env.Payload.ListingID, 10)

			above, aerr := c.Get("/stingray/api/home/details/aboveTheFold", map[string]string{
				"propertyId": pid, "listingId": lid, "accessLevel": "1",
			})
			if aerr != nil {
				fmt.Fprintf(os.Stderr, "warning: aboveTheFold failed: %v\n", aerr)
			}
			below, berr := c.Get("/stingray/api/home/details/belowTheFold", map[string]string{
				"propertyId": pid, "listingId": lid, "accessLevel": "1",
			})
			if berr != nil {
				fmt.Fprintf(os.Stderr, "warning: belowTheFold failed: %v\n", berr)
			}

			listing, perr := redfin.ParseListingDetail(initial, above, below)
			if perr != nil {
				return apiErr(perr)
			}
			if listing.URL == "" {
				listing.URL = path
			}

			// Cache the merged record so transcendence reads work offline.
			if s, oerr := openRedfinStore(cmd.Context()); oerr == nil {
				if uerr := upsertListingHome(s, listing); uerr != nil {
					fmt.Fprintf(os.Stderr, "warning: cache upsert failed: %v\n", uerr)
				}
				s.Close()
			}

			return printJSONFiltered(cmd.OutOrStdout(), listing, flags)
		},
	}
	return cmd
}
