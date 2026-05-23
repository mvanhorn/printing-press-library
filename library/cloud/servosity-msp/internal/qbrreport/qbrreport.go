// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

// Package qbrreport assembles and renders the Quarterly Business Review
// backup report for a single client. The package owns the data shape, the
// SQL joins against the local sync store, and three output renderers
// (Markdown, HTML, PDF-via-Chrome). The PDF renderer is a thin wrapper
// over RenderHTML + a headless-Chrome subprocess; if Chrome is not
// installed callers get a clean error suggesting --format md/html.
package qbrreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/snapshot"
)

// Company is the cover-page identification for a QBR.
type Company struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Reseller is the MSP partner name pulled from current_user so the cover
// page reads "Prepared by <MSP name>" instead of generic Servosity text.
type Reseller struct {
	Name string `json:"name"`
}

// DeviceRow is one row in the backup-coverage table: a device or host
// joined across all backup engines that protect it. Each engine string is
// empty when the device is not protected by that engine; the renderer
// shows checkmarks for non-empty fields.
type DeviceRow struct {
	Device         string    `json:"device"`
	Classic        string    `json:"classic,omitempty"`
	Restic         string    `json:"restic,omitempty"`
	DR             string    `json:"dr,omitempty"`
	LastSuccessful time.Time `json:"last_successful,omitempty"`
	Status         string    `json:"status,omitempty"`
}

// SuccessStats is the headline job-success number plus the breakdown that
// produced it. v1 derives this from a join over backups/restic_backups/
// dr_backups; rows missing a created_at fall through into the "unknown"
// bucket so the rate denominator stays honest.
type SuccessStats struct {
	Total            int     `json:"total"`
	HardFail         int     `json:"hard_fail"`
	Dirty            int     `json:"dirty"`
	RetriedSucceeded int     `json:"retried_then_succeeded"`
	Rate             float64 `json:"rate_pct"`
}

// RestoreEvent is one row in the restore-tests section. v1 surfaces DR
// reboot / snapshot events from dr_backups_* tables that occurred within
// the quarter's date range.
type RestoreEvent struct {
	Date    time.Time `json:"date"`
	Device  string    `json:"device"`
	Outcome string    `json:"outcome"`
	Notes   string    `json:"notes,omitempty"`
}

// Issue is one currently-open issue for the client. Closed/resolved/
// archived/ignored issues are filtered out at assembly time.
type Issue struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Severity string    `json:"severity,omitempty"`
	OpenedAt time.Time `json:"opened_at,omitempty"`
}

// TrendPoint is one (timestamp, storage_bytes) point on the storage trend
// line. Only populated when the snapshot store has history.
type TrendPoint struct {
	At    time.Time `json:"at"`
	Bytes int64     `json:"storage_bytes"`
}

// Report is the full QBR payload — what gets rendered to MD/HTML/PDF and
// what `--json` emits to stdout for downstream consumers.
type Report struct {
	Company      Company        `json:"company"`
	Reseller     Reseller       `json:"reseller"`
	Quarter      string         `json:"quarter"`
	From         time.Time      `json:"from"`
	To           time.Time      `json:"to"`
	GeneratedAt  time.Time      `json:"generated_at"`
	Coverage     []DeviceRow    `json:"coverage"`
	SuccessRate  SuccessStats   `json:"success_rate"`
	RestoreTests []RestoreEvent `json:"restore_tests"`
	OpenIssues   []Issue        `json:"open_issues"`
	StorageTrend []TrendPoint   `json:"storage_trend,omitempty"`
	TopIssue     string         `json:"top_issue,omitempty"`
}

