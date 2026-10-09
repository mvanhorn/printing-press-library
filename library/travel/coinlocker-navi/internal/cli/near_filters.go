// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"

	"github.com/spf13/cobra"
)

// nearFilters are the filters that need no extra request.
type nearFilters struct {
	size       string
	ic         bool
	walkUpOnly bool
	window     *locker.Window
	strict     bool
}

// applyCheapFilters drops records that fail a filter, counting each drop in
// excluded by reason (unknown values never match). With a window it sets
// open_during and orders yes, then train_hours, then unknown (stable);
// when the window touches the usual no-train hours, train_hours goes last.
func applyCheapFilters(rows []locker.Locker, f nearFilters, excluded map[string]int) []locker.Locker {
	kept := make([]locker.Locker, 0, len(rows))
	for _, r := range rows {
		if f.walkUpOnly && (r.WalkUp == nil || !*r.WalkUp) {
			if r.WalkUp == nil {
				excluded["walk_up_unknown"]++
			} else {
				excluded["not_walk_up"]++
			}
			continue
		}
		if f.size != "" && !hasSize(r, f.size) {
			if !r.SizesKnown {
				excluded["size_unknown"]++
			} else {
				excluded["size_too_small"]++
			}
			continue
		}
		if f.ic && (r.ICCard == nil || !*r.ICCard) {
			if r.ICCard == nil {
				excluded["payment_unknown"]++
			} else {
				excluded["no_ic_card"]++
			}
			continue
		}
		if f.window != nil {
			v := locker.OpenDuring(r.Hours, *f.window)
			r.OpenDuring = &v
			if v == locker.OpenNo {
				excluded["closed_during_window"]++
				continue
			}
			if f.strict && v != locker.OpenYes {
				excluded["hours_not_confirmed"]++
				continue
			}
		}
		kept = append(kept, r)
	}
	if f.window != nil {
		rank := map[string]int{locker.OpenYes: 0, locker.OpenTrainHours: 1, locker.OpenUnknown: 2}
		if locker.WindowInNoTrainHours(*f.window) {
			// Stations with 始発～終電 hours are usually shut at night, so
			// those records are less likely to be open than unknown ones.
			rank[locker.OpenTrainHours] = 3
		}
		sort.SliceStable(kept, func(i, j int) bool {
			return rank[strOr(kept[i].OpenDuring, "")] < rank[strOr(kept[j].OpenDuring, "")]
		})
	}
	return kept
}

// liveJoin tracks which records the Multi Ekicube pages read can speak for.
type liveJoin struct {
	lat, lon   float64
	radiusM    float64 // query radius; banks beyond it were never asked for
	needM      float64 // a record is checked when every bank within needM of it was read
	coveredM   float64 // every bank closer than this to the centre was read
	checked    map[string]bool
	notChecked int
	sourceMore bool
}

func (j *liveJoin) covers(r locker.Locker) bool {
	if r.Lat == nil || r.Lon == nil {
		return false
	}
	reach := math.Min(j.coveredM, j.radiusM)
	return locker.DistanceM(j.lat, j.lon, *r.Lat, *r.Lon)+j.needM <= reach
}

// done decides whether Multi Ekicube paging can stop. Without --gate it
// stops when every record is covered; with --gate when limit covered records
// have a confirmed gate match (text conflicts do not count).
func (j *liveJoin) done(kept []locker.Locker, gate string, limit int) func([]locker.EkicubeLocation, float64) bool {
	return func(read []locker.EkicubeLocation, cov float64) bool {
		j.coveredM = cov
		if gate == "" {
			for _, r := range kept {
				if r.Lat != nil && r.Lon != nil && !j.covers(r) {
					return false
				}
			}
			return true
		}
		matches := 0
		for _, r := range kept {
			if !j.covers(r) {
				continue
			}
			g := locker.GateFor(*r.Lat, *r.Lon, read)
			if gateMatches(g, gate) && locker.TextGateConflict(r, *g) == "" {
				matches++
			}
		}
		return matches >= limit
	}
}

