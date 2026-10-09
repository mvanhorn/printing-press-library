// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/sources"
)

const maxDateSpanDays = 92

type hoursView struct {
	Open          *string               `json:"open"`
	Close         *string               `json:"close"`
	Source        *string               `json:"source"`
	SourceKind    string                `json:"source_kind,omitempty"`
	FetchedAt     string                `json:"fetched_at,omitempty"`
	Attribution   string                `json:"attribution,omitempty"`
	Extra         []parks.ScheduleEntry `json:"extra_schedule,omitempty"`
	UnknownReason string                `json:"unknown_reason,omitempty"`
}

type saleRule struct {
	At       string `json:"at"`
	Observed bool   `json:"observed"`
	Basis    string `json:"basis"`
}

type ticketView struct {
	parks.TDRTicket
	SaleOpensAt *saleRule `json:"sale_opens_at,omitempty"`
}

type dateRow struct {
	Date                 string       `json:"date"`
	Weekday              string       `json:"weekday"`
	Park                 string       `json:"park"`
	ParkNameEN           string       `json:"park_name_en"`
	ParkNameJA           string       `json:"park_name_ja"`
	Hours                hoursView    `json:"hours"`
	OneDayStatus         *string      `json:"one_day_passport_status"`
	OneDayAdultYen       *int         `json:"one_day_passport_adult_yen"`
	Tickets              []ticketView `json:"tickets"`
	TicketsSource        *string      `json:"tickets_source"`
	TicketsFetchedAt     string       `json:"tickets_fetched_at,omitempty"`
	TicketsUnknownReason string       `json:"tickets_unknown_reason,omitempty"`
	SaleOpensAt          *saleRule    `json:"sale_opens_at,omitempty"`
	CalendarURL          string       `json:"calendar_url,omitempty"`
}

type tdrMonthState struct {
	cal       parks.TDRCalendar
	reason    string // why the month has no data (unpublished or fetch failure)
	err       error  // fetch failure; the month is unknown
	enErr     error  // English page fetch or parse error (names only)
	noPopup   bool   // HTTP 404 or the page had no ticketPopup line
	fetchedAt time.Time
}

func (st *tdrMonthState) published() bool { return len(st.cal) > 0 }

// scheduleResult is one park's ThemeParks.wiki schedule or its failure.
type scheduleResult struct {
	sched *parks.Schedule
	fetch sources.Fetch
	err   string
}

