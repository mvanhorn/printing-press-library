// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

type marketBand struct {
	Band             string   `json:"band"`
	N                int      `json:"n"`
	MedianPrice      *float64 `json:"median_price,omitempty"`
	MedianPricePerM2 *float64 `json:"median_price_per_m2,omitempty"`
}

type marketCommune struct {
	Commune          string       `json:"commune"`
	Label            string       `json:"label,omitempty"`
	PostalCodes      []string     `json:"postal_codes"`
	LiveCount        *int         `json:"live_count_on_immoweb,omitempty"`
	Sample           int          `json:"sample"`
	CoveragePct      *float64     `json:"coverage_pct,omitempty"`
	MedianPrice      *float64     `json:"median_price,omitempty"`
	MedianPricePerM2 *float64     `json:"median_price_per_m2,omitempty"`
	P25PricePerM2    *float64     `json:"p25_price_per_m2,omitempty"`
	P75PricePerM2    *float64     `json:"p75_price_per_m2,omitempty"`
	MedianDaysListed *float64     `json:"median_days_listed,omitempty"`
	DaysSample       int          `json:"days_listed_sample"`
	UnderOptionShare *float64     `json:"under_option_pct,omitempty"`
	GoneLast30d      int          `json:"gone_last_30d"`
	PriceCutShare    *float64     `json:"price_cut_pct,omitempty"`
	Bands            []marketBand `json:"bands"`
	RoomLetsExcluded int          `json:"room_lets_excluded,omitempty"`
	Note             string       `json:"note,omitempty"`
}

type marketView struct {
	Deal     string          `json:"deal"`
	Types    []string        `json:"types"`
	By       string          `json:"by"`
	Communes []marketCommune `json:"communes"`
	Warnings []string        `json:"warnings,omitempty"`
}