// ParseQuarter turns "2026-Q1" into the [from, to) UTC window. The window
// is half-open so a job at 23:59:59 on the last day counts; a job at
// 00:00:00 on the next quarter's first day does not.
func ParseQuarter(q string) (from, to time.Time, err error) {
	parts := strings.Split(q, "-Q")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid quarter %q (want YYYY-QN)", q)
	}
	var year, qn int
	if _, err := fmt.Sscanf(parts[0], "%d", &year); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid year in %q: %w", q, err)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &qn); err != nil || qn < 1 || qn > 4 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid quarter number in %q (want 1-4)", q)
	}
	startMonth := time.Month(((qn - 1) * 3) + 1)
	from = time.Date(year, startMonth, 1, 0, 0, 0, 0, time.UTC)
	to = from.AddDate(0, 3, 0)
	return from, to, nil
}

// CurrentQuarter returns the YYYY-QN label for `now`.
func CurrentQuarter(now time.Time) string {
	qn := ((int(now.Month()) - 1) / 3) + 1
	return fmt.Sprintf("%d-Q%d", now.Year(), qn)
}

// Assemble pulls the data for a single client's QBR out of the local
// SQLite store. Every section degrades gracefully: missing tables / empty
// rows yield empty slices rather than errors, so a partially-synced store
// still produces a useful report (and the renderers show "No data" where
// appropriate). Hard errors (db handle invalid, malformed quarter) bubble
// up so the caller can surface a real failure to the user.
func Assemble(ctx context.Context, db *sql.DB, companyID int64, quarter string) (*Report, error) {
	from, to, err := ParseQuarter(quarter)
	if err != nil {
		return nil, err
	}
	r := &Report{
		Quarter:     quarter,
		From:        from,
		To:          to,
		GeneratedAt: time.Now().UTC(),
	}

	if err := loadCompany(ctx, db, companyID, r); err != nil {
		return nil, err
	}
	loadReseller(ctx, db, r)

	r.Coverage = loadCoverage(ctx, db, companyID)
	r.SuccessRate = loadSuccessStats(ctx, db, companyID, from, to)
	r.RestoreTests = loadRestoreTests(ctx, db, companyID, from, to)
	r.OpenIssues = loadOpenIssues(ctx, db, companyID)
	r.StorageTrend = loadStorageTrend(ctx, db, companyID, from, to)
	r.TopIssue = topIssueLabel(r.OpenIssues)

	return r, nil
}

// LookupCompany resolves the user-supplied <company> argument to a single
// company row. The argument may be a numeric ID (exact match on
// companies.id) or a name substring (case-insensitive). When the
// substring matches more than one company the caller gets a sentinel
// AmbiguousError listing the matches so the CLI layer can print a useful
// hint.
func LookupCompany(ctx context.Context, db *sql.DB, arg string) (Company, error) {
	// Numeric ID path — exact match against companies.id (TEXT in store).
	if isAllDigits(arg) {
		var (
			id   string
			name sql.NullString
		)
		err := db.QueryRowContext(ctx,
			`SELECT id, COALESCE(name, '') FROM companies WHERE id = ? LIMIT 1`, arg,
		).Scan(&id, &name)
		if err == nil {
			parsedID, _ := parseInt64(id)
			return Company{ID: parsedID, Name: name.String}, nil
		}
		if err != sql.ErrNoRows {
			return Company{}, fmt.Errorf("lookup company by id: %w", err)
		}
		// Fall through and treat the digit string as a name substring; some
		// MSPs name clients with pure-digit identifiers ("12345 LLC").
	}

	pattern := "%" + strings.ToLower(arg) + "%"
	rows, err := db.QueryContext(ctx,
		`SELECT id, COALESCE(name, '') FROM companies
		  WHERE LOWER(COALESCE(name, '')) LIKE ?
		  ORDER BY name ASC LIMIT 20`,
		pattern,
	)
	if err != nil {
		return Company{}, fmt.Errorf("lookup company by name: %w", err)
	}
	defer rows.Close()

	var matches []Company
	for rows.Next() {
		var id string
		var name sql.NullString
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		parsedID, _ := parseInt64(id)
		matches = append(matches, Company{ID: parsedID, Name: name.String})
	}
	if err := rows.Err(); err != nil {
		return Company{}, fmt.Errorf("iterating company matches: %w", err)
	}
	switch len(matches) {
	case 0:
		return Company{}, fmt.Errorf("no company matched %q (run 'servosity-msp-pp-cli sync' if your store is empty)", arg)
	case 1:
		return matches[0], nil
	default:
		return Company{}, &AmbiguousError{Arg: arg, Matches: matches}
	}
}

