// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

// clockNow is replaceable in tests.
var clockNow = time.Now

// rangeFlags are the visit-date flags shared by onsale and availability. Each
// command declares the flags inline so verify-skill can see them.
type rangeFlags struct {
	date string
	from string
	to   string
}

var errNoDates = errors.New("--date or --from/--to is required (YYYY-MM-DD, JST)")

// resolve validates the visit range: given, today (JST) or later, and at most
// tickets.MaxRangeDays days. The day count is checked before any range is built.
func (r *rangeFlags) resolve(now time.Time) (tickets.Date, tickets.Date, error) {
	if r.date == "" && r.from == "" && r.to == "" {
		return tickets.Date{}, tickets.Date{}, errNoDates
	}
	if r.date != "" && (r.from != "" || r.to != "") {
		return tickets.Date{}, tickets.Date{}, errors.New("use --date or --from/--to, not both")
	}
	fromS, toS := r.from, r.to
	if r.date != "" {
		fromS, toS = r.date, r.date
	}
	if fromS == "" {
		fromS = toS
	}
	if toS == "" {
		toS = fromS
	}
	from, err := tickets.ParseDate(fromS)
	if err != nil {
		return tickets.Date{}, tickets.Date{}, err
	}
	to, err := tickets.ParseDate(toS)
	if err != nil {
		return tickets.Date{}, tickets.Date{}, err
	}
	today := tickets.DateOf(now)
	if from.Before(today) {
		return tickets.Date{}, tickets.Date{}, fmt.Errorf("visit date %s is before today (%s JST); past dates are not supported", from, today)
	}
	if to.Before(from) {
		return tickets.Date{}, tickets.Date{}, fmt.Errorf("--to %s is before --from %s", to, from)
	}
	if n := from.DaysUntil(to) + 1; n > tickets.MaxRangeDays {
		return tickets.Date{}, tickets.Date{}, fmt.Errorf("range has %d days; the maximum is %d", n, tickets.MaxRangeDays)
	}
	return from, to, nil
}

func resolveTZ(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return nil, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("invalid --tz %q: use an IANA zone such as Europe/London", name)
	}
	return loc, nil
}

func rejectLocalDataSource(flags *rootFlags) error {
	if flags != nil && flags.dataSource == "local" {
		return usageErr(errors.New("--data-source local is not supported: this command reads live official sources and keeps no local mirror"))
	}
	return nil
}

// newTicketsFetcher builds the read-only fetcher. --rate-limit above the
// per-host cap is clamped with a warning; --timeout below the per-request
// default shortens each request.
func newTicketsFetcher(stderr io.Writer, flags *rootFlags, budget int) *tickets.Fetcher {
	var perReq time.Duration
	rate := 0.0
	if flags != nil {
		if flags.timeout > 0 && flags.timeout < tickets.DefaultRequestTimeout {
			perReq = flags.timeout
		}
		if flags.rateLimit > 0 {
			rate = flags.rateLimit
		}
	}
	if rate > tickets.MaxPerHostRate {
		fmt.Fprintf(stderr, "warning: --rate-limit %g is above the per-host cap of %g requests/second; using %g\n", rate, tickets.MaxPerHostRate, tickets.MaxPerHostRate)
	}
	return tickets.NewFetcher(perReq, rate, budget)
}

// liveRun is one validated, fetched request shared by onsale and availability.
type liveRun struct {
	from, to tickets.Date
	sights   []tickets.Sight
	now      time.Time
	fetcher  *tickets.Fetcher
	data     map[string]*tickets.SightData
	failures []tickets.FetchFailure
}

// runLive validates the inputs, reads the selected sights and maps a total
// failure to a typed exit. withSlots reads slot stock for every date.
func runLive(cmd *cobra.Command, flags *rootFlags, rf *rangeFlags, sightsCSV string, withSlots bool) (*liveRun, error) {
	if err := rejectLocalDataSource(flags); err != nil {
		return nil, err
	}
	run := &liveRun{now: clockNow()}
	var err error
	run.from, run.to, err = rf.resolve(run.now)
	if errors.Is(err, errNoDates) {
		_ = cmd.Usage()
	}
	if err != nil {
		return nil, usageErr(err)
	}
	if run.sights, err = tickets.Select(sightsCSV); err != nil {
		return nil, usageErr(err)
	}
	var slotDates []tickets.Date
	if withSlots {
		if n := run.from.DaysUntil(run.to) + 1; n > tickets.MaxSlotDates {
			return nil, usageErr(fmt.Errorf("--slots reads at most %d dates; the range has %d", tickets.MaxSlotDates, n))
		}
		slotDates = tickets.DateRange(run.from, run.to)
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	run.fetcher = newTicketsFetcher(cmd.ErrOrStderr(), flags, tickets.RequestBudget(run.sights, len(slotDates)))
	run.data, run.failures = tickets.Collect(ctx, run.fetcher, run.sights, run.from, run.to, slotDates, clockNow)
	if err := ctx.Err(); err != nil {
		return nil, apiErr(fmt.Errorf("%s timed out: %w", cmd.Name(), err))
	}
	if err := failureError(run.failures, len(run.sights)); err != nil {
		return nil, err
	}
	if n := len(run.failures); n > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d sights could not be read; their rows fall back to stored rules with confidence estimated or unknown (see meta.fetch_failures)\n", n, len(run.sights))
	}
	return run, nil
}

