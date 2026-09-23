// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type showView struct {
	zimmo.Listing
	Source    string           `json:"source"`
	LocalObs  []store.PriceObs `json:"local_price_observations,omitempty"`
	FirstSeen string           `json:"first_seen,omitempty"`
}

func newShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var noStore bool
	cmd := &cobra.Command{
		Use:   "show [zimmo-code|url]",
		Short: "Show one listing in full: exact address, GPS, EPC kWh, flood and planning flags, rent, price history, agency",
		Long: `Fetch one listing by its Zimmo code (LAISZ), UUID or zimmo.be URL and print
every useful field, flattened: exact street and number, rooftop GPS, EPC
letter and kWh/m², renovation obligation, flood-risk and planning flags,
current rent when the property is let, Zimmo's own price history, days on
market and the agency. The listing is stored locally.
With --data-source local it reads the stored copy without calling Zimmo.`,
		Example: strings.Trim(`
  zimmo-pp-cli show LAISZ
  zimmo-pp-cli show https://www.zimmo.be/fr/lanaken-3620/a-vendre/maison/LAISZ --json
  zimmo-pp-cli show LAISZ --agent --select address,price,epc,epc_kwh_m2,flood_risk`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "auto", "pp:happy-args": "code=LAISZ"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch one Zimmo listing")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give exactly one Zimmo code, UUID or URL"))
			}
			ref, err := parseCodeArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			view := showView{}
			var liveErr error
			if flags.dataSource != "local" {
				l, err := fetchListing(ctx, zimmoClient(flags), ref)
				if err == nil {
					view.Listing, view.Source = l, "live"
					flags.agentSource = "live"
					if !noStore {
						if err := db.UpsertZimmoListings(ctx, []zimmo.Listing{l}, time.Now(), true); err != nil {
							return err
						}
					}
				} else if isNotFound(err) || flags.dataSource == "live" {
					return err
				} else {
					liveErr = err
				}
			}
			if view.Source == "" {
				rows, err := db.QueryZimmoListings(ctx, refFilter(ref))
				if err != nil {
					return err
				}
				if len(rows) == 0 {
					if liveErr != nil {
						return liveErr
					}
					return notFoundErr(fmt.Errorf("listing %s is not in the local store; run without --data-source local", ref))
				}
				view.Listing, view.Source, view.FirstSeen = rows[0].Listing, "local", rows[0].FirstSeen
				flags.agentSource = "local"
				if liveErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: Zimmo unreachable (%v); showing the stored copy\n", liveErr)
				}
			}
			if hist, err := db.AllZimmoPriceHistories(ctx); err == nil {
				view.LocalObs = hist[view.Code]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), view, flags)
			}
			return printListingDetail(cmd.OutOrStdout(), view.Listing)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not write the listing to the local store")
	return cmd
}

func printListingDetail(w io.Writer, l zimmo.Listing) error {
	line := func(k, v string) {
		if v != "" && v != "-" {
			fmt.Fprintf(w, "%-18s %s\n", k, termSafe(v))
		}
	}
	title := strings.ToLower(l.Type)
	if l.SubType != "" {
		title += " / " + strings.ToLower(l.SubType)
	}
	fmt.Fprintf(w, "%s  %s  %s\n", l.Code, strings.ToLower(l.Status), title)
	line("Address", l.Address)
	if l.Lat != nil && l.Lng != nil {
		line("GPS", fmt.Sprintf("%.6f, %.6f (%s)", *l.Lat, *l.Lng, strings.ToLower(l.GeoPrecision)))
	}
	line("Price", fmtPtrEUR(l.Price))
	line("€/m²", floatUnit(l.PricePerM2, " €"))
	line("Living surface", floatUnit(l.Surface, " m²"))
	line("Plot", floatUnit(l.Plot, " m²"))
	line("Bedrooms", intStr(l.Bedrooms))
	line("Bathrooms", intStr(l.Bathrooms))
	line("Built", intStr(l.Year))
	line("Condition", strings.ToLower(strings.ReplaceAll(l.Condition, "_", " ")))
	line("EPC", epcStr(l))
	line("Renovation duty", strings.ToLower(l.RenovationDuty))
	if l.Rented {
		line("Rented", fmtPtrEUR(l.RentPerYear)+" / year")
	}
	if len(l.FloodRisk) > 0 {
		line("Flood risk", strings.ToLower(strings.Join(l.FloodRisk, ", ")))
	}
	if len(l.PlanningFlags) > 0 {
		line("Planning", strings.ToLower(strings.Join(l.PlanningFlags, ", ")))
	}
	line("Subpoena", strings.ToLower(l.Subpoena))
	line("Free on", strings.ToLower(l.FreeOn))
	if l.DaysOnMarket != nil {
		line("On market", fmt.Sprintf("%d days (since %s)", *l.DaysOnMarket, dateOnly(l.PublishedAt)))
	}
	for _, ph := range l.PriceHistory {
		line("Price change", fmt.Sprintf("%s  %s → %s", dateOnly(ph.Date), fmtEUR(ph.Before), fmtEUR(ph.After)))
	}
	if l.TotalCutPct != nil {
		line("Total cut", fmt.Sprintf("%.1f%%", *l.TotalCutPct))
	}
	agency := l.Agency
	if l.AgencyPhone != "" {
		agency += " · " + l.AgencyPhone
	}
	line("Agency", agency)
	line("Photos", fmt.Sprintf("%d", len(l.Photos)))
	for _, d := range l.Documents {
		line("Document", d.Name)
	}
	line("URL", l.URL)
	if l.Description != "" {
		fmt.Fprintf(w, "\n%s\n", termSafe(firstN(l.Description, 600)))
	}
	return nil
}

func firstN(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newShowCmd(flags))
	})
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
