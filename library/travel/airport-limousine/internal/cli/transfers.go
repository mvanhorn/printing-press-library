// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/limousine"
	"github.com/spf13/cobra"
)

type limousineTransferView struct {
	From             string              `json:"from_airport"`
	To               string              `json:"to_airport"`
	ServiceDate      string              `json:"service_date_jst"`
	SourceURL        string              `json:"timetable_url"`
	Stations         []limousine.Station `json:"stations"`
	Journeys         []limousine.Journey `json:"journeys"`
	TotalJourneys    int                 `json:"total_journeys"`
	ReturnedJourneys int                 `json:"returned_journeys"`
	Truncated        bool                `json:"truncated"`
	CurrentEstimate  *limousine.Duration `json:"current_estimate"`
}

func newNovelTransfersCmd(flags *rootFlags) *cobra.Command {
	var date string
	var limit int
	cmd := &cobra.Command{Use: "transfers", Short: "Compare both airport-transfer directions for one JST service date", Example: "  airport-limousine-pp-cli transfers --limit 3 --agent", Annotations: limousineAnnotations("--limit=3"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "transfers")
		}
		if e := limousineLive(flags); e != nil {
			return e
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("transfers takes --date, not positional arguments"))
		}
		if e := limousineLimit(limit); e != nil {
			return e
		}
		day, e := limousineDate(date)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		p := limousine.New(flags.timeout, flags.rateLimit)
		rows := []limousineTransferView{}
		failures := []limousine.Failure{}
		sources := []string{}
		scanned := 0
		for _, item := range []struct{ route, from, to string }{{"Haneda-Narita", "haneda", "narita"}, {"Narita-Haneda", "narita", "haneda"}} {
			schedule, err := p.Timetable(ctx, item.route, day, "from-airport")
			if err != nil {
				var rate *cliutil.RateLimitError
				if errors.As(err, &rate) {
					return rateLimitErr(err)
				}
				failures = append(failures, limousine.Failure{SourceURL: limousine.PageURL("/en/timetable/detail/"+item.route, nil), Error: err.Error()})
				continue
			}
			scanned += schedule.ScannedTrains
			total := len(schedule.Journeys)
			if total > limit {
				schedule.Journeys = schedule.Journeys[:limit]
			}
			sources = append(sources, schedule.SourceURL)
			rows = append(rows, limousineTransferView{From: item.from, To: item.to, ServiceDate: day, SourceURL: schedule.SourceURL, Stations: schedule.Stations, Journeys: schedule.Journeys, TotalJourneys: total, ReturnedJourneys: len(schedule.Journeys), Truncated: total > len(schedule.Journeys)})
		}
		if len(rows) == 0 {
			return apiErr(fmt.Errorf("both airport-transfer timetable reads failed: %v", failures))
		}
		source := limousine.PageURL("/en/guide/realtime", nil)
		data, err := p.Data(ctx, "/en/guide/realtime/__data.json")
		if err != nil {
			var rate *cliutil.RateLimitError
			if errors.As(err, &rate) {
				return rateLimitErr(err)
			}
			failures = append(failures, limousine.Failure{SourceURL: source, Error: err.Error()})
		} else {
			sources = append(sources, source)
			for i := range rows {
				times, _, err := limousine.Durations(data, "", rows[i].From, "from-airport")
				if err != nil {
					failures = append(failures, limousine.Failure{SourceURL: source, Error: err.Error()})
					break
				}
				target := "Haneda Airport→Narita Airport"
				if rows[i].From == "narita" {
					target = "Narita Airport→Haneda Airport"
				}
				for _, t := range times {
					if t.Route == target {
						value := t
						rows[i].CurrentEstimate = &value
						break
					}
				}
			}
		}
		meta := p.Meta(sources, scanned, len(rows), len(rows), "Dated schedules are compared separately from current route-wide duration snapshots. Exact terminals remain in each table; current estimates have no terminal identity or source date and do not guarantee arrival or seats.")
		meta.FetchFailures = failures
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d provider reads failed; comparison contains %d verified timetable directions\n", len(failures), len(rows))
		}
		for _, r := range rows {
			if r.Truncated {
				meta.Truncated = true
			}
		}
		return flags.printJSON(cmd, limousine.Envelope{Meta: meta, Results: rows})
	}}
	cmd.Flags().StringVar(&date, "date", "", "JST service date YYYY-MM-DD; defaults to today in Tokyo")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum journeys retained per direction, from 1 to 200")
	return cmd
}
