// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/pushpress/internal/store"

	"github.com/spf13/cobra"
)

// PushPress /v3 transcendence — local-store-driven reports that the API
// doesn't expose directly. All commands assume the user has already run
// `pushpress-pp-cli sync --full` (or `sync customers,checkins`) to populate
// the local SQLite store.
//
// The /v3 Customer schema is narrow (id, name, email, phone, address only —
// no plan/status/dateAdded), so reports that would need those fields are
// stubbed in stubs.go alongside the other /v2-gated commands. The reports
// here use only fields /v3 actually exposes.

// ---------- helpers shared by the transcendence commands ----------

// openTranscendenceStore opens the local store and returns it, along with a
// helper that closes it. The store is the same one `sync` writes to.
func openTranscendenceStore(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("pushpress-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nHint: run 'pushpress-pp-cli sync --full' first", err)
	}
	return db, nil
}

// epochCutoff returns the Unix epoch (seconds) for "now - days days ago".
// Check-in times in the local store are stored as Unix seconds (INTEGER).
func epochCutoff(days int) int64 {
	return time.Now().AddDate(0, 0, -days).Unix()
}

// formatEpoch renders an epoch-seconds value as a short RFC3339 date or "-"
// when zero/null.
func formatEpoch(secs int64) string {
	if secs == 0 {
		return "-"
	}
	return time.Unix(secs, 0).UTC().Format("2006-01-02")
}

// daysAgo returns days since the given Unix seconds, or -1 when secs is zero.
func daysAgo(secs int64) int {
	if secs == 0 {
		return -1
	}
	d := time.Since(time.Unix(secs, 0)).Hours() / 24
	if d < 0 {
		return 0
	}
	return int(d)
}

// ---------- going-dark ----------

func newGoingDarkCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string
	var limit int
	var includeNever bool

	cmd := &cobra.Command{
		Use:   "going-dark",
		Short: "List members whose most-recent check-in is older than N days",
		Long: "Joins synced customers with their most-recent check-in (by customer FK). Returns members whose last visit " +
			"falls before the cutoff, optionally including members who have never checked in. Pure local query — no API call.",
		Example:     "  pushpress-pp-cli going-dark --days 14 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := epochCutoff(days)

			// LEFT JOIN customers to their MAX(checkin_time). When the customer
			// has zero check-ins, the joined row is NULL — drop those when
			// --include-never is off.
			q := `
				SELECT c.id, c.name, c.email, c.phone,
				       COALESCE(MAX(k.checkin_time), 0) AS last_visit
				FROM "customers" c
				LEFT JOIN "checkins" k ON k.customer = c.id
				GROUP BY c.id
				HAVING (last_visit > 0 AND last_visit < ?) OR (last_visit = 0 AND ?)
				ORDER BY last_visit ASC
				LIMIT ?
			`
			rows, err := db.Query(q, cutoff, includeNever, limit)
			if err != nil {
				return fmt.Errorf("querying customers: %w", err)
			}
			defer rows.Close()

			type hit struct {
				ID          string `json:"id"`
				Name        string `json:"name,omitempty"`
				Email       string `json:"email,omitempty"`
				Phone       string `json:"phone,omitempty"`
				LastVisit   string `json:"last_visit_date,omitempty"`
				DaysSince   int    `json:"days_since_last_visit"`
				LastVisitTs int64  `json:"last_visit_unix,omitempty"`
			}
			var hits []hit
			for rows.Next() {
				var id, name, email, phone sql.NullString
				var lastVisit int64
				if err := rows.Scan(&id, &name, &email, &phone, &lastVisit); err != nil {
					continue
				}
				hits = append(hits, hit{
					ID:          id.String,
					Name:        nameDisplay(name),
					Email:       email.String,
					Phone:       phone.String,
					LastVisit:   formatEpoch(lastVisit),
					DaysSince:   daysAgo(lastVisit),
					LastVisitTs: lastVisit,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"days":        days,
					"cutoff_unix": cutoff,
					"count":       len(hits),
					"members":     hits,
				}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No members going dark beyond %d days.\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d member(s) going dark (no check-in in %d+ days):\n\n", len(hits), days)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5s  %-22s  %-22s  %s\n", "ID", "DAYS", "NAME", "EMAIL", "LAST VISIT")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5s  %-22s  %-22s  %s\n",
				strings.Repeat("-", 26), strings.Repeat("-", 5),
				strings.Repeat("-", 22), strings.Repeat("-", 22), strings.Repeat("-", 10))
			for _, h := range hits {
				days := fmt.Sprintf("%d", h.DaysSince)
				if h.DaysSince < 0 {
					days = "n/a"
				}
				name := truncate(h.Name, 22)
				email := truncate(h.Email, 22)
				fmt.Fprintf(cmd.OutOrStdout(), "  %-26s  %-5s  %-22s  %-22s  %s\n", h.ID, days, name, email, h.LastVisit)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Members with no check-in in this many days are 'going dark'")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max rows to return")
	cmd.Flags().BoolVar(&includeNever, "include-never", false, "Also include members who have NEVER checked in")
	return cmd
}

