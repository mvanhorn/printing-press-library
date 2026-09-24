// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

type agencyRow struct {
	Agency       string   `json:"agency"`
	AgencyID     string   `json:"agency_id,omitempty"`
	Listings     int      `json:"listings"`
	Matching     int      `json:"matching_listings,omitempty"` // listings with one of the --epc letters (only with --epc)
	FG           int      `json:"peb_fg"`
	FGSharePct   float64  `json:"peb_fg_share_pct"`
	Private      bool     `json:"private_seller"`
	Software     string   `json:"software,omitempty"`
	Syndicated   *bool    `json:"syndicated_feed,omitempty"` // nil until enrich captured the software field
	MedianPPS    *float64 `json:"median_price_per_m2,omitempty"`
	Postcodes    []string `json:"postal_codes"`
	SampleRef    string   `json:"sample_reference"`
	DetailedRows int      `json:"listings_with_detail"`
}

type agenciesView struct {
	Postcodes   []string    `json:"postal_codes,omitempty"`
	EPCLetters  []string    `json:"epc_letters,omitempty"`
	Listings    int         `json:"listings"`
	PrivateRows int         `json:"private_seller_listings"`
	Results     []agencyRow `json:"results"`
	Note        string      `json:"note,omitempty"`
}

func newNovelAgenciesCmd(flags *rootFlags) *cobra.Command {
	var flagPostcode, flagEpc, flagDeal, dbPath string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "agencies",
		Short: "Which agencies list in a zone, their PEB F/G share, private-seller share and the CRM software behind each feed",
		Long: `Group the local store by seller. The 'software' column comes from the listing
page's dataLayer (captured by enrich): a CRM such as Whise or Omnicasa means a
syndicated feed that probably also reaches Immoweb; an empty value means the
listing was typed into Immovlan by hand and is likelier Rossel-exclusive.
--epc keeps the agencies that carry at least one listing with those PEB letters
(F means F only; a band such as bad expands to F,G); counts and shares are still
computed over all their stored listings.`,
		Example: strings.Trim(`
  immovlan-pp-cli agencies --postcode 1030 --epc F,G --agent
  immovlan-pp-cli agencies --postcode 1030,1210 --limit 15`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "aggregate stored listings by agency")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			f := store.ListingFilter{}
			var err error
			if f.Deal, err = parseDealFlag(flagDeal); err != nil {
				return err
			}
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			onlyLetters, err := parseEPCLetters(flagEpc)
			if err != nil {
				return err
			}
			view := agenciesView{Postcodes: f.PostalCodes, Results: []agencyRow{}}
			for l := range onlyLetters {
				view.EPCLetters = append(view.EPCLetters, l)
			}
			sort.Strings(view.EPCLetters)
			path, ok := localStoreExists(dbPath)
			if !ok {
				view.Note = noStoreNote(path, "find")
				return printAgencies(cmd, flags, view)
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
			view.Listings = len(rows)
			view.Results, view.PrivateRows = aggregateAgencies(rows, onlyLetters)
			if flagLimit > 0 && len(view.Results) > flagLimit {
				view.Results = view.Results[:flagLimit]
			}
			if view.Listings == 0 {
				view.Note = "no stored listings; run find first"
			} else if allNil := func() bool {
				for _, r := range view.Results {
					if r.Syndicated != nil {
						return false
					}
				}
				return true
			}(); allNil {
				view.Note = "software/syndication unknown until 'immovlan-pp-cli enrich' has read listing pages"
			}
			return printAgencies(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().StringVar(&flagEpc, "epc", "", "Keep agencies with at least one listing in these PEB letters (F) or Immovlan bands (bad → F,G)")
	cmd.Flags().StringVar(&flagDeal, "deal", "", "Only this deal")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Maximum agencies")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

// aggregateAgencies groups rows by seller; when onlyLetters is non-empty only
// sellers with a matching listing are returned, with their full counts.
func aggregateAgencies(rows []store.StoredListing, onlyLetters map[string]bool) ([]agencyRow, int) {
	type acc struct {
		row  agencyRow
		pps  []float64
		pcs  map[string]bool
		soft map[string]int
	}
	groups := map[string]*acc{}
	privateRows := 0
	for _, r := range rows {
		key := r.AgencyID
		name := r.Agency
		if r.Private {
			key, name = "private", "private sellers"
			privateRows++
		} else if key == "" {
			key = "name:" + immovlan.Fold(r.Agency)
		}
		if key == "name:" {
			key, name = "unknown", "unknown agency"
		}
		a := groups[key]
		if a == nil {
			a = &acc{row: agencyRow{Agency: name, AgencyID: r.AgencyID, Private: r.Private, SampleRef: r.ID}, pcs: map[string]bool{}, soft: map[string]int{}}
			groups[key] = a
		}
		if a.row.Agency == "" && r.Agency != "" {
			a.row.Agency = r.Agency
		}
		a.row.Listings++
		if onlyLetters[strings.ToUpper(r.EPC)] {
			a.row.Matching++
			a.row.SampleRef = r.ID
		}
		if r.EPC == "F" || r.EPC == "G" {
			a.row.FG++
		}
		if r.PricePerSqm != nil {
			a.pps = append(a.pps, *r.PricePerSqm)
		}
		a.pcs[r.PostalCode] = true
		if r.DetailAt != "" {
			a.row.DetailedRows++
			a.soft[r.Software]++
		}
	}
	out := []agencyRow{}
	for _, a := range groups {
		if len(onlyLetters) > 0 && a.row.Matching == 0 {
			continue
		}
		if a.row.Agency == "" && a.row.AgencyID != "" {
			a.row.Agency = "agency #" + a.row.AgencyID + " (run enrich for the name)"
		}
		if a.row.Listings > 0 {
			a.row.FGSharePct = pct1(float64(a.row.FG) / float64(a.row.Listings))
		}
		if m, ok := median(a.pps); ok {
			m = float64(int(m + 0.5))
			a.row.MedianPPS = &m
		}
		for pc := range a.pcs {
			a.row.Postcodes = append(a.row.Postcodes, pc)
		}
		sort.Strings(a.row.Postcodes)
		if a.row.DetailedRows > 0 {
			best, n := "", -1
			for s, c := range a.soft {
				if c > n {
					best, n = s, c
				}
			}
			a.row.Software = best
			synd := best != ""
			a.row.Syndicated = &synd
		}
		out = append(out, a.row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Listings != out[j].Listings {
			return out[i].Listings > out[j].Listings
		}
		if out[i].Agency != out[j].Agency {
			return out[i].Agency < out[j].Agency
		}
		if out[i].AgencyID != out[j].AgencyID {
			return out[i].AgencyID < out[j].AgencyID
		}
		return out[i].SampleRef < out[j].SampleRef
	})
	return out, privateRows
}

func printAgencies(cmd *cobra.Command, flags *rootFlags, view agenciesView) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printView(cmd.OutOrStdout(), view, flags)
	}
	w := cmd.OutOrStdout()
	if len(view.Results) == 0 {
		fmt.Fprintln(w, "No agencies in the store for these filters.")
	} else {
		tw := newTabWriter(w)
		fmt.Fprintln(tw, "AGENCY\tLISTINGS\tPEB F/G\tSHARE\tMEDIAN €/M²\tSOFTWARE\tPOSTCODES")
		for _, r := range view.Results {
			soft := termSafe(r.Software)
			if r.Syndicated == nil {
				soft = "?"
			} else if soft == "" {
				soft = "(manual)"
			}
			fmt.Fprintf(tw, "%s\t%d\t%d\t%.0f%%\t%s\t%s\t%s\n", termSafe(truncate(r.Agency, 32)), r.Listings, r.FG, r.FGSharePct, fmtPtrEUR(r.MedianPPS), soft, strings.Join(r.Postcodes, ","))
		}
		_ = tw.Flush()
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
	}
	return nil
}
