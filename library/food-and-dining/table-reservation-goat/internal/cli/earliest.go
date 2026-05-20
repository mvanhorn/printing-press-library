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
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
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

	// LocationResolved is the U7 typed-resolution annotation populated when
	// the caller passed --location (or legacy --metro) and ResolveLocation
	// returned a GeoContext. Omitted on the no-constraint path so existing
	// JSON consumers see no field-shape change.
	LocationResolved *LocationResolvedField `json:"location_resolved,omitempty"`
	// LocationWarning is attached alongside LocationResolved when the
	// resolution had material ambiguity (MEDIUM tier), the caller forced
	// past LOW with --batch-accept-ambiguous, or a numeric-ID venue resolved
	// outside the requested radius (numeric-ID exemption — soft-demote
	// with warning rather than hard-reject from post-filter).
	LocationWarning *LocationWarningField `json:"location_warning,omitempty"`
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
// closes the duplication bug Greptile flagged on PR #424: previously
// unresolved venues appeared in BOTH the results[] and unresolved[]
// arrays simultaneously.
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
		party               int
		within              string
		date                string
		tonight             bool
		noCache             bool
		flagLocation        string
		flagAcceptAmbiguous bool
		flagMetro           string
	)
	cmd := &cobra.Command{
		Use:   "earliest <slug1,slug2,...>",
		Short: "Soonest open slot per venue across OpenTable, Tock, and Resy",
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

			// Resolve --location / --metro into a typed GeoContext (or a
			// disambiguation envelope) before any provider call. The
			// resolved GeoContext flows into resolveEarliestForVenue and
			// drives the OT Autocomplete coordinate hint in place of the
			// hardcoded NYC fallback. When neither --location nor --metro
			// is set, gc is nil and resolveEarliestForVenue's per-venue
			// slug-suffix inference fires as the lowest-precedence
			// fallback (SourceExtractedFromQuery → soft-demote in post-
			// filter).
			gc, envelope, locationErr, acceptedAmbiguous := resolveLocationFlags(
				cmd.ErrOrStderr(),
				flagLocation,
				flagMetro,
				flagAcceptAmbiguous,
			)
			if locationErr != nil {
				return locationErr
			}
			if envelope != nil {
				// Disambiguation envelope replaces the result list entirely
				// — the caller must pick a location before per-venue
				// resolution makes sense.
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			if dryRunOK(flags) {
				rows := make([]earliestRow, 0, len(venues))
				for _, v := range venues {
					row := earliestRow{Venue: v, Network: "opentable", Available: false, Reason: "dry-run"}
					// Decorate with the resolved location context (or
					// fall back to slug-suffix inference when gc is nil
					// and the venue's slug carries a city suffix). This
					// keeps the dry-run shape consistent with the live
					// path's annotations.
					rowGC := gc
					if rowGC == nil {
						rowGC = inferGeoContextFromSlug(v)
					}
					row = applyGeoToVenueRow(row, rowGC, acceptedAmbiguous, v)
					rows = append(rows, row)
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
				// U8: pass the resolved GeoContext into the per-venue
				// resolver. When gc is nil (no explicit --location/--metro),
				// fall back to slug-suffix inference per venue — that path
				// constructs a SourceExtractedFromQuery GeoContext so the
				// post-filter soft-demotes rather than hard-rejects.
				rowGC := gc
				if rowGC == nil {
					rowGC = inferGeoContextFromSlug(v)
				}
				row := resolveEarliestForVenue(ctx, session, v, party, startDate, withinDays, noCache, rowGC)
				row = applyGeoToVenueRow(row, rowGC, acceptedAmbiguous, v)
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
	cmd.Flags().StringVar(&flagLocation, "location", "",
		"Free-form location: 'bellevue, wa', 'seattle', '47.6,-122.3', or 'seattle metro'. "+
			"Anchors the OT Autocomplete fallback on the resolved centroid. When omitted, "+
			"each venue's slug suffix is checked as a lowest-precedence fallback "+
			"(SourceExtractedFromQuery → soft-demote on out-of-radius hits).")
	cmd.Flags().BoolVar(&flagAcceptAmbiguous, "batch-accept-ambiguous", false,
		"BATCH-ONLY escape hatch: when --location is ambiguous, force-pick the top "+
			"candidate instead of returning a disambiguation envelope. Interactive "+
			"agents must NOT set this — it defeats the disambiguation contract.")
	cmd.Flags().StringVar(&flagMetro, "metro", "",
		"Metro slug (e.g., chicago, seattle). DEPRECATED — use --location <city>. "+
			"Implicit --batch-accept-ambiguous is canonical-only: a single-hit registry lookup "+
			"preserves the legacy result shape; ambiguous or unknown values return the standard "+
			"disambiguation envelope just like --location would.")
	return cmd
}

// inferGeoContextFromSlug constructs a GeoContext from the city-suffix
// hint embedded in a venue slug (e.g., "joey-bellevue" → Bellevue WA).
// This is the lowest-precedence fallback path on the `earliest` command:
// when the caller passed neither --location nor --metro, but a slug
// carries a recognizable city suffix, we synthesize a GeoContext so the
// OT Autocomplete coordinate hint anchors on the right metro. The
// resulting GeoContext carries Source=SourceExtractedFromQuery so the
// downstream post-filter soft-demotes out-of-radius rows rather than
// hard-rejecting (the slug-suffix hint is best-effort, not authoritative
// the way --location is).
//
// Resolution strategy:
//  1. inferMetroFromSlug_DEPRECATED — looks up the suffix against the
//     metro registry by slug (covers "joey-seattle", "joey-chicago" via
//     canonical metro slugs and aliases).
//  2. Fallback to ResolveLocation with AcceptAmbiguous=true on the
//     peeled suffix — covers same-name cities not registered as metros
//     ("joey-bellevue" → LookupByName("bellevue") → top-ranked
//     Bellevue WA via popularityPrior).
//
// Returns nil when neither path finds a match — the per-venue path then
// falls back to resolveOTSlugGeoAware's legacy NYC anchor.
func inferGeoContextFromSlug(venue string) *GeoContext {
	_, slug := parseNetworkSlug(venue)
	if slug == "" {
		return nil
	}

	// Strategy 1: metro-slug suffix match (e.g., "joey-seattle").
	metro, _, ok := inferMetroFromSlug_DEPRECATED(slug, getRegistry())
	if ok {
		radius := metro.RadiusKm
		if radius <= 0 {
			radius = defaultMetroRadiusKm
		}
		return &GeoContext{
			Origin:     slug,
			ResolvedTo: formatPlaceName(metro),
			Centroid:   [2]float64{metro.Lat, metro.Lng},
			RadiusKm:   radius,
			Score:      0.5,
			Tier:       ResolutionTierHigh,
			Source:     SourceExtractedFromQuery,
		}
	}

	// Strategy 2: peel hyphen-separated tokens from the end and try
	// LookupByName via ResolveLocation. AcceptAmbiguous=true forces a
	// pick on multi-candidate names (e.g., three Bellevues) so the
	// slug-suffix hint always lands on a usable GeoContext rather than
	// the envelope path — the slug suffix is a hint, not a question.
	tokens := strings.Split(slug, "-")
	for suffixLen := 3; suffixLen >= 1; suffixLen-- {
		if suffixLen > len(tokens) {
			continue
		}
		suffix := strings.Join(tokens[len(tokens)-suffixLen:], " ")
		gc, _, err := ResolveLocation(suffix, ResolveOptions{
			Source:          SourceExtractedFromQuery,
			AcceptAmbiguous: true,
		})
		if err != nil || gc == nil {
			continue
		}
		// The slug-suffix hint is a best-effort signal, not a question.
		// ResolveLocation may have stamped Tier=Low (3-way Bellevue,
		// 4-way Springfield, etc.) but the decoration intent here is
		// warn-and-continue: present the pick with alternates as a
		// LocationWarning rather than (nil, nil) from the LOW-no-bypass
		// path. Rewrite the tier to MEDIUM when alternates exist, HIGH
		// when there's a single candidate.
		if len(gc.Alternates) > 0 {
			gc.Tier = ResolutionTierMedium
		} else {
			gc.Tier = ResolutionTierHigh
		}
		return gc
	}
	return nil
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

// isNumericIDInput reports whether the venue argument is a pure numeric
// OpenTable ID (with or without the `opentable:` prefix). Tock has no
// numeric-ID convention, so a `tock:<digits>` input still counts as
// "not numeric" from this helper's perspective — Tock's numeric inputs
// take the typed-error path inside resolveEarliestForVenue, not the
// numeric-ID exemption path.
//
// Used by availability_check / multi-day to decide whether to hard-
// reject (slug input) vs soft-demote with LocationWarning (numeric-ID
// input) when a venue resolves outside the requested radius. Numeric
// IDs are unambiguous: the agent already knew exactly which venue it
// wanted, so the post-filter shouldn't second-guess it — instead we
// surface the geographic mismatch via a warning and let the caller
// decide.
func isNumericIDInput(venue string) bool {
	network, slug := parseNetworkSlug(venue)
	if network == "tock" {
		return false
	}
	if slug == "" {
		return false
	}
	for _, r := range slug {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// applyGeoToVenueRow attaches LocationResolved / LocationWarning to a
// resolved row according to the typed pipeline contract:
//
//   - gc == nil — no decoration; the caller didn't pass --location.
//   - numeric-ID input + gc non-nil — always decorate with
//     LocationResolved. If the row carries lat/lng and falls outside
//     the radius, attach a LocationWarning ("venue is outside your
//     stated location"). Never drop the row (numeric-ID exemption).
//   - non-numeric input + gc non-nil — decorate with LocationResolved.
//     Hard-reject for slug inputs is already enforced inside
//     resolveOTSlugGeoAware (it returns an error when no candidate
//     lands in-radius); a resolved row at this layer has already
//     passed the in-radius check, so no extra warning is needed.
//
// The acceptedAmbiguous flag flows from the caller's --batch-accept-ambiguous
// flag (and the --metro legacy implicit) into the tier inference so
// the MEDIUM-vs-LOW-with-bypass decoration matches the resolution
// shape.
func applyGeoToVenueRow(row earliestRow, gc *GeoContext, acceptedAmbiguous bool, venue string) earliestRow {
	if gc == nil {
		return row
	}
	resolved, warning := DecorateWithLocationContext(gc, inferTierFromGeoContext(gc, acceptedAmbiguous), acceptedAmbiguous)
	row.LocationResolved = resolved
	row.LocationWarning = warning

	// Numeric-ID exemption: soft-demote with a warning when the venue
	// resolved outside the stated radius. Slug inputs already had their
	// hard-reject enforced inside resolveOTSlugGeoAware, so this branch
	// only fires for numeric IDs that bypassed Autocomplete entirely.
	if !isNumericIDInput(venue) {
		return row
	}
	if row.Latitude == 0 && row.Longitude == 0 {
		// No lat/lng on the row — can't make a geo judgement. Keep
		// LocationResolved (caller stated the location explicitly) but
		// skip the warning rather than fire on missing data.
		return row
	}
	dist := haversineKm(gc.Centroid[0], gc.Centroid[1], row.Latitude, row.Longitude)
	radius := gc.RadiusKm
	if radius <= 0 {
		radius = defaultMetroRadiusKm
	}
	if dist <= radius {
		return row
	}
	// Outside radius. Attach (or overwrite) the warning so consumers see
	// the geographic mismatch without losing the resolved row.
	row.LocationWarning = &LocationWarningField{
		Picked:     venue,
		Alternates: nil,
		Reason: fmt.Sprintf(
			"venue is outside your stated location (%.1fkm from %s; radius %.0fkm)",
			dist, gc.ResolvedTo, radius),
	}
	return row
}

func parseNetworkSlug(input string) (network, slug string) {
	if i := strings.Index(input, ":"); i > 0 {
		net := strings.ToLower(input[:i])
		if net == "opentable" || net == "tock" || net == "resy" {
			return net, input[i+1:]
		}
	}
	return "", input
}

// resolveEarliestForVenue resolves a venue across both networks and
// returns its earliest open slot in the requested window.
//
// The optional gc parameter routes the OT Autocomplete fallback inside
// resolveOTSlugGeoAware: when gc is non-nil, gc.Centroid drives the
// coordinate hint in place of the hardcoded NYC fallback. When gc is
// nil, the NYC fallback is preserved (back-compat for callers that
// haven't yet wired through the typed location pipeline). The gc
// argument does not affect the numeric-ID short-circuit path — that
// path bypasses Autocomplete entirely and routes directly by ID.
//
// Numeric-ID exemption from the post-filter hard-reject is the caller's
// responsibility (availability_check / multi-day / future earliest):
// numeric IDs are an unambiguous addressing scheme, so an out-of-radius
// venue resolved via a numeric ID is a soft-demote (attach
// LocationWarning) rather than a hard-reject. See isNumericIDInput.
func resolveEarliestForVenue(ctx context.Context, s *auth.Session, venue string, party int, date string, within int, noCache bool, gc *GeoContext) earliestRow {
	network, slug := parseNetworkSlug(venue)
	row := earliestRow{Venue: venue}

	tryOT := network == "" || network == "opentable"
	tryTock := network == "" || network == "tock"
	tryResy := network == "resy" // Resy requires an explicit prefix; auto-fanout reserved for OT+Tock

	// Resy branch — runs only when the caller explicitly addresses
	// `resy:<numericVenueID>`. Resy's venue identifier is the numeric ID
	// returned by Search; non-numeric input is rejected with a typed reason
	// so the agent can pivot rather than burn an API call.
	if tryResy {
		if s == nil || s.Resy == nil || s.Resy.AuthToken == "" {
			row.Network = "resy"
			row.Available = false
			row.Reason = "resy: not authenticated; run `auth login --resy --email <you@example.com>` first"
			return row
		}
		if _, err := strconv.Atoi(slug); err != nil {
			row.Network = "resy"
			row.Available = false
			row.Reason = fmt.Sprintf("resy: %q is not a numeric venue id; use the `id` field from `goat <name> --network resy` output", slug)
			return row
		}
		return resolveEarliestForResy(ctx, s, slug, party, date, within, row)
	}

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
				// Pick the Autocomplete coordinate hint from the typed
				// GeoContext when the caller passed --location; otherwise
				// fall back to the legacy NYC anchor (40.7128, -74.0060)
				// so callers that haven't wired the typed pipeline yet
				// preserve their current behavior. The radius likewise
				// flows from gc when present so callers can widen/narrow
				// the slug-suffix resolver's in-radius window.
				anchorLat, anchorLng := 40.7128, -74.0060
				anchorRadius := defaultMetroRadiusKm
				if gc != nil {
					anchorLat, anchorLng = gc.Centroid[0], gc.Centroid[1]
					if gc.RadiusKm > 0 {
						anchorRadius = gc.RadiusKm
					}
				}
				restID, restName, restSlug, metroUsed, rerr = resolveOTSlugGeoAware(
					ctx, c, slug, anchorLat, anchorLng, anchorRadius,
				)
				if rerr != nil {
					// Slug-resolve failed. row.Network stays empty so
					// summarizeEarliest partitions this row into
					// `unresolved[]` (PR #424 round-3 fix). The row
					// carries the reason so agents can see why.
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

// resolveEarliestForResy fans out one Availability call per day across the
// `within` window, collects open slots, and returns the earliest. Resy's API
// is per-(venue, date, party) so multi-day scans must loop client-side; no
// equivalent of Tock's 60-day calendar endpoint exists for Resy.
func resolveEarliestForResy(ctx context.Context, s *auth.Session, venueID string, party int, date string, within int, row earliestRow) earliestRow {
	row.Network = "resy"
	// No row.URL synthesis: Resy URLs are /cities/<cityCode>/<slug>, but
	// `earliest` is called with the numeric venue id and we don't have
	// the slug or city code in scope. Synthesizing /cities/ny/<numericID>
	// (the previous shape) was wrong on two axes — it 404s because Resy
	// expects a slug not a numeric id, and the "ny" city code is wrong
	// for non-NYC venues. Agents wanting a clickable URL should resolve
	// it via `goat <name> --network resy`, which carries Slug + CityCode
	// from the search response and synthesizes the proper URL there.
	client := resy.New(resy.Credentials{
		APIKey:    s.Resy.APIKey,
		AuthToken: s.Resy.AuthToken,
		Email:     s.Resy.Email,
	})
	start, perr := time.Parse("2006-01-02", date)
	if perr != nil {
		start = time.Now()
	}
	if within < 1 {
		within = 1
	}
	var lastErr error
	successfulDays := 0
	bookable := make([]string, 0)
	seenResySlot := map[string]bool{}
	for d := 0; d < within; d++ {
		day := start.AddDate(0, 0, d).Format("2006-01-02")
		slots, err := client.Availability(ctx, resy.AvailabilityParams{
			VenueID:   venueID,
			Date:      day,
			PartySize: party,
		})
		if err != nil {
			lastErr = err
			// Keep walking — Resy occasionally 5xxes on a single day; one
			// bad day shouldn't terminate the whole scan.
			continue
		}
		successfulDays++
		// Resy returns one slot row per (date, time, seating-area-config)
		// tuple, so 19:00 with both "Dining Room" and "Bar" configs
		// produces two slots for the same timestamp. agents only care
		// about distinct (date, time) pairs at this layer — book/cancel
		// addresses individual configs via the slot token — so dedupe
		// before appending. Matches the OT and Tock dedup pattern.
		for _, sl := range slots {
			ts := day + "T" + sl.Time
			if seenResySlot[ts] {
				continue
			}
			seenResySlot[ts] = true
			bookable = append(bookable, ts)
		}
	}
	// Refuse to claim "no slots" when every single per-day call failed.
	// `availability check resy:<id>` uses the same resolver, so a scan
	// failure (expired token, network outage, repeated 5xxs) would
	// otherwise be indistinguishable from honest zero-availability —
	// agents would then make booking decisions based on phantom
	// emptiness. Return Available=false with a typed scan_failed
	// signal in the Reason so callers can branch.
	if successfulDays == 0 {
		row.Available = false
		row.ErrorKind = "scan_failed"
		if lastErr != nil {
			row.Reason = fmt.Sprintf("resy venue=%s: every per-day availability call failed; cannot distinguish from no slots. Last error: %v", venueID, lastErr)
		} else {
			row.Reason = fmt.Sprintf("resy venue=%s: scan ran zero successful per-day calls; cannot report availability", venueID)
		}
		return row
	}
	sort.Strings(bookable)
	if len(bookable) > 0 {
		row.Available = true
		row.SlotAt = bookable[0]
		row.BookableTimes = bookable
		row.Reason = fmt.Sprintf("resy venue=%s: %d open slot(s) for party=%d in %d-day window; earliest %s",
			venueID, len(bookable), party, within, bookable[0])
		return row
	}
	row.Available = false
	if lastErr != nil {
		row.Reason = fmt.Sprintf("resy venue=%s: no open slots in %d-day window for party=%d (last error on partial scan: %v)", venueID, within, party, lastErr)
	} else {
		row.Reason = fmt.Sprintf("resy venue=%s: no open slots in %d-day window for party=%d", venueID, within, party)
	}
	return row
}
