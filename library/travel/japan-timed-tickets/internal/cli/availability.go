// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

// availabilityRow is one sight x visit date.
type availabilityRow struct {
	Sight         string            `json:"sight"`
	NameEN        string            `json:"name_en"`
	NameJA        string            `json:"name_ja"`
	Date          string            `json:"date"`
	Status        tickets.DayStatus `json:"status"`
	Reason        string            `json:"reason"`
	PriceFrom     *int              `json:"adult_price_from_jpy"`
	SlotsRead     bool              `json:"slots_read"`
	Slots         []tickets.Slot    `json:"slots,omitempty"`
	SlotsFitParty *int              `json:"slots_fit_party,omitempty"`
	PartyNote     string            `json:"party_note,omitempty"`
	HandoffURL    string            `json:"handoff_url"`
}

type availabilityView struct {
	Meta    ticketsMeta       `json:"meta"`
	Results []availabilityRow `json:"results"`
}

var availabilityKeep = []string{"sight", "name_en", "name_ja", "date", "status", "reason", "adult_price_from_jpy", "slots_read", "slots", "slots_fit_party", "party_note", "handoff_url"}

func newNovelAvailabilityCmd(flags *rootFlags) *cobra.Command {
	var rf rangeFlags
	var sightsCSV string
	var slots bool
	var partyS string

	cmd := &cobra.Command{
		Use:   "availability [sight...]",
		Short: "Which visit dates (and teamLab time slots) are still open, sold out, closed or unknown",
		Long: strings.Trim(`
Use this command to see, per sight and visit date, the source-backed day status:
available, few, sold_out, closed, no_general_sale, not_released or unknown,
with a reason and the time the source was read (meta.sources[].fetched_at).

--slots reads 30-minute slot stock for teamLab venues (up to 7 dates). Ghibli
Museum per-date stock needs a Lawson Ticket member login and SHIBUYA SKY stock is
behind a virtual waiting room, so those stay 'unknown' with a handoff URL.

--party adults=N,children=N,infants=N keeps slots whose stock fits the whole
party, plus slots with unknown stock (no fits_party verdict), and adds channel
rules (for example SHIBUYA SKY child tickets are sold only at the counter). It
does not compute prices.

Do NOT use this command to find when a month goes on sale; use 'onsale'.`, "\n"),
		Example: strings.Trim(`
  japan-timed-tickets-pp-cli availability teamlab-borderless --from 2026-11-20 --to 2026-11-27 --agent
  japan-timed-tickets-pp-cli availability teamlab-planets --date 2026-11-24 --slots --party adults=2,children=2 --agent
  japan-timed-tickets-pp-cli availability ghibli-museum --from 2026-10-12 --to 2026-10-18`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "teamlab-borderless;--from=2026-10-20;--to=2026-10-22"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "availability (read official ticket calendars)")
			}
			party, err := tickets.ParseParty(partyS)
			if err != nil {
				return usageErr(err)
			}
			parts := append([]string{}, args...)
			if sightsCSV != "" {
				parts = append(parts, sightsCSV)
			}
			csv := strings.Join(parts, ",")
			run, err := runLive(cmd, flags, &rf, csv, slots)
			if err != nil {
				return err
			}
			onsale := tickets.BuildOnsale(run.data, run.sights, run.from, run.to, run.now, nil)
			rows := buildAvailability(run.data, run.sights, tickets.DateRange(run.from, run.to), onsale, slots, party, run.now)
			meta := run.meta(nil)
			if slots {
				for _, s := range run.sights {
					if !s.IsTeamLab() {
						meta.Warnings = append(meta.Warnings, s.ID+": --slots covers teamLab venues only; slot stock for this sight is unknown")
					}
				}
			}
			meta.Party = party
			view := availabilityView{Meta: meta, Results: rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, availabilityKeep...)
			}
			return renderAvailabilityHuman(cmd, rows, meta.Warnings)
		},
	}
	cmd.Flags().StringVar(&rf.date, "date", "", "Single visit date, YYYY-MM-DD (JST)")
	cmd.Flags().StringVar(&rf.from, "from", "", "First visit date, YYYY-MM-DD (JST); defaults to --to")
	cmd.Flags().StringVar(&rf.to, "to", "", "Last visit date, YYYY-MM-DD (JST); defaults to --from")
	cmd.Flags().StringVar(&sightsCSV, "sights", "", "Comma-separated sight IDs (same as positional args); 'teamlab' selects every teamLab venue; default all")
	cmd.Flags().BoolVar(&slots, "slots", false, "Also read teamLab time-slot stock (at most 7 dates)")
	cmd.Flags().StringVar(&partyS, "party", "", "Party size as adults=N,children=N,infants=N; keeps slots that fit everyone, plus slots with unknown stock")
	return cmd
}

