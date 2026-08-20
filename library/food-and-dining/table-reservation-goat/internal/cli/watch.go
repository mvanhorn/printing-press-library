package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/store"
)

const watchSchemaSQL = `
CREATE TABLE IF NOT EXISTS watches (
  id TEXT PRIMARY KEY,
  venue TEXT NOT NULL,
  network TEXT NOT NULL,
  slug TEXT NOT NULL,
  party_size INTEGER NOT NULL,
  window_spec TEXT,
  notify TEXT,
  state TEXT NOT NULL DEFAULT 'active',
  created_at DATETIME NOT NULL DEFAULT (datetime('now')),
  last_polled_at DATETIME,
  last_match_at DATETIME,
  match_count INTEGER NOT NULL DEFAULT 0,
  location_context TEXT
);
CREATE INDEX IF NOT EXISTS idx_watches_state ON watches(state);
`

// ensureWatchSchemaUpgrades performs idempotent in-place schema upgrades
// for the watches table. CREATE TABLE IF NOT EXISTS is a no-op when the
// table already exists, so columns added after a database was first
// initialized never land on older installs. We probe PRAGMA table_info
// once and ALTER TABLE per missing column.
//
// Currently handles the U15 location_context column added so
// pollOneWatch can anchor on a watch's persisted GeoContext rather
// than re-inferring from the slug suffix at tick time.
func ensureWatchSchemaUpgrades(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(watches)`)
	if err != nil {
		return fmt.Errorf("table_info watches: %w", err)
	}
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("scan table_info watches: %w", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate table_info watches: %w", err)
	}
	rows.Close()

	if !cols["location_context"] {
		if _, err := db.ExecContext(ctx, `ALTER TABLE watches ADD COLUMN location_context TEXT`); err != nil {
			// A concurrent migrator may have added the column between our
			// probe and the ALTER. The DB is now in the desired state
			// regardless of who won; absorb the duplicate-column error.
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("add column watches.location_context: %w", err)
			}
		}
	}
	return nil
}

type watchRow struct {
	ID           string     `json:"id"`
	Venue        string     `json:"venue"`
	Network      string     `json:"network"`
	Slug         string     `json:"slug"`
	PartySize    int        `json:"party_size"`
	WindowSpec   string     `json:"window_spec,omitempty"`
	Notify       string     `json:"notify,omitempty"`
	State        string     `json:"state"`
	CreatedAt    time.Time  `json:"created_at"`
	LastPolledAt *time.Time `json:"last_polled_at,omitempty"`
	LastMatchAt  *time.Time `json:"last_match_at,omitempty"`
	MatchCount   int        `json:"match_count"`
	// LocationResolved is the U8 typed-resolution annotation populated
	// when the caller passed --location (or legacy --metro) to
	// `watch add`. The resolution happens at subscription start so the
	// caller sees the resolved metro immediately; pollOneWatch falls
	// back to slug-suffix inference at tick time for already-persisted
	// watches.
	LocationResolved *LocationResolvedField `json:"location_resolved,omitempty"`
	// LocationWarning fires at subscription start under MEDIUM tier or
	// forced-LOW. The watch is created in either case (warn-and-continue,
	// not refuse), so the row persists with the resolved venue and the
	// caller can decide whether to cancel.
	LocationWarning *LocationWarningField `json:"location_warning,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Local cross-network cancellation watcher",
		Long: "Persistent watches across OpenTable, Tock, and Resy. The local SQLite watch " +
			"table holds your active watches; `watch tick` is intended to run from " +
			"cron / a scheduler — it polls each active watch's source and emits " +
			"matches as JSON events.",
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchCancelCmd(flags))
	cmd.AddCommand(newWatchTickCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var (
		party               int
		window              string
		notify              string
		flagLocation        string
		flagAcceptAmbiguous bool
		flagMetro           string
	)
	cmd := &cobra.Command{
		Use:     "add <venue>",
		Short:   "Add a watch for a venue (network-prefixed slug supported)",
		Example: "  table-reservation-goat-pp-cli watch add 'tock:alinea' --party 2 --window 'sat 7-9pm' --notify local",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			venue := strings.TrimSpace(args[0])
			if venue == "" || strings.Contains(venue, "__printing_press_invalid__") {
				return fmt.Errorf("invalid venue: %q (provide a slug like 'alinea' or 'tock:alinea')", args[0])
			}

			// U8: resolve --location / --metro at subscription start.
			// On envelope, surface to the caller before persisting — they
			// need to disambiguate before the watch is meaningful. On
			// resolution success, decorate the response so the caller
			// sees the resolved metro inline (warn-and-continue: MEDIUM
			// tier or forced-LOW writes a location_warning alongside,
			// but the watch still persists).
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
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}
			resolved, warning := decorateForList(gc, acceptedAmbiguous)
			// Print the location_warning to stderr at subscription start
			// so cron-driven follow-ups see it without re-parsing JSON.
			// Warn-and-continue: never block subscription on ambiguity.
			if warning != nil {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"location_warning: forced pick %q over alternates %v at subscription start — continuing watch\n",
					warning.Picked, warning.Alternates)
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), watchRow{
					ID: "watch_dryrun", Venue: args[0], PartySize: party, State: "active",
					CreatedAt:        time.Now().UTC(),
					LocationResolved: resolved,
					LocationWarning:  warning,
				}, flags)
			}
			network, slug := parseNetworkSlug(args[0])
			// Reject malformed Resy watches BEFORE inserting them into
			// the SQLite watches table. Resy venues are addressed by
			// numeric id (Resy's `id` field from search); a slug-shaped
			// input like `watch add resy:le-bernardin` (easy to type
			// because OT/Tock use slugs) would otherwise be persisted
			// as an active watch that fails forever during `watch tick`,
			// burning poll cycles and cluttering `watch list`. Match the
			// validation pattern earliest / book / drift use.
			if network == "resy" {
				if _, err := strconv.Atoi(slug); err != nil {
					return fmt.Errorf("resy: %q is not a numeric venue id; use the `id` field from `goat <name> --network resy` output (e.g. resy:1387)", slug)
				}
			}
			db, err := openWatchStore(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			if network == "" {
				network = "auto"
			}
			id := newWatchID()
			row := watchRow{
				ID: id, Venue: args[0], Network: network, Slug: slug,
				PartySize: party, WindowSpec: window, Notify: notify,
				State: "active", CreatedAt: time.Now().UTC(),
				LocationResolved: resolved,
				LocationWarning:  warning,
			}
			// U15: persist the resolved GeoContext as JSON so pollOneWatch
			// can anchor on it at tick time instead of re-inferring from
			// the slug suffix (which can drift back to NYC for slugs
			// without a city suffix). Nil gc → NULL column (preserves the
			// no-location no-decoration shape for pre-U8 callers).
			var locationContextJSON sql.NullString
			if gc != nil {
				if raw, mErr := json.Marshal(gc); mErr == nil {
					locationContextJSON = sql.NullString{String: string(raw), Valid: true}
				}
			}
			_, err = db.ExecContext(cmd.Context(),
				`INSERT INTO watches (id, venue, network, slug, party_size, window_spec, notify, state, location_context)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				row.ID, row.Venue, row.Network, row.Slug, row.PartySize,
				row.WindowSpec, row.Notify, row.State, locationContextJSON,
			)
			if err != nil {
				return fmt.Errorf("inserting watch: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), row, flags)
		},
	}
	cmd.Flags().IntVar(&party, "party", 2, "Party size")
	cmd.Flags().StringVar(&window, "window", "", "Time window (e.g., 'sat 7-9pm')")
	cmd.Flags().StringVar(&notify, "notify", "local", "Notification channel: local, slack, webhook (slack/webhook need extra config)")
	cmd.Flags().StringVar(&flagLocation, "location", "",
		"Free-form location: 'bellevue, wa', 'seattle', '47.6,-122.3', or 'seattle metro'. "+
			"Anchors the OT Autocomplete fallback at tick time and decorates the row "+
			"with location_resolved. Warn-and-continue under ambiguity — the watch is "+
			"created with a location_warning rather than refused.")
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

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var stateFilter string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List local cancellation watches with state, last poll, and match count, optionally filtered by state",
		Example: "  table-reservation-goat-pp-cli watch list --json --select id,venue,party_size,state",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openWatchStore(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			where := ""
			argsSQL := []any{}
			if stateFilter != "" {
				where = "WHERE state = ?"
				argsSQL = append(argsSQL, stateFilter)
			}
			query := `SELECT id, venue, network, slug, party_size, window_spec, notify, state,
				 created_at, last_polled_at, last_match_at, match_count, location_context
				 FROM watches ` + where + ` ORDER BY created_at DESC`
			rows, err := db.QueryContext(cmd.Context(), query, argsSQL...)
			if err != nil {
				return fmt.Errorf("query watches: %w", err)
			}
			defer rows.Close()
			out := []watchRow{}
			for rows.Next() {
				var r watchRow
				var window, notify, locationCtx sql.NullString
				var lastPolled, lastMatch sql.NullTime
				var created time.Time
				if err := rows.Scan(&r.ID, &r.Venue, &r.Network, &r.Slug, &r.PartySize,
					&window, &notify, &r.State, &created, &lastPolled, &lastMatch, &r.MatchCount, &locationCtx); err != nil {
					return fmt.Errorf("scan watch: %w", err)
				}
				if window.Valid {
					r.WindowSpec = window.String
				}
				if notify.Valid {
					r.Notify = notify.String
				}
				r.CreatedAt = created
				if lastPolled.Valid {
					t := lastPolled.Time
					r.LastPolledAt = &t
				}
				if lastMatch.Valid {
					t := lastMatch.Time
					r.LastMatchAt = &t
				}
				// U15: rehydrate location_context into the user-facing
				// LocationResolved annotation. Pre-migration rows (NULL
				// column) and pre-U8 rows (added before --location
				// existed) stay back-compat with no decoration. We do
				// not surface LocationWarning here because the warn-bypass
				// state was a one-shot at subscription time, not a
				// persisted contract.
				//
				// U19: pass acceptedAmbiguous=true on the rehydration
				// path so a persisted LOW-tier GeoContext (forced pick
				// at watch-add time via --batch-accept-ambiguous) still
				// surfaces location_resolved. The decision was already
				// made at subscription time; listing is showing what
				// was decided, not re-deciding. Without this, the LOW
				// branch in DecorateWithLocationContext returns (nil,
				// nil) because the (LOW, !bypass) shape is the
				// envelope path — which doesn't apply once the watch
				// is persisted. The warning is dropped explicitly here
				// (same one-shot reasoning as above).
				if locationCtx.Valid && locationCtx.String != "" {
					var gc GeoContext
					if jerr := json.Unmarshal([]byte(locationCtx.String), &gc); jerr == nil {
						resolved, _ := decorateForList(&gc, true)
						r.LocationResolved = resolved
					}
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&stateFilter, "state", "", "Filter by state: active, paused, cancelled")
	return cmd
}

func newWatchCancelCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "cancel <watch-id>",
		Short:   "Cancel a watch by ID (set state=cancelled; row preserved for audit)",
		Example: "  table-reservation-goat-pp-cli watch cancel wat_abc1234567890",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := strings.TrimSpace(args[0])
			if id == "" || strings.Contains(id, "__printing_press_invalid__") || !strings.HasPrefix(id, "wat_") {
				return fmt.Errorf("invalid watch ID: %q (expected `wat_<hex>`)", args[0])
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": args[0], "state": "cancelled", "dry_run": true}, flags)
			}
			db, err := openWatchStore(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.ExecContext(cmd.Context(), `UPDATE watches SET state = 'cancelled' WHERE id = ?`, args[0])
			if err != nil {
				return fmt.Errorf("cancel watch: %w", err)
			}
			n, _ := res.RowsAffected()
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"id": args[0], "cancelled": n > 0}, flags)
		},
	}
}

type tickResult struct {
	WatchID    string `json:"watch_id"`
	Venue      string `json:"venue"`
	Network    string `json:"network"`
	Polled     bool   `json:"polled"`
	HasMatch   bool   `json:"has_match"`
	Reason     string `json:"reason,omitempty"`
	PolledAt   string `json:"polled_at"`
	WindowSpec string `json:"window_spec,omitempty"`
}

func newWatchTickCmd(flags *rootFlags) *cobra.Command {
	var noCache bool
	cmd := &cobra.Command{
		Use:   "tick",
		Short: "Run one polling cycle across active watches (designed for cron)",
		Long: "Polls each active watch on its source network and updates the local " +
			"watches.last_polled_at and match_count columns. Emits one JSON line per " +
			"watch with the polling outcome.\n\n" +
			"OpenTable availability is cached on disk for 3 minutes by default; pass " +
			"`--no-cache` (or set `TRG_OT_NO_CACHE=1`) to force fresh fetches every tick.",
		Example: "  table-reservation-goat-pp-cli watch tick --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []tickResult{
					{WatchID: "watch_dryrun", Venue: "(dry-run)", Network: "opentable", Polled: false, PolledAt: time.Now().UTC().Format(time.RFC3339)},
				}, flags)
			}
			db, err := openWatchStore(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT id, venue, network, slug, party_size, COALESCE(window_spec, ''), COALESCE(location_context, '') FROM watches WHERE state = 'active' ORDER BY created_at`)
			if err != nil {
				return fmt.Errorf("listing active watches: %w", err)
			}
			defer rows.Close()
			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}
			ctx := cmd.Context()
			results := []tickResult{}
			for rows.Next() {
				var (
					id, venue, network, slug, windowSpec, locationCtx string
					party                                             int
				)
				if err := rows.Scan(&id, &venue, &network, &slug, &party, &windowSpec, &locationCtx); err != nil {
					return fmt.Errorf("scan watch: %w", err)
				}
				r := pollOneWatch(ctx, session, id, venue, network, slug, party, windowSpec, locationCtx, noCache)
				results = append(results, r)
				// Only persist last_polled_at when the poll actually ran
				// at least one successful per-network call. pollOneWatch
				// returns Polled=false when every per-day API call errored
				// (e.g., persistent auth_expired or Akamai cooldown). If
				// we bumped last_polled_at unconditionally, the DB would
				// claim "polled, no slots found" indistinguishably from
				// a real successful empty result — masking a multi-day
				// outage as healthy "0 matches" and silently letting
				// hot-target watches drift past their useful window.
				if !r.Polled {
					continue
				}
				now := time.Now().UTC()
				if r.HasMatch {
					_, _ = db.ExecContext(ctx,
						`UPDATE watches SET last_polled_at = ?, last_match_at = ?, match_count = match_count + 1 WHERE id = ?`,
						now, now, id)
				} else {
					_, _ = db.ExecContext(ctx,
						`UPDATE watches SET last_polled_at = ? WHERE id = ?`, now, id)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().BoolVar(&noCache, "no-cache", os.Getenv("TRG_OT_NO_CACHE") == "1", "Bypass the OT availability cache and force a fresh network fetch (env: TRG_OT_NO_CACHE=1).")
	return cmd
}

func pollOneWatch(ctx context.Context, s *auth.Session, id, venue, network, slug string, party int, windowSpec, locationContextJSON string, noCache bool) tickResult {
	r := tickResult{WatchID: id, Venue: venue, Network: network, PolledAt: time.Now().UTC().Format(time.RFC3339), WindowSpec: windowSpec}
	tryOT := network == "auto" || network == "opentable"
	tryTock := network == "auto" || network == "tock"
	tryResy := network == "resy"

	if tryResy {
		return pollOneWatchResy(ctx, s, slug, party, windowSpec, r)
	}
	if tryTock {
		// Tock's runtime XHR `/api/consumer/calendar/full/v2` returns ~60
		// days of per-(date, party, time) sold-out state in a single
		// protobuf payload. One call per tick beats the previous per-day
		// SSR walk and produces a real HasMatch instead of "venue exists
		// and has experiences".
		c, err := tock.New(s)
		if err == nil {
			r.Network = "tock"
			cal, calErr := c.Calendar(ctx, slug)
			if calErr == nil && cal != nil {
				today := time.Now()
				dateFrom := today.Format("2006-01-02")
				dateTo := today.AddDate(0, 0, 6).Format("2006-01-02")
				openSlot := ""
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
					if !slotMatchesWindowSpec(sl.Date, sl.Time, windowSpec) {
						continue
					}
					ts := sl.Date + "T" + sl.Time
					if openSlot == "" || ts < openSlot {
						openSlot = ts
					}
				}
				r.Polled = true
				switch {
				case openSlot != "":
					r.HasMatch = true
					if windowSpec != "" {
						r.Reason = fmt.Sprintf("tock %s: open slot for party=%d matching %q at %s (7-day horizon)", slug, party, windowSpec, openSlot)
					} else {
						r.Reason = fmt.Sprintf("tock %s: open slot for party=%d at %s (7-day horizon)", slug, party, openSlot)
					}
				case windowSpec != "":
					r.Reason = fmt.Sprintf("tock %s: no open slots for party=%d matching %q in 7-day window from %s", slug, party, windowSpec, dateFrom)
				default:
					r.Reason = fmt.Sprintf("tock %s: no open slots for party=%d in 7-day window from %s", slug, party, dateFrom)
				}
				return r
			}
			if calErr != nil {
				r.Reason = fmt.Sprintf("tock %s: %v", slug, calErr)
				// Fall through to OT path; do NOT mark Polled.
			}
		}
	}
	if tryOT && !r.Polled {
		c, err := opentable.New(s)
		if err == nil {
			// OT's Autocomplete returns INTERNAL_SERVER_ERROR with lat=0/lng=0
			// (the upstream `personalizer-autocomplete/v4` requires a coordinate
			// to anchor on). resolveWatchAnchor implements the precedence:
			// (1) persisted GeoContext from watches.location_context (U15 —
			// pinned by `watch add --location`); (2) slug-suffix inference
			// (U8); (3) NYC default. U15 caught Codex P1-E: a slug without a
			// city suffix and a `--location 'portland, me'` was drifting back
			// to NYC at tick time because location_context was decorated on
			// the response but never persisted.
			anchorLat, anchorLng := resolveWatchAnchor(slug, locationContextJSON)
			restID, restName, _, rerr := c.RestaurantIDFromQuery(ctx, slug, anchorLat, anchorLng)
			if rerr == nil && restID != 0 {
				todayT := time.Now()
				// Loop one call per day. The new OT GraphQL gateway hardcodes
				// `forwardDays: 0` in the request body and silently discards
				// any larger value passed here, so a single call with
				// forwardDays=7 only returns today's slots. Mirror earliest.go's
				// per-day loop to actually scan a 7-day horizon.
				const watchHorizonDays = 7
				var avail []opentable.RestaurantAvailability
				var aerr error
				for d := 0; d < watchHorizonDays; d++ {
					dayStr := todayT.AddDate(0, 0, d).Format("2006-01-02")
					dayAvail, derr := c.RestaurantsAvailability(ctx, []int{restID}, dayStr, "19:00", party, 0, 210, 0, noCache)
					if derr != nil {
						aerr = derr
						break
					}
					avail = append(avail, dayAvail...)
				}
				if aerr == nil {
					r.Polled = true
					r.Network = "opentable"
					// Match the Tock path: filter slots by both isAvailable
					// AND windowSpec. Without windowSpec filtering here, a
					// watch created with `--window "sat 7-9pm"` would fire
					// on any OT opening including a Wednesday lunch.
					// Slot date = today + d.DayOffset; slot time = 19:00 +
					// s.TimeOffsetMinutes (same computation as earliest.go).
					anyOpen := false
				outer:
					for _, ra := range avail {
						for _, d := range ra.AvailabilityDays {
							slotDate := d.Date
							if slotDate == "" {
								slotDate = todayT.AddDate(0, 0, d.DayOffset).Format("2006-01-02")
							}
							for _, sl := range d.Slots {
								if !sl.IsAvailable {
									continue
								}
								totalMin := 19*60 + sl.TimeOffsetMinutes
								hh := ((totalMin/60)%24 + 24) % 24
								mm := ((totalMin % 60) + 60) % 60
								slotTime := fmt.Sprintf("%02d:%02d", hh, mm)
								if !slotMatchesWindowSpec(slotDate, slotTime, windowSpec) {
									continue
								}
								anyOpen = true
								break outer
							}
						}
					}
					switch {
					case anyOpen:
						r.HasMatch = true
						if windowSpec != "" {
							r.Reason = fmt.Sprintf("opentable %s: at least one open slot matching %q", restName, windowSpec)
						} else {
							r.Reason = fmt.Sprintf("opentable %s: at least one open slot found", restName)
						}
					case windowSpec != "":
						r.Reason = fmt.Sprintf("opentable %s: no open slots matching %q in 7d window for party=%d", restName, windowSpec, party)
					default:
						r.Reason = fmt.Sprintf("opentable %s: no open slots in 7d window for party=%d", restName, party)
					}
					return r
				}
				r.Network = "opentable"
				r.Polled = false
				r.Reason = fmt.Sprintf("opentable %s (id=%d): %v", restName, restID, aerr)
				return r
			}
		}
	}
	if !r.Polled && r.Reason == "" {
		r.Reason = "could not resolve venue on either network"
	}
	return r
}

