package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/pocket"
	"github.com/spf13/cobra"
)

func newRestaurantsCmd(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "restaurants", Short: "Search summaries or retrieve one restaurant's full policy"}
	var query, areas, cuisines, date, service string
	var page, pages, limit, party, min, max int
	var instant bool
	search := &cobra.Command{Use: "search", Short: "Search public restaurant summaries with native source filters", Example: "  pocket-concierge-pp-cli " + "restaurants search --query Murase --limit 5", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--query=Murase;--limit=3"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Discover + Japanese identity per page", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if limit < 1 || limit > 50 || page < 1 || page > 1000 || pages < 1 || pages > 3 {
				return nil, pocket.Fail("usage", "--limit must be 1–50, --page 1–1000 and --pages 1–3")
			}
			if len([]rune(query)) > 200 {
				return nil, pocket.Fail("usage", "--query exceeds 200 characters")
			}
			a, e := parseIDs(areas)
			if e != nil {
				return nil, e
			}
			cu, e := parseIDs(cuisines)
			if e != nil {
				return nil, e
			}
			if party < 0 || party > 20 {
				return nil, pocket.Fail("usage", "--party must be 1–20 when provided")
			}
			if cmd.Flags().Changed("party") && party == 0 {
				return nil, pocket.Fail("usage", "--party must be 1–20 when provided")
			}
			if party > 0 && date == "" {
				return nil, pocket.Fail("usage", "--party requires --date to check date-specific availability")
			}
			if date != "" {
				if e = pocket.ValidateDate(date); e != nil {
					return nil, e
				}
			}
			service = strings.ToUpper(service)
			if service != "" && service != "DINNER" && service != "LUNCH" {
				return nil, pocket.Fail("usage", "--service must be LUNCH or DINNER")
			}
			if min < 0 || max < 0 || cmd.Flags().Changed("max-price") && min > max {
				return nil, pocket.Fail("usage", "JPY price bounds must be nonnegative and min-price <= max-price")
			}
			vars := map[string]any{}
			if query != "" {
				vars["keyword"] = query
			}
			if a != nil {
				vars["areaIds"] = a
			}
			if cu != nil {
				vars["cuisines"] = cu
			}
			if date != "" {
				vars["date"] = date
			}
			if party > 0 {
				vars["partySize"] = party
			}
			if cmd.Flags().Changed("min-price") {
				vars["min"] = min
			}
			if cmd.Flags().Changed("max-price") {
				vars["max"] = max
			}
			if cmd.Flags().Changed("instant") {
				vars["instant"] = instant
			}
			if service != "" {
				vars["services"] = []string{service}
			}
			ttl := 15 * time.Minute
			if date != "" {
				ttl = 0
			}
			items := []map[string]any{}
			seen := map[string]bool{}
			var last *pocket.Pagination
			scanned := 0
			used := 0
			for p := page; p < page+pages; p++ {
				vars["pagination"] = map[string]int{"page": p, "limit": limit}
				r, e := c.Search(ctx, f.lang, vars, ttl)
				if e != nil {
					return nil, e
				}
				last = r.Metadata
				used++
				ids := []string{}
				for _, v := range r.Collection {
					ids = append(ids, v.ID)
				}
				jp := map[string]pocket.Venue{}
				if f.lang == "en" {
					jp, e = c.JapaneseNames(ctx, ids, ttl)
					if e != nil {
						return nil, e
					}
				}
				for _, v := range r.Collection {
					scanned++
					if seen[v.ID] {
						continue
					}
					seen[v.ID] = true
					var j *pocket.Venue
					if f.lang == "ja" {
						j = &v
					} else if x, ok := jp[v.ID]; ok {
						j = &x
					}
					items = append(items, pocket.Summary(v, j, f.lang))
				}
				if last.CurrentPage >= last.TotalPages || len(r.Collection) == 0 {
					break
				}
			}
			missingJA := false
			for _, item := range items {
				if item["name_ja"] == (*string)(nil) {
					missingJA = true
				}
			}
			more := last.CurrentPage < last.TotalPages
			var next any
			if more {
				next = last.CurrentPage + 1
			}
			return map[string]any{"items": items, "query": map[string]any{"keyword": query, "area_ids": a, "cuisine_ids": cu, "date": nullable(date), "party": nullableInt(party), "service": nullable(service), "min_price_jpy": specifiedInt(cmd, "min-price", min), "max_price_jpy": specifiedInt(cmd, "max-price", max), "currency": "JPY"}, "pagination": map[string]any{"start_page": page, "last_page": last.CurrentPage, "pages_fetched": used, "per_page": last.LimitValue, "total_count": last.TotalCount, "total_pages": last.TotalPages, "next_page": next, "returned": len(items), "scanned": scanned}, "meta": map[string]any{"partial": page > 1 || more || missingJA, "missing_japanese_names": missingJA, "relevance": "Provider keyword ranking/filtering; inspect name, cuisine and blurb."}}, nil
		})
	}}
	search.Flags().StringVar(&query, "query", "", "Provider keyword query, bounded to 200 characters")
	search.Flags().StringVar(&areas, "area-id", "", "Comma-separated first-party area IDs from filters")
	search.Flags().StringVar(&cuisines, "cuisine-id", "", "Comma-separated first-party cuisine IDs from filters")
	search.Flags().StringVar(&date, "date", "", "Dining date as YYYY-MM-DD in Japan")
	search.Flags().StringVar(&service, "service", "", "Meal service filter: LUNCH or DINNER")
	search.Flags().IntVar(&party, "party", 0, "Guest count 1–20; requires a dining date")
	search.Flags().IntVar(&page, "page", 1, "First source page to retrieve, 1–1000")
	search.Flags().IntVar(&pages, "pages", 1, "Maximum source pages per command, 1–3")
	search.Flags().IntVar(&limit, "limit", 10, "Results per source page, 1–50")
	search.Flags().IntVar(&min, "min-price", 0, "Minimum source price per guest in JPY")
	search.Flags().IntVar(&max, "max-price", 0, "Maximum source price per guest in JPY")
	search.Flags().BoolVar(&instant, "instant", false, "Filter realTimeBooking; false selects reservation-request restaurants")
	var id string
	get := &cobra.Command{Use: "get", Short: "Retrieve restaurant description, explicit policies and service evidence", Example: "  pocket-concierge-pp-cli " + "restaurants get --id 245672", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=245672"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Restaurant detail + Japanese identity", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if e := requireID(id); e != nil {
				return nil, e
			}
			r, e := c.Detail(ctx, id, f.lang)
			return map[string]any{"restaurant": r}, e
		})
	}}
	get.Flags().StringVar(&id, "id", "", "Numeric restaurant source ID from discovery results")
	parent.AddCommand(search, get)
	return parent
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullableInt(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
func rangeBounds(offset, limit, max int) error {
	if offset < 0 || offset > 10000 || limit < 1 || limit > max {
		return pocket.Fail("usage", fmt.Sprintf("--offset must be 0–10000 and --limit 1–%d", max))
	}
	return nil
}
func bounds(n, offset, limit int) (int, int) {
	if offset > n {
		offset = n
	}
	end := offset + limit
	if end > n {
		end = n
	}
	return offset, end
}
func localPagination(n, offset, end int) map[string]any {
	var next any
	if end < n {
		next = end
	}
	return map[string]any{"total_count": n, "offset": offset, "returned": end - offset, "next_offset": next}
}
func newCoursesCmd(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "courses", Short: "Read course identity, JPY units and exact fee evidence"}
	var id string
	var limit, offset int
	cmd := &cobra.Command{Use: "list", Short: "List restaurant courses without inferring inclusive or extra fees", Example: "  pocket-concierge-pp-cli " + "courses list --id 245672", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=245672"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Restaurant courses + Japanese identity", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if e := requireID(id); e != nil {
				return nil, e
			}
			if e := rangeBounds(offset, limit, 50); e != nil {
				return nil, e
			}
			v, jp, e := c.Courses(ctx, id, f.lang)
			if e != nil {
				return nil, e
			}
			start, end := bounds(len(v.Courses), offset, limit)
			items := []map[string]any{}
			for _, x := range v.Courses[start:end] {
				items = append(items, pocket.CourseView(x, pocket.MatchCourse(jp, x.ID), id, f.lang))
			}
			return map[string]any{"restaurant": pocket.Summary(*v, jp, f.lang), "items": items, "restaurant_reservation_terms": v.ReservationTerms, "restaurant_fee_statements": pocket.FeeStatements("restaurant", map[string]*string{"reservation_terms": v.ReservationTerms}), "pagination": localPagination(len(v.Courses), start, end), "meta": map[string]any{"partial": start > 0 || end < len(v.Courses), "price_note": "Per-guest and optional fixed group prices are separate source fields; conflicting fee statements are preserved. No total inferred."}}, nil
		})
	}}
	cmd.Flags().StringVar(&id, "id", "", "Numeric restaurant source ID from discovery results")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum courses to return, from 1 to 50")
	cmd.Flags().IntVar(&offset, "offset", 0, "Zero-based course offset within this source response")
	parent.AddCommand(cmd)
	return parent
}
func newAvailabilityCmd(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "availability", Short: "Read fresh calendars and date/party sessions"}
	var id string
	var cached bool
	var limit, offset int
	dates := &cobra.Command{Use: "dates", Short: "List reservation dates and waitlist dates; party suitability unverified", Example: "  pocket-concierge-pp-cli " + "availability dates --id 245672 --limit 10", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=245672;--limit=5"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Restaurant availability calendar + Japanese identity", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if e := requireID(id); e != nil {
				return nil, e
			}
			if e := rangeBounds(offset, limit, 366); e != nil {
				return nil, e
			}
			ttl := time.Duration(0)
			if cached {
				ttl = 30 * time.Second
			}
			v, jp, e := c.Dates(ctx, id, f.lang, ttl)
			if e != nil {
				return nil, e
			}
			rows := []map[string]any{}
			for _, d := range v.Calendar.ReservationDates {
				rows = append(rows, map[string]any{"date": d, "status": pocket.BookingMode(v.RealTimeBooking), "source_status": "reservation_date", "party_eligible": nil})
			}
			for _, d := range v.Calendar.WaitlistDates {
				rows = append(rows, map[string]any{"date": d, "status": "waitlist", "source_status": "waitlist_date", "party_eligible": nil})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i]["date"].(string) < rows[j]["date"].(string) })
			start, end := bounds(len(rows), offset, limit)
			return map[string]any{"restaurant": pocket.Summary(*v, jp, f.lang), "items": rows[start:end], "pagination": localPagination(len(rows), start, end), "meta": map[string]any{"partial": start > 0 || end < len(rows), "calendar_note": "Calendar dates are restaurant-level signals. Check slots with date/party; a reservation date is not a confirmed reservation."}}, nil
		})
	}}
	dates.Flags().StringVar(&id, "id", "", "Numeric restaurant source ID from discovery results")
	dates.Flags().BoolVar(&cached, "cache-availability", false, "Permit at most 30 seconds of cached calendar inventory")
	dates.Flags().IntVar(&limit, "limit", 100, "Maximum calendar entries to return, 1–366")
	dates.Flags().IntVar(&offset, "offset", 0, "Zero-based calendar entry offset for explicit pagination")
	var sid, date, course string
	var party, slimit, soffset int
	var scached bool
	slots := &cobra.Command{Use: "slots", Short: "Read date/party sessions preserving source IDs and waitlist distinction", Example: "  pocket-concierge-pp-cli " + "availability slots --id 245672 --date 2026-10-05 --party 2", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=245672;--date=2026-10-05;--party=2;--limit=5"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Sessions + Japanese course identity", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if e := requireID(sid); e != nil {
				return nil, e
			}
			if e := pocket.ValidateDate(date); e != nil {
				return nil, e
			}
			if party < 1 || party > 20 {
				return nil, pocket.Fail("usage", "--party must be 1–20")
			}
			if course != "" {
				if e := pocket.ValidateID(course); e != nil {
					return nil, e
				}
			}
			if e := rangeBounds(soffset, slimit, 200); e != nil {
				return nil, e
			}
			ttl := time.Duration(0)
			if scached {
				ttl = 30 * time.Second
			}
			v, jp, raw, e := c.Slots(ctx, sid, date, f.lang, ttl)
			if e != nil {
				return nil, e
			}
			rows := []map[string]any{}
			unknown := 0
			for _, s := range raw {
				ok, e := pocket.PartyEligible(s, party)
				if e != nil {
					return nil, e
				}
				view, e := pocket.SlotView(s, *v, pocket.MatchCourse(jp, s.CourseID()), party, f.lang)
				if e != nil {
					return nil, e
				}
				if !ok || course != "" && s.Course.ID != course {
					continue
				}
				if s.MinPartySize == nil || s.MaxPartySize == nil {
					unknown++
				}
				rows = append(rows, view)
			}
			start, end := bounds(len(rows), soffset, slimit)
			return map[string]any{"restaurant": pocket.Summary(*v, jp, f.lang), "date": date, "party": party, "course_filter": nullable(course), "items": rows[start:end], "pagination": localPagination(len(rows), start, end), "meta": map[string]any{"partial": start > 0 || end < len(rows) || unknown > 0, "source_sessions": len(raw), "unknown_party_bounds": unknown, "note": "Availability options may change before booking; waitlists and reservation requests are not confirmations."}}, nil
		})
	}}
	slots.Flags().StringVar(&sid, "id", "", "Numeric restaurant source ID from discovery results")
	slots.Flags().StringVar(&date, "date", "", "Required dining date in YYYY-MM-DD, Japan time")
	slots.Flags().IntVar(&party, "party", 2, "Guest count for session suitability, 1–20")
	slots.Flags().StringVar(&course, "course-id", "", "Optional course source ID to filter sessions")
	slots.Flags().BoolVar(&scached, "cache-availability", false, "Permit at most 30 seconds of cached session inventory")
	slots.Flags().IntVar(&slimit, "limit", 50, "Maximum matching sessions to return, 1–200")
	slots.Flags().IntVar(&soffset, "offset", 0, "Zero-based matching-session offset for explicit pagination")
	parent.AddCommand(dates, slots)
	return parent
}
func newBookingCmd(f *rootFlags) *cobra.Command {
	parent := &cobra.Command{Use: "booking", Short: "Produce a canonical handoff without booking or launching a browser"}
	var id, course, session, date string
	var party int
	cmd := &cobra.Command{Use: "handoff", Short: "Validate selected source identities and return the canonical restaurant page", Example: "  pocket-concierge-pp-cli " + "booking handoff --id 245672 --course-id 182402", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--id=245672;--course-id=182402"}, Args: noArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd, f, args, "Restaurant/course ownership; optional fresh session validation", func(ctx context.Context, c *pocket.Client) (map[string]any, error) {
			if e := requireID(id); e != nil {
				return nil, e
			}
			if course != "" {
				if e := pocket.ValidateID(course); e != nil {
					return nil, e
				}
			}
			if session != "" {
				if e := pocket.ValidateID(session); e != nil {
					return nil, e
				}
				if date == "" {
					return nil, pocket.Fail("usage", "--session-id requires --date")
				}
			}
			if date != "" {
				if e := pocket.ValidateDate(date); e != nil {
					return nil, e
				}
			}
			if party < 1 || party > 20 {
				return nil, pocket.Fail("usage", "--party must be 1–20")
			}
			// Fresh course ownership and terms; no inferred reservation URL arguments.
			c.Refresh = true
			v, jp, e := c.Courses(ctx, id, f.lang)
			if e != nil {
				return nil, e
			}
			var selectedCourse any
			var selectedSession any
			var status any = pocket.BookingMode(v.RealTimeBooking)
			if course != "" {
				x := pocket.MatchCourse(v, course)
				if x == nil {
					return nil, pocket.Fail("not_found", "course does not belong to this public restaurant")
				}
				selectedCourse = pocket.CourseView(*x, pocket.MatchCourse(jp, course), id, f.lang)
			}
			if session != "" {
				sv, sjp, raw, e := c.Slots(ctx, id, date, f.lang, 0)
				if e != nil {
					return nil, e
				}
				var found *pocket.Slot
				for i := range raw {
					if raw[i].ID != nil && *raw[i].ID == session {
						found = &raw[i]
						break
					}
				}
				if found == nil {
					return nil, pocket.Fail("not_found", "session is absent on the requested date; refresh availability slots")
				}
				if course != "" && (found.Course == nil || found.Course.ID != course) {
					return nil, pocket.Fail("not_found", "session does not belong to the selected course")
				}
				ok, e := pocket.PartyEligible(*found, party)
				if e != nil {
					return nil, e
				}
				if !ok {
					return nil, pocket.Fail("not_found", "session does not support the requested party size")
				}
				selectedSession, e = pocket.SlotView(*found, *sv, pocket.MatchCourse(sjp, found.CourseID()), party, f.lang)
				if e != nil {
					return nil, e
				}
				status = selectedSession.(map[string]any)["status"]
				selectedCourse = selectedSession.(map[string]any)["course"]
			}
			return map[string]any{"restaurant": pocket.Summary(*v, jp, f.lang), "course": selectedCourse, "session": selectedSession, "requested_date": nullable(date), "requested_party": party, "booking_mode": status, "url": pocket.URL(id, f.lang), "reservation_created": false, "selection_prepopulated": false, "meta": map[string]any{"partial": selectedSession != nil && selectedSession.(map[string]any)["party_eligible"] == nil}, "restaurant_reservation_terms": v.ReservationTerms, "note": "Canonical restaurant page only. Select/recheck the course, date and party on Pocket Concierge; no reservation, payment or account change performed."}, nil
		})
	}}
	cmd.Flags().StringVar(&id, "id", "", "Numeric restaurant source ID from discovery results")
	cmd.Flags().StringVar(&course, "course-id", "", "Optional course ID; ownership verified against this restaurant")
	cmd.Flags().StringVar(&session, "session-id", "", "Optional reservable session ID; requires its source date")
	cmd.Flags().StringVar(&date, "date", "", "Dining date in YYYY-MM-DD; required with session ID")
	cmd.Flags().IntVar(&party, "party", 2, "Guest count to verify for selected session, 1–20")
	parent.AddCommand(cmd)
	return parent
}

func specifiedInt(cmd *cobra.Command, flag string, n int) any {
	if cmd.Flags().Changed(flag) {
		return n
	}
	return nil
}
