// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/sources"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

type rideView struct {
	RideID       int            `json:"ride_id"`
	Name         string         `json:"name"`
	Land         string         `json:"land,omitempty"`
	IsOpen       bool           `json:"is_open"`
	WaitMinutes  *int           `json:"wait_minutes"`
	LastUpdated  string         `json:"last_updated"`
	Typical      *parks.Typical `json:"typical,omitempty"`
	DeltaMinutes *float64       `json:"delta_vs_typical_minutes,omitempty"`
}

type waitsResult struct {
	Park           parkView   `json:"park"`
	Rides          []rideView `json:"rides"`
	OpenRides      int        `json:"open_rides"`
	TotalRides     int        `json:"total_rides"`
	HistoryUsed    bool       `json:"history_used"`
	HistoryDB      string     `json:"history_db,omitempty"`
	HistoryNote    string     `json:"history_note,omitempty"`
	ComparedWindow string     `json:"compared_window,omitempty"`
}

func newNovelWaitsCmd(flags *rootFlags) *cobra.Command {
	var openOnly bool
	var maxWait int
	var sortBy string
	var limit int
	var minSamples int
	var dbPath string
	var noHistory bool

	cmd := &cobra.Command{
		Use:   "waits [park]",
		Short: "Live ride waits for one park, with your usual wait for this hour when history exists",
		Long: strings.Trim(`
Show live ride waits from Queue-Times for one park (Powered by Queue-Times.com,
https://queue-times.com/). Waits are the park-published values that
Queue-Times collects about every 5 minutes; last_updated is the source time.

When the local snapshot history (written by 'snapshot') has samples for the
current weekday and hour in Asia/Tokyo, each ride also gets a typical block
(median, p75, sample count, distinct days, first and last date) and
delta_vs_typical_minutes. Without history these fields are absent; nothing
is estimated.

Use this command on the park day to choose the next ride.
Do NOT use this command for weekday or hour patterns; use 'typical' instead.`, "\n"),
		Example: strings.Trim(`
  japan-theme-parks-pp-cli waits tds
  japan-theme-parks-pp-cli waits 275 --open-only --max-wait 30
  japan-theme-parks-pp-cli waits usj --sort delta --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "park=275"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "waits")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("park is required (one of: %s)", strings.Join(parks.Keys(), ", ")))
			}
			park, err := lookupParkArg(args[0])
			if err != nil {
				return err
			}
			if minSamples < 1 {
				return usageErr(fmt.Errorf("--min-samples must be at least 1"))
			}
			switch sortBy {
			case "wait", "delta", "name":
			default:
				return usageErr(fmt.Errorf("--sort must be wait, delta or name (got %q)", sortBy))
			}
			if !noHistory {
				var err error
				if dbPath, err = resolveDBPath(dbPath); err != nil {
					return usageErr(err)
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := sources.New(flags.timeout, flags.rateLimit)
			f, err := client.QueueTimes(ctx, park.QueueTimesID)
			if err != nil {
				return sourceFailure(fmt.Errorf("fetching Queue-Times waits for %s: %w", park.NameEN, err), err)
			}
			rides, err := parks.ParseQueueTimes(f.Body)
			if err != nil {
				return apiErr(err)
			}

			res := waitsResult{Park: parkToView(park), Rides: make([]rideView, 0, len(rides)), TotalRides: len(rides)}
			now := time.Now().In(parks.Tokyo)
			typicalByRide := map[int]parks.Typical{}
			notes := []string{}
			if !noHistory {
				res.HistoryDB = dbPath
				res.ComparedWindow = fmt.Sprintf("%s %02d:00-%02d:59 Asia/Tokyo", parks.WeekdayName(now), now.Hour(), now.Hour())
				hist, err := loadHistory(ctx, cmd.ErrOrStderr(), flags, dbPath, store.WaitQuery{
					ParkID: park.QueueTimesID, Weekdays: []int{int(now.Weekday())},
					HourSet: true, HourFrom: now.Hour(), HourTo: now.Hour(),
				})
				if err != nil {
					return err
				}
				if !hist.Exists {
					res.HistoryNote = "no local history; run 'japan-theme-parks-pp-cli snapshot' on a schedule to add typical waits"
				} else {
					if hist.Bad > 0 {
						notes = append(notes, fmt.Sprintf("%d stored rows skipped: unreadable timestamps", hist.Bad))
					}
					for _, t := range parks.Summarize(samplesOf(hist.Rows), minSamples) {
						typicalByRide[t.RideID] = t
					}
					if len(typicalByRide) == 0 {
						res.HistoryNote = fmt.Sprintf("local history has no samples for %s; typical fields omitted", res.ComparedWindow)
					} else {
						res.HistoryUsed = true
					}
				}
			}

			for _, r := range rides {
				if r.IsOpen {
					res.OpenRides++
				}
				if openOnly && !r.IsOpen {
					continue
				}
				if maxWait > 0 && (!r.IsOpen || r.WaitMinutes > maxWait) {
					continue
				}
				v := rideView{RideID: r.ID, Name: r.Name, Land: r.Land, IsOpen: r.IsOpen, LastUpdated: jst(r.LastUpdated)}
				if r.IsOpen {
					w := r.WaitMinutes
					v.WaitMinutes = &w
				}
				if t, ok := typicalByRide[r.ID]; ok {
					v.Typical = &t
					if t.Median != nil && v.WaitMinutes != nil {
						d := float64(*v.WaitMinutes) - *t.Median
						v.DeltaMinutes = &d
					}
				}
				res.Rides = append(res.Rides, v)
			}
			sortRides(res.Rides, sortBy)
			if limit > 0 && len(res.Rides) > limit {
				res.Rides = res.Rides[:limit]
			}

			meta := newMeta()
			meta.Sources = append(meta.Sources, queueTimesRef(f))
			meta.Notes = append(meta.Notes, "wait_minutes is null when a ride is closed; closed is not the same as a 0-minute wait")
			meta.Notes = append(meta.Notes, notes...)
			out := struct {
				Meta    respMeta    `json:"meta"`
				Results waitsResult `json:"results"`
			}{meta, res}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), out, flags, "wait_minutes", "is_open", "last_updated", "land", "typical", "delta_vs_typical_minutes")
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s (%s) — %d of %d rides open — fetched %s\n", park.NameEN, park.NameJA, res.OpenRides, res.TotalRides, jst(f.FetchedAt))
			tw := newTabWriter(w)
			if res.HistoryUsed {
				fmt.Fprintf(tw, "RIDE\tLAND\tWAIT\tTYPICAL(%s)\tΔ\tN\n", res.ComparedWindow)
			} else {
				fmt.Fprintln(tw, "RIDE\tLAND\tWAIT")
			}
			for _, r := range res.Rides {
				wait := "closed"
				if r.WaitMinutes != nil {
					wait = fmt.Sprintf("%d min", *r.WaitMinutes)
				}
				if !res.HistoryUsed {
					fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Land, wait)
					continue
				}
				typ, delta, n := "-", "-", "0"
				if r.Typical != nil {
					n = fmt.Sprint(r.Typical.Samples)
					if r.Typical.Median != nil {
						typ = fmt.Sprintf("%.0f min", *r.Typical.Median)
					} else {
						typ = r.Typical.Status
					}
				}
				if r.DeltaMinutes != nil {
					delta = fmt.Sprintf("%+.0f", *r.DeltaMinutes)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.Land, wait, typ, delta, n)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if res.HistoryNote != "" {
				fmt.Fprintln(w, "note:", res.HistoryNote)
			}
			printAttribution(w, meta)
			return nil
		},
	}
	cmd.Flags().BoolVar(&openOnly, "open-only", false, "Show only rides that are open now")
	cmd.Flags().IntVar(&maxWait, "max-wait", 0, "Show only open rides with a live wait at or under this many minutes (0 = no filter)")
	cmd.Flags().StringVar(&sortBy, "sort", "wait", "Sort rides by: wait (longest first), delta (most below usual first), name")
	cmd.Flags().IntVar(&limit, "limit", 0, "Return at most this many rides (0 = all)")
	cmd.Flags().IntVar(&minSamples, "min-samples", 3, "Samples needed before a typical wait is shown instead of \"insufficient\"")
	cmd.Flags().StringVar(&dbPath, "db", "", "Snapshot history database path (default: the CLI data directory)")
	cmd.Flags().BoolVar(&noHistory, "no-history", false, "Do not read local snapshot history")
	return cmd
}

func sortRides(rides []rideView, by string) {
	waitOf := func(r rideView) int {
		if r.WaitMinutes == nil {
			return -1
		}
		return *r.WaitMinutes
	}
	sort.SliceStable(rides, func(i, j int) bool {
		a, b := rides[i], rides[j]
		switch by {
		case "name":
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		case "delta":
			if (a.DeltaMinutes == nil) != (b.DeltaMinutes == nil) {
				return a.DeltaMinutes != nil
			}
			if a.DeltaMinutes != nil && *a.DeltaMinutes != *b.DeltaMinutes {
				return *a.DeltaMinutes < *b.DeltaMinutes
			}
			if waitOf(a) != waitOf(b) {
				return waitOf(a) > waitOf(b)
			}
		default:
			if waitOf(a) != waitOf(b) {
				return waitOf(a) > waitOf(b)
			}
			if a.Name != b.Name {
				return a.Name < b.Name
			}
		}
		return a.RideID < b.RideID
	})
}