// ticketsMeta is the provenance block for every live command.
type ticketsMeta struct {
	Source        string                         `json:"source"`
	GeneratedAt   string                         `json:"generated_at"`
	Timezone      string                         `json:"timezone"`
	From          string                         `json:"from"`
	To            string                         `json:"to"`
	Sights        []string                       `json:"sights"`
	Requests      int                            `json:"requests"`
	ResponseBytes int64                          `json:"response_bytes"`
	Releases      []releaseMeta                  `json:"releases,omitempty"`
	Sources       map[string][]tickets.SourceRef `json:"sources"`
	ReadOnly      string                         `json:"read_only"`
	Party         *tickets.Party                 `json:"party,omitempty"`
	Warnings      []string                       `json:"warnings"`
	FetchFailures []tickets.FetchFailure         `json:"fetch_failures"`
}

type releaseMeta struct {
	Sight         string           `json:"sight"`
	CalendarUntil string           `json:"calendar_until,omitempty"`
	OnSaleJST     *string          `json:"on_sale_jst"`
	Release       *tickets.Release `json:"release"`
}

const readOnlyNote = "reads public pages only; never buys, adds to cart, joins a waiting room or logs in"

func (run *liveRun) meta(loc *time.Location) ticketsMeta {
	reqs, bytes := run.fetcher.Stats()
	tz := "Asia/Tokyo"
	if loc != nil {
		tz = loc.String()
	}
	meta := ticketsMeta{Source: "live", GeneratedAt: tickets.FormatJST(run.now), Timezone: tz, From: run.from.String(), To: run.to.String(),
		Requests: reqs, ResponseBytes: bytes, Sources: map[string][]tickets.SourceRef{}, ReadOnly: readOnlyNote,
		Sights: make([]string, 0, len(run.sights)), Warnings: make([]string, 0), FetchFailures: run.failures}
	for _, s := range run.sights {
		meta.Sights = append(meta.Sights, s.ID)
		sd := run.data[s.ID]
		if sd == nil {
			continue
		}
		meta.Sources[s.ID] = sd.Sources
		for _, w := range sd.Warnings {
			meta.Warnings = append(meta.Warnings, s.ID+": "+w)
		}
		if sd.CalendarUntil != nil || sd.Release != nil {
			rm := releaseMeta{Sight: s.ID, Release: sd.Release}
			if sd.CalendarUntil != nil {
				rm.CalendarUntil = sd.CalendarUntil.String()
			}
			if sd.Release != nil && sd.Release.OnSale != nil {
				v := tickets.FormatJST(*sd.Release.OnSale)
				rm.OnSaleJST = &v
			}
			meta.Releases = append(meta.Releases, rm)
		}
	}
	return meta
}

// failureError maps an all-sights failure to a typed exit code.
func failureError(failures []tickets.FetchFailure, selected int) error {
	if len(failures) == 0 || len(failures) < selected {
		return nil
	}
	msg := make([]string, 0, len(failures))
	for _, f := range failures {
		msg = append(msg, f.Sight+": "+oneLine(f.Error))
	}
	err := fmt.Errorf("no selected sight could be read: %s", strings.Join(msg, "; "))
	if tickets.IsRateLimited(failures) {
		return rateLimitErr(err)
	}
	return apiErr(err)
}

func printWarnings(w io.Writer, warnings []string) {
	for _, msg := range warnings {
		fmt.Fprintln(w, "warning: "+oneLine(msg))
	}
}

func ptrOr(s *string, alt string) string {
	if s == nil {
		return alt
	}
	return *s
}

// oneLine makes a remote string safe for one table cell: control and format
// characters are removed and tabs/newlines become spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(tickets.StripControl(s)), " ")
}