// AmbiguousError reports that a <company> substring matched more than
// one row. The CLI layer detects it via errors.As and prints the matches
// as a hint.
type AmbiguousError struct {
	Arg     string
	Matches []Company
}

func (e *AmbiguousError) Error() string {
	names := make([]string, 0, len(e.Matches))
	for _, m := range e.Matches {
		names = append(names, fmt.Sprintf("%d %s", m.ID, m.Name))
	}
	return fmt.Sprintf("ambiguous company %q matched %d rows: %s", e.Arg, len(e.Matches), strings.Join(names, "; "))
}

// ---- data loaders -------------------------------------------------------

func loadCompany(ctx context.Context, db *sql.DB, id int64, r *Report) error {
	var (
		idStr string
		name  sql.NullString
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, COALESCE(name, '') FROM companies WHERE id = ? LIMIT 1`,
		fmt.Sprintf("%d", id),
	).Scan(&idStr, &name)
	if err == sql.ErrNoRows {
		return fmt.Errorf("company id %d not found in local store (run 'servosity-msp-pp-cli sync')", id)
	}
	if err != nil {
		return fmt.Errorf("load company: %w", err)
	}
	parsedID, _ := parseInt64(idStr)
	r.Company = Company{ID: parsedID, Name: name.String}
	return nil
}

// loadReseller pulls a best-guess MSP name from current_user. Failures
// are non-fatal — we fall back to "Servosity Partner" on the cover page.
func loadReseller(ctx context.Context, db *sql.DB, r *Report) {
	var first, last sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(first_name, ''), COALESCE(last_name, '') FROM current_user LIMIT 1`,
	).Scan(&first, &last)
	if err != nil {
		r.Reseller = Reseller{Name: "Servosity Partner"}
		return
	}
	full := strings.TrimSpace(first.String + " " + last.String)
	if full == "" {
		full = "Servosity Partner"
	}
	r.Reseller = Reseller{Name: full}
}

