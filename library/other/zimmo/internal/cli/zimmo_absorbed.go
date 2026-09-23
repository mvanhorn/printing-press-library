// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

// Absorbed commands: photos, dump, agency, prices, shortlist, drops.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

// ---------------------------------------------------------------- photos

func newPhotosCmd(flags *rootFlags) *cobra.Command {
	var dir string
	var limit int
	cmd := &cobra.Command{
		Use:   "photos [zimmo-code|url]",
		Short: "List or download a listing's photos (largest size) from files.zimmo.be",
		Long: `Print the photo URLs of one listing in display order, or download them into
--dir. Downloads are pinned to files.zimmo.be; nothing else is fetched.`,
		Example: strings.Trim(`
  zimmo-pp-cli photos LAISZ
  zimmo-pp-cli photos LAISZ --dir ./laisz-photos --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "code=LAISZ;--limit=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list or download listing photos")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give exactly one Zimmo code, UUID or URL"))
			}
			ref, err := parseCodeArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			l, err := fetchListing(ctx, zimmoClient(flags), ref)
			if err != nil {
				return err
			}
			photos := l.Photos
			if limit > 0 && len(photos) > limit {
				photos = photos[:limit]
			}
			if photos == nil {
				photos = make([]string, 0)
			}
			type photoRow struct {
				N    int    `json:"n"`
				URL  string `json:"url"`
				File string `json:"file,omitempty"`
			}
			rows := make([]photoRow, 0, len(photos))
			for i, u := range photos {
				rows = append(rows, photoRow{N: i + 1, URL: u})
			}
			if dir != "" {
				if cliutil.IsAnyHarness() {
					return writeHarnessRefusal(cmd.OutOrStdout(), flags, "download photos")
				}
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return err
				}
				stem := safeFileStem(firstNonEmptyStr(l.Code, ref))
				for i := range rows {
					name := fmt.Sprintf("%s-%02d.jpg", stem, rows[i].N)
					if err := downloadZimmoFile(ctx, rows[i].URL, filepath.Join(dir, name)); err != nil {
						return fmt.Errorf("photo %d: %w", rows[i].N, err)
					}
					rows[i].File = filepath.Join(dir, name)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), map[string]any{"zimmo_code": l.Code, "count": len(l.Photos), "photos": rows}, flags)
			}
			for _, r := range rows {
				if r.File != "" {
					fmt.Fprintln(cmd.OutOrStdout(), r.File)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), r.URL)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Download the photos into this directory")
	cmd.Flags().IntVar(&limit, "limit", 0, "Keep only the first N photos")
	return cmd
}

func downloadZimmoFile(ctx context.Context, rawURL, path string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Host != "files.zimmo.be" {
		return fmt.Errorf("refusing to download %q: not a files.zimmo.be URL", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", zimmo.UserAgent)
	hc := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(r *http.Request, _ []*http.Request) error {
		if r.URL.Scheme != "https" || r.URL.Host != "files.zimmo.be" {
			return fmt.Errorf("refusing redirect to %s", r.URL.Host)
		}
		return nil
	}}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	const maxPhoto = 30 << 20
	tmp, err := os.CreateTemp(filepath.Dir(path), ".photo-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxPhoto+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > maxPhoto {
		return fmt.Errorf("photo larger than 30 MB")
	}
	return os.Rename(tmp.Name(), path)
}

// safeFileStem keeps letters, digits and dashes so an upstream code can
// never steer a file outside --dir.
func safeFileStem(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "listing"
	}
	return b.String()
}

// ---------------------------------------------------------------- dump

var zimmoCSVHeader = []string{"zimmo_code", "url", "status", "type", "subtype", "address", "postal_code", "locality", "lat", "lng", "price", "surface_m2", "price_per_m2", "bedrooms", "bathrooms", "plot_m2", "construction_year", "condition", "epc", "epc_kwh_m2", "renovation_obligation", "rented", "rent_per_year", "flood_risk", "agency", "agency_phone", "published_at", "days_on_market", "total_cut_pct", "first_seen", "last_seen", "gone_at"}

func fstr(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', -1, 64)
}

func istr(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func zimmoCSVRow(r store.StoredListing) []string {
	row := []string{r.Code, r.URL, r.Status, r.Type, r.SubType, r.Address, r.PostalCode, r.Locality, fstr(r.Lat), fstr(r.Lng),
		fstr(r.Price), fstr(r.Surface), fstr(r.PricePerM2), istr(r.Bedrooms), istr(r.Bathrooms), fstr(r.Plot), istr(r.Year), r.Condition,
		r.EPC, fstr(r.EPCKWh), r.RenovationDuty, strconv.FormatBool(r.Rented), fstr(r.RentPerYear), strings.Join(r.FloodRisk, "|"),
		r.Agency, r.AgencyPhone, r.PublishedAt, istr(r.DaysOnMarket), fstr(r.TotalCutPct), r.FirstSeen, r.LastSeen, r.GoneAt}
	for i := range row {
		row[i] = csvSafe(row[i])
	}
	return row
}

func newDumpCmd(flags *rootFlags) *cobra.Command {
	var format, postcode, status, typ, dbPath string
	var includeGone bool
	var limit int
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Export stored listings as JSON, CSV or GeoJSON (same columns as immoweb/immovlan dumps)",
		Long: `Export the local store for spreadsheets, maps or other tools. CSV cells are
formula-safe; GeoJSON includes only listings with coordinates.`,
		Example: strings.Trim(`
  zimmo-pp-cli dump --format csv > zimmo.csv
  zimmo-pp-cli dump --format geojson --postcode 1050 > ixelles.geojson
  zimmo-pp-cli dump --status sold --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--format=json;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export stored listings")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			switch format {
			case "json", "csv", "geojson":
			default:
				return usageErr(fmt.Errorf("--format must be json, csv or geojson"))
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			path, ok := localStoreExists(dbPath)
			rows := make([]store.StoredListing, 0)
			if ok {
				db, err := openZimmoStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				f := store.ListingFilter{Postcodes: splitCSV(postcode), IncludeGone: includeGone, Limit: limit}
				if status != "" {
					st, err := zimmo.ParseStatus(status)
					if err != nil {
						return usageErr(err)
					}
					f.Statuses = []string{st}
					if st == "SOLD" || st == "RENTED" {
						f.IncludeGone = true
					}
				}
				if typ != "" {
					cat, err := zimmo.ParseCategory(typ)
					if err != nil {
						return usageErr(err)
					}
					f.Types = []string{cat}
				}
				rows, err = db.QueryZimmoListings(ctx, f)
				if err != nil {
					return err
				}
			}
			if len(rows) == 0 {
				emptyStoreHint(cmd, path)
			}
			w := cmd.OutOrStdout()
			switch {
			case format == "csv" || flags.csv:
				cw := csv.NewWriter(w)
				_ = cw.Write(zimmoCSVHeader)
				for _, r := range rows {
					_ = cw.Write(zimmoCSVRow(r))
				}
				cw.Flush()
				return cw.Error()
			case format == "geojson":
				features := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					if r.Lat == nil || r.Lng == nil {
						continue
					}
					props := map[string]any{"zimmo_code": r.Code, "url": r.URL, "status": r.Status, "type": r.Type, "address": r.Address,
						"price": r.Price, "surface_m2": r.Surface, "price_per_m2": r.PricePerM2, "epc": r.EPC, "bedrooms": r.Bedrooms}
					features = append(features, map[string]any{"type": "Feature", "geometry": map[string]any{"type": "Point", "coordinates": []float64{*r.Lng, *r.Lat}}, "properties": props})
				}
				enc := json.NewEncoder(w)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{"type": "FeatureCollection", "features": features})
			default:
				return printZimmo(w, rows, flags)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "json, csv or geojson")
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&status, "status", "", "Only this status: sale, rent, sold, rented")
	cmd.Flags().StringVar(&typ, "type", "", "Only this property type")
	cmd.Flags().BoolVar(&includeGone, "include-gone", false, "Include listings no longer on the market")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// ---------------------------------------------------------------- agency

