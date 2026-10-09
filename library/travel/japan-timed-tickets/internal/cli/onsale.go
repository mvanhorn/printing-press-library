// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

type onsaleView struct {
	Meta    ticketsMeta         `json:"meta"`
	Results []tickets.OnsaleRow `json:"results"`
}

var onsaleKeep = []string{"sight", "name_en", "name_ja", "visit_date", "on_sale_jst", "on_sale_local", "on_sale_earliest_day", "on_sale_latest_day", "sale_ends_jst", "confidence", "sale_basis", "window", "state", "reason", "day_status", "handoff_url", "sunset"}

func newNovelOnsaleCmd(flags *rootFlags) *cobra.Command {
	var rf rangeFlags
	var sightsCSV string
	var tz string
	var ics bool

	cmd := &cobra.Command{
		Use:   "onsale",
		Short: "When tickets go on sale for each sight and visit date, and whether to book now, wait or drop the date",
		Long: strings.Trim(`
Use this command to decide, per sight and visit date in a trip range, whether to
book now, wait for an exact sale moment, or drop the date.

Each row gives the sale moment in JST (and in --tz), a confidence label
(exact = official rule confirmed live; announced = a published notice or
listing; estimated = vague or unconfirmed; unknown), the sale window
(not_yet_open / open / ended / unknown) and an act-now state:
book-now, few, opens-at, closed, sold-out, no-general-sale or unknown (with a
reason). SHIBUYA SKY rows also carry the computed sunset time, the 20-minute
slots to target and their web price tier.

Rules: Ghibli Museum sells a month at 10:00 JST on the 10th of the previous
month (Lawson Ticket). SHIBUYA SKY sells at 00:00 JST 14 days before entry.
teamLab venues sell up to a published calendar month and announce the next
release on their ticket sites.

Do NOT use this command for 30-minute slot stock; use 'availability --slots'
instead. Use --ics to export the future sale moments as calendar events.`, "\n"),
		Example: strings.Trim(`
  japan-timed-tickets-pp-cli onsale --from 2026-11-20 --to 2026-11-27 --tz Australia/Melbourne --agent
  japan-timed-tickets-pp-cli onsale --sights shibuya-sky --date 2026-11-24 --tz America/Los_Angeles --agent
  japan-timed-tickets-pp-cli onsale --sights ghibli-museum,teamlab --from 2027-01-05 --to 2027-01-12 --ics > sales.ics`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--sights=shibuya-sky,teamlab-borderless;--from=2026-11-20;--to=2026-11-22"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "onsale (read official sale rules and calendars)")
			}
			loc, err := resolveTZ(tz)
			if err != nil {
				return usageErr(err)
			}
			run, err := runLive(cmd, flags, &rf, sightsCSV, false)
			if err != nil {
				return err
			}
			rows := tickets.BuildOnsale(run.data, run.sights, run.from, run.to, run.now, loc)
			if ics {
				cal, events := tickets.BuildICS(rows, run.now)
				if events == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "no future sale moments in this range; no calendar written")
					return nil
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), cal)
				return err
			}
			meta := run.meta(loc)
			view := onsaleView{Meta: meta, Results: rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, onsaleKeep...)
			}
			return renderOnsaleHuman(cmd, rows, meta.Warnings)
		},
	}
	cmd.Flags().StringVar(&rf.date, "date", "", "Single visit date, YYYY-MM-DD (JST)")
	cmd.Flags().StringVar(&rf.from, "from", "", "First visit date, YYYY-MM-DD (JST); defaults to --to")
	cmd.Flags().StringVar(&rf.to, "to", "", "Last visit date, YYYY-MM-DD (JST); defaults to --from")
	cmd.Flags().StringVar(&sightsCSV, "sights", "", "Comma-separated sight IDs (see 'sights'); 'teamlab' selects every teamLab venue; default all")
	cmd.Flags().StringVar(&tz, "tz", "", "IANA time zone for on_sale_local and sunset_local, for example Europe/London")
	cmd.Flags().BoolVar(&ics, "ics", false, "Print future sale moments as an iCalendar (.ics) file instead of JSON")
	return cmd
}

func renderOnsaleHuman(cmd *cobra.Command, rows []tickets.OnsaleRow, warnings []string) error {
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(w, "SIGHT\tVISIT\tSTATE\tON SALE (JST)\tLOCAL\tCONFIDENCE\tNOTE")
	for _, r := range rows {
		onsale := ptrOr(r.OnSaleJST, "")
		if onsale == "" && r.OnSaleEarliest != nil {
			onsale = *r.OnSaleEarliest + ".." + ptrOr(r.OnSaleLatest, "")
		}
		if onsale == "" {
			onsale = "-"
		}
		note := r.Reason
		if r.Sunset != nil {
			slots := make([]string, 0, len(r.Sunset.TargetSlots))
			for _, s := range r.Sunset.TargetSlots {
				slots = append(slots, fmt.Sprintf("%s (¥%d)", s.Start, s.WebAdultJPY))
			}
			note = fmt.Sprintf("sunset %s; target %s", r.Sunset.SunsetJST[11:16], strings.Join(slots, ", "))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Sight, r.VisitDate, r.State, onsale, ptrOr(r.OnSaleLocal, "-"), r.Confidence, oneLine(note))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	printWarnings(cmd.ErrOrStderr(), warnings)
	return nil
}
