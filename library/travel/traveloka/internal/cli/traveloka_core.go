// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, f *rootFlags) {
		addNovelCommandIfAbsent(root, newTravelokaResolveCmd(f))
		for _, name := range []string{"flights", "hotels", "quotes"} {
			p, _, e := root.Find([]string{name})
			if e != nil || p == root {
				continue
			}
			switch name {
			case "flights":
				p.AddCommand(newTravelokaFlightSearchCmd(f), newTravelokaInspectCmd(f))
			case "hotels":
				p.AddCommand(newTravelokaHotelCmd(f, false), newTravelokaHotelCmd(f, true))
			case "quotes":
				p.AddCommand(newTravelokaCompareCmd(f))
			}
		}
	})
}
func newTravelokaResolveCmd(f *rootFlags) *cobra.Command {
	var query, kind, dbPath string
	var limit int
	cmd := &cobra.Command{Use: "resolve", Short: "Resolve ranked source cities, airports and hotel properties", Example: "  traveloka-pp-cli resolve --query Singapore --kind airport --limit 5 --agent", Annotations: travelokaAnnotations("live", "--query=Singapore;--kind=airport;--limit=3"), RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "resolve Traveloka locations")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named flags; this command does not accept positional arguments"})
		}
		if e := travelokaMode(f, "live"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if strings.TrimSpace(query) == "" || len(strings.TrimSpace(query)) > 200 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--query must contain 1 to 200 characters of city, airport or property text"})
		}
		if kind != "airport" && kind != "city" && kind != "property" && kind != "all" {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--kind must be airport, city, property or all"})
		}
		if e := travelokaBounds(limit, 50); e != nil {
			return travelokaFail(cmd, f, e)
		}
		shop, e := travelokaShopper(cmd)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		src, e := newTravelokaClient(cmd, f)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		view, e := src.Resolve(ctx, query, kind, shop, limit)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		db, e := travelokaDB(ctx, dbPath)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		defer db.Close()
		for _, loc := range view.Locations {
			b, e := json.Marshal(loc)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e = db.Upsert("locations", loc.Namespace+":"+loc.Type+":"+loc.ID, b); e != nil {
				return travelokaFail(cmd, f, e)
			}
		}
		return f.printJSON(cmd, view)
	}}
	cmd.Flags().StringVar(&query, "query", "", "City, airport or property text to resolve through ranked source matches")
	cmd.Flags().StringVar(&kind, "kind", "all", "Location kind to return: airport, city, property or all")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum ranked matching locations to return (1 to 50)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite snapshot and location store path")
	return cmd
}
func flightQueryFlags(cmd *cobra.Command, q *traveloka.Query) {
	cmd.Flags().StringVar(&q.Origin, "origin", "", "Three-letter origin airport code returned by resolve")
	cmd.Flags().StringVar(&q.Destination, "destination", "", "Three-letter destination airport code returned by resolve")
	cmd.Flags().StringVar(&q.Depart, "depart", "", "Departure calendar date in YYYY-MM-DD format")
	cmd.Flags().StringVar(&q.ReturnDate, "return-date", "", "Optional return date; retrieves both journeys and the combined source total")
	cmd.Flags().StringVar(&q.Cabin, "cabin", "ECONOMY", "Requested source cabin: ECONOMY, PREMIUM_ECONOMY, BUSINESS or FIRST")
	cmd.Flags().IntVar(&q.Adults, "adults", 1, "Number of adults in the dated flight search")
	cmd.Flags().IntVar(&q.Children, "children", 0, "Number of children in the source passenger category")
	cmd.Flags().IntVar(&q.Infants, "infants", 0, "Number of infants in the source passenger category")
	cmd.Flags().IntVar(&q.Limit, "limit", 3, "Maximum source-priced itinerary combinations to return (1 to 20)")
	cmd.Flags().IntVar(&q.MaxCandidates, "max-candidates", 3, "Maximum outbound candidates to price independently of output limit (1 to 10)")
}
func fillShopper(cmd *cobra.Command, q *traveloka.Query) error {
	s, e := travelokaShopper(cmd)
	if e != nil {
		return e
	}
	q.Market, q.Locale, q.Currency = s.Market, s.Locale, s.Currency
	q.ChildAges = []int{}
	return nil
}
func newTravelokaFlightSearchCmd(f *rootFlags) *cobra.Command {
	q := traveloka.Query{Kind: "flights"}
	var dbPath, file string
	cmd := &cobra.Command{Use: "search", Short: "Retrieve complete one-way or return itineraries and source-confirmed totals", Long: "Search explicit flight dates and passengers. Results preserve both return journeys, local dates/UTC offsets, operating airlines and exposed baggage/fare policies. Pricing covers a bounded set of candidates; it is indicative and may change at booking.", Example: "  traveloka-pp-cli flights search --origin SIN --destination CGK --depart 2026-11-20 --return-date 2026-11-27 --adults 1 --limit 3 --agent", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false", "mcp:local-write": "true", "pp:happy-args": "--origin=SIN;--destination=CGK;--depart=2026-11-20;--limit=1;--max-candidates=1"}, RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "search dated Traveloka flights")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named flags; this command does not accept positional arguments"})
		}
		if e := travelokaMode(f, "live"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e := fillShopper(cmd, &q); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e := travelokaBounds(q.Limit, 20); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if q.MaxCandidates < 1 || q.MaxCandidates > 10 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--max-candidates must be between 1 and 10 (CLI pricing bound)"})
		}
		if e := traveloka.ValidateQuery(q); e != nil {
			return travelokaFail(cmd, f, e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		src, e := newTravelokaClient(cmd, f)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		view, e := src.SearchFlights(ctx, q)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = travelokaSave(ctx, cmd, f, view, dbPath, file); e != nil {
			return travelokaFail(cmd, f, e)
		}
		// Machine snapshots carry polling-completion warnings in their warnings
		// array. Emit prose only for human output; fetch failures still warn.
		if wantsHumanTable(cmd.OutOrStdout(), f) {
			for _, w := range view.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
		} else if failed, ok := view.Coverage["prefetch_failed"].(int); ok && failed > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d source prefetches failed; prices cover %d successful offers; inspect coverage.prefetch_failures\n", failed, len(view.Offers))
		}
		return f.printJSON(cmd, view)
	}}
	flightQueryFlags(cmd, &q)
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite history store path for normalized public flight snapshots")
	cmd.Flags().StringVar(&file, "save-snapshot", "", "Optional public JSON snapshot path for explicit later comparisons")
	return cmd
}
func hotelQueryFlags(cmd *cobra.Command, q *traveloka.Query, ages *string) {
	cmd.Flags().StringVar(&q.PropertyName, "name", "", "Source destination/property display name; resolved from the source when omitted")
	cmd.Flags().StringVar(&q.CheckIn, "check-in", "", "Check-in calendar date in YYYY-MM-DD format")
	cmd.Flags().StringVar(&q.CheckOut, "check-out", "", "Check-out calendar date in YYYY-MM-DD format")
	cmd.Flags().IntVar(&q.Adults, "adults", 2, "Total adults requested across the hotel rooms")
	cmd.Flags().IntVar(&q.Children, "children", 0, "Total children requested with one explicit age per child")
	cmd.Flags().StringVar(ages, "child-ages", "", "Comma-separated child ages in source order, such as 8,5")
	cmd.Flags().IntVar(&q.Rooms, "rooms", 1, "Number of hotel rooms requested for the party")
	cmd.Flags().IntVar(&q.Limit, "limit", 10, "Maximum property or room-rate offers to return (1 to 50)")
}
func parseTravelokaAges(value string, children int) ([]int, error) {
	a := []int{}
	if value != "" {
		for _, v := range strings.Split(value, ",") {
			n, e := strconv.Atoi(strings.TrimSpace(v))
			if e != nil || n < 0 {
				return nil, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--child-ages must contain nonnegative integer ages"}
			}
			a = append(a, n)
		}
	}
	if len(a) != children {
		return nil, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--child-ages must contain exactly one age per --children"}
	}
	return a, nil
}
func newTravelokaHotelCmd(f *rootFlags, rooms bool) *cobra.Command {
	q := traveloka.Query{Kind: "hotels"}
	name := "search"
	short := "Retrieve dated hotel catalog offers with exact stay and nightly price bases"
	fixture := "--geo-id=10000045;--check-in=2027-01-06;--check-out=2027-01-08;--limit=2"
	var ages, dbPath, file string
	if rooms {
		q.Kind = "rooms"
		name = "rooms"
		short = "Inspect dated room/rate plans, occupancy, meals and cancellation/payment terms"
		fixture = "--property-id=9000000001714;--check-in=2027-01-06;--check-out=2027-01-08;--limit=2"
	}
	cmd := &cobra.Command{Use: "rooms", Short: "Retrieve dated hotel offers with explicit stays and party occupancy", Long: short + ". Child ages and room counts remain explicit. Mismatched occupancy alternatives are labelled. Returned stay totals are preserved separately from rounded per-room-per-night prices; missing policies remain unknown.", Annotations: travelokaAnnotations("live", fixture), RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "retrieve dated Traveloka hotel offers")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named flags; this command does not accept positional arguments"})
		}
		if e := travelokaMode(f, "live"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e := fillShopper(cmd, &q); e != nil {
			return travelokaFail(cmd, f, e)
		}
		var e error
		q.ChildAges, e = parseTravelokaAges(ages, q.Children)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = travelokaBounds(q.Limit, 50); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = traveloka.ValidateQuery(q); e != nil {
			return travelokaFail(cmd, f, e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		src, e := newTravelokaClient(cmd, f)
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		var view *traveloka.Snapshot
		if rooms {
			view, e = src.HotelRooms(ctx, q)
		} else {
			view, e = src.SearchHotels(ctx, q)
		}
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e = travelokaSave(ctx, cmd, f, view, dbPath, file); e != nil {
			return travelokaFail(cmd, f, e)
		}
		return f.printJSON(cmd, view)
	}}
	cmd.Use = name
	cmd.Short = short
	hotelQueryFlags(cmd, &q, &ages)
	if rooms {
		cmd.Flags().StringVar(&q.PropertyID, "property-id", "", "Traveloka source property ID returned by resolve or hotels search")
		cmd.Example = "  traveloka-pp-cli hotels rooms --property-id 9000000001714 --check-in 2027-01-06 --check-out 2027-01-08 --adults 2 --rooms 1 --limit 3 --agent"
	} else {
		cmd.Flags().StringVar(&q.GeoID, "geo-id", "", "Traveloka hotel geographic destination ID returned by resolve")
		cmd.Flags().IntVar(&q.Offset, "offset", 0, "Source catalog offset for skip/top pagination")
		cmd.Example = "  traveloka-pp-cli hotels search --geo-id 10000045 --check-in 2027-01-06 --check-out 2027-01-08 --adults 3 --rooms 2 --children 2 --child-ages 8,5 --limit 3 --agent"
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite history store path for normalized public hotel snapshots")
	cmd.Flags().StringVar(&file, "save-snapshot", "", "Optional public JSON snapshot path for explicit later comparisons")
	return cmd
}

// pp:data-source local
func newTravelokaInspectCmd(f *rootFlags) *cobra.Command {
	var file, id, offerID, dbPath string
	cmd := &cobra.Command{Use: "inspect", Short: "Inspect a source-backed flight offer from a saved retrieval snapshot", Example: "  traveloka-pp-cli flights inspect --snapshot /private/tmp/traveloka-flight.json --agent", Annotations: travelokaAnnotations("local", "--data-source=local"), RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "inspect retrieved flight snapshot")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named flags; this command does not accept positional arguments"})
		}
		if e := travelokaMode(f, "local"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		s, e := travelokaLoad(ctx, dbPath, file, id, "flights")
		if e != nil {
			return travelokaFail(cmd, f, e)
		}
		if s == nil {
			return f.printJSON(cmd, map[string]any{"status": "empty_local_cache", "offers": []traveloka.Offer{}, "hint": "Run flights search to retrieve and save source-backed offers"})
		}
		if s.Kind != "flights" {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--snapshot must contain a flight retrieval"})
		}
		offers := []traveloka.Offer{}
		for _, o := range s.Offers {
			if offerID == "" || o.ID == offerID {
				offers = append(offers, o)
			}
		}
		if offerID != "" && len(offers) == 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "NOT_FOUND", Message: "--offer-id was not returned in the selected retrieval snapshot"})
		}
		return f.printJSON(cmd, map[string]any{"status": "saved_snapshot", "retrieved_at": s.RetrievedAt, "query": s.Query, "offers": offers, "indicative": true, "freshness": "saved_snapshot"})
	}}
	cmd.Annotations["mcp:read-only"] = "true"
	cmd.Flags().StringVar(&file, "snapshot", "", "Optional saved public flight JSON file; otherwise use SQLite history")
	cmd.Flags().StringVar(&id, "snapshot-id", "", "Optional stored snapshot ID; omitted selects the latest flight retrieval")
	cmd.Flags().StringVar(&offerID, "offer-id", "", "Optional exact source offer ID within the selected snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read-only SQLite snapshot history path")
	return cmd
}