// resolveWatchAnchor returns the (lat, lng) OT Autocomplete should
// anchor on for a watch at tick time. Precedence:
//  1. Persisted location_context JSON (U15) — unmarshaled to a
//     *GeoContext and its centroid used. Pinned by `watch add
//     --location` so an explicit caller intent survives across ticks
//     even when the slug carries no city suffix.
//  2. Slug-suffix inference via inferGeoContextFromSlug (U8) — covers
//     pre-migration rows and watches added without --location, but
//     only fires for slugs like `joey-bellevue` that carry a hint.
//  3. NYC default (40.7128, -74.0060) — final fallback for slugs with
//     no recognizable city suffix and no persisted location.
//
// Defensive: a malformed locationContextJSON blob falls through to (2)
// rather than erroring. The on-disk shape may change over time; a tick
// should never break because a row was written by a future binary.
func resolveWatchAnchor(slug, locationContextJSON string) (lat, lng float64) {
	if locationContextJSON != "" {
		var gc GeoContext
		if err := json.Unmarshal([]byte(locationContextJSON), &gc); err == nil {
			if gc.Centroid[0] != 0 || gc.Centroid[1] != 0 {
				return gc.Centroid[0], gc.Centroid[1]
			}
		}
	}
	if gc := inferGeoContextFromSlug(slug); gc != nil {
		return gc.Centroid[0], gc.Centroid[1]
	}
	return 40.7128, -74.0060
}

