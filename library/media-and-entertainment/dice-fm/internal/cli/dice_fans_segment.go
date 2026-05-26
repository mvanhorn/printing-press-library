// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "fans segment" command: behavioral segmentation over the
// local order + ticket + event store. A fan must match ALL provided filters;
// omitting a filter leaves it open. This file is NOT generated and survives
// `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// fanSegmentRow is one result row returned by fans segment.
type fanSegmentRow struct {
	Email       string  `json:"email"`
	Name        string  `json:"name"`
	EventsCount int     `json:"events_count"`
	TotalSpend  float64 `json:"total_spend"`
	OptedIn     bool    `json:"opted_in"`
}

// segmentFilters holds the parsed flag values for fans segment.
type segmentFilters struct {
	minEvents  int
	ticketType string // case-insensitive substring match on ticketType.name
	tier       string // case-insensitive substring match on priceTier.name
	genre      string // case-insensitive substring match on genres / genreTypes
	eventName  string // case-insensitive substring match on event name
	minQty     int
	optedIn    bool
	fromDate   string // YYYY-MM-DD show-date lower bound
	toDate     string // YYYY-MM-DD show-date upper bound
}

// computeFansSegment filters fans from the local order, ticket, and event store.
// A fan must satisfy ALL provided (non-zero) filter values. Results are sorted
// by total_spend descending.
func computeFansSegment(ctx context.Context, db *sql.DB, f segmentFilters) ([]fanSegmentRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	eligible, dateFiltered, err := eligibleEventsByDate(ctx, db, f.fromDate, f.toDate)
	if err != nil {
		return nil, err
	}

	// Build event metadata index (name + genres) for genre/name filters.
	// When event metadata is absent we still let the order through (events row
	// may not have been synced); genre/name filters simply won't match.
	eventMeta := map[string]storeEvent{}
	if f.genre != "" || f.eventName != "" {
		events, eerr := readEvents(ctx, db)
		if eerr != nil {
			return nil, eerr
		}
		for _, e := range events {
			eventMeta[e.ID] = e
		}
	}

	// Build per-holder ticket index keyed by holder email for type/tier filters.
	// Ticket rows carry no event reference, so this is a global set for the
	// holder's purchased ticket types/tiers. The segment applies the filter as
	// "did this fan ever buy a ticket of this type/tier across any event".
	type ticketInfo struct {
		typeName string
		tierName string
	}
	holderTickets := map[string][]ticketInfo{} // email -> ticket infos
	if f.ticketType != "" || f.tier != "" {
		tickets, terr := readTickets(ctx, db)
		if terr != nil {
			return nil, terr
		}
		for _, t := range tickets {
			email := t.Holder.Email
			if email == "" {
				continue
			}
			holderTickets[email] = append(holderTickets[email], ticketInfo{
				typeName: t.TicketType.Name,
				tierName: t.PriceTier.Name,
			})
		}
	}

	wantGenre := strings.ToLower(f.genre)
	wantEventName := strings.ToLower(f.eventName)
	wantTicketType := strings.ToLower(f.ticketType)
	wantTier := strings.ToLower(f.tier)

	type agg struct {
		name             string
		totalCents       int64
		optedIn          bool
		eventSet         map[string]bool
		maxQty           int  // maximum quantity of any single order
		matchedEventName bool // fan has >=1 order whose event name matched --event-name
		matchedGenre     bool // fan has >=1 order whose event genre matched --genre
	}
	groups := map[string]*agg{}

	for _, o := range orders {
		// An order is at least one ticket even when DICE omits the quantity
		// field; mirror computeReturnsAnomalies/computeCapacity so a 0 quantity
		// counts as 1 in the per-fan max-quantity rollup that drives --min-qty.
		qty := o.Quantity
		if qty <= 0 {
			qty = 1
		}
		// --from/--to is a time-window scope, not a fan qualifier: it bounds the
		// universe of orders considered to the requested show-date window. The
		// other filters (--opted-in, --min-qty, --event-name, --genre) qualify
		// the FAN and never shrink total_spend/events_count to only the matching
		// orders — they are applied as per-fan flags during row building below.
		if dateFiltered && !eligible[o.Event.ID] {
			continue
		}

		email := o.Fan.Email
		if email == "" {
			continue
		}

		// Compute this order's match against the fan-qualifier filters, then OR
		// the result into the fan's running flags. A non-matching order still
		// contributes to the fan's spend and event totals.
		orderMatchesEventName := wantEventName == ""
		if wantEventName != "" {
			name := strings.ToLower(o.Event.Name)
			// Also check store event name in case the order's event name is truncated.
			storeName := ""
			if meta, ok := eventMeta[o.Event.ID]; ok {
				storeName = strings.ToLower(meta.Name)
			}
			orderMatchesEventName = strings.Contains(name, wantEventName) || strings.Contains(storeName, wantEventName)
		}
		orderMatchesGenre := wantGenre == ""
		if wantGenre != "" {
			if meta, ok := eventMeta[o.Event.ID]; ok {
				for _, gr := range meta.Genres {
					if strings.Contains(strings.ToLower(gr), wantGenre) {
						orderMatchesGenre = true
						break
					}
				}
				if !orderMatchesGenre {
					for _, gr := range meta.GenreTypes {
						if strings.Contains(strings.ToLower(gr), wantGenre) {
							orderMatchesGenre = true
							break
						}
					}
				}
			}
		}

		g := groups[email]
		if g == nil {
			g = &agg{eventSet: map[string]bool{}}
			groups[email] = g
		}
		if g.name == "" {
			g.name = joinName(o.Fan.FirstName, o.Fan.LastName)
		}
		if o.Fan.OptInPartners {
			g.optedIn = true
		}
		if orderMatchesEventName {
			g.matchedEventName = true
		}
		if orderMatchesGenre {
			g.matchedGenre = true
		}
		g.totalCents += o.Total
		if o.Event.ID != "" {
			g.eventSet[o.Event.ID] = true
		}
		if qty > g.maxQty {
			g.maxQty = qty
		}
	}

	rows := make([]fanSegmentRow, 0, len(groups))
	for email, g := range groups {
		if f.minEvents > 0 && len(g.eventSet) < f.minEvents {
			continue
		}
		if f.optedIn && !g.optedIn {
			continue
		}
		// --min-qty qualifies a fan when any single order met the threshold
		// (g.maxQty), without shrinking total_spend/events_count to only the
		// qualifying orders.
		if f.minQty > 0 && g.maxQty < f.minQty {
			continue
		}
		// --event-name / --genre qualify a fan when any of their orders matched,
		// without shrinking total_spend/events_count to only the matching orders.
		if wantEventName != "" && !g.matchedEventName {
			continue
		}
		if wantGenre != "" && !g.matchedGenre {
			continue
		}
		// Ticket type / tier filters: check whether this fan has any matching ticket.
		if wantTicketType != "" || wantTier != "" {
			tickets := holderTickets[email]
			matchedType := wantTicketType == ""
			matchedTier := wantTier == ""
			for _, ti := range tickets {
				if !matchedType && strings.Contains(strings.ToLower(ti.typeName), wantTicketType) {
					matchedType = true
				}
				if !matchedTier && strings.Contains(strings.ToLower(ti.tierName), wantTier) {
					matchedTier = true
				}
				if matchedType && matchedTier {
					break
				}
			}
			if !matchedType || !matchedTier {
				continue
			}
		}
		rows = append(rows, fanSegmentRow{
			Email:       email,
			Name:        g.name,
			EventsCount: len(g.eventSet),
			TotalSpend:  round2(float64(g.totalCents) / 100.0),
			OptedIn:     g.optedIn,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalSpend != rows[j].TotalSpend {
			return rows[i].TotalSpend > rows[j].TotalSpend
		}
		return rows[i].Email < rows[j].Email
	})
	return rows, nil
}

func newFansSegmentCmd(flags *rootFlags) *cobra.Command {
	var f segmentFilters
	cmd := &cobra.Command{
		Use:   "segment",
		Short: "Filter fans by purchasing behavior; all provided filters must match",
		Long: "Segment fans from the local order, ticket, and event store. " +
			"A fan must satisfy ALL provided (non-zero) filters. " +
			"The --opted-in/--min-qty/--ticket-type/--tier/--genre/--event-name " +
			"filters qualify the fan (matched on any of their orders) and do not " +
			"reduce the reported total_spend/events_count to only the matching " +
			"orders. --from/--to are different: they scope the order window by " +
			"show date, so spend and event counts reflect only that window. " +
			"Omitting all flags returns every fan with any order. " +
			"Results are sorted by total_spend descending.",
		Example:     "  dice-fm-pp-cli fans segment --min-events 3 --ticket-type VIP --opted-in --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if f.fromDate, err = parseDateFlag("from", f.fromDate); err != nil {
				return err
			}
			if f.toDate, err = parseDateFlag("to", f.toDate); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []fanSegmentRow{}, flags)
			}
			defer s.Close()
			rows, err := computeFansSegment(cmd.Context(), s.DB(), f)
			if err != nil {
				return fmt.Errorf("computing fan segment: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&f.minEvents, "min-events", 0, "Only fans with >= N distinct events purchased (0 = no minimum)")
	cmd.Flags().StringVar(&f.ticketType, "ticket-type", "", "Only fans with a ticket whose ticketType.name contains this string (case-insensitive)")
	cmd.Flags().StringVar(&f.tier, "tier", "", "Only fans with a ticket whose priceTier.name contains this string (case-insensitive)")
	cmd.Flags().StringVar(&f.genre, "genre", "", "Only fans with an order for an event whose genres/genreTypes contain this string (case-insensitive)")
	cmd.Flags().StringVar(&f.eventName, "event-name", "", "Only fans with an order for an event whose name contains this string (case-insensitive)")
	cmd.Flags().IntVar(&f.minQty, "min-qty", 0, "Only fans who placed at least one order with quantity >= N (0 = no minimum)")
	cmd.Flags().BoolVar(&f.optedIn, "opted-in", false, "Only fans with optInPartners == true")
	cmd.Flags().StringVar(&f.fromDate, "from", "", "Only orders for shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&f.toDate, "to", "", "Only orders for shows on or before this date (YYYY-MM-DD, by show date)")
	return cmd
}
