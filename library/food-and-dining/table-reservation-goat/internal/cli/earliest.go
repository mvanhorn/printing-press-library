package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.

// pp:client-call — `earliest` calls OpenTable and Tock clients per venue
// through `internal/source/opentable` and `internal/source/tock`. Dogfood's
// reimplementation_check sibling-import regex doesn't match multi-segment
// `internal/source/...` paths. Documented carve-out per AGENTS.md.

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

type earliestRow struct {
	Venue     string  `json:"venue"`
	Network   string  `json:"network"`
	SlotAt    string  `json:"slot_at,omitempty"`
	Available bool    `json:"available"`
	Reason    string  `json:"reason,omitempty"`
	URL       string  `json:"url,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`

	// BookableTimes lists every confirmed-open (date, time) pair found in
	// the search window for the requested party size. Empty when no slots
	// fit; one entry when only one slot fits; many entries when the venue
	// has broad availability. Format: "YYYY-MM-DDTHH:MM".
	BookableTimes []string `json:"bookable_times,omitempty"`

	// CachedAt / Stale / Source surface OT stale-cache-fallback metadata.
	// `Source: "cache_fallback"` means this row came from disk cache after
	// the live network path was blocked by Akamai. `Stale` indicates the
	// cache entry is past TTL. All zero/empty on fresh fetches.
	CachedAt string `json:"cached_at,omitempty"`
	Stale    bool   `json:"stale,omitempty"`
	Source   string `json:"source,omitempty"`
}

type earliestResponse struct {
	Venues    []string      `json:"venues"`
	Party     int           `json:"party"`
	Within    int           `json:"within_days"`
	Results   []earliestRow `json:"results"`
	QueriedAt string        `json:"queried_at"`
}

// newEarliestCmd computes "soonest open slot per venue across both networks"
// for a comma-separated list of restaurants. The crucial cross-network
// affordance: each venue may live on either OpenTable, Tock, or both —
// the command resolves the network heuristically (or via explicit
// network:slug prefix) and queries the right source.
func newEarliestCmd(flags *rootFlags) *cobra.Command {
	var (
		party   int
		within  string
		date    string
		tonight bool
		noCache bool
	)
	cmd := &cobra.Command{
		Use:   "earliest <slug1,slug2,...>",
		Short: "Soonest open slot per venue across OpenTable and Tock",
		Long: "Across a comma-separated list of restaurant slugs, return the " +
			"earliest open slot per venue within `--within N days`. Slugs may be " +
			"network-prefixed (`opentable:le-bernardin`, `tock:alinea`) for " +
			"explicit routing, otherwise both networks are tried. Use `--tonight` " +
			"as shorthand for `--date <today> --within 1d`.\n\n" +
			"OpenTable availability is cached on disk for 3 minutes by default; " +
			"pass `--no-cache` (or set `TRG_OT_NO_CACHE=1`) to force a fresh fetch. " +
			"To route OT traffic through a personal proxy or Tor SOCKS5, set " +
			"`HTTPS_PROXY`. Other env knobs: `TRG_OT_CACHE_TTL`, `TRG_OT_THROTTLE_RATE`.",
		Example: "  table-reservation-goat-pp-cli earliest 'canlis,spinasse,altura' --party 6 --tonight --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			venues := splitCSV(args[0])
			if len(venues) == 0 {
				return fmt.Errorf("provide a comma-separated list of restaurant slugs")
			}
			// `--tonight` is shorthand for "today only." Mutually exclusive
			// with `--date` and overrides `--within`.
			if tonight {
				if date != "" {
					return fmt.Errorf("--tonight and --date are mutually exclusive")
				}
				date = time.Now().Format("2006-01-02")
				within = "1d"
			}
			withinDays := parseDays(within)
			if withinDays == 0 {
				withinDays = 14
			}
			if dryRunOK(flags) {
				rows := make([]earliestRow, 0, len(venues))
				for _, v := range venues {
					rows = append(rows, earliestRow{Venue: v, Network: "opentable", Available: false, Reason: "dry-run"})
				}
				return printJSONFiltered(cmd.OutOrStdout(), earliestResponse{
					Venues: venues, Party: party, Within: withinDays, Results: rows,
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			startDate := date
			if startDate == "" {
				startDate = time.Now().Format("2006-01-02")
			}
			ctx := cmd.Context()
			rows := make([]earliestRow, 0, len(venues))
			for _, v := range venues {
				row := resolveEarliestForVenue(ctx, session, v, party, startDate, withinDays, noCache)
				rows = append(rows, row)
			}
			// Available rows first, then alphabetical
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Available != rows[j].Available {
					return rows[i].Available
				}
				if rows[i].Available && rows[j].Available {
					return rows[i].SlotAt < rows[j].SlotAt
				}
				return rows[i].Venue < rows[j].Venue
			})
			return printJSONFiltered(cmd.OutOrStdout(), earliestResponse{
				Venues: venues, Party: party, Within: withinDays, Results: rows,
				QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}, flags)
		},
	}
	cmd.Flags().IntVar(&party, "party", 2, "Party size")
	cmd.Flags().StringVar(&within, "within", "14d", "Search horizon (e.g., '14d', '7d', '30d' or a bare integer of days)")
	cmd.Flags().StringVar(&date, "date", "", "Start date YYYY-MM-DD (defaults to today)")
	cmd.Flags().BoolVar(&tonight, "tonight", false, "Shorthand for --date <today> --within 1d. Mutually exclusive with --date.")
	cmd.Flags().BoolVar(&noCache, "no-cache", os.Getenv("TRG_OT_NO_CACHE") == "1", "Bypass the OT availability cache and force a fresh network fetch (env: TRG_OT_NO_CACHE=1).")
	return cmd
}

