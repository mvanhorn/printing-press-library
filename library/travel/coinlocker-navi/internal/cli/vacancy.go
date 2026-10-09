// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"

	"github.com/spf13/cobra"
)

type vacancyView struct {
	Meta    lockerMeta               `json:"meta"`
	Results []locker.EkicubeLocation `json:"results"`
}

type maihamaView struct {
	Meta   lockerMeta          `json:"meta"`
	Result locker.MaihamaBoard `json:"results"`
}

func newVacancyCmd(flags *rootFlags) *cobra.Command {
	var (
		latS, lonS, gate, size, date, station string
		radius, limit, maxPages               int
	)
	cmd := &cobra.Command{
		Use:   "vacancy [keyword]",
		Short: "Live locker empties: Multi Ekicube banks by keyword or coordinates, or the Maihama map",
		Long: strings.Trim(`
Use this command for the full live board of lockers that publish vacancy.
Do NOT use this command to find walk-up lockers with sizes, prices and IC
cards near a place; use 'near' (add --live for the same live data).

Without --station: Multi Ekicube (JR East Smart Logistics) banks by keyword
(station name, Japanese or English) or --lat/--lon within --radius metres.
Each bank has Japanese and English names, gate side (inside/outside/both,
from the source's structured fields), hours, and per-size reservable empty
counts with the usage fee and the ¥500 reservation fee shown apart.
IMPORTANT: these are reservable counts. The source says there may still be
lockers available even when the search shows none; 0 does not mean full.

--station maihama: the Coin Locker Navi live map at Maihama Station (Tokyo
Disney Resort) with installed and physical empty boxes per block and the
source's as-of time.`, "\n"),
		Example: strings.Trim(`
  coinlocker-navi-pp-cli vacancy 東京駅 --gate outside --agent
  coinlocker-navi-pp-cli vacancy --lat 35.6812 --lon 139.7671 --radius 400 --size L --agent
  coinlocker-navi-pp-cli vacancy --station maihama --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			// A keyword with no matches is a valid empty result at the source.
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "keyword=東京駅;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "vacancy: GET api.multiecube.com/v1/location/ph2 (keyword or --lat/--lon), or GET www.coinlocker-navi.com/areamap/maihamaeki/ plus one ajax call per block for --station maihama")
			}
			lat, lon, err := parseLatLon(cmd, latS, lonS)
			if err != nil {
				return usageErr(err)
			}
			keyword := strings.TrimSpace(strings.Join(args, " "))
			station = strings.ToLower(strings.TrimSpace(station))
			if station != "" {
				if station != "maihama" {
					return usageErr(fmt.Errorf("--station %q: only maihama is supported", station))
				}
				if keyword != "" || lat != nil {
					return usageErr(fmt.Errorf("--station cannot be combined with a keyword or --lat/--lon"))
				}
			} else if (keyword == "") == (lat == nil) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give exactly one of a keyword, --lat/--lon, or --station maihama"))
			}
			if size, err = normSize(size); err != nil {
				return usageErr(err)
			}
			if gate, err = normGate(gate); err != nil {
				return usageErr(err)
			}
			if date, err = normDate(date); err != nil {
				return usageErr(err)
			}
			if err := validateLimitPages(limit, maxPages); err != nil {
				return usageErr(err)
			}
			if radius < 50 || radius > 5000 {
				return usageErr(fmt.Errorf("--radius %d: want 50 to 5000 metres", radius))
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}
			// Live-only: there is no local store, so reject --data-source local
			// before any network call.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newLockerClient(flags)
			fetchedAt := nowJST()

			if station == "maihama" {
				board, err := c.Maihama(ctx)
				if err != nil {
					return classifyLockerErr("coinlocker-navi Maihama map", err)
				}
				meta := newLockerMeta("vacancy", c, fetchedAt)
				meta.Query = map[string]any{"station": "maihama"}
				meta.Notes = []string{"counts are physical empty boxes from the source map at source_as_of; they can change before you arrive"}
				view := maihamaView{Meta: meta, Result: board}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				w := newSafeTextWriter(cmd.OutOrStdout())
				fmt.Fprintf(w, "Maihama Station (舞浜駅) as of %s\n", strOr(board.SourceAsOf, "unknown"))
				tw := &widthTable{}
				tw.add("BLOCK", "STATUS", "SIZES (empty/installed)", "LOCKER")
				for _, b := range board.Blocks {
					var parts []string
					for _, s := range b.Sizes {
						parts = append(parts, fmt.Sprintf("%s %s/%s", strOr(s.Label, s.LabelJA), intText(s.Empty), intText(s.Installed)))
					}
					tw.add(b.Name, strOr(b.Status, "-"), strings.Join(parts, ", "), strOr(b.LockerID, "-"))
				}
				return tw.write(w)
			}

			q := locker.EkicubeQuery{Keyword: keyword, Lat: lat, Lon: lon, Gate: gate, Date: date, MaxPages: maxPages}
			if lat != nil {
				q.RadiusM = radius
			}
			if size == "" {
				q.StopAfter = limit
			}
			banks, total, more, err := c.Ekicube(ctx, q)
			if err != nil {
				return classifyLockerErr("multiecube", err)
			}
			scanned := len(banks)
			excluded := map[string]int{}
			kept := make([]locker.EkicubeLocation, 0, len(banks))
			for _, b := range banks {
				if gate != "" && !gateMatches(b.Gate, gate) {
					if b.Gate == nil {
						excluded["gate_unknown"]++
					} else {
						excluded["other_gate"]++
					}
					continue
				}
				if size != "" && !bankHasSize(b, size) {
					if !bankSizesKnown(b) {
						excluded["size_unknown"]++
					} else {
						excluded["no_box_of_size"]++
					}
					continue
				}
				kept = append(kept, b)
			}
			if len(kept) > limit {
				kept = kept[:limit]
				more = true
			}
			meta := newLockerMeta("vacancy", c, fetchedAt)
			meta.Query = map[string]any{"keyword": keyword, "lat": lat, "lon": lon, "radius_m": q.RadiusM, "gate": gate, "size": size, "date": date, "limit": limit}
			meta.Scanned, meta.Returned, meta.SourceTotal, meta.MoreAtSource = intPtr(scanned), intPtr(len(kept)), intPtr(total), boolPtr(more)
			if len(excluded) > 0 {
				meta.Excluded = excluded
			}
			meta.Notes = []string{
				locker.ReservableCaveat,
				"usage_fee_yen is the source's usage fee per box; reservation_fee_yen applies only when you book online; the source does not state the walk-up price at the machine",
				"size_class groups source box keys (ss,s=S; sm,m=M; ml,l,lw,tl=L; xl=XL); this grouping is ours, not the source's",
				"observation time is fetched_at; the source publishes no as-of timestamp",
			}
			if len(kept) == 0 {
				meta.Notes = append(meta.Notes, fmt.Sprintf("no Multi Ekicube bank matched (scanned %d of %d); Multi Ekicube covers JR East and partner sites only; try near for other lockers", scanned, total))
			}
			view := vacancyView{Meta: meta, Results: kept}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := newSafeTextWriter(cmd.OutOrStdout())
			if len(kept) == 0 {
				fmt.Fprintln(w, "No Multi Ekicube banks matched.")
				return nil
			}
			tw := &widthTable{}
			tw.add("ID", "STATION", "AREA", "GATE", "RESERVABLE EMPTY (size:n ¥fee)")
			for _, b := range kept {
				var parts []string
				for _, x := range b.Boxes {
					parts = append(parts, fmt.Sprintf("%s:%d ¥%s", x.Key, x.ReservableEmpty, intText(x.UsageFeeYen)))
				}
				tw.add(fmt.Sprint(b.ID), strOr(b.Station.EN, strOr(b.Station.JA, "-")), truncateWidth(strOr(b.Area.JA, "-")+" "+strOr(b.Name.JA, ""), 40), strOr(b.Gate, "-"), strings.Join(parts, ", "))
			}
			if err := tw.write(w); err != nil {
				return err
			}
			fmt.Fprintf(w, "\nfetched %s. %s\n", meta.FetchedAt, locker.ReservableCaveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&station, "station", "", "Live map station: maihama (Tokyo Disney Resort)")
	cmd.Flags().StringVar(&latS, "lat", "", "Latitude (use with --lon)")
	cmd.Flags().StringVar(&lonS, "lon", "", "Longitude (use with --lat)")
	cmd.Flags().IntVar(&radius, "radius", 500, "Search radius in metres with --lat/--lon (50-5000)")
	cmd.Flags().StringVar(&gate, "gate", "", "Keep banks inside or outside the ticket gates")
	cmd.Flags().StringVar(&size, "size", "", "Keep banks that have a box of at least this size class: S, M, L or XL")
	cmd.Flags().StringVar(&date, "date", "", "Availability date YYYY-MM-DD (default today JST)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum banks to return (1-100)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Maximum source pages of 20 banks to read (1-10)")
	return cmd
}

func bankHasSize(b locker.EkicubeLocation, want string) bool {
	need := locker.SizeRank(want)
	for _, x := range b.Boxes {
		if x.SizeClass != nil && locker.SizeRank(*x.SizeClass) >= need {
			return true
		}
	}
	return false
}

func intText(p *int) string {
	if p == nil {
		return "?"
	}
	return fmt.Sprintf("%d", *p)
}

// bankSizesKnown reports whether any box of the bank has a size class.
func bankSizesKnown(b locker.EkicubeLocation) bool {
	for _, x := range b.Boxes {
		if x.SizeClass != nil {
			return true
		}
	}
	return false
}
