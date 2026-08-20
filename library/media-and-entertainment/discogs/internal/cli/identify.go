// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: catalog-number / barcode identity spine.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/store"

	"github.com/spf13/cobra"
)

type identifyMatch struct {
	ReleaseID int64  `json:"release_id"`
	Title     string `json:"title,omitempty"`
	Year      int    `json:"year,omitempty"`
	Country   string `json:"country,omitempty"`
	Format    string `json:"format,omitempty"`
	Catno     string `json:"catno,omitempty"`
	Type      string `json:"type,omitempty"`
}

type identifyView struct {
	Query      string         `json:"query"`
	Match      *identifyMatch `json:"match"`
	Owned      bool           `json:"owned"`
	Wanted     bool           `json:"wanted"`
	Lowest     *float64       `json:"current_lowest,omitempty"`
	NumForSale *int           `json:"num_for_sale,omitempty"`
	Note       string         `json:"note,omitempty"`
}

func newNovelIdentifyCmd(flags *rootFlags) *cobra.Command {
	var barcode string
	var catno string
	var currency string

	cmd := &cobra.Command{
		Use:   "identify",
		Short: "Resolve a physical record's catalog number or barcode to the exact release and show if you own it, want it, and its value.",
		Long: "Resolves a barcode or catalog number to a Discogs release, then reports whether it's in your synced " +
			"collection or wantlist and its current marketplace value — one answer for a buy/skip call.\n\n" +
			"Use this for a record in hand. For open-ended text search, use 'database search'.",
		Example:     "  discogs-pp-cli identify --barcode 0720642442524 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if barcode == "" && catno == "" && cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			if barcode == "" && catno == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide --barcode or --catno"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, err := openDiscogsStore(ctx, flags)
			if err != nil {
				return err
			}
			defer st.Close()

			params := map[string]string{"type": "release", "per_page": "5"}
			q := ""
			if barcode != "" {
				params["barcode"] = barcode
				q = "barcode=" + barcode
			} else {
				params["catno"] = catno
				q = "catno=" + catno
			}
			data, err := c.Get(ctx, "/database/search", params)
			if err != nil {
				return fmt.Errorf("searching Discogs: %w", err)
			}
			var sr struct {
				Results []struct {
					ID      int64    `json:"id"`
					Title   string   `json:"title"`
					Year    string   `json:"year"`
					Country string   `json:"country"`
					Format  []string `json:"format"`
					Catno   string   `json:"catno"`
					Type    string   `json:"type"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &sr); err != nil {
				return fmt.Errorf("parsing search results: %w", err)
			}
			view := identifyView{Query: q}
			if len(sr.Results) == 0 {
				view.Note = "no release matched; try 'database search' with more terms"
				return emitIdentify(cmd, flags, view)
			}
			r := sr.Results[0]
			m := &identifyMatch{ReleaseID: r.ID, Title: r.Title, Country: r.Country, Catno: r.Catno, Type: r.Type}
			if r.Year != "" {
				if y, perr := strconv.Atoi(r.Year); perr == nil {
					m.Year = y
				}
			}
			if len(r.Format) > 0 {
				m.Format = r.Format[0]
			}
			view.Match = m

			// Local collection / wantlist membership.
			view.Owned = ownedInCollection(ctx, st, r.ID)
			if raw, gerr := st.Get("wantlist", strconv.FormatInt(r.ID, 10)); gerr == nil && len(raw) > 0 {
				view.Wanted = true
			}

			// Current value (records a snapshot too).
			if stats, cerr := captureSnapshot(ctx, c, st, r.ID, currency); cerr == nil {
				if stats.LowestPrice != nil && stats.LowestPrice.Value != nil {
					v := *stats.LowestPrice.Value
					view.Lowest = &v
				}
				if stats.NumForSale != nil {
					n := *stats.NumForSale
					view.NumForSale = &n
				}
			}
			return emitIdentify(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&barcode, "barcode", "", "Barcode to resolve")
	cmd.Flags().StringVar(&catno, "catno", "", "Catalog number to resolve")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for the value lookup (e.g. USD)")
	return cmd
}

// ownedInCollection scans synced collection items for a release id (drain-first).
func ownedInCollection(ctx context.Context, st *store.Store, releaseID int64) bool {
	rows, err := st.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'collection'`)
	if err != nil {
		return false
	}
	var raws [][]byte
	for rows.Next() {
		var d []byte
		if err := rows.Scan(&d); err == nil {
			raws = append(raws, d)
		}
	}
	_ = rows.Err()
	_ = rows.Close()
	for _, d := range raws {
		var it collectionItem
		if json.Unmarshal(d, &it) == nil && it.BasicInfo.ID == releaseID {
			return true
		}
	}
	return false
}

func emitIdentify(cmd *cobra.Command, flags *rootFlags, view identifyView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if view.Match == nil {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		return nil
	}
	m := view.Match
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d) [%s %s %d]\n", m.Title, m.ReleaseID, m.Format, m.Country, m.Year)
	own := "not owned"
	if view.Owned {
		own = "OWNED"
	}
	want := ""
	if view.Wanted {
		want = ", on wantlist"
	}
	low := "—"
	if view.Lowest != nil {
		low = fmt.Sprintf("%.2f", *view.Lowest)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s%s   lowest %s\n", own, want, low)
	return nil
}