// parseDays accepts "14d", "14", "7d" and returns days as int. "" returns 0.
func parseDays(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "d")
	s = strings.TrimSuffix(s, "D")
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseNetworkSlug(input string) (network, slug string) {
	if i := strings.Index(input, ":"); i > 0 {
		net := strings.ToLower(input[:i])
		if net == "opentable" || net == "tock" {
			return net, input[i+1:]
		}
	}
	return "", input
}

func resolveEarliestForVenue(ctx context.Context, s *auth.Session, venue string, party int, date string, within int, noCache bool) earliestRow {
	network, slug := parseNetworkSlug(venue)
	row := earliestRow{Venue: venue}

	tryOT := network == "" || network == "opentable"
	tryTock := network == "" || network == "tock"

	// Try Tock first because it has working availability via SSR
	// `calendar.offerings`. Many venues (Canlis, Alinea, Atomix) exist on
	// both networks; preferring Tock means the user gets a real
	// `Available=true|false` answer rather than the OT-side honest no-op.
	if tryTock {
		// Tock's runtime availability XHR is POST /api/consumer/calendar/full/v2.
		// One call returns ~60 days of slot data including availableTickets,
		// minPurchaseSize, and maxPurchaseSize — exactly the per-(date, party,
		// time) sold-out state we need. Filter client-side to the requested
		// window and party.
		c, err := tock.New(s)
		if err == nil {
			cal, calErr := c.Calendar(ctx, slug)
			if calErr == nil && cal != nil {
				row.Network = "tock"
				row.URL = tock.Origin + "/" + slug
				start, perr := time.Parse("2006-01-02", date)
				if perr != nil {
					start = time.Now()
				}
				dateFrom := start.Format("2006-01-02")
				dateTo := start.AddDate(0, 0, within-1).Format("2006-01-02")
				seen := map[string]bool{}
				bookable := []string{}
				for _, sl := range cal.Slots {
					if sl.Date < dateFrom || sl.Date > dateTo {
						continue
					}
					if sl.MinPurchaseSize > 0 && int32(party) < sl.MinPurchaseSize {
						continue
					}
					if sl.MaxPurchaseSize > 0 && int32(party) > sl.MaxPurchaseSize {
						continue
					}
					if sl.AvailableTickets < int32(party) {
						continue
					}
					ts := sl.Date + "T" + sl.Time
					// Dedupe: a single (date, time) may appear in multiple
					// TicketGroup buckets (one per ticket type / seating area).
					// Users want the times, not the bucket count.
					if seen[ts] {
						continue
					}
					seen[ts] = true
					bookable = append(bookable, ts)
				}
				sort.Strings(bookable)
				if len(bookable) > 0 {
					row.Available = true
					row.SlotAt = bookable[0]
					row.BookableTimes = bookable
					row.Reason = fmt.Sprintf("tock %s: %d open slot(s) for party=%d in %d-day window; earliest %s",
						slug, len(bookable), party, within, bookable[0])
				} else {
					row.Available = false
					row.Reason = fmt.Sprintf("tock %s: no open slots for party=%d between %s and %s (calendar reports %d total slots; none match party-size + availability)",
						slug, party, dateFrom, dateTo, len(cal.Slots))
				}
				return row
			}
			if calErr != nil {
				row.Reason = fmt.Sprintf("tock %s: %v", slug, calErr)
				// Fall through to OT branch.
			}
		}
	}
	if tryOT {
		c, err := opentable.New(s)
		if err == nil {
			row.Network = "opentable"
			// Resolve slug → restaurant ID via Autocomplete. The OT
			// `RestaurantsAvailability` GraphQL takes a numeric
			// restaurantId, not a slug. Slug-format queries
			// (`le-bernardin`) are converted to spaced names.
			// OT's Autocomplete is broken when called with lat=0/lng=0 — its
			// `personalizer-autocomplete/v4` upstream returns INTERNAL_SERVER_ERROR
			// without a coordinate to anchor on. Defaulting to NYC (which has
			// the largest OT footprint) lets the GraphQL search the global
			// index and still match restaurants in any metro.
			restID, restName, restSlug, rerr := c.RestaurantIDFromQuery(ctx, slug, 40.7128, -74.0060)
			if rerr != nil {
				row.Available = false
				row.Reason = fmt.Sprintf("opentable: could not resolve %q (%v)", slug, rerr)
				return row
			}
			row.URL = fmt.Sprintf("%s/restaurant/profile/%d", opentable.Origin, restID)
			// New OT gateway (May 2026) returns single-day availability per
			// call (forwardDays=0); scan multi-day windows by looping the
			// caller's `--within` over consecutive dates and merging results.
			startDate, derr := time.Parse("2006-01-02", date)
			if derr != nil {
				startDate = time.Now()
			}
			var avail []opentable.RestaurantAvailability
			var aerr error
			for d := 0; d < within; d++ {
				dayStr := startDate.AddDate(0, 0, d).Format("2006-01-02")
				dayAvail, derr := c.RestaurantsAvailability(ctx, []int{restID}, dayStr, "19:00", party, 0, 210, 0, noCache)
				if derr != nil {
					// Akamai's WAF blocks `opname=RestaurantsAvailability` at the
					// edge for any non-real-Chrome client. Fall back to a brief
					// headless Chrome that navigates to the page and intercepts
					// its own runtime XHR — the real browser passes Akamai
					// because it runs the JS sensor naturally.
					if _, isBot := opentable.IsBotDetection(derr); isBot {
						chromeAvail, cerr := c.ChromeAvailability(ctx, restID, restSlug, dayStr, "19:00", party, 0)
						if cerr == nil {
							avail = append(avail, chromeAvail...)
							continue
						}
						aerr = fmt.Errorf("direct path blocked by Akamai (%v); chrome fallback also failed: %v", derr, cerr)
						break
					}
					aerr = derr
					break
				}
				avail = append(avail, dayAvail...)
			}
			if aerr != nil {
				row.Available = false
				row.Reason = fmt.Sprintf("opentable %s (id=%d): %v; venue exists, book directly at %s",
					restName, restID, aerr, row.URL)
				return row
			}
			// Find the earliest slot with isAvailable=true across all
			// returned days. The new GraphQL schema (May 2026) carries
			// `dayOffset` (days from the requested `date`) instead of a
			// literal `date` field, so we compute the actual date as
			// requestDate + dayOffset, and resolve slot time as
			// requestTime + timeOffsetMinutes.
			startDate, perr := time.Parse("2006-01-02", date)
			if perr != nil {
				startDate = time.Now()
			}
			anchorHH := 19
			anchorMM := 0
			var bookable []string
			for _, ra := range avail {
				if ra.RestaurantID != restID {
					continue
				}
				for _, d := range ra.AvailabilityDays {
					dayDate := d.Date
					if dayDate == "" {
						dayDate = startDate.AddDate(0, 0, d.DayOffset).Format("2006-01-02")
					}
					for _, s := range d.Slots {
						if !s.IsAvailable {
							continue
						}
						totalMin := anchorHH*60 + anchorMM + s.TimeOffsetMinutes
						hh := ((totalMin/60)%24 + 24) % 24
						mm := ((totalMin % 60) + 60) % 60
						bookable = append(bookable, fmt.Sprintf("%sT%02d:%02d", dayDate, hh, mm))
					}
				}
			}
			sort.Strings(bookable)
			seen := map[string]bool{}
			deduped := bookable[:0]
			for _, b := range bookable {
				if seen[b] {
					continue
				}
				seen[b] = true
				deduped = append(deduped, b)
			}
			bookable = deduped
			var earliestSlotAt string
			if len(bookable) > 0 {
				earliestSlotAt = bookable[0]
				row.BookableTimes = bookable
			}
			// Surface OT stale-cache-fallback metadata when present. The
			// underlying client tags rows with Source="cache_fallback"
			// when it served from disk after Akamai blocked the network.
			// Take metadata from the first availability chunk; all
			// chunks of a single response carry the same flags.
			cacheNote := ""
			for _, ra := range avail {
				if ra.Source != "" {
					row.Source = ra.Source
					row.Stale = ra.Stale
					if !ra.CachedAt.IsZero() {
						row.CachedAt = ra.CachedAt.Format(time.RFC3339)
						mins := int(time.Since(ra.CachedAt).Round(time.Minute).Minutes())
						if ra.Stale {
							cacheNote = fmt.Sprintf(" (served from cache fallback; data %dm old, past TTL — Akamai blocked the live fetch)", mins)
						} else {
							cacheNote = fmt.Sprintf(" (served from cache fallback; data %dm old — Akamai blocked the live fetch)", mins)
						}
					}
					break
				}
			}
			if earliestSlotAt != "" {
				row.Available = true
				row.SlotAt = earliestSlotAt
				row.Reason = fmt.Sprintf("opentable %s: earliest slot at %s%s", restName, earliestSlotAt, cacheNote)
			} else {
				row.Available = false
				row.Reason = fmt.Sprintf("opentable %s: no open slots in %d-day window for party=%d%s", restName, within, party, cacheNote)
			}
			return row
		}
	}
	if row.Network == "" {
		row.Network = "unknown"
		if row.Reason == "" {
			row.Reason = "could not resolve venue on OpenTable or Tock"
		}
	}
	return row
}