// ---------- recency ladder ----------

func newRecencyCmd(flags *rootFlags) *cobra.Command {
	var bucketsRaw string
	var dbPath string
	var samplePerBucket int

	cmd := &cobra.Command{
		Use:         "recency",
		Short:       "Histogram of members bucketed by days since last check-in",
		Long:        "Bucket every synced member by days-since-last-checkin. Returns count + a sample of names per bucket. Pure local query.",
		Example:     "  pushpress-pp-cli recency --bucket 7,14,30,60,90 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			boundaries, err := parseBuckets(bucketsRaw)
			if err != nil {
				return err
			}
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT c.id, c.name, COALESCE(MAX(k.checkin_time), 0) AS last_visit
				FROM "customers" c
				LEFT JOIN "checkins" k ON k.customer = c.id
				GROUP BY c.id
			`)
			if err != nil {
				return fmt.Errorf("querying customers: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				Label   string   `json:"label"`
				MaxDays int      `json:"max_days,omitempty"`
				Count   int      `json:"count"`
				Sample  []string `json:"sample,omitempty"`
			}
			buckets := make([]bucket, len(boundaries)+2)
			for i, b := range boundaries {
				buckets[i] = bucket{Label: fmt.Sprintf("0-%d days", b), MaxDays: b}
			}
			buckets[len(boundaries)] = bucket{Label: fmt.Sprintf("%d+ days", boundaries[len(boundaries)-1])}
			buckets[len(boundaries)+1] = bucket{Label: "never checked in"}

			for rows.Next() {
				var id, name sql.NullString
				var lastVisit int64
				if err := rows.Scan(&id, &name, &lastVisit); err != nil {
					continue
				}
				d := daysAgo(lastVisit)
				idx := len(boundaries) // default: "+over" bucket
				if d < 0 {
					idx = len(boundaries) + 1 // never bucket
				} else {
					for i, b := range boundaries {
						if d <= b {
							idx = i
							break
						}
					}
				}
				buckets[idx].Count++
				if displayName := nameDisplay(name); len(buckets[idx].Sample) < samplePerBucket && displayName != "" {
					buckets[idx].Sample = append(buckets[idx].Sample, displayName)
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"boundaries": boundaries,
					"buckets":    buckets,
				}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Recency ladder (days since last check-in):")
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "  %-22s  %-6s  %s\n", "BUCKET", "COUNT", "SAMPLE")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-22s  %-6s  %s\n", strings.Repeat("-", 22), strings.Repeat("-", 6), strings.Repeat("-", 30))
			for _, b := range buckets {
				sample := strings.Join(b.Sample, ", ")
				if len(sample) > 50 {
					sample = sample[:47] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-22s  %-6d  %s\n", b.Label, b.Count, sample)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&bucketsRaw, "bucket", "7,14,30,60,90", "Comma-separated day boundaries for the histogram")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	cmd.Flags().IntVar(&samplePerBucket, "sample", 3, "Max sample names per bucket")
	return cmd
}

// parseBuckets parses "7,14,30,60,90" → [7 14 30 60 90] sorted ascending.
func parseBuckets(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid bucket boundary %q: must be a positive integer", p)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--bucket cannot be empty")
	}
	sort.Ints(out)
	return out, nil
}

// ---------- roster ----------

func newRosterCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "roster",
		Short: "One line per synced member: id, name, email, phone, last_visit, days_since",
		Long: "Local join of customers × MAX(checkin.timestamp). Trainer-dashboard's default 'list my members' view. " +
			"Note: PushPress /v3 Customer schema does not expose plan/status fields — those will appear only when the /v2 follow-up is wired.",
		Example:     "  pushpress-pp-cli roster --json --limit 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT c.id, c.name, c.email, c.phone,
				       COALESCE(MAX(k.checkin_time), 0) AS last_visit
				FROM "customers" c
				LEFT JOIN "checkins" k ON k.customer = c.id
				GROUP BY c.id
				ORDER BY last_visit DESC NULLS LAST
				LIMIT ?
			`, limit)
			if err != nil {
				return fmt.Errorf("querying roster: %w", err)
			}
			defer rows.Close()

			type member struct {
				ID          string `json:"id"`
				Name        string `json:"name,omitempty"`
				Email       string `json:"email,omitempty"`
				Phone       string `json:"phone,omitempty"`
				LastVisit   string `json:"last_visit_date,omitempty"`
				DaysSince   int    `json:"days_since_last_visit"`
				LastVisitTs int64  `json:"last_visit_unix,omitempty"`
			}
			var members []member
			for rows.Next() {
				var id, name, email, phone sql.NullString
				var lastVisit int64
				if err := rows.Scan(&id, &name, &email, &phone, &lastVisit); err != nil {
					continue
				}
				members = append(members, member{
					ID:          id.String,
					Name:        nameDisplay(name),
					Email:       email.String,
					Phone:       phone.String,
					LastVisit:   formatEpoch(lastVisit),
					DaysSince:   daysAgo(lastVisit),
					LastVisitTs: lastVisit,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"count":   len(members),
					"members": members,
				}, flags)
			}
			if len(members) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No synced members. Run 'pushpress-pp-cli sync --full' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-22s  %-22s  %-14s  %-5s  %s\n",
				"ID", "NAME", "EMAIL", "PHONE", "DAYS", "LAST VISIT")
			fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-22s  %-22s  %-14s  %-5s  %s\n",
				strings.Repeat("-", 26), strings.Repeat("-", 22), strings.Repeat("-", 22),
				strings.Repeat("-", 14), strings.Repeat("-", 5), strings.Repeat("-", 10))
			for _, m := range members {
				days := fmt.Sprintf("%d", m.DaysSince)
				if m.DaysSince < 0 {
					days = "n/a"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-26s  %-22s  %-22s  %-14s  %-5s  %s\n",
					m.ID, truncate(m.Name, 22), truncate(m.Email, 22), truncate(m.Phone, 14), days, m.LastVisit)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Max rows to return")
	return cmd
}

