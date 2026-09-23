// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

type splitView struct {
	MinSurface  float64     `json:"min_surface_m2"`
	MinBedrooms int         `json:"min_bedrooms"`
	Postcodes   []string    `json:"postal_codes,omitempty"`
	Candidates  int         `json:"candidates"`
	Results     []rankedRow `json:"results"`
	Note        string      `json:"note,omitempty"`
}

func newNovelSplitCandidatesCmd(flags *rootFlags) *cobra.Command {
	var flagPostcode, flagTypes, dbPath string
	var flagMinSurface float64
	var flagMinBedrooms, flagMaxPrice, flagLimit int
	cmd := &cobra.Command{
		Use:   "split-candidates",
		Short: "Houses large enough to divide, ranked by €/m² within their postcode, with land, year, PEB and rented flags",
		Long: `Immovlan has no surface filter. split-candidates reads the local store for houses
(or any type with --type) at or above a living surface and bedroom count, ranks
them by €/m² percentile inside their postal code (computed once the postcode
holds at least 5 stored sale listings of the same type), and shows the fields a
trader needs to judge a division: land, terrace, year, condition, PEB, rented.`,
		Example: strings.Trim(`
  immovlan-pp-cli split-candidates --min-surface 200 --postcode 1030 --agent
  immovlan-pp-cli split-candidates --min-surface 250 --min-bedrooms 4 --max-price 1500000 --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--min-surface=150;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank large houses from the local store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if flagMinSurface <= 0 {
				flagMinSurface = 200
			}
			f := store.ListingFilter{Deal: immovlan.DealSale, MinSurface: flagMinSurface, MinBedrooms: flagMinBedrooms, MaxPrice: flagMaxPrice}
			if flagTypes == "" {
				flagTypes = "maison"
			}
			for _, t := range immovlan.SplitCSV(flagTypes) {
				v, err := immovlan.NormalizeType(t)
				if err != nil {
					return usageErr(err)
				}
				f.Types = append(f.Types, v)
			}
			var err error
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			view := splitView{MinSurface: flagMinSurface, MinBedrooms: flagMinBedrooms, Postcodes: f.PostalCodes, Results: []rankedRow{}}
			path, ok := localStoreExists(dbPath)
			if !ok {
				view.Note = noStoreNote(path, "find --type maison --deal sale")
				return printSplit(cmd, flags, view)
			}
			db, err := openVlanStore(ctx, path)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.QueryVlanListings(ctx, f)
			if err != nil {
				return err
			}
			all, err := db.QueryVlanListings(ctx, store.ListingFilter{Deal: immovlan.DealSale, Types: f.Types, PostalCodes: f.PostalCodes})
			if err != nil {
				return err
			}
			pool := ppsPoolByPostcode(all)
			now := time.Now()
			thinPool := 0
			for _, r := range rows {
				row := rankRow(r, pool, now)
				if row.PercentileInPostcode == nil && r.PricePerSqm != nil {
					thinPool++
				}
				view.Results = append(view.Results, row)
			}
			view.Candidates = len(view.Results)
			sort.SliceStable(view.Results, func(i, j int) bool { return byPricePerSqm(view.Results[i], view.Results[j]) })
			if flagLimit > 0 && len(view.Results) > flagLimit {
				view.Results = view.Results[:flagLimit]
			}
			notes := []string{}
			switch {
			case view.Candidates == 0:
				notes = append(notes, "no stored listing matches; widen --min-surface or run find --type maison --deal sale first")
			case thinPool > 0:
				notes = append(notes, fmt.Sprintf("percentile blank on %d rows: fewer than %d stored %s sale listings in their postcode (run find --pages 5 there)", thinPool, minPercentilePool, strings.Join(f.Types, "/")))
			}
			if len(view.Results) < view.Candidates {
				// The €/m² sort is global, so with several postcodes the cheapest
				// zone can fill the whole page; say so instead of hiding the rest.
				notes = append(notes, fmt.Sprintf("showing %d of %d candidates (cheapest €/m² first across all postcodes); raise --limit or narrow --postcode to see the rest", len(view.Results), view.Candidates))
			}
			view.Note = strings.Join(notes, "; ")
			return printSplit(cmd, flags, view)
		},
	}
	cmd.Flags().Float64Var(&flagMinSurface, "min-surface", 200, "Minimum living surface in m²")
	cmd.Flags().IntVar(&flagMinBedrooms, "min-bedrooms", 0, "Minimum bedrooms")
	cmd.Flags().IntVar(&flagMaxPrice, "max-price", 0, "Maximum asking price in EUR")
	cmd.Flags().StringVar(&flagTypes, "type", "maison", "Property types, comma-separated (default maison)")
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func printSplit(cmd *cobra.Command, flags *rootFlags, view splitView) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printView(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	if len(view.Results) == 0 {
		fmt.Fprintf(w, "No candidates ≥ %.0f m² in the store.\n", view.MinSurface)
	} else {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "ID\tLOCALITY\tPRICE\tM²\t€/M²\tPCTL\tBEDS\tLAND\tYEAR\tPEB\tRENTED\tCONDITION\tURL")
		for _, r := range view.Results {
			rented := "?"
			if r.Rented != nil {
				rented = map[bool]string{true: "yes", false: "no"}[*r.Rented]
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, locLabel(r.Listing), fmtPtrEUR(r.Price), floatUnit(r.Surface, ""), fmtPtrEUR(r.PricePerSqm), pctStr(r.PercentileInPostcode), intStr(r.Bedrooms), floatUnit(r.Land, ""), intStr(r.Year), termSafe(r.EPC), rented, termSafe(truncate(r.Condition, 14)), termSafe(r.URL))
		}
		_ = tw.Flush()
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
	}
	return nil
}
