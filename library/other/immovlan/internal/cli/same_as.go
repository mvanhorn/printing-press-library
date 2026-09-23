// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

// immowebRow is the slice of immoweb-pp-cli's store this command reads.
type immowebRow struct {
	ID         int64
	Street     string
	PostalCode string
	Price      *float64
	Surface    *float64
	Bedrooms   *int
	Agency     string
	Private    bool
	GoneAt     string
}

type sameAsMatch struct {
	ImmowebID    int64    `json:"immoweb_id"`
	ImmowebURL   string   `json:"immoweb_url"`
	ImmowebPrice *float64 `json:"immoweb_price,omitempty"`
	MatchedBy    string   `json:"matched_by"`          // address | surface_bedrooms_agency
	Confidence   string   `json:"confidence"`          // high | medium
	DeltaEUR     *float64 `json:"delta_eur,omitempty"` // immovlan − immoweb
	DeltaPct     *float64 `json:"delta_pct,omitempty"`
	ImmowebGone  bool     `json:"immoweb_gone"`
}

type sameAsRow struct {
	immovlan.Listing
	Match *sameAsMatch `json:"immoweb,omitempty"`
	// Flat copies of the decision fields, present on every row: --agent
	// (--compact) keeps only keys shared by most rows and would drop the nested
	// object, which exists on matched rows only.
	ImmowebID    int64    `json:"immoweb_id"` // 0 when Immovlan-only
	ImmowebURL   string   `json:"immoweb_url"`
	ImmowebPrice *float64 `json:"immoweb_price"`
	DeltaPct     *float64 `json:"delta_pct"`           // immovlan vs immoweb asking price, %
	MatchStatus  string   `json:"match_status"`        // address | surface_bedrooms_agency | none
	GoneStatus   string   `json:"immoweb_gone_status"` // live | gone | none
}

type sameAsView struct {
	ImmowebStore  string      `json:"immoweb_store"`
	ImmowebRows   int         `json:"immoweb_rows_scanned"`
	Checked       int         `json:"checked"`
	Matched       int         `json:"matched"`
	Unmatched     int         `json:"unmatched"`
	WithoutStreet int         `json:"without_street"`
	Results       []sameAsRow `json:"results"`
	Note          string      `json:"note,omitempty"`
}

