// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

// siblingRow is the slice of an immoweb-pp-cli / immovlan-pp-cli listing
// this command reads.
type siblingRow struct {
	Portal     string
	ID         string
	Street     string
	PostalCode string
	Price      *float64
	Surface    *float64
	Bedrooms   *int
	Agency     string
	Lat, Lng   *float64
	Gone       bool
	AddrKey    string
}

type sameAsMatch struct {
	Portal        string   `json:"portal"`
	ID            string   `json:"id"`
	URL           string   `json:"url"`
	Price         *float64 `json:"price"`
	MatchedBy     string   `json:"matched_by"` // address | gps_price | surface_bedrooms_price
	Confidence    string   `json:"confidence"` // high | medium
	DeltaPct      *float64 `json:"zimmo_vs_portal_pct,omitempty"`
	Gone          bool     `json:"gone"`
	StreetMissing bool     `json:"portal_street_missing"`
}

type sameAsRow struct {
	Code        string        `json:"zimmo_code"`
	Address     string        `json:"address"`
	Price       *float64      `json:"price"`
	URL         string        `json:"url"`
	Matches     []sameAsMatch `json:"matches"`
	MatchStatus string        `json:"match_status"` // matched | zimmo_only
	MaxGapPct   *float64      `json:"max_price_gap_pct"`
}

type sameAsView struct {
	Stores      map[string]string `json:"sibling_stores"`
	SiblingRows map[string]int    `json:"sibling_rows_scanned"`
	Checked     int               `json:"checked"`
	Matched     int               `json:"matched"`
	ZimmoOnly   int               `json:"zimmo_only"`
	Results     []sameAsRow       `json:"results"`
	Note        string            `json:"note,omitempty"`
}

func siblingDBPath(cli string) string {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, cli, "data.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", cli, "data.db")
}

