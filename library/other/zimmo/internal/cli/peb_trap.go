// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type pebRow struct {
	Code           string   `json:"zimmo_code"`
	Address        string   `json:"address"`
	Type           string   `json:"type"`
	Price          *float64 `json:"price"`
	PricePerM2     *float64 `json:"price_per_m2"`
	EPC            string   `json:"epc"`
	EPCKWh         *float64 `json:"epc_kwh_m2"`
	EPCSuspect     bool     `json:"epc_kwh_suspect,omitempty"`
	SaleFlags      []string `json:"sale_flags,omitempty"`
	RenovationDuty string   `json:"renovation_obligation"`
	Rented         bool     `json:"rented"`
	RentPerYear    *float64 `json:"rent_per_year"`
	GrossYieldPct  *float64 `json:"gross_yield_pct"`
	DaysOnMarket   *int     `json:"days_on_market"`
	TotalCutPct    *float64 `json:"total_cut_pct"`
	Condition      string   `json:"condition"`
	Agency         string   `json:"agency"`
	URL            string   `json:"url"`
}

func newNovelPebTrapCmd(flags *rootFlags) *cobra.Command {
	var postcode, epc, typ, dbPath string
	var maxPrice, minKWh float64
	var rentedOnly, dutyOnly bool
	var limit int
	cmd := &cobra.Command{
		Use:   "peb-trap",
		Short: "Rank stored energy-poor listings (EPC F/G or high kWh/m²), rented ones first with their rent, with the renovation-obligation flag",
		Long: `Lists stored for-sale listings with a bad EPC (F/G by default, or a kWh/m²
above --min-kwh), showing Zimmo's renovation-obligation flag and, when the
property is let, the current rent and gross yield. Rented energy-poor
properties come first: their owners face both a renovation deadline and a
tenant, the classic motivated seller.
Use this command to list energy-poor listings (EPC F/G or high kWh/m²) with a
renovation obligation. Search results already carry the EPC kWh and rent;
run 'enrich' to refresh older rows. 'find --epc F,G' only filters by label.`,
		Example: strings.Trim(`
  zimmo-pp-cli peb-trap --postcode 1030 --max-price 450000
  zimmo-pp-cli peb-trap --rented-only --json
  zimmo-pp-cli peb-trap --epc G --min-kwh 500 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list energy-poor stored listings")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			letters, err := zimmo.ParseEPC(epc)
			if err != nil {
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
			rows := make([]pebRow, 0)
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
			f := store.ListingFilter{Postcodes: splitCSV(postcode), Statuses: []string{"FOR_SALE"}, Types: types}
			if maxPrice > 0 {
				f.MaxPrice = &maxPrice
			}
			listings, err := db.QueryZimmoListings(ctx, f)
			if err != nil {
				return err
			}
			if len(listings) == 0 {
				emptyStoreHint(cmd, path)
			}
			rows = pebTrapRows(listings, letters, minKWh, rentedOnly, dutyOnly)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No energy-poor listing among stored for-sale listings.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "CODE\tEPC\tKWH\tRENOV\tRENTED\tYIELD\tPRICE\t€/M²\tDAYS\tADDRESS")
			for _, r := range rows {
				rented, yield := "-", "-"
				if r.Rented {
					rented = fmtPtrEUR(r.RentPerYear) + "/y"
				}
				if r.GrossYieldPct != nil {
					yield = fmt.Sprintf("%.1f%%", *r.GrossYieldPct)
				}
				kwh := floatUnit(r.EPCKWh, "")
				if r.EPCSuspect {
					kwh += "?"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Code, r.EPC, kwh, strings.ToLower(r.RenovationDuty), rented, yield,
					fmtPtrEUR(r.Price), fmtPtrEUR(r.PricePerM2), intStr(r.DaysOnMarket), termSafe(r.Address))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&postcode, "postcode", "", "Only these postcodes (comma-separated)")
	cmd.Flags().StringVar(&epc, "epc", "F,G", "EPC letters that qualify")
	cmd.Flags().Float64Var(&minKWh, "min-kwh", 0, "Also qualify listings at or above this kWh/m² whatever their letter")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum asking price in EUR")
	cmd.Flags().StringVar(&typ, "type", "", "Only this property type")
	cmd.Flags().BoolVar(&rentedOnly, "rented-only", false, "Only properties that are currently let")
	cmd.Flags().BoolVar(&dutyOnly, "renovation-duty", false, "Only listings whose renovation obligation is YES")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func pebTrapRows(listings []store.StoredListing, letters []string, minKWh float64, rentedOnly, dutyOnly bool) []pebRow {
	bad := map[string]bool{}
	for _, l := range letters {
		bad[l] = true
	}
	rows := make([]pebRow, 0)
	for _, l := range listings {
		qualifies := bad[zimmo.EPCLetter(l.EPC)] || (minKWh > 0 && l.EPCKWh != nil && !l.EPCSuspect && *l.EPCKWh >= minKWh)
		if !qualifies || (rentedOnly && !l.Rented) || (dutyOnly && l.RenovationDuty != "YES") {
			continue
		}
		r := pebRow{Code: l.Code, Address: l.Address, Type: l.Type, Price: l.Price, PricePerM2: l.PricePerM2, EPC: l.EPC, EPCKWh: l.EPCKWh,
			EPCSuspect: l.EPCSuspect, SaleFlags: l.SaleFlags, RenovationDuty: l.RenovationDuty, Rented: l.Rented, RentPerYear: l.RentPerYear, DaysOnMarket: l.DaysOnMarket, TotalCutPct: l.TotalCutPct,
			Condition: l.Condition, Agency: l.Agency, URL: l.URL}
		if r.RenovationDuty == "" {
			r.RenovationDuty = "UNKNOWN"
		}
		if l.Rented && l.RentPerYear != nil && l.Price != nil && *l.Price > 0 {
			y := math.Round(*l.RentPerYear / *l.Price * 1000) / 10
			r.GrossYieldPct = &y
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Rented != rows[j].Rented {
			return rows[i].Rented
		}
		li, lj := zimmo.EPCLetter(rows[i].EPC), zimmo.EPCLetter(rows[j].EPC)
		if li != lj {
			return li > lj // G before F
		}
		return plausibleKWh(rows[i]) > plausibleKWh(rows[j])
	})
	return rows
}

// plausibleKWh sorts suspect kWh values (input errors) last within a letter.
func plausibleKWh(r pebRow) float64 {
	if r.EPCKWh == nil || r.EPCSuspect {
		return 0
	}
	return *r.EPCKWh
}
