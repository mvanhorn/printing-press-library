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
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

type earliestRow struct {
	Venue     string `json:"venue"`
	Network   string `json:"network"`
	SlotAt    string `json:"slot_at,omitempty"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// ErrorKind discriminates anti-bot block types so agents can branch
	// on the recovery strategy without parsing Reason. Issue #406
	// failure 5. Values: "session_blocked" → run `auth login --chrome`;
	// "operation_blocked" → pivot to a numeric OT ID via
	// `restaurants list` to bypass the blocked GraphQL op. Empty for
	// non-bot errors.
	ErrorKind string  `json:"error_kind,omitempty"`
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

type earliestMeta struct {
	// VenuesRequested is the count of input slugs (including duplicates).
	VenuesRequested int `json:"venues_requested"`
	// Resolved is the count of slugs that the source-client successfully
	// mapped to a real network (OpenTable or Tock). Resolved-but-blocked
	// counts here too — they got past slug→ID resolution, just couldn't
	// fetch slots.
	Resolved int `json:"resolved"`
	// Unresolved is VenuesRequested - Resolved. Issue #406 failure 4:
	// without this field, a request that resolved zero slugs returned
	// `{}` (under `--select results.X` with no rows), indistinguishable
	// from "resolved fine, just no slots open." Surfacing the count
	// always lets agents branch on the case.
	Unresolved int `json:"unresolved"`
	// Available is the count of resolved venues with at least one bookable
	// slot in the window.
	Available int `json:"available"`
}

type unresolvedRow struct {
	Venue  string `json:"venue"`
	Reason string `json:"reason,omitempty"`
}

type earliestResponse struct {
	Venues  []string      `json:"venues"`
	Party   int           `json:"party"`
	Within  int           `json:"within_days"`
	Meta    earliestMeta  `json:"meta"`
	Results []earliestRow `json:"results"`
	// Unresolved is emitted as `[]` (not omitted) when empty, mirroring
	// `Results`. Agents checking `"unresolved" in response` would
	// otherwise see a false negative when ALL venues resolved (key
	// absent) vs SOME unresolved (key present). Symmetry with Results
	// keeps the response shape predictable.
	Unresolved []unresolvedRow `json:"unresolved"`
	QueriedAt  string          `json:"queried_at"`
}

// summarizeEarliest partitions the row set into resolved-only Results
// and unresolved companions, and computes the meta summary alongside.
//
// A row is considered "unresolved" when its Network is empty or "unknown" —
// the resolver short-circuited before assigning a network. Resolved-but-
// blocked rows (Network set, Available=false, Reason mentions Akamai etc.)
// stay in Results but don't count toward Available.
//
// Partitioning here (rather than passing the raw `rows` to Results)
// closes the duplication bug Greptile flagged on PR #424 round-2:
// previously unresolved venues appeared in BOTH the results[] and
// unresolved[] arrays simultaneously.
//
// PRECONDITION: callers must pass `rows` produced from `venues` so that
// `len(rows) == len(venues)` and entries correspond positionally. The
// invariant `Resolved + Unresolved == VenuesRequested` only holds under
// this condition; mismatched slices silently produce diverging counts.
// All current callers (newEarliestCmd's dry-run and live paths) satisfy
// this by appending one row per input venue in order.
func summarizeEarliest(venues []string, rows []earliestRow) (earliestMeta, []earliestRow, []unresolvedRow) {
	// Initialize as empty slices (not nil) so JSON serialization emits
	// `[]` rather than `null`. Symmetry across results + unresolved
	// matters for the agent contract — both keys should always be
	// present so consumers can iterate without nil-checks.
	results := []earliestRow{}
	unresolved := []unresolvedRow{}
	var available int
	for _, r := range rows {
		if r.Network == "" || r.Network == "unknown" {
			unresolved = append(unresolved, unresolvedRow{Venue: r.Venue, Reason: r.Reason})
			continue
		}
		results = append(results, r)
		if r.Available {
			available++
		}
	}
	return earliestMeta{
		VenuesRequested: len(venues),
		Resolved:        len(results),
		Unresolved:      len(unresolved),
		Available:       available,
	}, results, unresolved
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
			"explicit routing, otherwise both networks are tried. Numeric IDs " +
			"from `restaurants list --json` (the `id` field) work as inputs too. " +
			"Use `--tonight` as shorthand for `--date <today> --within 1d`.\n\n" +
			"Response shape:\n" +
			"  • `meta.venues_requested`, `meta.resolved`, `meta.unresolved`, " +
			"`meta.available` — summary counts always present, regardless of\n" +
			"    `--select` path, so agents can distinguish \"checked, no\n" +
			"    slots\" from \"couldn't resolve any input.\"\n" +
			"  • `results[]` — one row per resolved venue with slot data.\n" +
			"  • `unresolved[]` — venues that didn't resolve, with reason\n" +
			"    strings. Empty when all resolve.\n\n" +
			"Common `--select` paths: `results.venue`, `results.network`,\n" +
			"`results.slot_at`, `results.bookable_times`, `meta.resolved`,\n" +
			"`meta.available`, `unresolved.venue`, `unresolved.reason`.\n\n" +
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
				meta, results, unresolved := summarizeEarliest(venues, rows)
				return printJSONFiltered(cmd.OutOrStdout(), earliestResponse{
					Venues: venues, Party: party, Within: withinDays,
					Meta: meta, Results: results, Unresolved: unresolved,
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

			// Hydrate the metro registry from Tock's metroArea SSR so
			// slug-suffix inference inside resolveOTSlugGeoAware can
			// resolve the full 248-metro dynamic list (vs the 20-entry
			// static fallback). Cached 24h on disk; first call ~200ms,
			// subsequent calls <1ms. Silent on failure — falls back to
			// static. (PR #425 round-2 Greptile finding: this call
			// existed on the goat path but was missing here, so
			// `earliest 'joey-bellevue'` couldn't infer the bellevue
			// suffix on a fresh install.)
			hydrateMetrosFromTock(ctx, session)

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
			meta, results, unresolved := summarizeEarliest(venues, rows)
			return printJSONFiltered(cmd.OutOrStdout(), earliestResponse{
				Venues: venues, Party: party, Within: withinDays,
				Meta: meta, Results: results, Unresolved: unresolved,
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

	// Tock uses domain-name slugs (`canlis`, `farzi-cafe-bellevue`), not
	// numeric IDs. If the caller passed `tock:<digits>` it's a category
	// mismatch — surface a typed error rather than running the Calendar
	// fetch against a non-existent slug.
	if tryTock && network == "tock" {
		if _, err := strconv.Atoi(slug); err == nil {
			row.Network = "tock"
			row.Available = false
			row.Reason = fmt.Sprintf("tock: %q looks like a numeric ID, but Tock venues are addressed by domain-name slug (e.g. 'canlis', 'farzi-cafe-bellevue'). Numeric IDs are an OpenTable-only convention; try 'opentable:%s' instead.", slug, slug)
			return row
		}
	}

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
			// NOTE: `row.Network = "opentable"` is deliberately NOT set
			// here. PR #424 round-3 Greptile finding: setting Network
			// before slug resolution caused slug-resolve failures to be
			// miscounted as `meta.resolved` (Network was already
			// "opentable" when summarizeEarliest's partition ran). The
			// assignment moves AFTER we have a confirmed valid restID
			// so the partition correctly categorizes failures as
			// `meta.unresolved`.

			// Numeric-ID short-circuit (issue #406, failure 2): `restaurants
			// list` emits numeric OpenTable IDs (e.g. id=3688 for "Daniel's
			// Broiler - Bellevue") but the Autocomplete-based slug resolver
			// can't match them — it does name-similarity search, not ID
			// lookup. Without this shortcut, agents who try
			// `availability check opentable:3688` get "could not resolve"
			// even though the ID came directly from this CLI's own output.
			// When the slug is pure digits, skip Autocomplete and pass the
			// ID straight to RestaurantsAvailability. Slug-resolver
			// misfires (the well-known global-fuzzy-match bug) are also
			// bypassed on this path, so agents can route around the
			// resolver via the numeric ID.
			var restID int
			var restName, restSlug string
			if numID, numErr := strconv.Atoi(slug); numErr == nil && numID > 0 {
				restID = numID
				// restName/restSlug stay empty; row.URL still resolves
				// canonically below from the numeric ID. The downstream
				// chrome-avail SSR fetch can hydrate the name later if
				// needed, but for agents the URL is the canonical anchor.
			} else {
				// Resolve slug → restaurant ID. Issue #406 failure 1: the
				// previous resolver called RestaurantIDFromQuery directly,
				// which picks the first Autocomplete hit by name — so
				// `joey-bellevue` resolved to "Joey's Bold Flavors"
				// (Tampa, FL) because the `-bellevue` suffix was dropped
				// on the floor. resolveOTSlugGeoAware detects the city
				// suffix, anchors Autocomplete on the inferred metro's
				// centroid, and picks the geo-closest in-radius match.
				// When no city suffix is detected, falls through to the
				// existing RestaurantIDFromQuery behavior (NYC anchor).
				var (
					rerr      error
					metroUsed Metro
				)
				restID, restName, restSlug, metroUsed, rerr = resolveOTSlugGeoAware(
					ctx, c, slug, 40.7128, -74.0060, defaultMetroRadiusKm,
				)
				if rerr != nil {
					// Slug-resolve failed. row.Network stays empty so
					// summarizeEarliest partitions this row into
					// `unresolved[]` (PR #424 round-3 fix).
					row.Available = false
					row.Reason = fmt.Sprintf("opentable: could not resolve %q (%v)", slug, rerr)
					// If the slug-resolve itself failed because Autocomplete
					// is WAF-blocked, surface the typed kind so the agent knows
					// to pivot to a numeric ID (which bypasses Autocomplete
					// entirely) — issue #406 failure 5.
					if bde, isBot := opentable.IsBotDetection(rerr); isBot {
						row.ErrorKind = string(bde.Kind)
					}
					return row
				}
				_ = metroUsed // hooks into future per-row geo annotation
			}
			// Slug resolution succeeded (numeric path or named path).
			// Claim the row for OpenTable so downstream partitioning
			// counts this venue as resolved, even if the subsequent
			// availability fetch is blocked by Akamai.
			row.Network = "opentable"
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
						// `restSlug` may be empty when the caller passed a
						// numeric OpenTable ID (the numeric short-circuit
						// skips Autocomplete so we never populate restSlug).
						// ChromeAvailability handles the empty-slug case by
						// falling back to `/restaurant/profile/<id>?...`
						// instead of `/r/<slug>?...` — Akamai treats both
						// routes as legitimate user navigation, so the
						// fallback URL is equivalent for WAF acceptance.
						// (PR #423 round-2 Greptile P1 — documenting that
						// the empty-slug pass-through is intentional, not
						// a missing-data bug.)
						chromeAvail, cerr := c.ChromeAvailability(ctx, restID, restSlug, dayStr, "19:00", party, 0)
						if cerr == nil {
							avail = append(avail, chromeAvail...)
							continue
						}
						// Use %w (not %v) on derr so the wrapped *BotDetectionError
						// remains unwrappable by errors.As — downstream
						// IsBotDetection(aerr) needs to surface row.ErrorKind even
						// in the dual-failure case. (PR #426 round-2 Greptile P1.)
						aerr = fmt.Errorf("direct path blocked by Akamai (%w); chrome fallback also failed: %v", derr, cerr)
						break
					}
					aerr = derr
					break
				}
				avail = append(avail, dayAvail...)
			}
			// When the caller passed a numeric ID, restName is empty (we
			// didn't hit Autocomplete). Fall back to "restaurant #<id>" so
			// Reason strings read naturally instead of "opentable : ...".
			venueLabel := restName
			if venueLabel == "" {
				venueLabel = fmt.Sprintf("restaurant #%d", restID)
			}
			if aerr != nil {
				row.Available = false
				row.Reason = fmt.Sprintf("opentable %s (id=%d): %v; venue exists, book directly at %s",
					venueLabel, restID, aerr, row.URL)
				// Surface typed kind so the agent can branch — operation_blocked
				// means "try a different op/numeric ID"; session_blocked means
				// "run auth login --chrome". Issue #406 failure 5.
				if bde, isBot := opentable.IsBotDetection(aerr); isBot {
					row.ErrorKind = string(bde.Kind)
				}
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
				row.Reason = fmt.Sprintf("opentable %s: earliest slot at %s%s", venueLabel, earliestSlotAt, cacheNote)
			} else {
				row.Available = false
				row.Reason = fmt.Sprintf("opentable %s: no open slots in %d-day window for party=%d%s", venueLabel, within, party, cacheNote)
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
