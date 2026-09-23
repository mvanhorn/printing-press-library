// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type compRow struct {
	Code       string   `json:"zimmo_code"`
	Status     string   `json:"status"`
	DistanceM  int      `json:"distance_m"`
	Price      *float64 `json:"price"`
	Surface    *float64 `json:"surface_m2"`
	PricePerM2 *float64 `json:"price_per_m2"`
	Bedrooms   *int     `json:"bedrooms"`
	EPC        string   `json:"epc"`
	Year       *int     `json:"construction_year"`
	Date       string   `json:"date"`
	Scope      string   `json:"scope"` // radius | postcode
	Address    string   `json:"address"`
	URL        string   `json:"url"`
}

type compsSubject struct {
	Code       string   `json:"zimmo_code,omitempty"`
	Address    string   `json:"address"`
	Lat        float64  `json:"lat"`
	Lng        float64  `json:"lng"`
	Type       string   `json:"type"`
	Price      *float64 `json:"price,omitempty"`
	Surface    *float64 `json:"surface_m2,omitempty"`
	PricePerM2 *float64 `json:"price_per_m2,omitempty"`
}

type compsStats struct {
	Count        int      `json:"count"`
	MedianPPM2   *float64 `json:"median_price_per_m2"`
	P25PPM2      *float64 `json:"p25_price_per_m2"`
	P75PPM2      *float64 `json:"p75_price_per_m2"`
	SubjectGap   *float64 `json:"subject_vs_median_pct,omitempty"`
	ImpliedValue *float64 `json:"implied_value_at_median,omitempty"`
	ImpliedRent  *float64 `json:"implied_monthly_rent_at_median,omitempty"`
}

type compsView struct {
	Subject      compsSubject `json:"subject"`
	Statuses     []string     `json:"statuses"`
	RadiusM      int          `json:"radius_m"`
	WidenedTo    string       `json:"widened_to,omitempty"`
	Months       int          `json:"months"`
	Stats        compsStats   `json:"stats"`
	Comps        []compRow    `json:"comps"`
	ScannedComps int          `json:"scanned_listings"`
	MaxScanPages int          `json:"max_scan_pages"`
	Note         string       `json:"note,omitempty"`
}

func ptrF(v float64) *float64 { return &v }