type agencyView struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Category     string          `json:"category,omitempty"`
	Phone        string          `json:"phone,omitempty"`
	Website      string          `json:"website,omitempty"`
	Address      string          `json:"address,omitempty"`
	VAT          string          `json:"vat_number,omitempty"`
	ReviewScore  *float64        `json:"review_score_10,omitempty"`
	ReviewCount  int             `json:"review_count,omitempty"`
	ListingCount int             `json:"listings_count"`
	ForSale      int             `json:"for_sale"`
	ToRent       int             `json:"to_rent"`
	ZimmoURL     string          `json:"zimmo_url,omitempty"`
	Stored       []zimmo.Listing `json:"stored_listings,omitempty"`
}

func newAgencyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "agency [dealer-uuid|zimmo-code]",
		Short: "Show an agency: contact, VAT, review score, listings for sale and to rent, and its listings in your store",
		Long: `Give a dealer UUID, or a listing's Zimmo code to show the agency selling it.
Stored listings of that agency are appended from the local store.`,
		Example: strings.Trim(`
  zimmo-pp-cli agency LAISZ
  zimmo-pp-cli agency b3f3119c-dfa5-4397-b653-605944f1c3a1 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "ref=LAISZ"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch an agency")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give one dealer UUID or listing Zimmo code"))
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			zc := zimmoClient(flags)
			id := strings.ToLower(strings.TrimSpace(args[0]))
			if !(len(id) == 36 && strings.Count(id, "-") == 4) {
				ref, err := parseCodeArg(args[0])
				if err != nil {
					return usageErr(err)
				}
				l, err := fetchListing(ctx, zc, ref)
				if err != nil {
					return err
				}
				if l.AgencyID == "" {
					return notFoundErr(fmt.Errorf("listing %s has no agency (private seller or project)", l.Code))
				}
				id = l.AgencyID
			}
			raw, err := zc.Dealer(ctx, id)
			if err != nil {
				if isNotFound(err) {
					return notFoundErr(fmt.Errorf("agency %s not found on Zimmo", id))
				}
				return err
			}
			var d struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Category    string `json:"category"`
				PhoneNumber string `json:"phoneNumber"`
				Website     string `json:"website"`
				VatNumber   string `json:"vatNumber"`
				Location    struct {
					Street       string            `json:"street"`
					StreetNumber string            `json:"streetNumber"`
					PostalCode   string            `json:"postalCode"`
					Locality     map[string]string `json:"locality"`
				} `json:"location"`
				Enriched struct {
					URL   map[string]string `json:"url"`
					Stats struct {
						ListingsCount int `json:"listingsCount"`
						ForSale       int `json:"forSale"`
						ToRent        int `json:"toRent"`
					} `json:"stats"`
					Reviews struct {
						Combined struct {
							Score float64 `json:"score"`
							Count int     `json:"count"`
						} `json:"combined"`
					} `json:"reviews"`
				} `json:"enriched"`
			}
			if err := json.Unmarshal(raw, &d); err != nil {
				return fmt.Errorf("decoding agency: %w", err)
			}
			v := agencyView{ID: d.ID, Name: d.Name, Category: d.Category, Phone: d.PhoneNumber, Website: d.Website, VAT: d.VatNumber,
				ListingCount: d.Enriched.Stats.ListingsCount, ForSale: d.Enriched.Stats.ForSale, ToRent: d.Enriched.Stats.ToRent,
				ZimmoURL: d.Enriched.URL["fr"], ReviewCount: d.Enriched.Reviews.Combined.Count,
				Address: zimmo.FormatAddress(d.Location.Street, d.Location.StreetNumber, "", d.Location.PostalCode, d.Location.Locality["fr"])}
			if s := d.Enriched.Reviews.Combined.Score; s > 0 {
				v.ReviewScore = &s
			}
			if path, ok := localStoreExists(dbPath); ok {
				if db, err := openZimmoStore(ctx, path); err == nil {
					rows, _ := db.QueryZimmoListings(ctx, store.ListingFilter{})
					for _, r := range rows {
						if r.AgencyID == d.ID {
							v.Stored = append(v.Stored, r.Listing)
						}
					}
					_ = db.Close()
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (%s)\n", termSafe(v.Name), v.Category)
			fmt.Fprintf(w, "%s · %s · %s\n", termSafe(v.Address), v.Phone, v.Website)
			if v.ReviewScore != nil {
				fmt.Fprintf(w, "Reviews: %.1f/10 (%d)\n", *v.ReviewScore, v.ReviewCount)
			}
			fmt.Fprintf(w, "Listings: %d (%d for sale, %d to rent)\n", v.ListingCount, v.ForSale, v.ToRent)
			if len(v.Stored) > 0 {
				fmt.Fprintf(w, "\nIn your store:\n")
				return printListingTable(w, v.Stored)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// ---------------------------------------------------------------- prices

type pricesView struct {
	PlaceID  int                `json:"place_id"`
	Commune  string             `json:"commune"`
	Postcode string             `json:"postcode,omitempty"`
	PriceM2  float64            `json:"price_per_m2_all"`
	Types    []zimmo.TypePrice  `json:"by_type"`
	History  []zimmo.PricePoint `json:"history"`
	Change   map[string]float64 `json:"change_12m_pct,omitempty"`
}

func newPricesCmd(flags *rootFlags) *cobra.Command {
	var postcode, commune, since, dbPath string
	var placeID int
	cmd := &cobra.Command{
		Use:   "prices",
		Short: "Price per m² for a commune by property type, with the monthly history and 12-month change",
		Long: `Zimmo's price indicator: current €/m² for a commune (houses, apartments and
all types, with the number of listings behind each), the monthly series since
--since, and the change over the last 12 months per type.`,
		Example: strings.Trim(`
  zimmo-pp-cli prices --postcode 1050
  zimmo-pp-cli prices --commune schaerbeek --since 2023-01-01 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--postcode=1050"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch commune price per m²")
			}
			if postcode == "" && commune == "" && placeID == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give --postcode, --commune or --place-id"))
			}
			if since != "" {
				if _, err := time.Parse("2006-01-02", since); err != nil {
					return usageErr(fmt.Errorf("--since must be YYYY-MM-DD"))
				}
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			zc := zimmoClient(flags)
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			v := pricesView{PlaceID: placeID, Postcode: postcode}
			switch {
			case placeID > 0:
			case commune != "":
				id, err := resolveCommune(ctx, zc, db, commune)
				if err != nil {
					return err
				}
				v.PlaceID, v.Commune = id, commune
			default:
				id, err := communePlaceID(ctx, zc, db, zimmo.Listing{Code: postcode, PostalCode: postcode})
				if err != nil {
					return notFoundErr(err)
				}
				v.PlaceID = id
			}
			if since == "" {
				since = time.Now().AddDate(-2, 0, 0).Format("2006-01-02")
			}
			lp, err := zc.LocalityPriceFor(ctx, v.PlaceID, since, false)
			if err != nil {
				return err
			}
			if v.Commune == "" && postcode != "" {
				if places, err := lookupPlaces(ctx, zc, db, postcode, ""); err == nil {
					for _, p := range places {
						if p.ID == v.PlaceID {
							v.Commune = p.Name()
						}
					}
				}
			}
			v.PriceM2, v.Types, v.History = lp.Price, lp.Types, lp.History
			if v.Types == nil {
				v.Types = make([]zimmo.TypePrice, 0)
			}
			if v.History == nil {
				v.History = make([]zimmo.PricePoint, 0)
			}
			v.Change = change12m(lp.History)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (place %d): %s/m² all types\n", termSafe(v.Commune), v.PlaceID, fmtEUR(v.PriceM2))
			for _, t := range v.Types {
				fmt.Fprintf(w, "  %-10s %s/m²  (%d listings)", strings.ToLower(t.Type), fmtEUR(t.Price), t.Count)
				if c, ok := v.Change[t.Type]; ok {
					fmt.Fprintf(w, "  %+.1f%% over 12 months", c)
				}
				fmt.Fprintln(w)
			}
			tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "\nMONTH\tAPARTMENT\tHOUSE")
			for _, p := range v.History {
				m := map[string]float64{}
				for _, t := range p.Types {
					m[t.Type] = t.Price
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", monthOf(p.Date), eurOrDash(m["APARTMENT"]), eurOrDash(m["HOUSE"]))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&postcode, "postcode", "", "Postcode of the commune (1050)")
	cmd.Flags().StringVar(&commune, "commune", "", "Commune name (ixelles, elsene)")
	cmd.Flags().IntVar(&placeID, "place-id", 0, "Zimmo place id (see 'places')")
	cmd.Flags().StringVar(&since, "since", "", "History start date YYYY-MM-DD (default: 2 years ago)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// monthOf labels a series point by its month. Zimmo stamps months at local
// midnight in UTC ("2023-12-31T23:00:00Z" is January 2024).
func monthOf(date string) string {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return dateOnly(date)
	}
	return t.Add(12 * time.Hour).Format("2006-01")
}

func eurOrDash(v float64) string {
	if v <= 0 {
		return "-"
	}
	return fmtEUR(v)
}

// change12m compares the last point with the one 12 months earlier.
func change12m(h []zimmo.PricePoint) map[string]float64 {
	if len(h) < 13 {
		return nil
	}
	last, prev := h[len(h)-1], h[len(h)-13]
	out := map[string]float64{}
	pm := map[string]float64{}
	for _, t := range prev.Types {
		pm[t.Type] = t.Price
	}
	for _, t := range last.Types {
		if p := pm[t.Type]; p > 0 && t.Price > 0 {
			out[t.Type] = pct1((t.Price - p) / p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ---------------------------------------------------------------- shortlist

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	var dbPath, note string
	var remove bool
	cmd := &cobra.Command{
		Use:   "shortlist [zimmo-code]",
		Short: "Star listings with a note, or list your starred listings with their current stored state",
		Long: `Without arguments, lists starred listings. With a Zimmo code, stars it (and
stores it if it is not stored yet); --remove un-stars it. Local only; no
Zimmo account involved.`,
		Example: strings.Trim(`
  zimmo-pp-cli shortlist LAISZ --note "visit saturday"
  zimmo-pp-cli shortlist
  zimmo-pp-cli shortlist LAISZ --remove`, "\n"),
		Annotations: map[string]string{"mcp:local-write": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "update or list the shortlist")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			flags.agentSource = "local"
			path, exists := localStoreExists(dbPath)
			if len(args) == 0 {
				type row struct {
					store.ShortlistEntry
					Listing *store.StoredListing `json:"listing,omitempty"`
				}
				rows := make([]row, 0)
				if exists {
					db, err := openZimmoStore(ctx, path)
					if err != nil {
						return err
					}
					defer db.Close()
					entries, err := db.Shortlist(ctx)
					if err != nil {
						return err
					}
					for _, e := range entries {
						r := row{ShortlistEntry: e}
						if ls, err := db.QueryZimmoListings(ctx, store.ListingFilter{Codes: []string{e.Code}, IncludeGone: true}); err == nil && len(ls) == 1 {
							r.Listing = &ls[0]
						}
						rows = append(rows, r)
					}
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printZimmo(cmd.OutOrStdout(), rows, flags)
				}
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "Shortlist is empty. Add one with: zimmo-pp-cli shortlist <code> --note \"...\"")
					return nil
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "CODE\tPRICE\tADDRESS\tSTATE\tNOTE")
				for _, r := range rows {
					price, addr, state := "-", "", "not stored"
					if r.Listing != nil {
						price, addr, state = fmtPtrEUR(r.Listing.Price), r.Listing.Address, strings.ToLower(r.Listing.Status)
						if r.Listing.GoneAt != "" {
							state = "gone " + dateOnly(r.Listing.GoneAt)
						}
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Code, price, termSafe(addr), state, termSafe(r.Note))
				}
				return tw.Flush()
			}
			ref, err := parseCodeArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			db, err := openZimmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			if remove {
				ok, err := db.ShortlistRemove(ctx, ref)
				if err != nil {
					return err
				}
				return printZimmo(cmd.OutOrStdout(), map[string]any{"zimmo_code": ref, "removed": ok}, flags)
			}
			if ls, _ := db.QueryZimmoListings(ctx, refFilter(ref)); len(ls) == 1 {
				ref = ls[0].Code
			} else {
				l, err := fetchListing(ctx, zimmoClient(flags), ref)
				if err == nil {
					_ = db.UpsertZimmoListings(ctx, []zimmo.Listing{l}, time.Now(), true)
					ref = l.Code
				} else if isNotFound(err) {
					return err
				} else if isUUID(ref) {
					return fmt.Errorf("cannot resolve %s to a Zimmo code offline: %w", ref, err)
				}
			}
			if err := db.ShortlistAdd(ctx, ref, note, time.Now()); err != nil {
				return err
			}
			return printZimmo(cmd.OutOrStdout(), map[string]any{"zimmo_code": ref, "starred": true, "note": note}, flags)
		},
	}
	cmd.Flags().StringVar(&note, "note", "", "Note to keep with the starred listing")
	cmd.Flags().BoolVar(&remove, "remove", false, "Un-star the listing")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// ---------------------------------------------------------------- drops

type dropRow struct {
	Code       string   `json:"zimmo_code"`
	Address    string   `json:"address"`
	Type       string   `json:"type"`
	From       float64  `json:"from_price"`
	To         float64  `json:"to_price"`
	CutEUR     float64  `json:"cut_eur"`
	CutPct     float64  `json:"cut_pct"`
	Since      string   `json:"since"`
	Source     string   `json:"source"` // zimmo_history | local_observations
	URL        string   `json:"url"`
	PricePerM2 *float64 `json:"price_per_m2"`
}

func newDropsCmd(flags *rootFlags) *cobra.Command {
	var postcode, since, dbPath string
	var minPct float64
	var limit int
	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Price cuts on stored listings, from Zimmo's own price history and your sync observations",
		Long: `Lists stored listings whose asking price went down. Zimmo publishes each
listing's price history (visible on most listings), so cuts made before you
started watching are included; cuts seen between your own find/watch runs
are added from local observations.`,
		Example: strings.Trim(`
  zimmo-pp-cli drops --since 30d
  zimmo-pp-cli drops --postcode 1030,1210 --min-pct 5 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--since=90d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list price cuts")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			cutoff := ""
			if since != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				cutoff = time.Now().Add(-d).UTC().Format(time.RFC3339)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows := make([]dropRow, 0)
			path, ok := localStoreExists(dbPath)
			if ok {
				db, err := openZimmoStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				listings, err := db.QueryZimmoListings(ctx, store.ListingFilter{Postcodes: splitCSV(postcode)})
				if err != nil {
					return err
				}
				obs, err := db.AllZimmoPriceHistories(ctx)
				if err != nil {
					return err
				}
				rows = computeDrops(listings, obs, cutoff, minPct)
			}
			if len(rows) == 0 && !ok {
				emptyStoreHint(cmd, path)
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No price cuts among stored listings.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tFROM\tTO\tCUT\tSINCE\tADDRESS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t-%.1f%%\t%s\t%s\n", r.Code, fmtEUR(r.From), fmtEUR(r.To), r.CutPct, dateOnly(r.Since), termSafe(r.Address))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&since, "since", "", "Only cuts newer than this window (30d, 12w)")
	cmd.Flags().Float64Var(&minPct, "min-pct", 0, "Minimum cut in percent")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// computeDrops merges Zimmo's price history with local observations and
// keeps listings whose latest price is below the earliest known price.
func computeDrops(listings []store.StoredListing, obs map[string][]store.PriceObs, cutoff string, minPct float64) []dropRow {
	rows := make([]dropRow, 0)
	for _, l := range listings {
		type pt struct {
			at    string
			price float64
		}
		var pts []pt
		for _, ph := range l.PriceHistory {
			if ph.Before > 0 {
				pts = append(pts, pt{ph.Date, ph.Before})
			}
			if ph.After > 0 {
				pts = append(pts, pt{ph.Date, ph.After})
			}
		}
		src := "zimmo_history"
		for _, o := range obs[l.Code] {
			pts = append(pts, pt{o.ObservedAt, o.Price})
		}
		if len(l.PriceHistory) == 0 {
			src = "local_observations"
		}
		if len(pts) < 2 {
			continue
		}
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].at < pts[j].at })
		first, last := pts[0], pts[len(pts)-1]
		// The cut date is the first point at the final price.
		since := last.at
		for i := len(pts) - 1; i >= 0 && pts[i].price == last.price; i-- {
			since = pts[i].at
		}
		if last.price >= first.price || first.price <= 0 {
			continue
		}
		if cutoff != "" && since < cutoff {
			continue
		}
		pct := pct1((first.price - last.price) / first.price)
		if pct < minPct {
			continue
		}
		rows = append(rows, dropRow{Code: l.Code, Address: l.Address, Type: l.Type, From: first.price, To: last.price, CutEUR: first.price - last.price,
			CutPct: pct, Since: since, Source: src, URL: l.URL, PricePerM2: l.PricePerM2})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CutPct > rows[j].CutPct })
	return rows
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPhotosCmd(flags))
		addNovelCommandIfAbsent(root, newDumpCmd(flags))
		addNovelCommandIfAbsent(root, newAgencyCmd(flags))
		addNovelCommandIfAbsent(root, newPricesCmd(flags))
		addNovelCommandIfAbsent(root, newShortlistCmd(flags))
		addNovelCommandIfAbsent(root, newDropsCmd(flags))
	})
}