// loadCoverage builds the device/engine matrix. v1 union-joins the three
// backup tables on the company column, then groups by display_name (the
// human-readable device label) so a single host running multiple engines
// shows up as one row with multiple engine columns populated.
func loadCoverage(ctx context.Context, db *sql.DB, companyID int64) []DeviceRow {
	type rawRow struct {
		Device  string
		Engine  string
		Login   string
		Created sql.NullTime
		State   sql.NullString
	}
	var raws []rawRow

	cid := fmt.Sprintf("%d", companyID)

	// Classic backups
	rows, err := db.QueryContext(ctx, `
		SELECT COALESCE(display_name, login, id) AS device,
		       COALESCE(login, ''),
		       created_at,
		       COALESCE(state, '')
		  FROM backups
		 WHERE json_extract(data, '$.company') = ?
		    OR json_extract(data, '$.company') = ?`,
		cid, companyID,
	)
	if err == nil {
		for rows.Next() {
			var rr rawRow
			rr.Engine = "classic"
			if scanErr := rows.Scan(&rr.Device, &rr.Login, &rr.Created, &rr.State); scanErr == nil {
				raws = append(raws, rr)
			}
		}
		rows.Close()
	}

	// Restic backups — has a dedicated company column.
	rows, err = db.QueryContext(ctx, `
		SELECT COALESCE(display_name, device_name, id) AS device,
		       COALESCE(device_name, ''),
		       created_at,
		       COALESCE(state, '')
		  FROM restic_backups
		 WHERE company = ? OR company = ?`,
		cid, companyID,
	)
	if err == nil {
		for rows.Next() {
			var rr rawRow
			rr.Engine = "restic"
			if scanErr := rows.Scan(&rr.Device, &rr.Login, &rr.Created, &rr.State); scanErr == nil {
				raws = append(raws, rr)
			}
		}
		rows.Close()
	}

	// DR backups — store-level table has no top-level company column, but
	// the JSON `data` blob includes it.
	rows, err = db.QueryContext(ctx, `
		SELECT COALESCE(json_extract(data, '$.display_name'),
		                json_extract(data, '$.device_name'),
		                id) AS device,
		       COALESCE(json_extract(data, '$.login'), ''),
		       NULL,
		       COALESCE(json_extract(data, '$.state'), '')
		  FROM dr_backups
		 WHERE json_extract(data, '$.company') = ?
		    OR json_extract(data, '$.company') = ?`,
		cid, companyID,
	)
	if err == nil {
		for rows.Next() {
			var rr rawRow
			rr.Engine = "dr"
			if scanErr := rows.Scan(&rr.Device, &rr.Login, &rr.Created, &rr.State); scanErr == nil {
				raws = append(raws, rr)
			}
		}
		rows.Close()
	}

	// Group by device label.
	byDevice := map[string]*DeviceRow{}
	for _, rr := range raws {
		dev := strings.TrimSpace(rr.Device)
		if dev == "" {
			dev = "(unnamed)"
		}
		d, ok := byDevice[dev]
		if !ok {
			d = &DeviceRow{Device: dev}
			byDevice[dev] = d
		}
		switch rr.Engine {
		case "classic":
			d.Classic = nonEmpty(rr.Login, "yes")
		case "restic":
			d.Restic = nonEmpty(rr.Login, "yes")
		case "dr":
			d.DR = nonEmpty(rr.Login, "yes")
		}
		if rr.Created.Valid && rr.Created.Time.After(d.LastSuccessful) {
			d.LastSuccessful = rr.Created.Time
		}
		if rr.State.Valid && rr.State.String != "" && d.Status == "" {
			d.Status = rr.State.String
		}
	}

	out := make([]DeviceRow, 0, len(byDevice))
	for _, d := range byDevice {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Device) < strings.ToLower(out[j].Device) })
	return out
}

// loadSuccessStats counts jobs and failures for the quarter. v1 uses the
// `state` column as the success/failure signal; the press has no failure-
// log table to join, so this is the best read at this scope. Rows
// without a created_at fall outside the quarter window and don't count.
func loadSuccessStats(ctx context.Context, db *sql.DB, companyID int64, from, to time.Time) SuccessStats {
	cid := fmt.Sprintf("%d", companyID)
	var stats SuccessStats

	type stateCount struct {
		state string
		n     int
	}

	queries := []struct {
		name string
		sql  string
	}{
		{
			"classic",
			`SELECT COALESCE(state,''), COUNT(*) FROM backups
			  WHERE (json_extract(data, '$.company') = ? OR json_extract(data, '$.company') = ?)
			    AND created_at >= ? AND created_at < ?
			  GROUP BY state`,
		},
		{
			"restic",
			`SELECT COALESCE(state,''), COUNT(*) FROM restic_backups
			  WHERE (company = ? OR company = ?)
			    AND created_at >= ? AND created_at < ?
			  GROUP BY state`,
		},
	}

	for _, q := range queries {
		rows, err := db.QueryContext(ctx, q.sql, cid, companyID, from.Format(time.RFC3339), to.Format(time.RFC3339))
		if err != nil {
			continue
		}
		for rows.Next() {
			var sc stateCount
			if err := rows.Scan(&sc.state, &sc.n); err != nil {
				continue
			}
			stats.Total += sc.n
			switch strings.ToLower(sc.state) {
			case "failed", "error", "hard_fail", "hardfail":
				stats.HardFail += sc.n
			case "dirty", "warning":
				stats.Dirty += sc.n
			case "retried", "retried_then_succeeded":
				stats.RetriedSucceeded += sc.n
			}
		}
		rows.Close()
	}

	if stats.Total > 0 {
		success := stats.Total - stats.HardFail - stats.Dirty
		stats.Rate = (float64(success) / float64(stats.Total)) * 100.0
	}
	return stats
}

