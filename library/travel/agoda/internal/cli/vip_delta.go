// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
)

type vipRow struct {
	PropertyID     int     `json:"property_id"`
	Name           string  `json:"name"`
	AnonymousAllIn float64 `json:"anonymous_all_in"`
	MemberAllIn    float64 `json:"member_all_in"`
	SavingAmount   float64 `json:"saving_amount"`
	SavingPct      float64 `json:"saving_pct"`
}

type vipResult struct {
	Destination     string   `json:"destination"`
	CityID          int      `json:"city_id"`
	CheckIn         string   `json:"checkin"`
	Nights          int      `json:"nights"`
	Currency        string   `json:"currency"`
	SessionDetected bool     `json:"session_detected"`
	Compared        int      `json:"properties_compared"`
	Discounted      int      `json:"properties_discounted"`
	MedianSavingPct float64  `json:"median_saving_pct"`
	BestSavingPct   float64  `json:"best_saving_pct"`
	Results         []vipRow `json:"results"`
	Note            string   `json:"note,omitempty"`
}

func newNovelVipDeltaCmd(flags *rootFlags) *cobra.Command {
	sf := &searchFlags{}

	cmd := &cobra.Command{
		Use:   "delta [destination]",
		Short: "Measure what your AgodaVIP tier is actually worth on a real search",
		Long: `Run the same search signed-in and anonymously, then diff the prices.

AgodaVIP advertises tier discounts, but whether any given search is discounted -
and by how much - depends on the property, the dates, and your tier. This command
issues the identical search twice and reports the measured per-property
difference rather than the marketing claim.

A zero delta is a real and useful answer: it means your tier does not discount
this search. Requires a session cookie in AGODA_COOKIE; without one, only the
anonymous side can be measured and the command says so instead of guessing.`,
		Example: "  agoda-pp-cli vip delta Tokyo --checkin 2026-10-15 --nights 2 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "destination=Tokyo;--nights=2",
			// A missing session is a legitimate outcome, reported with the
			// framework's auth-required code rather than a generic failure.
			"pp:typed-exit-codes": "0,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "vip delta")
			}
			dest := ""
			if len(args) > 0 {
				dest = args[0]
			}
			if dest == "" && sf.cityID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a destination argument or --city-id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newAgodaClient(flags)
			d, err := resolveCity(ctx, c, dest, sf.cityID)
			if err != nil {
				return err
			}
			opts, err := sf.searchOptions(d.CityID, false)
			if err != nil {
				return err
			}

			out := vipResult{
				Destination:     displayDestination(d, dest),
				CityID:          d.CityID,
				CheckIn:         opts.CheckIn,
				Nights:          opts.Nights,
				Currency:        opts.Currency,
				SessionDetected: hasAgodaSession(),
				Results:         make([]vipRow, 0),
			}

			anon, err := c.CitySearch(ctx, opts)
			if err != nil {
				return err
			}

			if !out.SessionDetected {
				out.Note = "no Agoda session found, so member pricing could not be measured; " +
					"set AGODA_COOKIE to your logged-in agoda.com cookie and re-run"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
						return err
					}
					return &cliError{code: 4, err: fmt.Errorf("no Agoda session available; set AGODA_COOKIE to measure member pricing")}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
				return &cliError{code: 4, err: fmt.Errorf("no Agoda session available; set AGODA_COOKIE to measure member pricing")}
			}

			memberOpts := opts
			memberOpts.Authenticated = true
			member, err := c.CitySearch(ctx, memberOpts)
			if err != nil {
				return err
			}

			byID := make(map[int]agoda.Property, len(anon))
			for _, p := range anon {
				if p.PriceAllIn > 0 {
					byID[p.PropertyID] = p
				}
			}
			rows := make([]vipRow, 0, len(member))
			for _, m := range member {
				a, ok := byID[m.PropertyID]
				if !ok || m.PriceAllIn <= 0 {
					continue
				}
				saving := a.PriceAllIn - m.PriceAllIn
				pct := 0.0
				if a.PriceAllIn > 0 {
					pct = round2Pct(saving / a.PriceAllIn * 100)
				}
				if saving > 0.005 {
					out.Discounted++
				}
				rows = append(rows, vipRow{
					PropertyID:     m.PropertyID,
					Name:           m.Name,
					AnonymousAllIn: a.PriceAllIn,
					MemberAllIn:    m.PriceAllIn,
					SavingAmount:   round2Pct(saving),
					SavingPct:      pct,
				})
			}
			out.Compared = len(rows)
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].SavingPct > rows[j].SavingPct })
			if len(rows) > 0 {
				out.BestSavingPct = rows[0].SavingPct
				out.MedianSavingPct = medianSavingPct(rows)
			}
			if out.Discounted == 0 && out.Compared > 0 {
				out.Note = "no member discount applied to any property on this search; " +
					"this is a real result, not a failure"
			}
			if sf.limit > 0 && len(rows) > sf.limit {
				rows = rows[:sf.limit]
			}
			out.Results = rows

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printAgodaJSON(cmd.OutOrStdout(), out, flags, "live")
			}
			return renderVIP(cmd, out)
		},
	}
	bindSearchFlags(cmd, sf)
	return cmd
}

func medianSavingPct(rows []vipRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	vals := make([]float64, 0, len(rows))
	for _, r := range rows {
		vals = append(vals, r.SavingPct)
	}
	sort.Float64s(vals)
	mid := len(vals) / 2
	if len(vals)%2 == 1 {
		return vals[mid]
	}
	return round2Pct((vals[mid-1] + vals[mid]) / 2)
}

func renderVIP(cmd *cobra.Command, res vipResult) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (city %d) - check-in %s, %d night(s), prices in %s\n\n",
		res.Destination, res.CityID, res.CheckIn, res.Nights, res.Currency)
	if len(res.Results) == 0 {
		fmt.Fprintln(out, "No comparable properties returned for both the signed-in and anonymous search.")
		if res.Note != "" {
			fmt.Fprintf(out, "%s\n", res.Note)
		}
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROPERTY\tANONYMOUS\tMEMBER\tSAVING")
	for _, r := range res.Results {
		name := r.Name
		if len(name) > 42 {
			name = name[:39] + "..."
		}
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.1f%%\n", name, r.AnonymousAllIn, r.MemberAllIn, r.SavingPct)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\n%d properties compared, %d discounted. Best %.1f%%, median %.1f%%.\n",
		res.Compared, res.Discounted, res.BestSavingPct, res.MedianSavingPct)
	if res.Note != "" {
		fmt.Fprintf(out, "%s\n", res.Note)
	}
	return nil
}