// ---------- kpi today ----------

func newKPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "kpi",
		Short:       "Cross-entity KPI tickers from the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newKPITodayCmd(flags))
	return cmd
}

func newKPITodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "today",
		Short: "One-line metric ticker for today (check-ins, total synced members, going-dark counts)",
		Long: "Aggregates from the local store. Note: `signups_today` is NOT computed because PushPress /v3 does not expose " +
			"`dateAdded` on the Customer schema — see `signups recent --help` for the /v2 follow-up.",
		Example:     "  pushpress-pp-cli kpi today --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			now := time.Now()
			midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
			cutoff14 := epochCutoff(14)
			cutoff30 := epochCutoff(30)

			report := map[string]any{
				"window":         now.Format("2006-01-02"),
				"midnight_unix":  midnight,
				"members_synced": scalarInt(db.DB(), `SELECT COUNT(*) FROM "customers"`),
				"checkins_today": scalarInt(db.DB(), `SELECT COUNT(*) FROM "checkins" WHERE checkin_time >= ?`, midnight),
				"checkins_total": scalarInt(db.DB(), `SELECT COUNT(*) FROM "checkins"`),
				"going_dark_14d": goingDarkCount(db.DB(), cutoff14),
				"going_dark_30d": goingDarkCount(db.DB(), cutoff30),
				"signups_today":  nil, // gated on /v3 dateAdded field which isn't exposed
				"signups_status": "not supported by /v3 (no Customer.dateAdded field) — see `signups recent --help`",
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"KPI %s | members:%v checkins_today:%v going_dark_14d:%v going_dark_30d:%v (signups: /v2-gated)\n",
				report["window"],
				report["members_synced"], report["checkins_today"],
				report["going_dark_14d"], report["going_dark_30d"],
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	return cmd
}