// loadRestoreTests pulls DR test events from dr_backups_snapshots and
// dr_backups_reboot_events for the quarter window. Failures here are
// silent: the report's restore-tests section just shows "no tests" when
// the data isn't available.
func loadRestoreTests(ctx context.Context, db *sql.DB, companyID int64, from, to time.Time) []RestoreEvent {
	cid := fmt.Sprintf("%d", companyID)
	var events []RestoreEvent

	// dr_backups_snapshots is keyed by dr_backups_id; join up to dr_backups
	// to filter by company. v1: we accept "any snapshot taken during the
	// quarter" as a restore-test event since the press doesn't expose a
	// dedicated "restore test" event type.
	rows, err := db.QueryContext(ctx, `
		SELECT json_extract(s.data, '$.created_at'),
		       COALESCE(json_extract(d.data, '$.display_name'), json_extract(d.data, '$.login'), s.dr_backups_id),
		       COALESCE(json_extract(s.data, '$.status'), ''),
		       COALESCE(json_extract(s.data, '$.notes'), '')
		  FROM dr_backups_snapshots s
		  JOIN dr_backups d ON d.id = s.dr_backups_id
		 WHERE (json_extract(d.data, '$.company') = ? OR json_extract(d.data, '$.company') = ?)
		   AND json_extract(s.data, '$.created_at') >= ?
		   AND json_extract(s.data, '$.created_at') <  ?`,
		cid, companyID, from.Format(time.RFC3339), to.Format(time.RFC3339),
	)
	if err == nil {
		for rows.Next() {
			var (
				createdAt sql.NullString
				device    sql.NullString
				outcome   sql.NullString
				notes     sql.NullString
			)
			if scanErr := rows.Scan(&createdAt, &device, &outcome, &notes); scanErr != nil {
				continue
			}
			ev := RestoreEvent{
				Device:  device.String,
				Outcome: outcome.String,
				Notes:   notes.String,
			}
			if createdAt.Valid {
				if t, perr := time.Parse(time.RFC3339, createdAt.String); perr == nil {
					ev.Date = t
				}
			}
			events = append(events, ev)
		}
		rows.Close()
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Date.Before(events[j].Date) })
	return events
}

