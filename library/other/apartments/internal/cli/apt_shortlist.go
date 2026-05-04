// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

type shortlistRow struct {
	URL        string  `json:"url"`
	Tag        string  `json:"tag,omitempty"`
	Note       string  `json:"note,omitempty"`
	AddedAt    string  `json:"added_at,omitempty"`
	MaxRent    int     `json:"max_rent,omitempty"`
	Beds       int     `json:"beds,omitempty"`
	Baths      float64 `json:"baths,omitempty"`
	Title      string  `json:"title,omitempty"`
	PropertyID string  `json:"property_id,omitempty"`
	Sqft       int     `json:"sqft,omitempty"`
}

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "shortlist",
		Short:       "Tag-based local shortlist; add/show/remove listings with notes and tags.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli shortlist add https://www.apartments.com/the-domain-austin-tx/abc123/ --tag favorite --note "rooftop pool"
  apartments-pp-cli shortlist show --tag favorite --json
  apartments-pp-cli shortlist remove https://www.apartments.com/the-domain-austin-tx/abc123/
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newShortlistAddCmd(flags))
	cmd.AddCommand(newShortlistShowCmd(flags))
	cmd.AddCommand(newShortlistRemoveCmd(flags))
	return cmd
}

func newShortlistAddCmd(flags *rootFlags) *cobra.Command {
	var tag string
	var note string
	cmd := &cobra.Command{
		Use:         "add <url>",
		Short:       "Add a listing URL to the local shortlist.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		Example: strings.Trim(`
  apartments-pp-cli shortlist add https://www.apartments.com/foo/abc123/ --tag favorite
  apartments-pp-cli shortlist add the-domain-austin-tx --tag visit --note "pet friendly"
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			url := normalizeListingURL(args[0])
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.DB().Exec(
				`INSERT INTO shortlist (listing_url, tag, note)
				 VALUES (?, ?, ?)
				 ON CONFLICT(listing_url, tag) DO UPDATE SET note = excluded.note`,
				url, tag, note,
			)
			if err != nil {
				return err
			}
			out := map[string]any{"url": url, "tag": tag, "note": note}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Optional tag (e.g. favorite, visit).")
	cmd.Flags().StringVar(&note, "note", "", "Optional free-text note.")
	return cmd
}

func newShortlistShowCmd(flags *rootFlags) *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show all shortlisted listings, joined to the latest cached listing data.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli shortlist show --json
  apartments-pp-cli shortlist show --tag favorite
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			query := `SELECT listing_url, tag, note, added_at FROM shortlist`
			var qargs []any
			if cmd.Flags().Changed("tag") {
				query += ` WHERE tag = ?`
				qargs = append(qargs, tag)
			}
			query += ` ORDER BY added_at DESC`
			rows, err := db.DB().Query(query, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			var out []shortlistRow
			for rows.Next() {
				var (
					r       shortlistRow
					t       sql.NullString
					n       sql.NullString
					addedAt sql.NullString
				)
				if err := rows.Scan(&r.URL, &t, &n, &addedAt); err != nil {
					return err
				}
				r.Tag = t.String
				r.Note = n.String
				r.AddedAt = addedAt.String
				// Join to the cached listing.
				propertyID := apt.ListingURLToPropertyID(r.URL)
				if propertyID != "" {
					var data string
					qerr := db.DB().QueryRow(`SELECT data FROM listing WHERE id = ?`, propertyID).Scan(&data)
					if qerr == nil && data != "" {
						var li apt.Listing
						if json.Unmarshal([]byte(data), &li) == nil {
							r.MaxRent = li.MaxRent
							r.Beds = li.Beds
							r.Baths = li.Baths
							r.Title = li.Title
							r.PropertyID = li.PropertyID
							r.Sqft = li.Sqft
						}
					}
				}
				out = append(out, r)
			}
			if out == nil {
				out = []shortlistRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag.")
	return cmd
}

func newShortlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var tag string
	cmd := &cobra.Command{
		Use:         "remove <url>",
		Short:       "Remove a listing URL from the local shortlist.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		Example: strings.Trim(`
  apartments-pp-cli shortlist remove https://www.apartments.com/foo/abc123/
  apartments-pp-cli shortlist remove the-domain-austin-tx --tag visit
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			url := normalizeListingURL(args[0])
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			query := `DELETE FROM shortlist WHERE listing_url = ?`
			qargs := []any{url}
			if cmd.Flags().Changed("tag") {
				query += ` AND tag = ?`
				qargs = append(qargs, tag)
			}
			res, err := db.DB().Exec(query, qargs...)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			out := map[string]any{"url": url, "removed": n}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Only remove the row with this tag.")
	return cmd
}

func normalizeListingURL(arg string) string {
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return arg
	}
	return "https://www.apartments.com/" + strings.Trim(arg, "/") + "/"
}
