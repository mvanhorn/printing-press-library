// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "fans segment" command: behavioral segmentation over the
// local order + ticket + event store. A fan must match ALL provided filters;
// omitting a filter leaves it open. This file is NOT generated and survives
// `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
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

	// Normalized-axis filters — resolved via entity_crosswalk + tier_attributes.
	// Require normalization to have been run; emit a warning and return no rows
	// when the crosswalk table is empty.
	accessClass  string // match fans with any ticket whose access_class equals this value
	salesStage   string // match fans with any ticket whose sales_stage equals this value
	entryWindow  string // match fans with any ticket whose entry_window_type equals this value
	comp         bool   // match fans with any ticket whose comp_flag is true
	minGroupSize int    // match fans with any ticket whose group_size >= N (N>0 to activate)
}

// ticketAxisAttrs holds all five tier-axis values for a single ticketType.name,
// resolved from entity_crosswalk + tier_attributes, plus the canonical tier name
// from canonical_entity. found=false means the type has no crosswalk row (or
// method='unmatched').
type ticketAxisAttrs struct {
	found         bool
	canonicalName string
	accessClass   string
	salesStage    string
	entryWindow   string
	compFlag      bool
	groupSize     int
}

// loadAllTicketTypeAxes fetches entity_crosswalk + tier_attributes for all
// ticket_type entries and returns a name→ticketAxisAttrs map for O(1) lookup.
func loadAllTicketTypeAxes(ctx context.Context, db *sql.DB) (map[string]ticketAxisAttrs, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
			ec.source_value,
			ec.method,
			COALESCE(ce.canonical_name,    '') AS canonical_name,
			COALESCE(ta.access_class,      '') AS access_class,
			COALESCE(ta.sales_stage,       '') AS sales_stage,
			COALESCE(ta.entry_window_type, '') AS entry_window_type,
			COALESCE(ta.comp_flag,          0) AS comp_flag,
			COALESCE(ta.group_size,         0) AS group_size
		FROM entity_crosswalk ec
		LEFT JOIN tier_attributes ta ON ta.canonical_id = ec.canonical_id
		LEFT JOIN canonical_entity ce
			ON ce.canonical_id = ec.canonical_id
			AND ce.entity_type = ec.entity_type
		WHERE ec.entity_type   = 'ticket_type'
		  AND ec.source_system = 'dice'
	`)
	if err != nil {
		return nil, fmt.Errorf("loading ticket type axes: %w", err)
	}
	defer rows.Close()
	m := map[string]ticketAxisAttrs{}
	for rows.Next() {
		var sourceVal, method, cn, ac, ss, ew string
		var compInt, gs int
		if err := rows.Scan(&sourceVal, &method, &cn, &ac, &ss, &ew, &compInt, &gs); err != nil {
			return nil, fmt.Errorf("scanning ticket axis row: %w", err)
		}
		if method == "unmatched" {
			m[sourceVal] = ticketAxisAttrs{found: false}
		} else {
			m[sourceVal] = ticketAxisAttrs{
				found:         true,
				canonicalName: cn,
				accessClass:   ac,
				salesStage:    ss,
				entryWindow:   ew,
				compFlag:      compInt != 0,
				groupSize:     gs,
			}
		}
	}
	return m, rows.Err()
}

// hasAxisFilters reports whether any normalized-axis filter is active.
func hasAxisFilters(f segmentFilters) bool {
	return f.accessClass != "" || f.salesStage != "" || f.entryWindow != "" || f.comp || f.minGroupSize > 0
}

// tierMatches reports whether a held ticket matches a --tier value (already
// lowercased as wantTier). A match is any of:
//  1. the raw priceTier.name contains wantTier (case-insensitive; back-compat), or
//  2. the ticketType.name resolves via the classifier (axisMap) to a normalized
//     tier whose canonical name contains wantTier, or whose access_class /
//     sales_stage / entry_window_type equals wantTier.
//
// axisMap may be nil (normalization not run); then only the raw path applies.
func tierMatches(wantTier, tierName, typeName string, axisMap map[string]ticketAxisAttrs) bool {
	if wantTier == "" {
		return true
	}
	// Raw priceTier.name substring (back-compat).
	if strings.Contains(strings.ToLower(tierName), wantTier) {
		return true
	}
	if axisMap == nil {
		return false
	}
	attrs, ok := axisMap[typeName]
	if !ok || !attrs.found {
		return false
	}
	// Canonical tier name: free text -> substring.
	if attrs.canonicalName != "" && strings.Contains(strings.ToLower(attrs.canonicalName), wantTier) {
		return true
	}
	// Axis values are enum-like -> equality (case-insensitive against want).
	return strings.ToLower(attrs.accessClass) == wantTier ||
		strings.ToLower(attrs.salesStage) == wantTier ||
		strings.ToLower(attrs.entryWindow) == wantTier
}

// computeFansSegment filters fans from the local order, ticket, and event store.
// A fan must satisfy ALL provided (non-zero) filter values. Results are sorted
// by total_spend descending.
func computeFansSegment(ctx context.Context, db *sql.DB, f segmentFilters) ([]fanSegmentRow, error) {
	return computeFansSegmentWithStderr(ctx, db, f, os.Stderr)
}

// computeFansSegmentWithStderr is the internal implementation with an injectable
// stderr writer for testability of warning emission.
func computeFansSegmentWithStderr(ctx context.Context, db *sql.DB, f segmentFilters, stderr io.Writer) ([]fanSegmentRow, error) {
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

	// Normalized-axis filter path: when any axis filter is requested, check
	// whether the crosswalk has been populated. If not, warn and return no rows —
	// the filter cannot be satisfied. If populated, load all axes at once.
	//
	// --tier ALSO consults the classifier (canonical name + axis values) on top
	// of its raw priceTier.name substring match, so it loads the axisMap too. But
	// --tier must keep working without normalization: an empty crosswalk is not a
	// hard stop for --tier, it just leaves the classifier path inert and falls
	// back to the raw substring match.
	var axisMap map[string]ticketAxisAttrs
	if hasAxisFilters(f) || f.tier != "" {
		var crosswalkCount int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM entity_crosswalk WHERE entity_type = 'ticket_type'`,
		).Scan(&crosswalkCount); err != nil {
			return nil, fmt.Errorf("checking crosswalk for axis filters: %w", err)
		}
		if crosswalkCount == 0 {
			// Hard axis filters cannot be satisfied without normalization.
			if hasAxisFilters(f) {
				fmt.Fprintf(stderr, "warning: normalization not run; --access-class/--sales-stage/--entry-window/--comp/--min-group-size require `normalize` — run it first\n")
				return []fanSegmentRow{}, nil
			}
			// --tier only: fall through with a nil axisMap so the raw substring
			// match still applies.
		} else {
			axisMap, err = loadAllTicketTypeAxes(ctx, db)
			if err != nil {
				return nil, err
			}
		}
	}

	// Build per-holder ticket index keyed by holder email for type/tier filters
	// and for axis filters. Ticket rows carry no event reference, so this is a
	// global set for the holder's purchased ticket types/tiers. The segment
	// applies the filter as "did this fan ever buy a ticket of this type/tier
	// across any event".
	type ticketInfo struct {
		typeName string
		tierName string
	}
	holderTickets := map[string][]ticketInfo{} // email -> ticket infos
	if f.ticketType != "" || f.tier != "" || axisMap != nil {
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
		// --tier matches on EITHER the raw priceTier.name substring (back-compat)
		// OR the classifier: the ticketType.name resolved via axisMap to a
		// normalized tier whose canonical name contains the want value, or whose
		// access_class / sales_stage / entry_window_type equals it (axis values
		// are enum-like, so equality; canonical name is free text, so substring).
		if wantTicketType != "" || wantTier != "" {
			tickets := holderTickets[email]
			matchedType := wantTicketType == ""
			matchedTier := wantTier == ""
			for _, ti := range tickets {
				if !matchedType && strings.Contains(strings.ToLower(ti.typeName), wantTicketType) {
					matchedType = true
				}
				if !matchedTier && tierMatches(wantTier, ti.tierName, ti.typeName, axisMap) {
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
		// Normalized-axis filters: a fan qualifies if ANY of their tickets resolves
		// to the requested axis value. ALL requested axis filters must be satisfied
		// (AND semantics). total_spend/events_count are NOT shrunk.
		if axisMap != nil {
			tickets := holderTickets[email]
			wantAC := f.accessClass != ""
			wantSS := f.salesStage != ""
			wantEW := f.entryWindow != ""
			wantComp := f.comp
			wantGS := f.minGroupSize > 0

			matchedAC := !wantAC
			matchedSS := !wantSS
			matchedEW := !wantEW
			matchedComp := !wantComp
			matchedGS := !wantGS

			for _, ti := range tickets {
				attrs, ok := axisMap[ti.typeName]
				if !ok || !attrs.found {
					continue
				}
				if wantAC && !matchedAC && attrs.accessClass == f.accessClass {
					matchedAC = true
				}
				if wantSS && !matchedSS && attrs.salesStage == f.salesStage {
					matchedSS = true
				}
				if wantEW && !matchedEW && attrs.entryWindow == f.entryWindow {
					matchedEW = true
				}
				if wantComp && !matchedComp && attrs.compFlag {
					matchedComp = true
				}
				if wantGS && !matchedGS && attrs.groupSize >= f.minGroupSize {
					matchedGS = true
				}
				if matchedAC && matchedSS && matchedEW && matchedComp && matchedGS {
					break
				}
			}
			if !matchedAC || !matchedSS || !matchedEW || !matchedComp || !matchedGS {
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
			rows, err := computeFansSegmentWithStderr(cmd.Context(), s.DB(), f, cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("computing fan segment: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&f.minEvents, "min-events", 0, "Only fans with >= N distinct events purchased (0 = no minimum)")
	cmd.Flags().StringVar(&f.ticketType, "ticket-type", "", "Only fans with a ticket whose ticketType.name contains this string (case-insensitive)")
	cmd.Flags().StringVar(&f.tier, "tier", "", "Only fans with a ticket matching this tier (case-insensitive). Matches the raw priceTier.name substring, OR — when normalization has run — the ticket's normalized tier: canonical tier name (substring) or access_class/sales_stage/entry_window value (exact). Falls back to raw priceTier.name when not normalized")
	cmd.Flags().StringVar(&f.genre, "genre", "", "Only fans with an order for an event whose genres/genreTypes contain this string (case-insensitive)")
	cmd.Flags().StringVar(&f.eventName, "event-name", "", "Only fans with an order for an event whose name contains this string (case-insensitive)")
	cmd.Flags().IntVar(&f.minQty, "min-qty", 0, "Only fans who placed at least one order with quantity >= N (0 = no minimum)")
	cmd.Flags().BoolVar(&f.optedIn, "opted-in", false, "Only fans with optInPartners == true")
	cmd.Flags().StringVar(&f.fromDate, "from", "", "Only orders for shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&f.toDate, "to", "", "Only orders for shows on or before this date (YYYY-MM-DD, by show date)")
	// Normalized-axis filters (require `normalize` to have been run first).
	cmd.Flags().StringVar(&f.accessClass, "access-class", "", "Only fans holding a ticket whose normalized access_class equals this value (e.g. vip, ga); requires normalize")
	cmd.Flags().StringVar(&f.salesStage, "sales-stage", "", "Only fans holding a ticket whose normalized sales_stage equals this value (e.g. early_bird); requires normalize")
	cmd.Flags().StringVar(&f.entryWindow, "entry-window", "", "Only fans holding a ticket whose normalized entry_window_type equals this value (deadline|anytime|door); requires normalize")
	cmd.Flags().BoolVar(&f.comp, "comp", false, "Only fans holding at least one comp/guestlist ticket (comp_flag=true); requires normalize")
	cmd.Flags().IntVar(&f.minGroupSize, "min-group-size", 0, "Only fans holding a ticket whose normalized group_size >= N (0 = no minimum); requires normalize")
	return cmd
}