func newNovelCompsCmd(flags *rootFlags) *cobra.Command {
	var radius, statusCSV, typ, address string
	var months, limit, maxScanPages, band int
	var noWiden bool
	cmd := &cobra.Command{
		Use:   "comps [zimmo-code]",
		Short: "Sold or rented listings around one property with median €/m² and the subject's gap to it",
		Long: `Finds comparable SOLD (default) or RENTED listings of the same property type
inside --radius of the subject's exact GPS point, within a living-surface band,
updated in the last --months, and reports the median, p25 and p75 €/m² with
the subject's gap to the median. For --status rented the prices are monthly
rents. Give a Zimmo code, or --address plus --type for a property that is not
listed. The sold date is Zimmo's last update of the listing.
Use this command for comparable SOLD/RENTED listings around one property.
Do NOT use this command to rank many stored listings against commune prices;
use 'underpriced' instead.`,
		Example: strings.Trim(`
  zimmo-pp-cli comps LAISZ --radius 800m --months 24 --agent
  zimmo-pp-cli comps LRWGW --status rented --radius 500m
  zimmo-pp-cli comps --address "Rue Anatole France 37, 1030 Schaerbeek" --type house --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "code=LRWGW;--radius=1000m;--months=24"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "find sold/rented comparables")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && address == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a Zimmo code or --address"))
			}
			radiusM, err := parseMeters(radius)
			if err != nil {
				return usageErr(err)
			}
			var statuses []string
			for _, s := range splitCSV(statusCSV) {
				st, err := zimmo.ParseStatus(s)
				if err != nil {
					return usageErr(err)
				}
				statuses = append(statuses, st)
			}
			if len(statuses) == 0 {
				statuses = []string{"SOLD"}
			}
			rentalCount := 0
			for _, st := range statuses {
				if st == "RENTED" || st == "TO_RENT" {
					rentalCount++
				}
			}
			if rentalCount > 0 && rentalCount < len(statuses) {
				return usageErr(fmt.Errorf("--status mixes sale and rent prices; run comps once with sold and once with rented"))
			}
			if cliutil.IsAnyHarness() && maxScanPages > 1 {
				maxScanPages = 1
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			zc := zimmoClient(flags)
			subj := compsSubject{}
			subjPostcode := ""
			if len(args) == 1 {
				ref, err := parseCodeArg(args[0])
				if err != nil {
					return usageErr(err)
				}
				l, err := fetchListing(ctx, zc, ref)
				if err != nil {
					return err
				}
				subj = compsSubject{Code: l.Code, Address: l.Address, Type: l.Type, Price: l.Price, Surface: l.Surface, PricePerM2: l.PricePerM2}
				subjPostcode = l.PostalCode
				if l.Lat != nil && l.GeoPrecision != "NO_DATA" {
					subj.Lat, subj.Lng = *l.Lat, *l.Lng
				} else if l.Street != "" {
					g, err := zc.Geocode(ctx, zimmo.FormatAddress(l.Street, l.Number, "", l.PostalCode, l.Locality))
					if err != nil {
						return fmt.Errorf("listing %s has no GPS and its address could not be geocoded: %w", l.Code, err)
					}
					subj.Lat, subj.Lng = g.Latitude, g.Longitude
				} else {
					return notFoundErr(fmt.Errorf("listing %s has neither GPS nor a street address; use --address", l.Code))
				}
			} else {
				g, err := zc.Geocode(ctx, address)
				if err != nil {
					return err
				}
				subj = compsSubject{Address: address, Lat: g.Latitude, Lng: g.Longitude}
				for _, p := range g.Places {
					if p.Area.PostalCode != "" {
						subjPostcode = p.Area.PostalCode
					}
				}
			}
			if typ != "" {
				cat, err := zimmo.ParseCategory(typ)
				if err != nil {
					return usageErr(err)
				}
				subj.Type = cat
			}
			if subj.Type == "" || subj.Type == "PROJECT" {
				return usageErr(fmt.Errorf("give --type (house, apartment...) for this subject"))
			}
			crit := zimmo.Criteria{Statuses: statuses, Categories: []string{subj.Type}, Polygon: zimmo.SquareAround(subj.Lat, subj.Lng, float64(radiusM)), Sort: "newest"}
			if band > 0 && subj.Surface != nil && *subj.Surface > 0 {
				lo := int(math.Floor(*subj.Surface * (1 - float64(band)/100)))
				hi := int(math.Ceil(*subj.Surface * (1 + float64(band)/100)))
				crit.MinSurface, crit.MaxSurface = &lo, &hi
			}
			res, err := walkSearch(ctx, zc, crit, maxScanPages, 0)
			if err != nil {
				return err
			}
			since := time.Now().AddDate(0, -months, 0).UTC().Format(time.RFC3339)
			rows := make([]compRow, 0)
			seen := map[string]bool{}
			var pps []float64
			collect := func(ls []zimmo.Listing, scope string) {
				for _, l := range ls {
					if l.Code == subj.Code || seen[l.Code] || l.IsProject {
						continue
					}
					date := firstNonEmptyStr(l.UpdatedAt, l.PublishedAt)
					if months > 0 && date != "" && date < since {
						continue
					}
					dist := -1
					precise := l.Lat != nil && l.Lng != nil && l.GeoPrecision == "ROOFTOP"
					if precise {
						dist = int(zimmo.DistanceM(subj.Lat, subj.Lng, *l.Lat, *l.Lng))
					}
					// Inside the radius only rooftop-precise points count: other
					// precisions sit on the commune centre.
					if scope == "radius" && (!precise || dist > radiusM) {
						continue
					}
					seen[l.Code] = true
					rows = append(rows, compRow{Code: l.Code, Status: l.Status, DistanceM: dist, Price: l.Price, Surface: l.Surface, PricePerM2: l.PricePerM2,
						Bedrooms: l.Bedrooms, EPC: l.EPC, Year: l.Year, Date: dateOnly(date), Scope: scope, Address: l.Address, URL: l.URL})
					if l.PricePerM2 != nil && *l.PricePerM2 > 0 {
						pps = append(pps, *l.PricePerM2)
					}
				}
			}
			collect(res.Listings, "radius")
			scanned, total := res.Scanned, res.Total
			v := compsView{Subject: subj, Statuses: statuses, RadiusM: radiusM, Months: months, MaxScanPages: maxScanPages}
			if len(pps) < 5 && subjPostcode != "" && !noWiden {
				wide := crit
				wide.Polygon = nil
				wide.Postcodes = []string{subjPostcode}
				if more, err := walkSearch(ctx, zc, wide, maxScanPages, 0); err == nil {
					collect(more.Listings, "postcode")
					scanned += more.Scanned
					v.WidenedTo = "postcode " + subjPostcode
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Scope != rows[j].Scope {
					return rows[i].Scope == "radius"
				}
				return rows[i].DistanceM < rows[j].DistanceM
			})
			v.Comps, v.ScannedComps = rows, scanned
			rental := true
			for _, st := range statuses {
				if st != "RENTED" && st != "TO_RENT" {
					rental = false
				}
			}
			v.Stats.Count = len(pps)
			if m, ok := zimmo.Median(pps); ok {
				v.Stats.MedianPPM2 = ptrF(math.Round(m))
				q1, _ := zimmo.Quantile(pps, 0.25)
				q3, _ := zimmo.Quantile(pps, 0.75)
				v.Stats.P25PPM2, v.Stats.P75PPM2 = ptrF(math.Round(q1)), ptrF(math.Round(q3))
				switch {
				case rental && subj.Surface != nil:
					v.Stats.ImpliedRent = ptrF(math.Round(m * *subj.Surface))
				case !rental && len(pps) >= 3:
					if subj.PricePerM2 != nil && m > 0 {
						v.Stats.SubjectGap = ptrF(pct1((*subj.PricePerM2 - m) / m))
					}
					if subj.Surface != nil {
						v.Stats.ImpliedValue = ptrF(math.Round(m * *subj.Surface))
					}
				}
			}
			if limit > 0 && len(v.Comps) > limit {
				v.Comps = v.Comps[:limit]
			}
			switch {
			case len(rows) == 0 && scanned >= total:
				v.Note = fmt.Sprintf("no %s comparable within %dm in the last %d months; widen --radius or --months, or drop the surface band with --surface-band 0", strings.ToLower(strings.Join(statuses, "/")), radiusM, months)
			case res.Scanned < res.Total:
				v.Note = fmt.Sprintf("scanned %d of %d candidates; raise --max-scan-pages for a complete set", res.Scanned, res.Total)
			case v.WidenedTo != "" && v.Stats.Count < 5:
				v.Note = "Zimmo lists few " + strings.ToLower(strings.Join(statuses, "/")) + " comparables here, even across the postcode: treat the median as indicative"
			case v.WidenedTo != "":
				v.Note = "fewer than 5 comparables inside the radius; widened to the postcode (scope=postcode rows)"
			case v.Stats.Count < 5:
				v.Note = "fewer than 5 priced comparables: treat the median as indicative"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), v, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Subject: %s  %s  %s  %s/m²\n", subj.Code, termSafe(subj.Address), fmtPtrEUR(subj.Price), fmtPtrEUR(subj.PricePerM2))
			if v.Stats.MedianPPM2 != nil {
				fmt.Fprintf(w, "%d comps: median %s/m² (p25 %s, p75 %s)", v.Stats.Count, fmtEUR(*v.Stats.MedianPPM2), fmtEUR(*v.Stats.P25PPM2), fmtEUR(*v.Stats.P75PPM2))
				if v.Stats.SubjectGap != nil {
					fmt.Fprintf(w, "; subject %+.1f%% vs median", *v.Stats.SubjectGap)
				}
				if v.Stats.ImpliedRent != nil {
					fmt.Fprintf(w, "; implied rent %s/month", fmtEUR(*v.Stats.ImpliedRent))
				}
				fmt.Fprintln(w)
			}
			tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tSTATUS\tDIST\tPRICE\tM²\t€/M²\tEPC\tDATE\tADDRESS")
			for _, r := range v.Comps {
				dist := fmt.Sprintf("%dm", r.DistanceM)
				if r.DistanceM < 0 {
					dist = "postcode"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Code, strings.ToLower(r.Status), dist, fmtPtrEUR(r.Price), floatUnit(r.Surface, ""), fmtPtrEUR(r.PricePerM2), r.EPC, r.Date, termSafe(r.Address))
			}
			_ = tw.Flush()
			if v.Note != "" {
				fmt.Fprintln(w, "note: "+v.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&radius, "radius", "800m", "Search radius around the subject (500m, 1.5km)")
	cmd.Flags().IntVar(&months, "months", 24, "Only comparables updated in the last N months (0 = any)")
	cmd.Flags().StringVar(&statusCSV, "status", "sold", "sold (sale prices) or rented (monthly rents); also sale or rent for current listings")
	cmd.Flags().StringVar(&typ, "type", "", "Property type when it differs from the subject or with --address")
	cmd.Flags().StringVar(&address, "address", "", "Subject address when the property is not a Zimmo listing")
	cmd.Flags().IntVar(&band, "surface-band", 30, "Keep comparables within ±N% of the subject's living surface (0 disables)")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum comparables listed (stats use all)")
	cmd.Flags().BoolVar(&noWiden, "no-widen", false, "Do not widen to the whole postcode when the radius holds fewer than 5 comparables")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Maximum result pages (100 each) to scan")
	return cmd
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseMeters accepts 800, 800m, 1.5km.
func parseMeters(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "km"):
		mult, s = 1000, strings.TrimSuffix(s, "km")
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid distance %q (use 800m or 1.5km)", s)
	}
	m := int(v * mult)
	if m > 20000 {
		return 0, fmt.Errorf("radius above 20km is not a comparable set")
	}
	return m, nil
}