// pollOneWatchResy scans the next 7 days of Resy availability for the given
// venue id, applying the user's optional --window filter and returning a
// HasMatch=true result for the earliest qualifying slot. `slug` here is
// expected to be a numeric Resy venue id (validated at `watch add` time).
//
// Resy has no equivalent of Tock's 60-day calendar endpoint — each
// /4/find call is per-(venue, date, party), so we loop client-side. The
// poll budget is 7 days to mirror the OT and Tock branches; tighter
// cadences are appropriate for hot targets via per-watch `--cadence`.
func pollOneWatchResy(ctx context.Context, s *auth.Session, slug string, party int, windowSpec string, r tickResult) tickResult {
	r.Network = "resy"
	// Numeric venue id check runs BEFORE the auth check. This is the
	// belt-and-suspenders backstop for old watch rows that were written
	// before `watch add` learned to validate Resy slugs; for new rows
	// the add-time check rejects malformed input first. Validation
	// before auth means a bad-slug watch surfaces "fix your venue id"
	// instead of cycling auth_required forever after a token expires.
	if _, err := strconv.Atoi(slug); err != nil {
		r.Reason = fmt.Sprintf("resy: %q is not a numeric venue id; use `goat <name> --network resy` to discover ids", slug)
		return r
	}
	if s == nil || s.Resy == nil || s.Resy.AuthToken == "" {
		r.Reason = "resy: not authenticated; run `auth login --resy --email <you@example.com>` first"
		return r
	}
	client := resy.New(resy.Credentials{
		APIKey:    s.Resy.APIKey,
		AuthToken: s.Resy.AuthToken,
		Email:     s.Resy.Email,
	})
	today := time.Now()
	dateFrom := today.Format("2006-01-02")
	openSlot := ""
	successfulDays := 0
	var lastErr error
	for d := 0; d < 7; d++ {
		day := today.AddDate(0, 0, d).Format("2006-01-02")
		slots, err := client.Availability(ctx, resy.AvailabilityParams{
			VenueID:   slug,
			Date:      day,
			PartySize: party,
		})
		if err != nil {
			lastErr = err
			continue
		}
		successfulDays++
		for _, sl := range slots {
			if !slotMatchesWindowSpec(day, sl.Time, windowSpec) {
				continue
			}
			ts := day + "T" + sl.Time
			if openSlot == "" || ts < openSlot {
				openSlot = ts
			}
		}
	}
	// Polled is true only when at least one day's call succeeded. Setting
	// it unconditionally would let the caller persist last_polled_at after
	// a 7-day stretch of auth_expired or network errors, masking a
	// real outage as "polled, no slots found." If every day errored,
	// surface the last error in Reason and leave Polled=false so the
	// caller's update path skips the timestamp bump.
	if successfulDays == 0 {
		if lastErr != nil {
			r.Reason = fmt.Sprintf("resy venue=%s: every per-day call failed; last error: %v", slug, lastErr)
		} else {
			r.Reason = fmt.Sprintf("resy venue=%s: poll did not run any successful per-day calls", slug)
		}
		return r
	}
	r.Polled = true
	switch {
	case openSlot != "":
		r.HasMatch = true
		if windowSpec != "" {
			r.Reason = fmt.Sprintf("resy venue=%s: open slot for party=%d matching %q at %s (7-day horizon)", slug, party, windowSpec, openSlot)
		} else {
			r.Reason = fmt.Sprintf("resy venue=%s: open slot for party=%d at %s (7-day horizon)", slug, party, openSlot)
		}
	case windowSpec != "":
		r.Reason = fmt.Sprintf("resy venue=%s: no open slots for party=%d matching %q in 7-day window from %s", slug, party, windowSpec, dateFrom)
	default:
		r.Reason = fmt.Sprintf("resy venue=%s: no open slots for party=%d in 7-day window from %s", slug, party, dateFrom)
	}
	return r
}

