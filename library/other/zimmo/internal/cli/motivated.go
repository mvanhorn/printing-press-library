// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type motivatedRow struct {
	Code         string   `json:"zimmo_code"`
	Score        float64  `json:"score"`
	Reasons      []string `json:"reasons"`
	DaysOnMarket *int     `json:"days_on_market"`
	TotalCutPct  *float64 `json:"total_cut_pct"`
	RelistedFrom []string `json:"relisted_from,omitempty"`
	MarketDays   *int     `json:"days_since_first_listing,omitempty"`
	EPC          string   `json:"epc"`
	Rented       bool     `json:"rented"`
	Price        *float64 `json:"price"`
	PricePerM2   *float64 `json:"price_per_m2"`
	Address      string   `json:"address"`
	URL          string   `json:"url"`
}

func newNovelMotivatedCmd(flags *rootFlags) *cobra.Command {
	var postcode, typ, dbPath string
	var minDays, limit int
	cmd := &cobra.Command{
		Use:   "motivated",
		Short: "Rank listings by seller pressure: days on market, total price cut, re-listing, EPC.",
		Long: `Scores stored for-sale listings on seller-pressure signals and shows why:
days on market (Zimmo's publication date), the total cut over Zimmo's own
price history plus your observations, re-listing (another stored Zimmo code
at the same exact address, which resets the on-market counter on the site),
and an F/G EPC. Score = months on market (from 90 days, capped at 12) +
cut % + 5 per re-listing + 3 for F/G. --min-days keeps only listings on the
market at least that long (re-listings count their first publication).
Use this command to rank stored listings by seller-pressure signals (age,
total cut, re-listing, EPC). Do NOT use it for price cuts observed between
your own syncs; use 'drops' instead.`,
		Example: strings.Trim(`
  zimmo-pp-cli motivated --postcode 1030 --min-days 120
  zimmo-pp-cli motivated --type house --agent --select zimmo_code,score,reasons`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank stored listings by seller pressure")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			var types []string
			if typ != "" {
				cat, err := zimmo.ParseCategory(typ)
				if err != nil {
					return usageErr(err)
				}
				types = []string{cat}
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			rows := make([]motivatedRow, 0)
			path, ok := localStoreExists(dbPath)
			if !ok {
				emptyStoreHint(cmd, path)
				return printZimmo(cmd.OutOrStdout(), rows, flags)
			}
			db, err := openZimmoStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			active, err := db.QueryZimmoListings(ctx, store.ListingFilter{Postcodes: splitCSV(postcode), Statuses: []string{"FOR_SALE"}, Types: types})
			if err != nil {
				return err
			}
			everything, err := db.QueryZimmoListings(ctx, store.ListingFilter{IncludeGone: true})
			if err != nil {
				return err
			}
			obs, err := db.AllZimmoPriceHistories(ctx)
			if err != nil {
				return err
			}
			if len(active) == 0 {
				emptyStoreHint(cmd, path)
			}
			rows = scoreMotivated(active, everything, obs, minDays)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stored listing shows seller-pressure signals.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tSCORE\tPRICE\tWHY\tADDRESS")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%.1f\t%s\t%s\t%s\n", r.Code, r.Score, fmtPtrEUR(r.Price), strings.Join(r.Reasons, "; "), termSafe(r.Address))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&typ, "type", "", "Only this property type")
	cmd.Flags().IntVar(&minDays, "min-days", 0, "Keep only listings on the market at least this many days")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

// motivatedAgeSignalDays is when time on market starts to count.
const motivatedAgeSignalDays = 90

// scoreMotivated is pure so it can be unit-tested.
func scoreMotivated(active, everything []store.StoredListing, obs map[string][]store.PriceObs, minDays int) []motivatedRow {
	byAddr := map[string][]store.StoredListing{}
	for _, l := range everything {
		if k := store.AddrKey(l.PostalCode, l.Street+" "+l.Number); k != "" {
			byAddr[k] = append(byAddr[k], l)
		}
	}
	drops := computeDrops(active, obs, "", 0)
	cut := map[string]float64{}
	for _, d := range drops {
		cut[d.Code] = d.CutPct
	}
	rows := make([]motivatedRow, 0)
	for _, l := range active {
		r := motivatedRow{Code: l.Code, DaysOnMarket: l.DaysOnMarket, EPC: l.EPC, Rented: l.Rented, Price: l.Price, PricePerM2: l.PricePerM2, Address: l.Address, URL: l.URL}
		days := 0
		if l.DaysOnMarket != nil {
			days = *l.DaysOnMarket
		}
		// Re-listing: an older code at the same exact address (same type,
		// different unit box excluded) extends the true time on market.
		if k := store.AddrKey(l.PostalCode, l.Street+" "+l.Number); k != "" {
			for _, o := range byAddr[k] {
				// Only an earlier sale listing counts: a flat once let or sold
				// years ago and now for sale is a resale, not a re-listing.
				if o.Code == l.Code || (o.Status != "FOR_SALE" && o.Status != "TAKE_OVER") || !sameUnit(o.Listing, l.Listing) {
					continue
				}
				if relistedBefore(o.PublishedAt, l.PublishedAt) {
					r.RelistedFrom = append(r.RelistedFrom, o.Code)
					if t, err := time.Parse(time.RFC3339, o.PublishedAt); err == nil {
						if md := int(time.Since(t).Hours() / 24); md > days && (r.MarketDays == nil || md > *r.MarketDays) {
							r.MarketDays = &md
						}
					}
				}
			}
		}
		effDays := days
		if r.MarketDays != nil {
			effDays = *r.MarketDays
		}
		if effDays < minDays {
			continue
		}
		if effDays >= motivatedAgeSignalDays {
			r.Score += math.Min(float64(effDays)/30, 12)
			r.Reasons = append(r.Reasons, fmt.Sprintf("%d days on market", effDays))
		}
		if c := cut[l.Code]; c > 0 {
			r.TotalCutPct = ptrF(c)
			r.Score += c
			r.Reasons = append(r.Reasons, fmt.Sprintf("price cut %.1f%%", c))
		}
		if len(r.RelistedFrom) > 0 {
			r.Score += 5 * float64(len(r.RelistedFrom))
			r.Reasons = append(r.Reasons, "re-listed (was "+strings.Join(r.RelistedFrom, ", ")+")")
		}
		if letter := zimmo.EPCLetter(l.EPC); letter == "F" || letter == "G" {
			r.Score += 3
			r.Reasons = append(r.Reasons, "EPC "+l.EPC)
		}
		if len(r.Reasons) == 0 || (effDays < motivatedAgeSignalDays && r.TotalCutPct == nil && len(r.RelistedFrom) == 0) {
			continue
		}
		r.Score = math.Round(r.Score*10) / 10
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
	return rows
}

// sameUnit tells a re-listing of one property apart from sibling units of
// one building: same type and box, and living surface within 5% when both
// are known, and the same bedroom count when both are known.
func sameUnit(a, b zimmo.Listing) bool {
	if a.Type != b.Type || a.Box != b.Box {
		return false
	}
	if a.Surface == nil || b.Surface == nil || !within(a.Surface, b.Surface, 0.05) {
		return false
	}
	if a.Bedrooms != nil && b.Bedrooms != nil && *a.Bedrooms != *b.Bedrooms {
		return false
	}
	return true
}

// relistedBefore requires the older publication to precede the newer by at
// least 14 days: units of one building are published together.
func relistedBefore(older, newer string) bool {
	to, err1 := time.Parse(time.RFC3339, older)
	tn, err2 := time.Parse(time.RFC3339, newer)
	if err1 != nil || err2 != nil {
		return false
	}
	gap := tn.Sub(to)
	return gap >= 14*24*time.Hour && gap <= 3*365*24*time.Hour
}