// buildAvailability merges calendar days, sale windows and slots into one row
// per sight and date. Slots that already started at now are left out.
func buildAvailability(data map[string]*tickets.SightData, sights []tickets.Sight, dates []tickets.Date, onsale []tickets.OnsaleRow, slots bool, party *tickets.Party, now time.Time) []availabilityRow {
	nowJST := tickets.FormatJST(now)
	byKey := map[string]tickets.OnsaleRow{}
	for _, r := range onsale {
		byKey[r.Sight+"|"+r.VisitDate] = r
	}
	rows := make([]availabilityRow, 0)
	for _, s := range sights {
		sd := data[s.ID]
		if sd == nil {
			continue
		}
		note := tickets.PartyNote(s, party)
		isTeamLab := s.IsTeamLab()
		for _, d := range dates {
			key := d.String()
			row := availabilityRow{Sight: s.ID, NameEN: s.NameEN, NameJA: s.NameJA, Date: key, Status: tickets.DayUnknown, HandoffURL: s.HandoffURL, PartyNote: note}
			day, hasDay := sd.Days[key]
			if hasDay {
				row.Status, row.Reason, row.PriceFrom = day.Status, day.Reason, day.PriceFrom
			}
			os := byKey[s.ID+"|"+key]
			switch {
			case os.State == tickets.StateOpensAt && (!hasDay || day.Status == tickets.DayUnknown):
				row.Status = tickets.DayNotReleased
				when := ptrOr(os.OnSaleJST, "")
				if when == "" && os.OnSaleEarliest != nil {
					when = "between " + *os.OnSaleEarliest + " and " + ptrOr(os.OnSaleLatest, "?") + " (estimated)"
				}
				row.Reason = "not on sale yet; opens " + when + " (see onsale)"
			case !hasDay && isTeamLab && sd.CalendarUntil != nil && sd.CalendarUntil.Before(d):
				row.Status, row.Reason = tickets.DayNotReleased, "beyond the published calendar (ends "+sd.CalendarUntil.String()+"); see onsale"
			case !hasDay && isTeamLab:
				row.Reason = "ticket calendar could not be read in this run"
			case row.Reason == "" && row.Status == tickets.DayUnknown:
				row.Reason = os.Reason
			}
			if row.Reason == "" {
				row.Reason = tickets.DefaultReason(row.Status)
			}
			if slots && isTeamLab {
				daySlots := make([]tickets.Slot, 0)
				read := false
				for _, sl := range sd.Slots {
					if sl.Date != key {
						continue
					}
					read = true
					if sl.StartJST == "" || sl.StartJST >= nowJST {
						daySlots = append(daySlots, sl)
					}
				}
				row.SlotsRead = read
				if read {
					// The store's "few" flag is a day-level label, not a
					// slot count; say so when the slot totals sit next to it.
					if row.Status == tickets.DayFew && row.Reason == tickets.DefaultReason(tickets.DayFew) {
						row.Reason = "official calendar flags this day as few left (store-wide day label, not a slot count)"
					}
					row.Reason += slotStockNote(daySlots)
				}
				if party != nil {
					var n int
					daySlots, n = tickets.ApplyParty(daySlots, party)
					row.SlotsFitParty = &n
				}
				row.Slots = daySlots
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// slotStockNote sums, per product, the stock the slot read reports for the
// upcoming slots of one day. The official calendar label (for example "few")
// is the store's own day-level flag, so the slot totals are shown next to it.
func slotStockNote(slots []tickets.Slot) string {
	type total struct {
		stock, capacity, slots, soldOut int
		capKnown                        bool
	}
	order := make([]string, 0)
	sums := map[string]*total{}
	for _, sl := range slots {
		if sl.Stock == nil {
			continue
		}
		t := sums[sl.Product]
		if t == nil {
			t = &total{capKnown: true}
			sums[sl.Product] = t
			order = append(order, sl.Product)
		}
		t.stock += *sl.Stock
		t.slots++
		if *sl.Stock <= 0 {
			t.soldOut++
		}
		if sl.Capacity == nil {
			t.capKnown = false
		} else {
			t.capacity += *sl.Capacity
		}
	}
	if len(order) == 0 {
		return ""
	}
	parts := make([]string, 0, len(order))
	for _, p := range order {
		t := sums[p]
		name := p
		if name == "" {
			name = "tickets"
		}
		unit := "slots"
		if t.slots == 1 {
			unit = "slot"
		}
		var part string
		if t.capKnown {
			part = fmt.Sprintf("%s %d of %d places open in %d %s", name, t.stock, t.capacity, t.slots, unit)
		} else {
			part = fmt.Sprintf("%s %d places open in %d %s", name, t.stock, t.slots, unit)
		}
		if t.soldOut > 0 {
			part += fmt.Sprintf(" (%d sold out)", t.soldOut)
		}
		parts = append(parts, part)
	}
	return "; slot read: " + strings.Join(parts, ", ")
}

// slotStartRange returns the earliest and latest slot start times. Slots
// arrive grouped by product, so the first and last list items are not the
// day's time range when a venue sells more than one product.
func slotStartRange(starts []string) (first, last string, ok bool) {
	minutes := func(s string) int {
		var h, m int
		if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
			return -1
		}
		return h*60 + m
	}
	firstMin, lastMin := 0, 0
	for _, s := range starts {
		v := minutes(s)
		if v < 0 {
			continue
		}
		if !ok || v < firstMin {
			first, firstMin = s, v
		}
		if !ok || v > lastMin {
			last, lastMin = s, v
		}
		ok = true
	}
	return first, last, ok
}

func renderAvailabilityHuman(cmd *cobra.Command, rows []availabilityRow, warnings []string) error {
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(w, "SIGHT\tDATE\tSTATUS\tFROM ¥\tSLOTS\tREASON")
	for _, r := range rows {
		price := "-"
		if r.PriceFrom != nil {
			price = fmt.Sprintf("%d", *r.PriceFrom)
		}
		slotInfo := "-"
		if r.SlotsRead || r.SlotsFitParty != nil {
			// Only available and few slots count as open. A slot whose stock
			// could not be read is reported separately, never as open.
			open := make([]string, 0, len(r.Slots))
			unknown := 0
			for _, sl := range r.Slots {
				switch sl.Status {
				case tickets.SlotAvailable, tickets.SlotFew:
					open = append(open, sl.Start)
				case tickets.SlotSoldOut:
				default:
					unknown++
				}
			}
			slotInfo = fmt.Sprintf("%d open", len(open))
			if first, last, ok := slotStartRange(open); ok {
				slotInfo += " (" + first + "..." + last + ")"
			}
			if unknown > 0 {
				slotInfo += fmt.Sprintf(", %d unknown", unknown)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Sight, r.Date, r.Status, price, oneLine(slotInfo), oneLine(r.Reason))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	printWarnings(cmd.ErrOrStderr(), warnings)
	return nil
}
