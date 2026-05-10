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
  match_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_watches_state ON watches(state);
`

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
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Local cross-network cancellation watcher",
		Long: "Persistent watches across OpenTable and Tock. The local SQLite watch " +
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
		party  int
		window string
		notify string
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
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), watchRow{
					ID: "watch_dryrun", Venue: args[0], PartySize: party, State: "active",
					CreatedAt: time.Now().UTC(),
				}, flags)
			}
			db, err := openWatchStore(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			network, slug := parseNetworkSlug(args[0])
			if network == "" {
				network = "auto"
			}
			id := newWatchID()
			row := watchRow{
				ID: id, Venue: args[0], Network: network, Slug: slug,
				PartySize: party, WindowSpec: window, Notify: notify,
				State: "active", CreatedAt: time.Now().UTC(),
			}
			_, err = db.ExecContext(cmd.Context(),
				`INSERT INTO watches (id, venue, network, slug, party_size, window_spec, notify, state)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				row.ID, row.Venue, row.Network, row.Slug, row.PartySize,
				row.WindowSpec, row.Notify, row.State,
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
				 created_at, last_polled_at, last_match_at, match_count
				 FROM watches ` + where + ` ORDER BY created_at DESC`
			rows, err := db.QueryContext(cmd.Context(), query, argsSQL...)
			if err != nil {
				return fmt.Errorf("query watches: %w", err)
			}
			defer rows.Close()
			out := []watchRow{}
			for rows.Next() {
				var r watchRow
				var window, notify sql.NullString
				var lastPolled, lastMatch sql.NullTime
				var created time.Time
				if err := rows.Scan(&r.ID, &r.Venue, &r.Network, &r.Slug, &r.PartySize,
					&window, &notify, &r.State, &created, &lastPolled, &lastMatch, &r.MatchCount); err != nil {
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
				`SELECT id, venue, network, slug, party_size, COALESCE(window_spec, '') FROM watches WHERE state = 'active' ORDER BY created_at`)
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
					id, venue, network, slug, windowSpec string
					party                                int
				)
				if err := rows.Scan(&id, &venue, &network, &slug, &party, &windowSpec); err != nil {
					return fmt.Errorf("scan watch: %w", err)
				}
				r := pollOneWatch(ctx, session, id, venue, network, slug, party, windowSpec, noCache)
				results = append(results, r)
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

func pollOneWatch(ctx context.Context, s *auth.Session, id, venue, network, slug string, party int, windowSpec string, noCache bool) tickResult {
	r := tickResult{WatchID: id, Venue: venue, Network: network, PolledAt: time.Now().UTC().Format(time.RFC3339), WindowSpec: windowSpec}
	tryOT := network == "auto" || network == "opentable"
	tryTock := network == "auto" || network == "tock"
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
			// to anchor on). Default to NYC — same approach earliest.go uses.
			// The matcher still finds the venue regardless of metro.
			restID, restName, _, rerr := c.RestaurantIDFromQuery(ctx, slug, 40.7128, -74.0060)
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