func newNovelSameAsCmd(flags *rootFlags) *cobra.Command {
	var flagAll, flagUnmatched, flagPriceGap bool
	var flagPostcode, flagImmowebDB, dbPath string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "same-as",
		Short: "Tell whether a stored Immovlan listing is also on Immoweb, at what price, or is Immovlan-only",
		Long: `Joins this CLI's local store with immoweb-pp-cli's local SQLite store: a listing
matches when the normalised street + number + postcode agree (high confidence) or
when price, living surface, bedrooms and agency all agree (medium). --all checks
every stored listing; --unmatched keeps only Immovlan-only listings; --price-gap
keeps only matches whose asking price differs between the portals.
Use this command to find the Immoweb counterpart of a stored Immovlan listing,
or with --all --unmatched to list Immovlan-only listings. It reads
immoweb-pp-cli's local store only and never fetches Immoweb. Run 'enrich' first
so street addresses are present; use 'show' to read one listing's own fields.`,
		Example: strings.Trim(`
  immovlan-pp-cli same-as vbe69761 --agent
  immovlan-pp-cli same-as --all --unmatched --postcode 1030 --agent
  immovlan-pp-cli same-as --all --price-gap --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--all;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "match stored listings against immoweb-pp-cli's store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && !flagAll {
				return usageErr(fmt.Errorf("give a listing reference or --all"))
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			f := store.ListingFilter{}
			if len(args) == 1 {
				ref, err := immovlan.ParseReference(args[0])
				if err != nil {
					return usageErr(err)
				}
				f.IDs = []string{ref}
				f.IncludeGone = true
			}
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			rows, err := db.QueryVlanListings(ctx, f)
			if err != nil {
				return err
			}
			if len(args) == 1 && len(rows) == 0 {
				return notFoundErr(fmt.Errorf("listing %s is not in the local store; run: immovlan-pp-cli show %s", args[0], strings.ToLower(args[0])))
			}
			immoPath := immowebStorePath(flagImmowebDB)
			view := sameAsView{ImmowebStore: immoPath, Results: []sameAsRow{}}
			if len(rows) == 0 {
				view.Note = "no stored Immovlan listings to check; run find first"
				return printView(cmd.OutOrStdout(), view, flags)
			}
			if _, err := os.Stat(immoPath); err != nil {
				return notFoundErr(fmt.Errorf("no immoweb-pp-cli store at %s: install immoweb-pp-cli and run a find or pull there, or pass --immoweb-db", immoPath))
			}
			pcs := map[string]bool{}
			for _, r := range rows {
				pcs[r.PostalCode] = true
			}
			immoRows, err := readImmowebRows(ctx, immoPath, pcs)
			if err != nil {
				return fmt.Errorf("reading immoweb store: %w", err)
			}
			view.ImmowebRows = len(immoRows)
			view.Results, view.Matched, view.Unmatched, view.WithoutStreet = matchSameAs(rows, immoRows, flagUnmatched, flagPriceGap)
			view.Checked = len(rows)
			if flagLimit > 0 && len(view.Results) > flagLimit {
				view.Results = view.Results[:flagLimit]
			}
			if view.WithoutStreet > 0 {
				view.Note = fmt.Sprintf("%d listings have no street yet (only price+surface matching possible); run: immovlan-pp-cli enrich --missing street", view.WithoutStreet)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d checked · %d also on Immoweb · %d Immovlan-only (immoweb store: %d rows in these postcodes)\n", view.Checked, view.Matched, view.Unmatched, view.ImmowebRows)
			if len(view.Results) > 0 {
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "IMMOVLAN\tLOCALITY\tSTREET\tPRICE\tIMMOWEB\tIMMOWEB €\tDELTA\tBY")
				for _, r := range view.Results {
					if r.Match == nil {
						fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t-\t\t\tImmovlan-only\n", r.ID, locLabel(r.Listing), termSafe(truncate(r.Street, 26)), fmtPtrEUR(r.Price))
						continue
					}
					delta := "-"
					if r.Match.DeltaPct != nil {
						delta = fmt.Sprintf("%+.1f%%", *r.Match.DeltaPct)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\n", r.ID, locLabel(r.Listing), termSafe(truncate(r.Street, 26)), fmtPtrEUR(r.Price), r.Match.ImmowebID, fmtPtrEUR(r.Match.ImmowebPrice), delta, r.Match.MatchedBy)
				}
				_ = tw.Flush()
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagAll, "all", false, "Check every stored listing (optionally narrowed by --postcode)")
	cmd.Flags().BoolVar(&flagUnmatched, "unmatched", false, "Keep only listings with no Immoweb counterpart (Immovlan exclusives)")
	cmd.Flags().BoolVar(&flagPriceGap, "price-gap", false, "Keep only matches whose asking price differs between the portals")
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().StringVar(&flagImmowebDB, "immoweb-db", "", "Path to immoweb-pp-cli's SQLite store (default: its data directory, or $IMMOWEB_PP_DB)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	// Both are file paths: kept off the MCP tool schema (env var covers non-default locations).
	_ = cmd.Flags().MarkHidden("db")
	_ = cmd.Flags().MarkHidden("immoweb-db")
	return cmd
}

func immowebStorePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("IMMOWEB_PP_DB"); env != "" {
		return env
	}
	if dir, err := cliutil.DataDir(); err == nil {
		return filepath.Join(filepath.Dir(dir), "immoweb-pp-cli", "data.db")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "immoweb-pp-cli", "data.db")
}

// readImmowebRows opens immoweb-pp-cli's store read-only (never migrating
// it) and returns the listings in the given postcodes.
func readImmowebRows(ctx context.Context, path string, pcs map[string]bool) ([]immowebRow, error) {
	if strings.ContainsAny(path, "?#") {
		return nil, fmt.Errorf("store path must be a plain file path")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	codes := make([]string, 0, len(pcs))
	args := []any{}
	for pc := range pcs {
		codes = append(codes, "?")
		args = append(args, pc)
	}
	where := ""
	if len(codes) > 0 {
		where = " WHERE postal_code IN (" + strings.Join(codes, ",") + ")"
	}
	// #nosec G202 -- column list and placeholders are constants; postcodes travel as args.
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(street,''), COALESCE(postal_code,''), price, surface, bedrooms, COALESCE(agency,''), COALESCE(private,0), COALESCE(gone_at,'') FROM immo_listings`+where, args...)
	if err != nil {
		return nil, err
	}
	out := []immowebRow{}
	for rows.Next() {
		var r immowebRow
		var price, surface sql.NullFloat64
		var beds sql.NullInt64
		var priv int
		if err := rows.Scan(&r.ID, &r.Street, &r.PostalCode, &price, &surface, &beds, &r.Agency, &priv, &r.GoneAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if price.Valid {
			v := price.Float64
			r.Price = &v
		}
		if surface.Valid {
			v := surface.Float64
			r.Surface = &v
		}
		if beds.Valid {
			v := int(beds.Int64)
			r.Bedrooms = &v
		}
		r.Private = priv == 1
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// matchSameAs pairs stored Immovlan listings with Immoweb rows: by address
// key first (high confidence), else by postcode+price+surface+bedrooms+agency
// (medium). Among candidates a live Immoweb listing wins, then the highest id.
func matchSameAs(rows []store.StoredListing, immoRows []immowebRow, unmatchedOnly, priceGapOnly bool) (results []sameAsRow, matched, unmatched, withoutStreet int) {
	results = []sameAsRow{}
	byAddr := map[string][]immowebRow{}
	byTriple := map[string][]immowebRow{}
	for _, ir := range immoRows {
		if k := store.AddrKey(ir.PostalCode, ir.Street); k != "" {
			byAddr[k] = append(byAddr[k], ir)
		}
		if k := tripleKey(ir.PostalCode, ir.Surface, ir.Bedrooms, ir.Agency); k != "" {
			byTriple[k] = append(byTriple[k], ir)
		}
	}
	for _, r := range rows {
		if r.Street == "" {
			withoutStreet++
		}
		row := sameAsRow{Listing: r.Listing}
		var cands []immowebRow
		by, conf := "", ""
		if k := r.AddrKey; k != "" && len(byAddr[k]) > 0 {
			cands, by, conf = byAddr[k], "address", "high"
		} else if k := tripleKey(r.PostalCode, r.Surface, r.Bedrooms, r.Agency); k != "" {
			// same postcode, surface, bedrooms and agency; the asking price may
			// differ between portals, so it filters (±15 %) instead of keying.
			for _, c := range byTriple[k] {
				if priceWithin(r.Price, c.Price, sameAsPriceTolerance) {
					cands = append(cands, c)
				}
			}
			if len(cands) > 0 {
				by, conf = "surface_bedrooms_agency", "medium"
			}
		}
		if len(cands) > 0 {
			cands = append([]immowebRow(nil), cands...)
			sort.SliceStable(cands, func(i, j int) bool {
				if (cands[i].GoneAt == "") != (cands[j].GoneAt == "") {
					return cands[i].GoneAt == ""
				}
				return cands[i].ID > cands[j].ID
			})
			c := cands[0]
			m := &sameAsMatch{ImmowebID: c.ID, ImmowebURL: fmt.Sprintf("https://www.immoweb.be/en/classified/%d", c.ID), ImmowebPrice: c.Price, MatchedBy: by, Confidence: conf, ImmowebGone: c.GoneAt != ""}
			if r.Price != nil && c.Price != nil && *c.Price > 0 {
				d := *r.Price - *c.Price
				p := pct1(d / *c.Price)
				m.DeltaEUR, m.DeltaPct = &d, &p
			}
			row.Match = m
			row.ImmowebID, row.ImmowebURL, row.ImmowebPrice, row.DeltaPct, row.MatchStatus, row.GoneStatus = m.ImmowebID, m.ImmowebURL, m.ImmowebPrice, m.DeltaPct, m.MatchedBy, "live"
			if m.ImmowebGone {
				row.GoneStatus = "gone"
			}
			matched++
		} else {
			row.MatchStatus, row.GoneStatus = "none", "none"
			unmatched++
		}
		switch {
		case unmatchedOnly && row.Match != nil:
			continue
		case priceGapOnly && (row.Match == nil || row.Match.DeltaEUR == nil || *row.Match.DeltaEUR == 0):
			continue
		}
		results = append(results, row)
	}
	return results, matched, unmatched, withoutStreet
}

// sameAsPriceTolerance is the largest asking-price gap (relative to the
// Immoweb price) still accepted for a surface+bedrooms+agency match.
const sameAsPriceTolerance = 0.15

// priceWithin requires two usable prices: without them the surface+bedrooms+
// agency key alone would pair distinct units of one development.
func priceWithin(a, b *float64, tol float64) bool {
	if a == nil || b == nil || *b <= 0 {
		return false
	}
	d := *a - *b
	if d < 0 {
		d = -d
	}
	return d <= *b*tol
}

// tripleKey identifies a home without its price: postcode, living surface,
// bedrooms and agency.
func tripleKey(pc string, surface *float64, beds *int, agency string) string {
	if surface == nil || *surface <= 0 {
		return ""
	}
	b := -1
	if beds != nil {
		b = *beds
	}
	a := immovlan.Fold(agency)
	if len(a) > 12 {
		a = a[:12]
	}
	return fmt.Sprintf("%s|%.0f|%d|%s", pc, *surface, b, a)
}