func openWatchStore(flags *rootFlags) (*sql.DB, error) {
	dbPath := defaultDBPath("table-reservation-goat-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	if _, err := db.DB().ExecContext(context.Background(), watchSchemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("ensuring watches schema: %w", err)
	}
	// Upgrade existing tables that predate columns added by newer
	// binaries (U15 added location_context). CREATE TABLE IF NOT EXISTS
	// is a no-op when the table already exists, so the additive column
	// only lands via ALTER TABLE here.
	if err := ensureWatchSchemaUpgrades(context.Background(), db.DB()); err != nil {
		db.Close()
		return nil, fmt.Errorf("upgrading watches schema: %w", err)
	}
	// Returning the raw *sql.DB keeps watch SQL self-contained. The Store
	// wrapper lifecycle (Close) is shed because the only resource it owns is
	// this *sql.DB, which the caller is responsible for closing.
	return db.DB(), nil
}

func newWatchID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "wat_" + hex.EncodeToString(b)
}

// _ keeps strings/json imports stable.
var (
	_ = strings.TrimSpace
	_ = json.Marshal
)

// slotMatchesWindowSpec applies a minimal v1 matcher against the user's
// `--window` spec. Returns true when the spec is empty (no filter) OR
// when the slot's day-of-week and hour fall within the spec.
//
// Supported spec shapes (v1 — free-form strings stored from `watch add`):
//   - empty / blank → no filter (true for any slot)
//   - day-of-week prefix: "sat", "sun", "mon", "tue", "wed", "thu", "fri"
//     (case-insensitive; matches when slot's date falls on that day)
//   - hour range: "7pm-9pm", "7-9pm", "19:00-21:00" (24h or 12h, inclusive)
//   - combined: "sat 7-9pm" (both must match)
//
// A spec the matcher can't parse (e.g., "next saturday around dinner")
// returns true — better to over-fire a match than silently drop watches
// the user explicitly wanted polled. Date and time arguments are
// "YYYY-MM-DD" and "HH:MM"; malformed inputs return true.
func slotMatchesWindowSpec(date, hhmm, spec string) bool {
	spec = strings.ToLower(strings.TrimSpace(spec))
	if spec == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return true
	}
	hr, mn, perr := parseHHMM(hhmm)
	if perr != nil {
		return true
	}
	// Day-of-week filter — accept first 3 chars of day name as prefix.
	dayPrefix := strings.ToLower(t.Weekday().String()[:3])
	dayFilters := map[string]bool{}
	for _, p := range []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"} {
		if strings.Contains(spec, p) {
			dayFilters[p] = true
		}
	}
	if len(dayFilters) > 0 && !dayFilters[dayPrefix] {
		return false
	}
	// Hour range filter — look for "<hr>(am|pm)?-<hr>(am|pm)?" or "HH:MM-HH:MM".
	rangeRE := regexp.MustCompile(`(\d{1,2})(?::(\d{2}))?\s*(am|pm)?\s*[-–to ]+\s*(\d{1,2})(?::(\d{2}))?\s*(am|pm)?`)
	m := rangeRE.FindStringSubmatch(spec)
	if m == nil {
		// No parseable range — day-of-week filter alone passed (or spec was DOW-only).
		return true
	}
	startMin := hourMinute(m[1], m[2], m[3], m[6])
	endMin := hourMinute(m[4], m[5], m[6], m[6])
	slotMin := hr*60 + mn
	if startMin <= endMin {
		return slotMin >= startMin && slotMin <= endMin
	}
	// Range wraps midnight (rare for restaurants but defensive).
	return slotMin >= startMin || slotMin <= endMin
}

// parseHHMM parses an "HH:MM" 24-hour time string.
func parseHHMM(s string) (int, int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time %q", s)
	}
	hr, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	mn, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hr, mn, nil
}

// hourMinute converts a (hour-string, minute-string, period, fallback-period)
// quad from the rangeRE submatches into total minutes since midnight.
// The fallback-period covers ranges like "7-9pm" where only the second
// boundary carries an explicit am/pm.
func hourMinute(hStr, mStr, period, fallback string) int {
	h, _ := strconv.Atoi(hStr)
	m := 0
	if mStr != "" {
		m, _ = strconv.Atoi(mStr)
	}
	p := period
	if p == "" {
		p = fallback
	}
	if p == "pm" && h < 12 {
		h += 12
	} else if p == "am" && h == 12 {
		h = 0
	}
	return h*60 + m
}