func newNovelSameAsCmd(flags *rootFlags) *cobra.Command {
	var all, unmatched, priceGap, exportAddress bool
	var postcode, immowebDB, immovlanDB, dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "same-as",
		Short: "Find the Immoweb or Immovlan twin of a Zimmo listing and the price gap between portals.",
		Long: `Joins this CLI's store with the local SQLite stores of immoweb-pp-cli and
immovlan-pp-cli (read-only). A listing matches on the same normalised street +
number + postcode (high confidence), or on coordinates within 30 m and a price
within 10%, or on postcode + living surface (±2 m²) + bedrooms + price within
5% (medium). --all checks every stored for-sale listing; --unmatched keeps
Zimmo-only listings; --price-gap keeps matches whose price differs;
--export-address keeps matches whose portal copy has no street, so Zimmo's
exact address fills the gap.
Give a stored Zimmo code as the argument, or --all.
Use this command to find the Immoweb/Immovlan counterpart of a Zimmo listing,
or with --all --unmatched to list Zimmo-only listings. It reads the sibling
CLIs' local stores only and never fetches those portals.`,
		Example: strings.Trim(`
  zimmo-pp-cli same-as LRWGW --agent
  zimmo-pp-cli same-as --all --unmatched --postcode 1030 --agent
  zimmo-pp-cli same-as --all --price-gap --json
  zimmo-pp-cli same-as --all --export-address --csv`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--all;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "match stored listings against immoweb/immovlan stores")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a Zimmo code or --all"))
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if immowebDB == "" {
				immowebDB = siblingDBPath("immoweb-pp-cli")
			}
			if immovlanDB == "" {
				immovlanDB = siblingDBPath("immovlan-pp-cli")
			}
			v := sameAsView{Stores: map[string]string{}, SiblingRows: map[string]int{}, Results: make([]sameAsRow, 0)}
			var siblings []siblingRow
			for portal, p := range map[string]string{"immoweb": immowebDB, "immovlan": immovlanDB} {
				rows, err := readSiblingStore(ctx, portal, p)
				if err != nil {
					v.Stores[portal] = "unavailable: " + err.Error()
					continue
				}
				v.Stores[portal] = p
				v.SiblingRows[portal] = len(rows)
				siblings = append(siblings, rows...)
			}
			if len(siblings) == 0 {
				v.Note = "no immoweb-pp-cli or immovlan-pp-cli store found; run their find/search commands first or pass --immoweb-db / --immovlan-db"
			}
			path, ok := localStoreExists(dbPath)
			if !ok {
				emptyStoreHint(cmd, path)
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			db, err := openZimmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			f := store.ListingFilter{Postcodes: splitCSV(postcode), Statuses: []string{"FOR_SALE"}}
			if len(args) == 1 {
				ref, err := parseCodeArg(args[0])
				if err != nil {
					return usageErr(err)
				}
				f = store.ListingFilter{Codes: []string{ref}, IncludeGone: true}
			}
			listings, err := db.QueryZimmoListings(ctx, f)
			if err != nil {
				return err
			}
			if len(args) == 1 && len(listings) == 0 {
				return notFoundErr(fmt.Errorf("listing %s is not in the local store; run 'show %s' first", args[0], args[0]))
			}
			idx := indexSiblings(siblings)
			for _, l := range listings {
				v.Checked++
				row := matchSiblings(l.Listing, idx)
				if row.MatchStatus == "matched" {
					v.Matched++
				} else {
					v.ZimmoOnly++
				}
				switch {
				case unmatched && row.MatchStatus != "zimmo_only":
					continue
				case priceGap && (row.MaxGapPct == nil || math.Abs(*row.MaxGapPct) < 0.5):
					continue
				case exportAddress && !anyStreetMissing(row.Matches):
					continue
				}
				v.Results = append(v.Results, row)
			}
			if limit > 0 && len(v.Results) > limit {
				v.Results = v.Results[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "checked %d Zimmo listings: %d also on Immoweb/Immovlan, %d Zimmo-only\n", v.Checked, v.Matched, v.ZimmoOnly)
			if v.Note != "" {
				fmt.Fprintln(w, "note: "+v.Note)
			}
			tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tPRICE\tMATCH\tPORTAL PRICE\tGAP\tADDRESS")
			for _, r := range v.Results {
				if len(r.Matches) == 0 {
					fmt.Fprintf(tw, "%s\t%s\tzimmo only\t-\t-\t%s\n", r.Code, fmtPtrEUR(r.Price), termSafe(r.Address))
					continue
				}
				for _, m := range r.Matches {
					gap := "-"
					if m.DeltaPct != nil {
						gap = fmt.Sprintf("%+.1f%%", *m.DeltaPct)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s %s (%s)\t%s\t%s\t%s\n", r.Code, fmtPtrEUR(r.Price), m.Portal, m.ID, m.MatchedBy, fmtPtrEUR(m.Price), gap, termSafe(r.Address))
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Check every stored for-sale listing")
	cmd.Flags().BoolVar(&unmatched, "unmatched", false, "Keep only Zimmo-only listings")
	cmd.Flags().BoolVar(&priceGap, "price-gap", false, "Keep only matches whose asking price differs between portals")
	cmd.Flags().BoolVar(&exportAddress, "export-address", false, "Keep only matches whose portal copy lacks a street (Zimmo gives the exact address)")
	cmd.Flags().StringVar(&postcode, "postcode", "", "With --all: only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&immowebDB, "immoweb-db", "", "immoweb-pp-cli store (default ~/.local/share/immoweb-pp-cli/data.db)")
	cmd.Flags().StringVar(&immovlanDB, "immovlan-db", "", "immovlan-pp-cli store (default ~/.local/share/immovlan-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func anyStreetMissing(ms []sameAsMatch) bool {
	for _, m := range ms {
		if m.StreetMissing {
			return true
		}
	}
	return false
}

// readSiblingStore opens a sibling CLI store read-only and loads its listings.
func readSiblingStore(ctx context.Context, portal, path string) ([]siblingRow, error) {
	if strings.ContainsAny(path, "?#%") {
		return nil, fmt.Errorf("store path %q contains ?, # or %%; move or symlink it to a plain path", path)
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("no store at %s", path)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	q := `SELECT CAST(id AS TEXT), COALESCE(street,''), COALESCE(postal_code,''), price, surface, bedrooms, COALESCE(agency,''), lat, lng, gone_at IS NOT NULL FROM `
	switch portal {
	case "immoweb":
		q += `immo_listings WHERE COALESCE(deal,'') NOT LIKE '%rent%'`
	case "immovlan":
		q += `vlan_listings WHERE COALESCE(deal,'') NOT LIKE '%louer%' AND COALESCE(deal,'') NOT LIKE '%rent%'`
	default:
		return nil, fmt.Errorf("unknown portal %s", portal)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("reading %s store: %w", portal, err)
	}
	defer rows.Close()
	out := make([]siblingRow, 0)
	for rows.Next() {
		var r siblingRow
		var price, surface, lat, lng sql.NullFloat64
		var beds sql.NullInt64
		if err := rows.Scan(&r.ID, &r.Street, &r.PostalCode, &price, &surface, &beds, &r.Agency, &lat, &lng, &r.Gone); err != nil {
			return nil, err
		}
		r.Portal = portal
		if price.Valid && price.Float64 > 0 {
			v := price.Float64
			r.Price = &v
		}
		if surface.Valid && surface.Float64 > 0 {
			v := surface.Float64
			r.Surface = &v
		}
		if beds.Valid {
			v := int(beds.Int64)
			r.Bedrooms = &v
		}
		if lat.Valid && lng.Valid {
			a, b := lat.Float64, lng.Float64
			r.Lat, r.Lng = &a, &b
		}
		r.AddrKey = store.AddrKey(r.PostalCode, r.Street)
		out = append(out, r)
	}
	return out, rows.Err()
}

type siblingIndex struct {
	byAddr     map[string][]siblingRow
	byPostcode map[string][]siblingRow
}

func indexSiblings(rows []siblingRow) siblingIndex {
	idx := siblingIndex{byAddr: map[string][]siblingRow{}, byPostcode: map[string][]siblingRow{}}
	for _, r := range rows {
		if r.AddrKey != "" {
			idx.byAddr[r.AddrKey] = append(idx.byAddr[r.AddrKey], r)
		}
		idx.byPostcode[r.PostalCode] = append(idx.byPostcode[r.PostalCode], r)
	}
	return idx
}

func within(a, b *float64, tol float64) bool {
	if a == nil || b == nil || *b == 0 {
		return false
	}
	return math.Abs(*a-*b)/(*b) <= tol
}

func siblingURL(r siblingRow) string {
	if r.Portal == "immoweb" {
		return "https://www.immoweb.be/en/classified/" + r.ID
	}
	return "https://immovlan.be/fr/detail/" + strings.ToLower(r.ID)
}

// matchSiblings finds portal twins of one Zimmo listing (pure, testable).
func matchSiblings(l zimmo.Listing, idx siblingIndex) sameAsRow {
	row := sameAsRow{Code: l.Code, Address: l.Address, Price: l.Price, URL: l.URL, Matches: make([]sameAsMatch, 0), MatchStatus: "zimmo_only"}
	seen := map[string]bool{}
	add := func(r siblingRow, by, conf string) {
		key := r.Portal + ":" + r.ID
		if seen[key] {
			return
		}
		seen[key] = true
		m := sameAsMatch{Portal: r.Portal, ID: r.ID, URL: siblingURL(r), Price: r.Price, MatchedBy: by, Confidence: conf, Gone: r.Gone, StreetMissing: r.AddrKey == ""}
		if l.Price != nil && r.Price != nil && *r.Price > 0 {
			m.DeltaPct = ptrF(pct1((*l.Price - *r.Price) / *r.Price))
		}
		row.Matches = append(row.Matches, m)
	}
	key := store.AddrKey(l.PostalCode, l.Street+" "+l.Number)
	if key != "" {
		cands := idx.byAddr[key]
		// Several units at one address: a high-confidence match needs
		// surface or price agreement. If none is close, do not claim every
		// flat at that street number is this property. A single listing at
		// the number still matches on the address alone.
		var close []siblingRow
		for _, c := range cands {
			if within(c.Surface, l.Surface, 0.05) || within(c.Price, l.Price, 0.05) {
				close = append(close, c)
			}
		}
		switch {
		case len(close) > 0:
			cands = close
		case len(cands) > 1:
			cands = nil
		}
		for _, c := range cands {
			add(c, "address", "high")
		}
	}
	for _, c := range idx.byPostcode[l.PostalCode] {
		switch {
		case l.Lat != nil && c.Lat != nil && l.GeoPrecision == "ROOFTOP" &&
			zimmo.DistanceM(*l.Lat, *l.Lng, *c.Lat, *c.Lng) <= 30 && within(c.Price, l.Price, 0.10):
			add(c, "gps_price", "medium")
		case l.Surface != nil && c.Surface != nil && math.Abs(*l.Surface-*c.Surface) <= 2 &&
			l.Bedrooms != nil && c.Bedrooms != nil && *l.Bedrooms == *c.Bedrooms && within(c.Price, l.Price, 0.05):
			add(c, "surface_bedrooms_price", "medium")
		}
	}
	sort.SliceStable(row.Matches, func(i, j int) bool { return row.Matches[i].Confidence == "high" && row.Matches[j].Confidence != "high" })
	if len(row.Matches) > 0 {
		row.MatchStatus = "matched"
		for _, m := range row.Matches {
			if m.DeltaPct != nil && (row.MaxGapPct == nil || math.Abs(*m.DeltaPct) > math.Abs(*row.MaxGapPct)) {
				d := *m.DeltaPct
				row.MaxGapPct = &d
			}
		}
	}
	return row
}