// pp:data-source local
func newTravelokaCompareCmd(f *rootFlags) *cobra.Command {
	var files []string
	var dbPath string
	var limit int
	cmd := &cobra.Command{Use: "compare", Short: "Compare retrieved offers only under matching shopper and travel context", Example: "  traveloka-pp-cli quotes compare --snapshots /private/tmp/traveloka-flight.json --agent", Annotations: travelokaAnnotations("local", "--data-source=local"), RunE: func(cmd *cobra.Command, args []string) error {
		if travelokaBareHelp(cmd, args) {
			return cmd.Help()
		}
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "compare matching retrieved offers")
		}
		if len(args) > 0 {
			return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Use named flags; this command does not accept positional arguments"})
		}
		if e := travelokaMode(f, "local"); e != nil {
			return travelokaFail(cmd, f, e)
		}
		if e := travelokaBounds(limit, 100); e != nil {
			return travelokaFail(cmd, f, e)
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		views := []*traveloka.Snapshot{}
		if len(files) == 0 {
			for _, kind := range []string{"flights", "rooms", "hotels"} {
				s, e := travelokaLoad(ctx, dbPath, "", "", kind)
				if e != nil {
					return travelokaFail(cmd, f, e)
				}
				if s != nil {
					views = append(views, s)
					break
				}
			}
		} else {
			if len(files) > 8 {
				return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "--snapshots accepts at most 8 files (CLI input bound)"})
			}
			for _, p := range files {
				s, e := traveloka.ReadSnapshotFile(p)
				if e != nil {
					return travelokaFail(cmd, f, e)
				}
				views = append(views, s)
			}
		}
		offers := []map[string]any{}
		retrievals := []map[string]any{}
		if len(views) == 0 {
			return f.printJSON(cmd, map[string]any{"status": "empty_local_cache", "offers": offers, "hint": "Run a flight or hotel search to save comparable source offers"})
		}
		basis := "party_stay_total"
		if views[0].Kind == "flights" {
			basis = "party_trip_total"
		}
		for _, s := range views {
			if !views[0].Query.SameContext(s.Query) {
				return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Snapshots have different market, currency, dates, party or product context"})
			}
			retrievals = append(retrievals, map[string]any{"id": s.ID, "retrieved_at": s.RetrievedAt, "status": s.Status, "search_complete": s.SearchComplete, "coverage": s.Coverage})
			for _, o := range s.Offers {
				price, e := travelokacompare.ExactTotal(o.Price.Total, s.Query.Currency)
				if e != nil {
					return travelokaFail(cmd, f, e)
				}
				unit, _ := o.Details["price_basis"].(string)
				if unit != "" && unit != basis {
					return travelokaFail(cmd, f, &traveloka.APIError{Code: "INVALID_INPUT", Message: "Snapshot price basis differs from the expected party trip/stay total"})
				}
				status := "known_source_total"
				if unit == "" {
					status = "unknown_price_basis"
				} else if price == nil {
					status = "unknown_total"
				}
				offers = append(offers, map[string]any{"offer": o, "snapshot_id": s.ID, "retrieved_at": s.RetrievedAt, "comparison_status": status})
			}
		}
		sort.SliceStable(offers, func(i, j int) bool {
			a, b := offers[i]["offer"].(traveloka.Offer), offers[j]["offer"].(traveloka.Offer)
			var pa, pb *big.Rat
			if offers[i]["comparison_status"] == "known_source_total" {
				pa = comparisonTotal(a.Price.Total)
			}
			if offers[j]["comparison_status"] == "known_source_total" {
				pb = comparisonTotal(b.Price.Total)
			}
			if pa == nil {
				return false
			}
			if pb == nil {
				return true
			}
			return pa.Cmp(pb) < 0
		})
		scanned := len(offers)
		if len(offers) > limit {
			offers = offers[:limit]
		}
		return f.printJSON(cmd, map[string]any{"status": "saved_snapshot_comparison", "query": views[0].Query, "offers": offers, "retrievals": retrievals, "snapshot_count": len(views), "freshness": "saved_snapshot", "indicative": true, "price_basis": basis, "ordering": "known_source_total_ascending", "scanned_offers": scanned, "truncated": scanned > limit, "note": "Source total/per-person/per-night prices are separate; occupancy alternatives remain labelled. No conversions or inferred totals."})
	}}
	cmd.Annotations["mcp:read-only"] = "true"
	cmd.Flags().StringSliceVar(&files, "snapshots", nil, "Comma-separated public JSON snapshots from matching travel context")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read-only SQLite snapshot history when no files are provided")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum retrieved offers to include in this comparison (1 to 100)")
	return cmd
}

// comparisonTotal orders only exact source totals with explicitly known units.
func comparisonTotal(m *traveloka.Money) *big.Rat {
	if m == nil {
		return nil
	}
	value, _ := travelokacompare.ExactTotal(m, m.Currency)
	return value
}