// runLiveJoin reads Multi Ekicube (and the Maihama map near Maihama) and
// attaches gate and live data to the covered records in kept.
func runLiveJoin(ctx context.Context, cmd *cobra.Command, c *locker.Client, kept []locker.Locker, lat, lon *float64, live bool, gate string, limit int, date string, fetchedAt time.Time) (*liveJoin, []string, []map[string]string, error) {
	cLat, cLon, radius, ok := center(kept, lat, lon)
	if !ok {
		return nil, nil, nil, nil
	}
	var notes []string
	var failures []map[string]string
	observedAt := fetchedAt.Format(time.RFC3339)
	j := &liveJoin{lat: cLat, lon: cLon, radiusM: float64(radius), needM: float64(locker.GateRadiusM), checked: map[string]bool{}}
	if live {
		j.needM = float64(locker.MatchRadiusM)
	}
	epages := 3
	if cliutil.IsDogfoodEnv() {
		epages = 1
	}
	banks, _, emore, err := c.Ekicube(ctx, locker.EkicubeQuery{Lat: &cLat, Lon: &cLon, RadiusM: radius, Date: date, MaxPages: epages, JapaneseOnly: true, Done: j.done(kept, gate, limit)})
	if err != nil {
		if gate != "" {
			return nil, nil, nil, classifyLockerErr("multiecube (needed for --gate)", err)
		}
		failures = append(failures, map[string]string{"source": "multiecube", "error": err.Error()})
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: multiecube live data failed; results carry no live data: %v\n", err)
	} else {
		j.sourceMore = emore
		for i := range kept {
			if j.covers(kept[i]) {
				j.checked[kept[i].ID] = true
				locker.AttachEkicube(kept[i:i+1], banks, observedAt, live)
			} else if kept[i].Lat != nil {
				j.notChecked++
			}
		}
		if j.notChecked > 0 {
			notes = append(notes, fmt.Sprintf("%d records lie beyond the Multi Ekicube banks read (%d m around the centre; paging stops when enough records are checked); they carry no live or gate data", j.notChecked, int(math.Min(j.coveredM, j.radiusM))))
		} else if emore {
			notes = append(notes, fmt.Sprintf("Multi Ekicube had more banks in the %d m radius than the pages read", radius))
		}
	}
	if live && locker.DistanceM(cLat, cLon, locker.MaihamaLat, locker.MaihamaLon) <= 2000 {
		board, merr := c.Maihama(ctx)
		if merr != nil {
			failures = append(failures, map[string]string{"source": "coinlocker-navi-maihama", "error": merr.Error()})
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: Maihama live map failed: %v\n", merr)
		} else {
			locker.AttachMaihama(kept, board, observedAt)
		}
	}
	if live {
		notes = append(notes, fmt.Sprintf("live data is attached only where a source publishes it: the nearest Multi Ekicube bank within %d m whose gate side does not contradict the record's 改札内/改札外 text (may be a neighbouring bank; reservable empties; 0 does not mean full) and Maihama Station blocks matched by locker id (physical empties); live bank names are Japanese only here, use vacancy for English names", locker.MatchRadiusM))
	}
	return j, notes, failures, nil
}

// filterByGate keeps records whose gate matches, counting the rest by
// reason: gate_conflict, gate_not_checked (beyond the banks read),
// gate_unknown, other_gate.
func filterByGate(kept []locker.Locker, gate string, j *liveJoin, excluded map[string]int) []locker.Locker {
	out := make([]locker.Locker, 0, len(kept))
	for _, r := range kept {
		if gateMatches(r.Gate, gate) {
			out = append(out, r)
			continue
		}
		switch {
		case r.GateConflict != nil:
			excluded["gate_conflict"]++
		case r.Gate == nil && r.Lat != nil && (j == nil || !j.checked[r.ID]):
			excluded["gate_not_checked"]++
		case r.Gate == nil:
			excluded["gate_unknown"]++
		default:
			excluded["other_gate"]++
		}
	}
	return out
}

// gateMatches reports whether a known gate value satisfies --gate ("both"
// satisfies either side). A nil gate never matches.
func gateMatches(g *string, want string) bool {
	return g != nil && (*g == want || *g == "both")
}