// loadOpenIssues mirrors the attention-skill filter: closed, resolved,
// archived, and ignored states are dropped. companies_issues stores rows
// as JSON blobs keyed by companies_id; we extract title/severity/state
// at read time.
func loadOpenIssues(ctx context.Context, db *sql.DB, companyID int64) []Issue {
	cid := fmt.Sprintf("%d", companyID)
	rows, err := db.QueryContext(ctx, `
		SELECT id, data FROM companies_issues
		 WHERE companies_id = ? OR companies_id = ?`,
		cid, companyID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var issues []Issue
	for rows.Next() {
		var id string
		var data []byte
		if scanErr := rows.Scan(&id, &data); scanErr != nil {
			continue
		}
		var row struct {
			Title    string `json:"title"`
			Subject  string `json:"subject"`
			Message  string `json:"message"`
			State    string `json:"state"`
			Severity string `json:"severity"`
			OpenedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(data, &row); err != nil {
			continue
		}
		switch strings.ToLower(row.State) {
		case "closed", "resolved", "archived", "ignored":
			continue
		}
		title := row.Title
		if title == "" {
			title = row.Subject
		}
		if title == "" {
			title = row.Message
		}
		if title == "" {
			title = "(no title)"
		}
		iss := Issue{ID: id, Title: title, Severity: row.Severity}
		if row.OpenedAt != "" {
			if t, perr := time.Parse(time.RFC3339, row.OpenedAt); perr == nil {
				iss.OpenedAt = t
			}
		}
		issues = append(issues, iss)
	}
	return issues
}

// loadStorageTrend reads snapshot history. The "attention" snapshot is
// the most reliable surface — every run records a `totals` map that
// (when present) includes a fleet-wide storage figure. v1 returns the
// per-snapshot total bytes within the quarter window; per-company
// storage breakdowns land in v0.2 alongside charts.
func loadStorageTrend(ctx context.Context, db *sql.DB, companyID int64, from, to time.Time) []TrendPoint {
	snaps, err := snapshot.List(ctx, db, "attention", 0)
	if err != nil || len(snaps) == 0 {
		return nil
	}
	var trend []TrendPoint
	for _, s := range snaps {
		if s.TakenAt.Before(from) || !s.TakenAt.Before(to) {
			continue
		}
		var payload struct {
			Companies []struct {
				CompanyID    int64 `json:"company_id"`
				StorageBytes int64 `json:"storage_bytes"`
			} `json:"companies"`
			Totals map[string]int64 `json:"totals"`
		}
		if err := json.Unmarshal(s.Data, &payload); err != nil {
			continue
		}
		var bytes int64
		for _, c := range payload.Companies {
			if c.CompanyID == companyID && c.StorageBytes > 0 {
				bytes = c.StorageBytes
				break
			}
		}
		if bytes == 0 {
			if v, ok := payload.Totals["storage_bytes"]; ok {
				bytes = v
			}
		}
		if bytes == 0 {
			continue
		}
		trend = append(trend, TrendPoint{At: s.TakenAt, Bytes: bytes})
	}
	sort.Slice(trend, func(i, j int) bool { return trend[i].At.Before(trend[j].At) })
	return trend
}

func topIssueLabel(issues []Issue) string {
	if len(issues) == 0 {
		return ""
	}
	// Severity wins; fall back to oldest open.
	sevRank := func(s string) int {
		switch strings.ToLower(s) {
		case "critical":
			return 4
		case "high":
			return 3
		case "medium":
			return 2
		case "low":
			return 1
		}
		return 0
	}
	top := issues[0]
	for _, i := range issues[1:] {
		if sevRank(i.Severity) > sevRank(top.Severity) {
			top = i
			continue
		}
		if sevRank(i.Severity) == sevRank(top.Severity) && !i.OpenedAt.IsZero() && i.OpenedAt.Before(top.OpenedAt) {
			top = i
		}
	}
	return top.Title
}

// ---- renderers ----------------------------------------------------------

// RenderMarkdown emits a plain-text Markdown report. Tables are simple
// pipe-delimited; the storage trend renders as an ASCII sparkline.
func RenderMarkdown(r *Report, w io.Writer) error {
	bw := &strings.Builder{}
	fmt.Fprintf(bw, "# Quarterly Backup Review — %s\n", r.Company.Name)
	fmt.Fprintf(bw, "_Prepared by %s · %s · generated %s_\n\n",
		r.Reseller.Name, r.Quarter, r.GeneratedAt.Format("2006-01-02"))

	fmt.Fprintln(bw, "## Executive summary")
	fmt.Fprintln(bw)
	fmt.Fprintln(bw, executiveSummary(r))
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Backup coverage")
	fmt.Fprintln(bw)
	if len(r.Coverage) == 0 {
		fmt.Fprintln(bw, "_No protected devices found in the local store._")
	} else {
		fmt.Fprintln(bw, "| Device | Classic | Restic | DR | Last successful | Status |")
		fmt.Fprintln(bw, "|---|---|---|---|---|---|")
		for _, d := range r.Coverage {
			last := "—"
			if !d.LastSuccessful.IsZero() {
				last = d.LastSuccessful.Format("2006-01-02")
			}
			fmt.Fprintf(bw, "| %s | %s | %s | %s | %s | %s |\n",
				escapePipe(d.Device),
				checkmark(d.Classic),
				checkmark(d.Restic),
				checkmark(d.DR),
				last,
				escapePipe(d.Status),
			)
		}
	}
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Job success rate")
	fmt.Fprintln(bw)
	if r.SuccessRate.Total == 0 {
		fmt.Fprintln(bw, "_No backup jobs recorded in this quarter._")
	} else {
		fmt.Fprintf(bw, "**%.1f%% over %s** (%d jobs total, %d hard-fail, %d dirty, %d retried-then-succeeded)\n\n",
			r.SuccessRate.Rate, r.Quarter, r.SuccessRate.Total, r.SuccessRate.HardFail, r.SuccessRate.Dirty, r.SuccessRate.RetriedSucceeded)
		fmt.Fprintf(bw, "`%s`\n", asciiBar(r.SuccessRate.Rate))
	}
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Restore tests")
	fmt.Fprintln(bw)
	if len(r.RestoreTests) == 0 {
		fmt.Fprintln(bw, "_No restore tests recorded in this quarter._")
	} else {
		fmt.Fprintf(bw, "**%d test(s) recorded.**\n\n", len(r.RestoreTests))
		fmt.Fprintln(bw, "| Date | Device | Outcome | Notes |")
		fmt.Fprintln(bw, "|---|---|---|---|")
		for _, e := range r.RestoreTests {
			date := "—"
			if !e.Date.IsZero() {
				date = e.Date.Format("2006-01-02")
			}
			fmt.Fprintf(bw, "| %s | %s | %s | %s |\n",
				date, escapePipe(e.Device), escapePipe(e.Outcome), escapePipe(e.Notes))
		}
	}
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Open issues")
	fmt.Fprintln(bw)
	if len(r.OpenIssues) == 0 {
		fmt.Fprintln(bw, "_No open issues. Nice quarter._")
	} else {
		fmt.Fprintln(bw, "| Severity | Opened | Title |")
		fmt.Fprintln(bw, "|---|---|---|")
		for _, i := range r.OpenIssues {
			opened := "—"
			if !i.OpenedAt.IsZero() {
				opened = i.OpenedAt.Format("2006-01-02")
			}
			fmt.Fprintf(bw, "| %s | %s | %s |\n", nonEmpty(i.Severity, "—"), opened, escapePipe(i.Title))
		}
	}
	fmt.Fprintln(bw)

	fmt.Fprintln(bw, "## Storage trend")
	fmt.Fprintln(bw)
	if len(r.StorageTrend) == 0 {
		fmt.Fprintln(bw, "_No history yet — storage trend will populate as snapshots accumulate._")
	} else {
		fmt.Fprintf(bw, "%d data point(s) across %s.\n\n", len(r.StorageTrend), r.Quarter)
		fmt.Fprintf(bw, "`%s`\n\n", asciiSparkline(r.StorageTrend))
		fmt.Fprintln(bw, "| Date | Storage |")
		fmt.Fprintln(bw, "|---|---|")
		for _, p := range r.StorageTrend {
			fmt.Fprintf(bw, "| %s | %s |\n", p.At.Format("2006-01-02"), humanBytes(p.Bytes))
		}
	}
	fmt.Fprintln(bw)

	_, err := io.WriteString(w, bw.String())
	return err
}

// RenderHTML writes a self-contained HTML document to w. All styling is
// inlined; the page break after the cover keeps the PDF rendering tidy.
func RenderHTML(r *Report, w io.Writer) error {
	t, err := template.New("qbr").Funcs(template.FuncMap{
		"fmtDate": func(t time.Time) string {
			if t.IsZero() {
				return "—"
			}
			return t.Format("2006-01-02")
		},
		"fmtBytes": humanBytes,
		"check": func(s string) template.HTML {
			if s == "" {
				return template.HTML("&middot;")
			}
			return template.HTML("&#10003;")
		},
		"nonEmpty": nonEmpty,
		"summary":  executiveSummary,
		"pct":      func(f float64) string { return fmt.Sprintf("%.1f%%", f) },
		"successBar": func(stats SuccessStats) template.HTML {
			if stats.Total == 0 {
				return template.HTML("")
			}
			pct := stats.Rate
			return template.HTML(fmt.Sprintf(
				`<div class="bar-outer"><div class="bar-inner" style="width:%.1f%%"></div></div>`, pct))
		},
		"hasTrend": func(p []TrendPoint) bool { return len(p) > 0 },
	}).Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return t.Execute(w, r)
}

// RenderPDF writes the HTML to a temp file, shells out to headless
// Chrome to convert it, and removes the temp on success or failure. The
// Chrome binary is detected by trying a known list in order; on macOS the
// .app bundle path matches the most-common user install. If no Chrome is
// found we return a directive error so the user gets clean guidance to
// switch to --format md/html.
func RenderPDF(r *Report, outPath string) error {
	chrome, err := findChrome()
	if err != nil {
		return err
	}
	if outPath == "" {
		return fmt.Errorf("--out is required for --format pdf")
	}

	tmp, err := os.CreateTemp("", "qbr-*.html")
	if err != nil {
		return fmt.Errorf("create temp html: %w", err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup; we want to remove the temp on both success and
	// failure but ignore errors since the OS will reap /tmp anyway.
	defer os.Remove(tmpPath)

	if err := RenderHTML(r, tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("render html for pdf: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp html: %w", err)
	}

	cmd := exec.Command(chrome,
		"--headless",
		"--disable-gpu",
		"--no-sandbox",
		"--print-to-pdf="+outPath,
		"file://"+tmpPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chrome PDF render failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func findChrome() (string, error) {
	candidates := []string{
		"chrome",
		"google-chrome",
		"chromium",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	}
	for _, c := range candidates {
		if strings.Contains(c, "/") {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
			continue
		}
		if path, err := exec.LookPath(c); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("PDF rendering requires Chrome; install Chrome or use --format md/html")
}

// ---- helpers ------------------------------------------------------------

func executiveSummary(r *Report) string {
	switch {
	case r.SuccessRate.Total == 0 && len(r.OpenIssues) == 0:
		return fmt.Sprintf("No backup activity recorded for %s in %s. Sync the store and re-run to populate the report.",
			r.Company.Name, r.Quarter)
	case r.SuccessRate.Total == 0:
		return fmt.Sprintf("%d open issue(s) for %s as of %s. No backup jobs landed in this quarter — confirm sync coverage.",
			len(r.OpenIssues), r.Company.Name, r.Quarter)
	}
	topNote := ""
	if r.TopIssue != "" {
		topNote = fmt.Sprintf(" Top open issue: %q.", r.TopIssue)
	}
	tests := fmt.Sprintf("%d restore test(s)", len(r.RestoreTests))
	return fmt.Sprintf(
		"%s ran %d backup jobs in %s with a %.1f%% success rate. %s recorded.%s",
		r.Company.Name, r.SuccessRate.Total, r.Quarter, r.SuccessRate.Rate, tests, topNote,
	)
}

func checkmark(v string) string {
	if v == "" {
		return "·"
	}
	return "✓"
}

func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func escapePipe(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func asciiBar(pct float64) string {
	const width = 30
	filled := int((pct / 100.0) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func asciiSparkline(points []TrendPoint) string {
	if len(points) == 0 {
		return ""
	}
	glyphs := []rune("▁▂▃▄▅▆▇█")
	var min, max int64 = points[0].Bytes, points[0].Bytes
	for _, p := range points {
		if p.Bytes < min {
			min = p.Bytes
		}
		if p.Bytes > max {
			max = p.Bytes
		}
	}
	rng := max - min
	out := make([]rune, len(points))
	for i, p := range points {
		var idx int
		if rng > 0 {
			idx = int(float64(p.Bytes-min) / float64(rng) * float64(len(glyphs)-1))
		}
		if idx < 0 {
			idx = 0
		}
		if idx > len(glyphs)-1 {
			idx = len(glyphs) - 1
		}
		out[i] = glyphs[idx]
	}
	return string(out)
}

func humanBytes(b int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	}
	return fmt.Sprintf("%d B", b)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func parseInt64(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