// scalarInt runs a single-int SELECT and returns 0 on any error. Used for
// KPI tickers where missing tables / empty stores must not crash.
func scalarInt(db *sql.DB, query string, args ...any) int {
	var n int
	row := db.QueryRow(query, args...)
	_ = row.Scan(&n)
	return n
}

func goingDarkCount(db *sql.DB, cutoff int64) int {
	row := db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT c.id, COALESCE(MAX(k.checkin_time), 0) AS last_visit
			FROM "customers" c
			LEFT JOIN "checkins" k ON k.customer = c.id
			GROUP BY c.id
			HAVING last_visit > 0 AND last_visit < ?
		)
	`, cutoff)
	var n int
	_ = row.Scan(&n)
	return n
}

// ---------- member 360 ----------

// jsonOrString returns a decoded object/array when s parses as JSON, or
// the raw trimmed string otherwise. PushPress /v3 stores name and
// address as JSON-text columns, so passing them through as plain
// strings would double-encode them in the output map.
func jsonOrString(s sql.NullString) any {
	if !s.Valid {
		return nil
	}
	trimmed := strings.TrimSpace(s.String)
	if trimmed == "" {
		return s.String
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var v any
		if err := json.Unmarshal([]byte(trimmed), &v); err == nil {
			return v
		}
	}
	return s.String
}

// nameDisplay returns a human-friendly one-line name when the column
// holds a JSON name object ({first, last, nickname, ...}); falls back
// to the raw string otherwise.
func nameDisplay(s sql.NullString) string {
	v := jsonOrString(s)
	switch n := v.(type) {
	case nil:
		return ""
	case string:
		return n
	case map[string]any:
		first, _ := n["first"].(string)
		last, _ := n["last"].(string)
		joined := strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
		if joined != "" {
			return joined
		}
		if d, ok := n["display"].(string); ok && d != "" {
			return d
		}
	}
	return s.String
}

func newMemberCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var recentLimit int

	cmd := &cobra.Command{
		Use:   "member <id-or-email>",
		Short: "Single-page profile: customer + last 10 check-ins + streak + cadence trend",
		Long: "One command, one screen, for pre-session coach prep. Looks up the customer by id or email in the local store " +
			"(or fetches live if not synced), then attaches the last N check-ins, current streak, and last-4-weeks-vs-prior-4-weeks " +
			"cadence trend. Note: plan/status fields aren't in PushPress /v3 — those appear as `null` until the /v2 follow-up lands.",
		Example:     "  pushpress-pp-cli member user@example.com\n  pushpress-pp-cli member 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.TrimSpace(args[0])
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			row := db.DB().QueryRow(`
				SELECT id, name, email, phone, address, data
				FROM "customers"
				WHERE id = ? OR LOWER(email) = LOWER(?)
				LIMIT 1
			`, needle, needle)
			var custID, name, email, phone, address sql.NullString
			var raw []byte
			if err := row.Scan(&custID, &name, &email, &phone, &address, &raw); err != nil {
				return fmt.Errorf("no member matching %q in the local store. Run 'pushpress-pp-cli sync --full' or try `customers get %s`", needle, needle)
			}

			// Recent check-ins
			ckRows, _ := db.DB().Query(`
				SELECT id, name, checkin_time, data
				FROM "checkins"
				WHERE customer = ?
				ORDER BY checkin_time DESC
				LIMIT ?
			`, custID.String, recentLimit)
			type ck struct {
				ID         string `json:"id"`
				Name       string `json:"name,omitempty"`
				At         string `json:"at,omitempty"`
				AtUnix     int64  `json:"at_unix,omitempty"`
				RawDetails any    `json:"raw,omitempty"`
			}
			var recent []ck
			var allCheckinsTs []int64
			if ckRows != nil {
				defer ckRows.Close()
				for ckRows.Next() {
					var id, cname sql.NullString
					var ts int64
					var raw []byte
					_ = ckRows.Scan(&id, &cname, &ts, &raw)
					recent = append(recent, ck{
						ID:     id.String,
						Name:   cname.String,
						At:     formatEpoch(ts),
						AtUnix: ts,
					})
				}
			}
			// Pull every check-in for cadence math (cheap; one member's check-ins is bounded)
			allRows, _ := db.DB().Query(`SELECT checkin_time FROM "checkins" WHERE customer = ? AND checkin_time > 0 ORDER BY checkin_time DESC`, custID.String)
			if allRows != nil {
				defer allRows.Close()
				for allRows.Next() {
					var ts int64
					_ = allRows.Scan(&ts)
					allCheckinsTs = append(allCheckinsTs, ts)
				}
			}
			lastVisit := int64(0)
			if len(allCheckinsTs) > 0 {
				lastVisit = allCheckinsTs[0]
			}
			streak := currentDayStreak(allCheckinsTs)
			recentCadence, priorCadence := cadenceTrend(allCheckinsTs)

			view := map[string]any{
				"id":                    custID.String,
				"name":                  jsonOrString(name),
				"email":                 email.String,
				"phone":                 phone.String,
				"address":               jsonOrString(address),
				"plan":                  nil,
				"status":                nil,
				"plan_status_note":      "not exposed by PushPress /v3 (gated on /v2 browser-sniff follow-up)",
				"total_checkins":        len(allCheckinsTs),
				"last_visit_date":       formatEpoch(lastVisit),
				"last_visit_unix":       lastVisit,
				"days_since_last_visit": daysAgo(lastVisit),
				"current_day_streak":    streak,
				"cadence_last_4_weeks":  recentCadence,
				"cadence_prior_4_weeks": priorCadence,
				"trend":                 cadenceLabel(recentCadence, priorCadence),
				"recent_checkins":       recent,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Member: %s\n", custID.String)
			fmt.Fprintf(cmd.OutOrStdout(), "  Name:   %s\n", nameDisplay(name))
			fmt.Fprintf(cmd.OutOrStdout(), "  Email:  %s\n", email.String)
			fmt.Fprintf(cmd.OutOrStdout(), "  Phone:  %s\n", phone.String)
			fmt.Fprintf(cmd.OutOrStdout(), "  Plan / Status: (not exposed by /v3)\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Total check-ins: %d\n", len(allCheckinsTs))
			fmt.Fprintf(cmd.OutOrStdout(), "  Last visit:      %s  (%d days ago)\n", formatEpoch(lastVisit), daysAgo(lastVisit))
			fmt.Fprintf(cmd.OutOrStdout(), "  Current streak:  %d days\n", streak)
			fmt.Fprintf(cmd.OutOrStdout(), "  Cadence:         last-4w %.1f/wk  vs  prior-4w %.1f/wk  (%s)\n",
				recentCadence, priorCadence, cadenceLabel(recentCadence, priorCadence))
			if len(recent) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Last %d check-ins:\n", len(recent))
				for _, c := range recent {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s  %s\n", c.At, c.Name)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	cmd.Flags().IntVar(&recentLimit, "recent", 10, "Number of recent check-ins to include in the view")
	return cmd
}

// currentDayStreak returns the number of consecutive days (counting back from
// today) in which the member checked in. Zero if today wasn't a check-in day.
func currentDayStreak(timestamps []int64) int {
	if len(timestamps) == 0 {
		return 0
	}
	// Build a set of "YYYY-MM-DD" strings for which the member checked in.
	days := make(map[string]struct{}, len(timestamps))
	for _, ts := range timestamps {
		days[time.Unix(ts, 0).UTC().Format("2006-01-02")] = struct{}{}
	}
	now := time.Now().UTC()
	streak := 0
	for offset := 0; offset < 366; offset++ {
		day := now.AddDate(0, 0, -offset).Format("2006-01-02")
		if _, ok := days[day]; !ok {
			break
		}
		streak++
	}
	return streak
}

// cadenceTrend returns (checkins/week in last 4 weeks, checkins/week in the
// prior 4 weeks). Useful for "is this member trending down".
func cadenceTrend(timestamps []int64) (recent float64, prior float64) {
	now := time.Now()
	cutoffRecent := now.AddDate(0, 0, -28).Unix()
	cutoffPrior := now.AddDate(0, 0, -56).Unix()
	for _, ts := range timestamps {
		if ts >= cutoffRecent {
			recent++
		} else if ts >= cutoffPrior {
			prior++
		}
	}
	return recent / 4.0, prior / 4.0
}

func cadenceLabel(recent, prior float64) string {
	if prior == 0 && recent == 0 {
		return "no activity"
	}
	if prior == 0 {
		return "new attendance"
	}
	delta := (recent - prior) / prior
	switch {
	case delta < -0.25:
		return "trending DOWN"
	case delta > 0.25:
		return "trending UP"
	default:
		return "steady"
	}
}

// ---------- class-mix ----------

func newClassMixCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "class-mix",
		Short: "Histogram of class names from local check-ins over a window",
		Long: "Reads `checkins.name` (the class/event name promoted to a column) from the local store and computes counts " +
			"+ percent share per class over the window. Only class-side signal possible without /v2 calendar endpoints.",
		Example:     "  pushpress-pp-cli class-mix --days 30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openTranscendenceStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := epochCutoff(days)
			rows, err := db.Query(`
				SELECT name, COUNT(*) AS n
				FROM "checkins"
				WHERE name IS NOT NULL AND name != '' AND checkin_time >= ?
				GROUP BY name
				ORDER BY n DESC
			`, cutoff)
			if err != nil {
				return fmt.Errorf("querying check-ins: %w", err)
			}
			defer rows.Close()

			type bucket struct {
				ClassName string  `json:"class_name"`
				Count     int     `json:"count"`
				Share     float64 `json:"share"`
			}
			var hits []bucket
			total := 0
			for rows.Next() {
				var name sql.NullString
				var n int
				if err := rows.Scan(&name, &n); err != nil {
					continue
				}
				hits = append(hits, bucket{ClassName: name.String, Count: n})
				total += n
			}
			for i := range hits {
				if total > 0 {
					hits[i].Share = float64(hits[i].Count) / float64(total)
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"days":           days,
					"window_start":   formatEpoch(cutoff),
					"total_checkins": total,
					"classes":        hits,
				}, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No class check-ins in the last %d days.\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Class mix (last %d days, %d total check-ins):\n\n", days, total)
			fmt.Fprintf(cmd.OutOrStdout(), "  %-7s  %-7s  %s\n", "COUNT", "SHARE", "CLASS")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-7s  %-7s  %s\n", strings.Repeat("-", 7), strings.Repeat("-", 7), strings.Repeat("-", 30))
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-7d  %5.1f%%  %s\n", h.Count, h.Share*100, h.ClassName)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Look back this many days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/pushpress-pp-cli/data.db)")
	return cmd
}

// truncate is defined in helpers.go (generator-emitted). Reuse rather than redefine.

// Reference the encoding/json import so it isn't dropped by goimports when we
// later remove the explicit json.Encoder usage. Keeps the file imports stable
// across the file's evolution.
var _ = json.Marshal
