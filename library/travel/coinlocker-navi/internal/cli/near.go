// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"

	"github.com/spf13/cobra"
)

type nearView struct {
	Meta    lockerMeta      `json:"meta"`
	Results []locker.Locker `json:"results"`
}

func newNovelNearCmd(flags *rootFlags) *cobra.Command {
	var (
		latS, lonS, size, gate, openDuring, date string
		limit, maxPages                          int
		ic, walkUpOnly, live, strict             bool
	)
	cmd := &cobra.Command{
		Use:   "near [keyword]",
		Short: "Walk-up lockers by station/area keyword or coordinates, with sizes, prices, IC cards, hours",
		Long: strings.Trim(`
Use this command for walk-up lockers near a place or coordinates, with live
empties attached where a source publishes them (--live). Do NOT use this
command for a full live board of one vacancy-publishing station; use
'vacancy' instead.

A keyword runs the source's text search (names and notes that contain the
keyword, 10 per source page). --lat/--lon runs the source's nearest search
(about 100 records sorted by distance).

Filters never show unknown values as matches. --size L keeps records that
list an L or XL box; --ic keeps records whose payment list names an IC card
(Suica, PASMO, ICOCA ...); --walk-up-only drops bookable ecbo cloak rows;
--gate keeps records that have exactly one Multi Ekicube bank within 15 m
whose structured gate field says inside or outside (both counts for either);
--open-during keeps records whose hours cover the window (train-hours and unknown hours stay, flagged,
unless --strict). meta.excluded counts every record dropped for an unknown.
With --gate, Multi Ekicube pages are read nearest first and paging stops once
--limit matches are confirmed; records beyond the banks read are counted as
gate_not_checked, not as matches. When the record's own name or note says the
other side (改札内/改札外), gate stays null with gate_conflict set. With
--open-during, results are ordered yes, then train_hours, then unknown; when the window
touches the usual no-train hours (about 01:00-04:30) train_hours records go last.`, "\n"),
		Example: strings.Trim(`
  coinlocker-navi-pp-cli near 東京駅 --size L --ic --agent
  coinlocker-navi-pp-cli near --lat 35.6896 --lon 139.7006 --live --limit 5 --agent
  coinlocker-navi-pp-cli near 新宿 --open-during 10:00-21:30 --walk-up-only --agent
  coinlocker-navi-pp-cli near --lat 35.6896 --lon 139.7006 --gate outside --limit 3 --agent`, "\n"),
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
				return writeDryRun(cmd.OutOrStdout(), flags, "near: GET www.coinlocker-navi.com/search?q={keyword} (+ POST /search/search_more per extra page), or POST /search/gps/nearest_cl for --lat/--lon; --live/--gate add GET api.multiecube.com/v1/location/ph2 and the Maihama map near Maihama")
			}
			lat, lon, err := parseLatLon(cmd, latS, lonS)
			if err != nil {
				return usageErr(err)
			}
			keyword := strings.TrimSpace(strings.Join(args, " "))
			if (keyword == "") == (lat == nil) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give exactly one of a keyword or --lat/--lon"))
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
			var window *locker.Window
			if openDuring != "" {
				w, err := locker.ParseWindow(openDuring)
				if err != nil {
					return usageErr(fmt.Errorf("--open-during: %w", err))
				}
				window = &w
			}
			if strict && window == nil {
				return usageErr(fmt.Errorf("--strict needs --open-during"))
			}
			if err := validateLimitPages(limit, maxPages); err != nil {
				return usageErr(err)
			}
			if cliutil.IsDogfoodEnv() && maxPages > 2 {
				maxPages = 2
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

			var rows []locker.Locker
			more := false
			if keyword != "" {
				scanLimit := 10 * maxPages
				filtered := size != "" || ic || walkUpOnly || openDuring != "" || gate != ""
				if !filtered && limit < scanLimit {
					scanLimit = limit
				}
				rows, more, err = c.Search(ctx, keyword, scanLimit, maxPages)
				if err != nil {
					return classifyLockerErr("coinlocker-navi search", err)
				}
			} else {
				rows, err = c.Nearest(ctx, *lat, *lon)
				if err != nil {
					return classifyLockerErr("coinlocker-navi nearest", err)
				}
			}
			scanned := len(rows)
			notes := []string{locker.NoRecordDateNote}
			excluded := map[string]int{}

			// Cheap filters first, so live lookups cover only candidate rows.
			kept := applyCheapFilters(rows, nearFilters{size: size, ic: ic, walkUpOnly: walkUpOnly, window: window, strict: strict}, excluded)
			// Without --gate the limit can apply before the live join.
			if gate == "" && len(kept) > limit {
				kept = kept[:limit]
				more = true
			}

			var failures []map[string]string
			var join *liveJoin
			if (live || gate != "") && len(kept) > 0 {
				var jnotes []string
				var jerr error
				join, jnotes, failures, jerr = runLiveJoin(ctx, cmd, c, kept, lat, lon, live, gate, limit, date, fetchedAt)
				if jerr != nil {
					return jerr
				}
				notes = append(notes, jnotes...)
				if join != nil && (join.notChecked > 0 || join.sourceMore) {
					more = true
				}
			}
			if gate != "" {
				notes = append(notes, fmt.Sprintf("gate side comes only from Multi Ekicube structured fields, copied when exactly one bank lies within %d m of the record; otherwise gate is null and never matches --gate", locker.GateRadiusM))
				kept = filterByGate(kept, gate, join, excluded)
			}
			if len(kept) > limit {
				kept = kept[:limit]
				more = true
			}

			meta := newLockerMeta("near", c, fetchedAt)
			meta.Query = map[string]any{"keyword": keyword, "lat": lat, "lon": lon, "size": size, "ic": ic, "walk_up_only": walkUpOnly, "gate": gate, "open_during": openDuring, "strict": strict, "live": live, "date": date, "limit": limit}
			meta.Scanned, meta.Returned, meta.MoreAtSource = intPtr(scanned), intPtr(len(kept)), boolPtr(more)
			if len(excluded) > 0 {
				meta.Excluded = excluded
			}
			if window != nil {
				notes = append(notes, "open_during: yes = published clock hours cover the window; train_hours = 始発～終電 (exact times not published); unknown = no hours published; no = excluded")
				if locker.WindowInNoTrainHours(*window) {
					notes = append(notes, "the window touches the usual no-train hours (about 01:00-04:30 JST); train_hours (始発～終電) records are likely shut then and are listed last")
				}
			}
			if scanned == 0 && keyword != "" {
				notes = append(notes, fmt.Sprintf("the source found no records for %q; try the station name in Japanese (for example 新宿駅) or --lat/--lon", keyword))
			}
			if len(kept) == 0 && scanned > 0 {
				notes = append(notes, fmt.Sprintf("scanned %d records and none matched the filters; see meta.excluded; for a keyword search raise --max-pages to scan more", scanned))
			}
			meta.Notes = notes
			meta.FetchFailures = failures
			view := nearView{Meta: meta, Results: kept}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "live", "open_during", "gate", "gate_source")
			}
			return renderLockerTable(cmd, kept, view.Meta)
		},
	}
	cmd.Flags().StringVar(&latS, "lat", "", "Latitude of the place (use with --lon)")
	cmd.Flags().StringVar(&lonS, "lon", "", "Longitude of the place (use with --lat)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum records to return (1-100)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Keyword search: maximum source pages of 10 to scan (1-10)")
	cmd.Flags().StringVar(&size, "size", "", "Keep records with a box of at least this size: S, M, L or XL")
	cmd.Flags().BoolVar(&ic, "ic", false, "Keep records whose payment list names an IC card (Suica, PASMO, ICOCA ...)")
	cmd.Flags().BoolVar(&walkUpOnly, "walk-up-only", false, "Drop bookable services such as ecbo cloak")
	cmd.Flags().BoolVar(&live, "live", false, fmt.Sprintf("Attach live empties from Multi Ekicube (within %d m) and the Maihama map", locker.MatchRadiusM))
	cmd.Flags().StringVar(&gate, "gate", "", fmt.Sprintf("Keep records inside or outside the ticket gates (only from a Multi Ekicube bank within %d m)", locker.GateRadiusM))
	cmd.Flags().StringVar(&openDuring, "open-during", "", "Keep records open across HH:MM-HH:MM (drop-off to pick-up, JST)")
	cmd.Flags().BoolVar(&strict, "strict", false, "With --open-during, keep only records whose published clock hours cover the window")
	cmd.Flags().StringVar(&date, "date", "", "Date for Multi Ekicube availability, YYYY-MM-DD (default today JST)")
	return cmd
}

func hasSize(r locker.Locker, want string) bool {
	need := locker.SizeRank(want)
	for _, s := range r.Sizes {
		if s.Label != nil && locker.SizeRank(*s.Label) >= need {
			return true
		}
	}
	return false
}

// center picks the query point for the live join and a radius that covers
// the records.
func center(rows []locker.Locker, lat, lon *float64) (float64, float64, int, bool) {
	var cLat, cLon float64
	if lat != nil && lon != nil {
		cLat, cLon = *lat, *lon
	} else {
		n := 0
		for _, r := range rows {
			if r.Lat != nil && r.Lon != nil {
				cLat += *r.Lat
				cLon += *r.Lon
				n++
			}
		}
		if n == 0 {
			return 0, 0, 0, false
		}
		cLat, cLon = cLat/float64(n), cLon/float64(n)
	}
	maxD := 0.0
	for _, r := range rows {
		if r.Lat != nil && r.Lon != nil {
			maxD = math.Max(maxD, locker.DistanceM(cLat, cLon, *r.Lat, *r.Lon))
		}
	}
	radius := int(maxD) + 100
	if radius < 300 {
		radius = 300
	}
	if radius > 3000 {
		radius = 3000
	}
	return cLat, cLon, radius, true
}
