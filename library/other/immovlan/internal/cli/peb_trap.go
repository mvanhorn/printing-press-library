// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

type pebTrapView struct {
	Postcodes   []string    `json:"postal_codes,omitempty"`
	StoredFG    int         `json:"stored_fg"`
	Unenriched  int         `json:"fg_without_rented_flag"`
	Results     []rankedRow `json:"results"`
	Note        string      `json:"note,omitempty"`
	LettersFrom string      `json:"letters_from"`
}

func newNovelPebTrapCmd(flags *rootFlags) *cobra.Command {
	var flagPostcode, flagDeal, flagSort, dbPath string
	var flagMaxPrice, flagLimit int
	var includeUnknownRented bool
	cmd := &cobra.Command{
		Use:   "peb-trap",
		Short: "List PEB F or G properties that are currently rented: owners who can no longer re-let in Brussels since 2026",
		Long: `Since 1 January 2026 a Brussels home rated PEB F or G cannot be re-let without
works. A landlord with a tenant in place must renovate or sell. peb-trap lists
the stored listings whose detail page says PEB F/G and "bien actuellement loué :
oui", ranked by €/m² percentile inside their postal code (computed once the
postcode holds at least 5 stored listings of the same deal).
Use this command to list PEB F/G properties that are currently rented. It needs
the detail letter and rented flag, so run 'enrich --missing epc,rented' first.
'find --epc F' only returns the search-card band (bad) without the rented flag.
--deal defaults to sale.`,
		Example: strings.Trim(`
  immovlan-pp-cli enrich --missing epc,rented --top 200 --agent && immovlan-pp-cli peb-trap --postcode 1030,1210 --agent
  immovlan-pp-cli peb-trap --max-price 2500000 --sort ppm2 --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list rented PEB F/G listings from the local store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			switch flagSort {
			case "ppm2", "price", "days":
			default:
				return usageErr(fmt.Errorf("unknown --sort %q (ppm2, price or days)", flagSort))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			f := store.ListingFilter{EPC: []string{"F", "G"}, MaxPrice: flagMaxPrice}
			var err error
			if f.Deal, err = parseDealFlag(flagDeal); err != nil {
				return err
			}
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			view := pebTrapView{Postcodes: f.PostalCodes, Results: []rankedRow{}, LettersFrom: "detail page meta (enrich) or card watermark"}
			path, ok := localStoreExists(dbPath)
			if !ok {
				view.Note = noStoreNote(path, "find then enrich")
				return printPebTrap(cmd, flags, view)
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
			view.StoredFG = len(rows)
			// €/m² pool per postcode over every stored listing of the same deal (not only F/G).
			all, err := db.QueryVlanListings(ctx, store.ListingFilter{Deal: f.Deal, PostalCodes: f.PostalCodes})
			if err != nil {
				return err
			}
			pool := ppsPoolByPostcode(all)
			now := time.Now()
			for _, r := range rows {
				if r.Rented == nil {
					view.Unenriched++
					if !includeUnknownRented {
						continue
					}
				} else if !*r.Rented {
					continue
				}
				view.Results = append(view.Results, rankRow(r, pool, now))
			}
			sort.SliceStable(view.Results, func(i, j int) bool {
				a, b := view.Results[i], view.Results[j]
				switch flagSort {
				case "price":
					return deref(a.Price) < deref(b.Price)
				case "days":
					return derefI(a.DaysListed) > derefI(b.DaysListed)
				}
				return byPricePerSqm(a, b)
			})
			if flagLimit > 0 && len(view.Results) > flagLimit {
				view.Results = view.Results[:flagLimit]
			}
			if view.Unenriched > 0 && !includeUnknownRented {
				view.Note = fmt.Sprintf("%d stored F/G listings have no rented flag yet; run: immovlan-pp-cli enrich --missing rented (pages that never state it stay unknown: pass --include-unknown to list them too)", view.Unenriched)
			}
			return printPebTrap(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().StringVar(&flagDeal, "deal", "sale", "Deal to inspect")
	cmd.Flags().IntVar(&flagMaxPrice, "max-price", 0, "Maximum asking price in EUR")
	cmd.Flags().StringVar(&flagSort, "sort", "ppm2", "ppm2 (cheapest per m² first), price or days")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows")
	cmd.Flags().BoolVar(&includeUnknownRented, "include-unknown", false, "Also list F/G listings whose rented flag is not enriched yet")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func printPebTrap(cmd *cobra.Command, flags *rootFlags, view pebTrapView) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printView(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	if len(view.Results) == 0 {
		fmt.Fprintf(w, "No rented PEB F/G listings in the store (%d F/G stored).\n", view.StoredFG)
	} else {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "ID\tLOCALITY\tPRICE\tM²\t€/M²\tPCTL\tPEB\tDAYS\tSELLER\tURL")
		for _, r := range view.Results {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, locLabel(r.Listing), fmtPtrEUR(r.Price), floatUnit(r.Surface, ""), fmtPtrEUR(r.PricePerSqm), pctStr(r.PercentileInPostcode), termSafe(r.EPC), intStr(r.DaysListed), sellerLabel(r.Listing), termSafe(r.URL))
		}
		_ = tw.Flush()
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
	}
	return nil
}