func newNovelMarketCmd(flags *rootFlags) *cobra.Command {
	var flagType, flagDeal, flagBy, postcodes, dbPath, maxAge string
	var maxPages int
	var noPull, includeRooms bool

	cmd := &cobra.Command{
		Use:   "market [commune...]",
		Short: "Median asking price, €/m², price-cut share and recent disappearances for one or several communes",
		Long: `Commune market snapshot rebuilt from Immoweb listings (the site's own price map is gone).
For each commune: exact live listing count, median price and €/m² (with p25/p75),
share under option or reduced, listings gone in the last 30 days, and a breakdown
by bedrooms, EPC label, property type or postcode. Median days listed appears once
enough listings carry a publication date (search results omit it; deal, show and
triage --enrich fetch it). Brussels communes use their own postal codes only;
student rooms and per-room lets are left out (--include-rooms keeps them). Stale
or missing areas are pulled automatically (use --no-pull or --data-source local
to stay offline).

Use this command for commune-level statistics (count, median price, €/m², days listed, disappearances) and side-by-side commune comparison.
Do NOT use this command to judge one specific listing; use 'deal' instead.
Do NOT use it for rental return; use 'yield' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli market ixelles saint-gilles --type apartment --deal rent --by bedrooms --agent
  immoweb-pp-cli market namur --type house --deal sale --by epc
  immoweb-pp-cli market --postcode 4000,4020 --type apartment --deal sale --no-pull`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source": "auto",
			"pp:happy-args":  "commune=ixelles;--type=apartment;--deal=rent;--by=bedrooms",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "market")
			}
			useHarvestRate(cmd, flags)
			names := append([]string{}, args...)
			for _, p := range immo.SplitCSV(postcodes) {
				names = append(names, p)
			}
			if len(names) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give at least one commune (e.g. market ixelles) or --postcode"))
			}
			if flagType == "" || flagDeal == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--type and --deal are required (e.g. --type apartment --deal rent)"))
			}
			types, err := immo.NormalizeTypes(flagType)
			if err != nil {
				return usageErr(err)
			}
			deal, err := immo.NormalizeDeal(flagDeal)
			if err != nil {
				return usageErr(err)
			}
			by := strings.ToLower(flagBy)
			if by != "bedrooms" && by != "epc" && by != "type" && by != "postcode" {
				return usageErr(fmt.Errorf("--by must be bedrooms, epc, type or postcode"))
			}
			age, err := parseMaxAge(maxAge)
			if err != nil {
				return usageErr(err)
			}
			offline := noPull || flags.dataSource == "local"
			if offline {
				flags.agentSource = "local" // envelope reports local reads
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view := marketView{Deal: deal, Types: types, By: by, Communes: make([]marketCommune, 0, len(names))}
			var c *client.Client
			// Harvest every commune concurrently first; the per-commune loop
			// below then finds fresh pulls and only reads the store.
			pulled := make([]pullReport, len(names))
			pullErrs := make([]error, len(names))
			if !offline {
				if c, err = flags.newClient(); err != nil {
					return err
				}
				parallel(len(names), 3, func(i int) {
					crit := immo.Criteria{Types: types, Deal: deal}
					if pc := immo.NormalizePostalCode(names[i]); pc != "" {
						crit.PostalCodes = []string{pc}
					} else {
						crit.Communes = []string{names[i]}
					}
					pulled[i], _, pullErrs[i] = ensureArea(ctx, cmd, c, db, crit, age, dogfoodPages(maxPages))
				})
				for _, e := range pullErrs {
					if e != nil {
						return e
					}
				}
			}
			var hists map[int64][]store.PriceObs
			for idx, name := range names {
				mc := marketCommune{Commune: name, Bands: make([]marketBand, 0)}
				crit := immo.Criteria{Types: types, Deal: deal}
				if pc := immo.NormalizePostalCode(name); pc != "" {
					crit.PostalCodes = []string{pc}
				} else {
					crit.Communes = []string{name}
				}
				if offline {
					if len(crit.Communes) > 0 {
						pcs, err := localPostcodesFor(ctx, db, name)
						if err != nil {
							return err
						}
						crit.PostalCodes = pcs
					}
				} else {
					rep := pulled[idx]
					if len(rep.Communes) > 0 {
						mc.Label = rep.Communes[0].Label
						crit.PostalCodes = rep.Communes[0].PostalCodes
					}
					if n, err := fetchCount(ctx, c, immo.Criteria{Types: types, Deal: deal, PostalCodes: crit.PostalCodes}.Params()); err == nil {
						mc.LiveCount = &n
					}
				}
				mc.PostalCodes = crit.PostalCodes
				if len(crit.PostalCodes) == 0 {
					mc.Note = "commune not found in the local store; run without --no-pull once"
					view.Communes = append(view.Communes, mc)
					continue
				}
				if !hintIfUnsynced(cmd, db, store.ImmoResource) {
					hintIfStale(cmd, db, store.ImmoResource, flags.maxAge)
				}
				rows, err := db.QueryListings(ctx, store.ListingFilter{Deal: deal, Types: types, PostalCodes: crit.PostalCodes})
				if err != nil {
					return err
				}
				gone, err := db.QueryListings(ctx, store.ListingFilter{Deal: deal, Types: types, PostalCodes: crit.PostalCodes, GoneSince: time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)})
				if err != nil {
					return err
				}
				if !includeRooms {
					rows, mc.RoomLetsExcluded = dropRoomLets(rows)
					gone, _ = dropRoomLets(gone)
				}
				if hists == nil {
					if hists, err = db.AllPriceHistories(ctx); err != nil {
						return err
					}
				}
				fillMarket(&mc, rows, len(gone), hists, by, time.Now())
				if mc.LiveCount != nil && *mc.LiveCount > 0 {
					cov := immo.Round1(float64(mc.Sample+mc.RoomLetsExcluded) / float64(*mc.LiveCount) * 100)
					if cov > 100 {
						cov = 100
					}
					mc.CoveragePct = &cov
				}
				view.Communes = append(view.Communes, mc)
			}
			view.Warnings = sharedPostcodeWarnings(view.Communes)
			for _, wmsg := range view.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+wmsg)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			for _, mc := range view.Communes {
				head := mc.Commune
				if mc.Label != "" {
					head += " — " + mc.Label
				}
				fmt.Fprintf(w, "%s  (%s %s)\n", bold(head), strings.ToLower(strings.Join(types, ",")), strings.ToLower(deal))
				if mc.Sample == 0 {
					if mc.RoomLetsExcluded > 0 {
						fmt.Fprintf(w, "  only student rooms / per-room lets (%d); use --include-rooms to analyse them\n\n", mc.RoomLetsExcluded)
					} else {
						fmt.Fprintf(w, "  no stored listings. %s\n\n", mc.Note)
					}
					continue
				}
				live := "?"
				if mc.LiveCount != nil {
					live = fmt.Sprint(*mc.LiveCount)
				}
				fmt.Fprintf(w, "  listings: %s on Immoweb · %d analysed · %d gone in 30 days\n", live, mc.Sample, mc.GoneLast30d)
				fmt.Fprintf(w, "  median price %s · median €/m² %s (p25 %s, p75 %s)\n",
					fmtPtrEUR(mc.MedianPrice), fmtPtrEUR(mc.MedianPricePerM2), fmtPtrEUR(mc.P25PricePerM2), fmtPtrEUR(mc.P75PricePerM2))
				if mc.MedianDaysListed != nil {
					fmt.Fprintf(w, "  median %s days listed (over %d listings with a known publication date)\n", fmtPtr0(mc.MedianDaysListed), mc.DaysSample)
				} else {
					fmt.Fprintf(w, "  days listed: publication date known for %d listings only (search results omit it)\n", mc.DaysSample)
				}
				if mc.Sample < minComparables {
					fmt.Fprintf(w, "  only %d listings: treat medians as indicative\n", mc.Sample)
				}
				if mc.RoomLetsExcluded > 0 {
					fmt.Fprintf(w, "  %d student rooms / per-room lets left out (--include-rooms keeps them)\n", mc.RoomLetsExcluded)
				}
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\tN\tMEDIAN PRICE\tMEDIAN €/M²\n", strings.ToUpper(by))
				for _, b := range mc.Bands {
					fmt.Fprintf(tw, "  %s\t%d\t%s\t%s\n", b.Band, b.N, fmtPtrEUR(b.MedianPrice), fmtPtrEUR(b.MedianPricePerM2))
				}
				_ = tw.Flush()
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Property type(s): house, apartment, land, ... (comma-separated)")
	cmd.Flags().StringVar(&flagDeal, "deal", "", "sale or rent")
	cmd.Flags().StringVar(&flagBy, "by", "bedrooms", "Breakdown: bedrooms, epc, type or postcode")
	cmd.Flags().StringVar(&postcodes, "postcode", "", "Postal code(s) to analyse in addition to commune names")
	cmd.Flags().StringVar(&maxAge, "refresh-after", "24h", "Pull an area again when its last pull is older than this (e.g. 12h, 7d)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 40, "Search pages (30 each) to walk for large areas")
	cmd.Flags().BoolVar(&noPull, "no-pull", false, "Use only what is already stored locally")
	cmd.Flags().BoolVar(&includeRooms, "include-rooms", false, "Keep student rooms (kots) and per-room lets in the statistics")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

func fillMarket(mc *marketCommune, rows []store.StoredListing, goneCount int, hists map[int64][]store.PriceObs, by string, now time.Time) {
	mc.Sample = len(rows)
	mc.GoneLast30d = goneCount
	if len(rows) == 0 {
		return
	}
	mc.MedianPrice = medianPtr(priceValues(rows))
	pps := ppsValues(rows)
	mc.MedianPricePerM2 = medianPtr(pps)
	mc.P25PricePerM2 = quantilePtr(pps, 0.25)
	mc.P75PricePerM2 = quantilePtr(pps, 0.75)
	days := []float64{}
	uo, cut := 0, 0
	for _, r := range rows {
		if d := daysOf(r, now); d != nil {
			days = append(days, float64(*d))
		}
		if r.UnderOption {
			uo++
		}
		if _, ok := priceCut(r, hists[r.ID]); ok || r.NewPrice {
			cut++
		}
	}
	mc.DaysSample = len(days)
	if len(days) >= minComparables {
		mc.MedianDaysListed = medianPtr(days)
	}
	daysNote := ""
	if mc.MedianDaysListed == nil {
		daysNote = fmt.Sprintf("median days listed needs publication dates, known for %d listings (search results omit them; deal, show and triage --enrich fetch them)", len(days))
	}
	u := immo.Round1(float64(uo) / float64(len(rows)) * 100)
	mc.UnderOptionShare = &u
	p := immo.Round1(float64(cut) / float64(len(rows)) * 100)
	mc.PriceCutShare = &p
	groups := map[string][]store.StoredListing{}
	for _, r := range rows {
		var key string
		switch by {
		case "epc":
			key = r.EPC
			if key == "" {
				key = "unknown"
			}
		case "type":
			key = strings.ToLower(r.Type)
		case "postcode":
			key = r.PostalCode
		default:
			key = immo.BedroomBand(r.Bedrooms)
		}
		groups[key] = append(groups[key], r)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if by == "bedrooms" {
			return bandOrder(keys[i]) < bandOrder(keys[j])
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		g := groups[k]
		mc.Bands = append(mc.Bands, marketBand{Band: k, N: len(g), MedianPrice: medianPtr(priceValues(g)), MedianPricePerM2: medianPtr(ppsValues(g))})
	}
	notes := []string{}
	if mc.Sample < minComparables {
		notes = append(notes, fmt.Sprintf("only %d listings: treat medians as indicative", mc.Sample))
	}
	if daysNote != "" {
		notes = append(notes, daysNote)
	}
	mc.Note = strings.Join(notes, "; ")
}

// bandOrder sorts bedroom bands in immo.BedroomBands order.
func bandOrder(b string) int {
	for i, v := range immo.BedroomBands {
		if v == b {
			return i
		}
	}
	return len(immo.BedroomBands)
}

// sharedPostcodeWarnings flags communes whose postal codes overlap, since
// their statistics then share listings.
func sharedPostcodeWarnings(cs []marketCommune) []string {
	var out []string
	for i := 0; i < len(cs); i++ {
		for j := i + 1; j < len(cs); j++ {
			shared := []string{}
			for _, a := range cs[i].PostalCodes {
				for _, b := range cs[j].PostalCodes {
					if a == b {
						shared = append(shared, immo.BarePostalCode(a))
					}
				}
			}
			if len(shared) > 0 {
				out = append(out, fmt.Sprintf("%s and %s share postal code(s) %s: their statistics overlap; compare with --postcode for a clean split", cs[i].Commune, cs[j].Commune, strings.Join(shared, ",")))
			}
		}
	}
	return out
}

func fmtPtrEUR(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmtEUR(*p)
}

func fmtPtr0(p *float64) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *p)
}

func parseMaxAge(s string) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return 24 * time.Hour, nil
	}
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 12h, 7d)", s)
	}
	return d, nil
}

// localPostcodesFor maps a postal code or commune name to postal codes
// without a network call: a postal code as is, a Brussels commune from the
// ownership table, any other name from the localities seen in the store.
// Every offline command resolves places through it.
func localPostcodesFor(ctx context.Context, db *store.Store, name string) ([]string, error) {
	if pc := immo.NormalizePostalCode(name); pc != "" {
		return []string{pc}, nil
	}
	if codes := immo.BrusselsPostcodes(name); codes != nil {
		return codes, nil
	}
	rows, err := db.QueryListings(ctx, store.ListingFilter{IncludeGone: true})
	if err != nil {
		return nil, err
	}
	want := immo.Fold(name)
	out := []string{}
	for _, r := range rows {
		if immo.Fold(r.Locality) == want {
			out = appendUniq(out, "BE-"+r.PostalCode)
		}
	}
	return out, nil
}