func newNovelDatesCmd(flags *rootFlags) *cobra.Command {
	var fromArg, toArg, parkCSV, ticketCSV, statusCSV string

	cmd := &cobra.Command{
		Use:   "dates [date]",
		Short: "Per date and park: TDR ticket sale status and price, and park hours (TDL, TDS, USJ, Fuji-Q)",
		Long: strings.Trim(`
For each date and park, show what decides a park day:

  - Tokyo Disneyland / Tokyo DisneySea: sale status per ticket type (on_sale,
    few_left, sold_out, not_yet_on_sale, no_online_sale), adult/junior/child
    price in yen, and opening hours, from the official TDR ticket calendar
    (Japanese and English names). The official calendar symbol mixes all
    ticket types; this command keeps them apart.
  - Universal Studios Japan / Fuji-Q Highland: opening hours from ThemeParks.wiki
    (third-party aggregator, not official; Powered by ThemeParks.wiki,
    https://themeparks.wiki). It covers about one month ahead; later dates show
    hours null with the reason "beyond third-party source horizon".
    USJ dated Studio Pass price and Express Pass availability are null with a
    reason: they are only in the queued USJ web store, which this CLI does not read.

For dates that are not on sale yet, sale_opens_at applies the published TDR
rule (14:00 JST, two months ahead) and is marked observed=false.
Read-only: this command never buys, holds or adds tickets to a cart.

Dates: YYYY-MM-DD, today, tomorrow or +Nd. One date, or --from/--to covering
at most 92 days (both ends included).

Use this command to compare dates across parks (sale status, price and hours in one row).
Do NOT use this command for ride waits; use 'waits' or 'typical' instead.`, "\n"),
		Example: strings.Trim(`
  japan-theme-parks-pp-cli dates 2026-11-14
  japan-theme-parks-pp-cli dates --from 2026-11-13 --to 2026-11-15 --park tds,usj
  japan-theme-parks-pp-cli dates --from +7d --to +60d --park tds --ticket T1 --status on-sale,few-left --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "date=+14d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dates")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			now := time.Now().In(parks.Tokyo)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, parks.Tokyo)
			from, to, err := resolveDateRange(args, fromArg, toArg, today)
			if err != nil {
				return usageErr(err)
			}
			list, err := parks.LookupList(parkCSV, []string{"tdl", "tds", "usj", "fujiq"})
			if err != nil {
				return usageErr(err)
			}
			statusFilter := map[string]bool{}
			for _, s := range parks.SplitCSV(strings.ToLower(statusCSV)) {
				st, ok := parks.StatusAliases[s]
				if !ok {
					return usageErr(fmt.Errorf("unknown --status %q (use on-sale, few-left, sold-out, not-yet-on-sale, no-online-sale)", s))
				}
				statusFilter[st] = true
			}
			ticketFilter := parks.SplitCSV(ticketCSV)

			needTDR, scheduleCount := false, 0
			for _, p := range list {
				if p.TDRCode != "" {
					needTDR = true
				}
				if p.ThemeParksID != "" {
					scheduleCount++
				}
			}
			monthList := []string{}
			if needTDR {
				monthList = parks.MonthsBetween(from, to)
			}
			ctx, cancel := boundCtxN(cmd.Context(), flags, 2*len(monthList)+scheduleCount)
			defer cancel()
			client := sources.New(flags.timeout, flags.rateLimit)
			meta := newMeta()
			errOut := cmd.ErrOrStderr()
			tally := fetchTally{meta: &meta}

			// Tokyo Disney Resort months (JA for data, EN for names). Only a
			// changed page structure is fatal; a fetch failure marks that
			// month unknown and the other parks still get their rows.
			months := map[string]*tdrMonthState{}
			for _, ym := range monthList {
				st, err := fetchTDRMonth(ctx, client, ym, &meta)
				if err != nil {
					return apiErr(fmt.Errorf("%w. Coverage: TDR ticket status, price and hours are unavailable until the parser is updated; nothing was guessed", err))
				}
				months[ym] = st
				if st.enErr != nil {
					meta.FetchFailures = append(meta.FetchFailures, fetchFailure{Source: parks.SourceTDR, URL: enFailureURL(st.enErr, ym), Error: "English names unavailable: " + sources.Describe(st.enErr)})
					fmt.Fprintf(errOut, "warning: TDR English calendar %s unavailable; ticket names are Japanese only: %s\n", ym, sources.Describe(st.enErr))
				}
				switch {
				case st.err != nil:
					tally.fail(parks.SourceTDR, st.err)
					fmt.Fprintf(errOut, "warning: TDR ticket calendar %s could not be fetched; TDL/TDS rows for that month are unknown: %s\n", ym, st.reason)
				case st.noPopup && monthShouldBeOnSale(ym, now):
					// Sales for this month have opened, so the calendar must
					// be on the page: the page structure changed.
					return apiErr(fmt.Errorf("%w (month %s is already on sale; %s). Coverage: TDR ticket status, price and hours are unavailable until the parser is updated; nothing was guessed", parks.ErrNoTicketPopup, ym, st.reason))
				default:
					tally.succeed()
				}
			}

			// ThemeParks.wiki schedules (third-party) for USJ and Fuji-Q.
			schedules := map[string]scheduleResult{}
			for _, p := range list {
				if p.ThemeParksID == "" {
					continue
				}
				f, err := client.ThemeParksSchedule(ctx, p.ThemeParksID)
				if err == nil {
					var s parks.Schedule
					s, err = parks.ParseSchedule(f.Body)
					if err == nil {
						schedules[p.Key] = scheduleResult{sched: &s, fetch: f}
						tally.succeed()
						meta.Sources = append(meta.Sources, themeParksRef(f))
						continue
					}
				}
				schedules[p.Key] = scheduleResult{err: sources.Describe(err)}
				tally.fail(parks.SourceThemeParks, err)
				fmt.Fprintf(errOut, "warning: ThemeParks.wiki schedule for %s failed; hours shown as null: %s\n", p.NameEN, sources.Describe(err))
			}

			if err := tally.allFailed("no source could be fetched"); err != nil {
				return err
			}

			rows := make([]dateRow, 0)
			for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
				iso := d.Format("2006-01-02")
				for _, p := range list {
					row := dateRow{Date: iso, Weekday: parks.WeekdayName(d), Park: p.Key, ParkNameEN: p.NameEN, ParkNameJA: p.NameJA, Tickets: []ticketView{}}
					switch {
					case p.TDRCode != "":
						fillTDRRow(&row, p, d, months[d.Format("200601")], ticketFilter)
					case p.ThemeParksID != "":
						fillThirdPartyHours(&row, iso, schedules[p.Key])
						row.TicketsUnknownReason = p.TicketsUnknownWhy
					default:
						row.Hours.UnknownReason = p.HoursUnknownWhy()
						row.TicketsUnknownReason = p.TicketsUnknownWhy
					}
					if len(statusFilter) > 0 {
						kept := make([]ticketView, 0, len(row.Tickets))
						for _, t := range row.Tickets {
							if statusFilter[t.Status] {
								kept = append(kept, t)
							}
						}
						if len(kept) == 0 {
							continue
						}
						row.Tickets = kept
					}
					rows = append(rows, row)
				}
			}
			if len(statusFilter) > 0 {
				meta.Notes = append(meta.Notes, "--status keeps only TDR rows with a matching ticket; USJ and Fuji-Q have no dated ticket status in these sources")
			}
			meta.Notes = append(meta.Notes,
				"one_day_passport_* fields always describe the TDR 1-Day Passport (T1), independent of --ticket and --status",
				"hours from api.themeparks.wiki are third-party (not official); confirm on the park's official site before travel")

			out := struct {
				Meta    respMeta  `json:"meta"`
				Results []dateRow `json:"results"`
			}{meta, rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), out, flags, "hours", "tickets", "tickets_source", "tickets_fetched_at", "tickets_unknown_reason", "sale_opens_at", "calendar_url", "one_day_passport_status", "one_day_passport_adult_yen", "park_name_en", "park_name_ja", "weekday")
			}
			return printDatesHuman(cmd, rows, meta, len(ticketFilter) > 0 || len(statusFilter) > 0)
		},
	}
	cmd.Flags().StringVar(&fromArg, "from", "", "First date (YYYY-MM-DD, today, tomorrow or +Nd)")
	cmd.Flags().StringVar(&toArg, "to", "", "Last date, inclusive (default: same as --from; at most 92 days counting both ends)")
	cmd.Flags().StringVar(&parkCSV, "park", "", "Comma-separated parks (tdl, tds, usj, fujiq; default: all four)")
	cmd.Flags().StringVar(&ticketCSV, "ticket", "", "TDR ticket ids (T1, T2, T7, T8) or name text such as \"After 3\" (default: all)")
	cmd.Flags().StringVar(&statusCSV, "status", "", "Keep TDR tickets with these states: on-sale, few-left, sold-out, not-yet-on-sale, no-online-sale")
	return cmd
}

// fetchTDRMonth fetches one month. It returns an error only when the page
// structure changed; fetch failures and unpublished months come back as a
// state with a reason.
func fetchTDRMonth(ctx context.Context, client *sources.Client, ym string, meta *respMeta) (*tdrMonthState, error) {
	ja, err := client.TDRMonth(ctx, ym, "ja")
	if err != nil {
		if sources.ErrorStatus(err) == http.StatusNotFound {
			return &tdrMonthState{noPopup: true, reason: "TDR has no ticket calendar page for this month yet (HTTP 404)"}, nil
		}
		return &tdrMonthState{err: err, reason: "TDR ticket calendar fetch failed: " + sources.Describe(err)}, nil
	}
	meta.Sources = append(meta.Sources, tdrRef(ja))
	raw, err := parks.ExtractTicketPopup(ja.Body)
	if errors.Is(err, parks.ErrNoTicketPopup) {
		return &tdrMonthState{noPopup: true, fetchedAt: ja.FetchedAt, reason: "the TDR calendar page for this month has no ticket data (not published yet, or the page changed)"}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ja.URL, err)
	}
	cal, err := parks.ParseTicketPopup(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ja.URL, err)
	}
	st := &tdrMonthState{cal: cal, fetchedAt: ja.FetchedAt}
	if !st.published() {
		st.reason = "TDR has not published the ticket calendar for this month yet"
		return st, nil
	}
	st.enErr = mergeEnglish(ctx, client, ym, cal, meta)
	return st, nil
}

// mergeEnglish adds English ticket names from the EN calendar page. An error
// only means the names stay Japanese.
func mergeEnglish(ctx context.Context, client *sources.Client, ym string, cal parks.TDRCalendar, meta *respMeta) error {
	en, err := client.TDRMonth(ctx, ym, "en")
	if err != nil {
		return err
	}
	meta.Sources = append(meta.Sources, tdrRef(en))
	raw, err := parks.ExtractTicketPopup(en.Body)
	if err != nil {
		return err
	}
	calEN, err := parks.ParseTicketPopup(raw)
	if err != nil {
		return err
	}
	cal.MergeEnglish(calEN)
	return nil
}

func fillTDRRow(row *dateRow, p parks.Park, d time.Time, st *tdrMonthState, ticketFilter []string) {
	src := parks.SourceTDR
	ym := d.Format("200601")
	row.CalendarURL = sources.TDRMonthURL(ym, "ja")
	rule := &saleRule{At: parks.SaleOpensAt(d).Format(time.RFC3339), Observed: false, Basis: parks.SaleRuleBasis}
	if !st.published() {
		row.TicketsUnknownReason = st.reason
		row.Hours.UnknownReason = st.reason
		if st.err == nil {
			row.SaleOpensAt = rule
		}
		row.Tickets = nil
		return
	}
	day := st.cal[d.Format("2006-01-02")][p.TDRCode]
	if day == nil {
		row.TicketsUnknownReason = "date not in the published TDR ticket calendar"
		row.Hours.UnknownReason = "date not in the published TDR ticket calendar"
		row.SaleOpensAt = rule
		row.Tickets = nil
		return
	}
	row.TicketsSource = &src
	row.TicketsFetchedAt = rfc(st.fetchedAt)
	if day.Open != "" && day.Close != "" {
		open, close := day.Open, day.Close
		row.Hours = hoursView{Open: &open, Close: &close, Source: &src, SourceKind: "official", FetchedAt: rfc(st.fetchedAt)}
	} else {
		row.Hours.UnknownReason = "TDR calendar row has no readable opening and closing time"
	}
	for _, t := range day.Tickets {
		// The 1-Day Passport summary always describes T1, whatever the
		// --ticket and --status filters keep in tickets[].
		if t.ID == "T1" && row.OneDayStatus == nil {
			s := t.Status
			row.OneDayStatus = &s
			if a, ok := t.PricesYen["adult"]; ok {
				row.OneDayAdultYen = &a
			}
		}
		if !ticketMatches(t, ticketFilter) {
			continue
		}
		tv := ticketView{TDRTicket: t}
		if t.Status == parks.StatusNotYetOnSale {
			tv.SaleOpensAt = rule
		}
		row.Tickets = append(row.Tickets, tv)
	}
}

func ticketMatches(t parks.TDRTicket, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		lf := strings.ToLower(f)
		if strings.EqualFold(t.ID, f) || strings.Contains(strings.ToLower(t.NameEN), lf) || strings.Contains(t.NameJA, f) {
			return true
		}
	}
	return false
}

func fillThirdPartyHours(row *dateRow, iso string, res scheduleResult) {
	if res.sched == nil {
		row.Hours.UnknownReason = "ThemeParks.wiki schedule fetch failed: " + res.err
		return
	}
	open, close, extra, reason := res.sched.HoursFor(iso)
	src := parks.SourceThemeParks
	row.Hours = hoursView{Source: &src, SourceKind: "third-party aggregator (not official)", FetchedAt: rfc(res.fetch.FetchedAt), Attribution: parks.ThemeParksAttribution, Extra: extra}
	if reason != "" {
		row.Hours.UnknownReason = reason
		return
	}
	row.Hours.Open, row.Hours.Close = &open, &close
}

func resolveDateRange(args []string, fromArg, toArg string, today time.Time) (time.Time, time.Time, error) {
	if len(args) > 1 {
		return time.Time{}, time.Time{}, fmt.Errorf("pass one date, or use --from/--to")
	}
	if len(args) == 1 {
		if fromArg != "" || toArg != "" {
			return time.Time{}, time.Time{}, fmt.Errorf("pass either a date argument or --from/--to, not both")
		}
		fromArg = args[0]
	}
	if fromArg == "" && toArg == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("a date or --from is required")
	}
	var from, to time.Time
	var err error
	if fromArg == "" {
		from = today
	} else if from, err = parseDay(fromArg, today); err != nil {
		return from, to, err
	}
	if toArg == "" {
		to = from
	} else if to, err = parseDay(toArg, today); err != nil {
		return from, to, err
	}
	if from.Before(today) {
		return from, to, fmt.Errorf("date %s is in the past (today is %s in Asia/Tokyo)", from.Format("2006-01-02"), today.Format("2006-01-02"))
	}
	if to.Before(from) {
		return from, to, fmt.Errorf("--to %s is before --from %s", to.Format("2006-01-02"), from.Format("2006-01-02"))
	}
	if to.After(from.AddDate(0, 0, maxDateSpanDays-1)) {
		return from, to, fmt.Errorf("date range covers more than %d days (inclusive); narrow --from/--to", maxDateSpanDays)
	}
	return from, to, nil
}

func parseDay(s string, today time.Time) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch {
	case s == "today":
		return today, nil
	case s == "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case strings.HasPrefix(s, "+") && strings.HasSuffix(s, "d"):
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(s, "+"), "d"))
		if err != nil || n < 0 || n > 400 {
			return time.Time{}, fmt.Errorf("invalid relative date %q (use +Nd, for example +14d)", s)
		}
		return today.AddDate(0, 0, n), nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, parks.Tokyo)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD, today, tomorrow or +Nd)", s)
	}
	return t, nil
}

// datesNote builds the NOTE cell. When the hours are unknown for a reason
// that differs from the ticket reason, both reasons show, so a user can see
// why the HOURS cell says "unknown".
func datesNote(r dateRow) string {
	ticket := r.TicketsUnknownReason
	if r.SaleOpensAt != nil {
		ticket = "sale opens " + r.SaleOpensAt.At + " (rule)"
	}
	hours := ""
	if r.Hours.Open == nil && r.Hours.UnknownReason != "" && r.Hours.UnknownReason != r.TicketsUnknownReason {
		hours = r.Hours.UnknownReason
	}
	switch {
	case hours != "" && ticket != "":
		return "hours: " + hours + "; tickets: " + ticket
	case hours != "":
		return "hours: " + hours
	default:
		return ticket
	}
}

// printDatesHuman prints one summary row per date and park. The 1-DAY
// PASSPORT and ADULT columns always describe T1. When --ticket or --status
// selects tickets, a second table lists the selected tickets themselves.
func printDatesHuman(cmd *cobra.Command, rows []dateRow, meta respMeta, showTickets bool) error {
	w := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(w, "No matching dates.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "DATE\tDAY\tPARK\tHOURS\t1-DAY PASSPORT\tADULT ¥\tNOTE")
	for _, r := range rows {
		hours := "unknown"
		if r.Hours.Open != nil {
			hours = *r.Hours.Open + "-" + *r.Hours.Close
			if r.Hours.SourceKind != "official" {
				hours += " (3rd-party)"
			}
		}
		status, adult := "-", "-"
		if r.OneDayStatus != nil {
			status = *r.OneDayStatus
		}
		if r.OneDayAdultYen != nil {
			adult = strconv.Itoa(*r.OneDayAdultYen)
		}
		note := datesNote(r)
		note = truncate(cliutil.ScrubTerminal(note), 110)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Date, r.Weekday, r.Park, hours, status, adult, note)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if showTickets {
		if err := printSelectedTickets(w, rows); err != nil {
			return err
		}
	}
	printAttribution(w, meta)
	return nil
}

// printSelectedTickets lists every ticket kept by --ticket or --status with
// its name, state, adult price and sale-opening time.
func printSelectedTickets(w io.Writer, rows []dateRow) error {
	hasTickets := false
	for _, r := range rows {
		if len(r.Tickets) > 0 {
			hasTickets = true
			break
		}
	}
	fmt.Fprintln(w)
	if !hasTickets {
		fmt.Fprintln(w, "Selected tickets: none in these rows (USJ and Fuji-Q have no dated ticket status in these sources).")
		return nil
	}
	fmt.Fprintln(w, "Selected tickets:")
	tw := newTabWriter(w)
	fmt.Fprintln(tw, "DATE\tPARK\tTICKET\tNAME\tSTATUS\tADULT ¥\tSALE OPENS")
	for _, r := range rows {
		for _, t := range r.Tickets {
			name := t.NameEN
			if name == "" {
				name = t.NameJA
			}
			adult := "-"
			if a, ok := t.PricesYen["adult"]; ok {
				adult = strconv.Itoa(a)
			}
			opens := "-"
			if t.SaleOpensAt != nil && t.SaleOpensAt.At != "" {
				opens = t.SaleOpensAt.At
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Date, r.Park, t.ID,
				truncate(cliutil.ScrubTerminal(name), 40), t.Status, adult, opens)
		}
	}
	return tw.Flush()
}

// monthShouldBeOnSale reports whether the published TDR rule has already
// opened sales for the first day of month ym (YYYYMM).
func monthShouldBeOnSale(ym string, now time.Time) bool {
	first, err := time.ParseInLocation("200601", ym, parks.Tokyo)
	if err != nil {
		return false
	}
	return parks.SaleOpensAt(first).Before(now)
}

// enFailureURL is the fetched URL when the error carries one, else the
// calendar page URL (parse errors).
func enFailureURL(err error, ym string) string {
	if u := sources.ErrorURL(err); u != "" {
		return u
	}
	return sources.TDRMonthURL(ym, "en")
}
