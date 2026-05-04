// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"
	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/store"

	"github.com/spf13/cobra"
)

func newListingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "listing [property-id-or-url]",
		Short:       "Fetch one apartments.com listing detail page and parse schema.org microdata.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli listing the-domain-austin-tx
  apartments-pp-cli listing https://www.apartments.com/the-domain-austin-tx/abc123/
  apartments-pp-cli listing the-domain-austin-tx --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would GET listing:", args[0])
				return nil
			}
			arg := args[0]
			absURL := arg
			path := "/" + strings.Trim(arg, "/") + "/"
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				u, err := url.Parse(arg)
				if err != nil {
					return usageErr(fmt.Errorf("invalid listing URL: %w", err))
				}
				path = u.Path
				if !strings.HasSuffix(path, "/") {
					path += "/"
				}
				absURL = arg
			} else {
				absURL = "https://www.apartments.com" + path
			}

			// Listing detail pages have stricter Akamai protection than search
			// pages. Surf clears /city-state/ search URLs but not most
			// /property-slug/ detail URLs. Try the live fetch first; fall back
			// to the latest snapshot in the local store on 403 so the placard
			// data captured by `rentals` and `sync-search` remains useful.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, gerr := c.Get(path, nil)
			if gerr == nil {
				li, perr := apt.ParseListing([]byte(data), absURL)
				if perr != nil {
					return apiErr(perr)
				}
				if li.PropertyID != "" {
					if db, derr := store.OpenWithContext(cmd.Context(), defaultDBPath("apartments-pp-cli")); derr == nil {
						if raw, mErr := json.Marshal(li); mErr == nil {
							_ = db.UpsertListing(raw)
						}
						db.Close()
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), li, flags)
			}

			// Live fetch failed. Try the local store: rentals/sync-search
			// already cached placards under listing_snapshots keyed on the
			// canonical URL. Pull the most recent snapshot for this URL.
			fallback, fbErr := lookupListingSnapshot(cmd.Context(), absURL)
			if fbErr == nil && fallback != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: live fetch returned 403 (apartments.com listing pages have stricter protection than search). Falling back to most-recent snapshot from `rentals`/`sync-search`.")
				return printJSONFiltered(cmd.OutOrStdout(), fallback, flags)
			}
			return classifyAPIError(gerr)
		},
	}
	return cmd
}
