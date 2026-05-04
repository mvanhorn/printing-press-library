// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

func newAptSyncCmd(flags *rootFlags) *cobra.Command {
	rf := &rentalsFlags{}
	cmd := &cobra.Command{
		Use:         "sync-search <saved-search>",
		Short:       "Run a saved-search and append the results to listing_snapshots.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli sync-search austin-2br --city austin --state tx --beds 2 --price-max 2500
  apartments-pp-cli sync-search downtown-pet-friendly --city austin --state tx --pets dog --json
  apartments-pp-cli sync-search east-side --zip 78704 --beds-min 1 --all
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := args[0]
			if dryRunOK(flags) {
				opts := rf.toOptions()
				path := apt.BuildSearchURL(opts)
				fmt.Fprintln(cmd.OutOrStdout(), "would sync:", slug, "->", path)
				return nil
			}
			if err := rf.validate(); err != nil {
				return usageErr(err)
			}
			opts := rf.toOptions()
			path := apt.BuildSearchURL(opts)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, gerr := c.Get(path, nil)
			fetchStatus := 200
			if gerr != nil {
				fetchStatus = 0
				return classifyAPIError(gerr)
			}
			placards, perr := apt.ParsePlacards([]byte(data), c.BaseURL)
			if perr != nil {
				return apiErr(perr)
			}

			db, derr := openAptStore(cmd.Context())
			if derr != nil {
				return derr
			}
			defer db.Close()

			now := time.Now().UTC()
			for _, p := range placards {
				raw, _ := json.Marshal(p)
				_, ierr := apt.InsertSnapshot(db.DB(), apt.SnapshotInsert{
					ListingURL:  p.URL,
					PropertyID:  p.PropertyID,
					SavedSearch: slug,
					MaxRent:     p.MaxRent,
					Beds:        p.Beds,
					Baths:       p.Baths,
					FetchStatus: fetchStatus,
					Raw:         raw,
				})
				if ierr != nil {
					return ierr
				}
				// Also upsert into the canonical `listing` table so
				// transcendence commands (rank, value, market, etc.) see
				// this data. Detail-only fields (sqft, amenities,
				// pet_policy fees) stay zero — populated only on listing
				// command success, which apartments.com 403s most of the
				// time. Placard data still drives rent and bed-count
				// rankings.
				li := apt.Listing{
					URL:        p.URL,
					PropertyID: p.PropertyID,
					Title:      p.Title,
					Beds:       p.Beds,
					Baths:      p.Baths,
					MaxRent:    p.MaxRent,
				}
				if liRaw, mErr := json.Marshal(li); mErr == nil {
					_ = db.UpsertListing(liRaw)
				}
			}
			if uerr := apt.UpsertSavedSearch(db.DB(), slug, opts, len(placards)); uerr != nil {
				return uerr
			}
			out := map[string]any{
				"saved_search": slug,
				"count":        len(placards),
				"synced_at":    now.Format(time.RFC3339),
				"path":         path,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	addRentalsFlags(cmd, rf)
	return cmd
}
