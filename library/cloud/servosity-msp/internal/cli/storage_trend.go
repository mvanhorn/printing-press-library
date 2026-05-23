// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/snapshot"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// storageMeasurement is one (taken_at, bytes) sample for the regression and
// for the human/JSON measurement list.
type storageMeasurement struct {
	TakenAt time.Time `json:"taken_at"`
	Bytes   int64     `json:"bytes"`
}

// storageTrendResult is the JSON envelope returned in machine mode and the
// source of truth for the human render below it.
type storageTrendResult struct {
	CompanyID         int64                `json:"company_id"`
	CompanyName       string               `json:"company_name"`
	CurrentBytes      int64                `json:"current_bytes"`
	TrendBytesPerWeek *int64               `json:"trend_bytes_per_week,omitempty"`
	ThresholdBytes    int64                `json:"threshold_bytes"`
	WeeksToThreshold  *float64             `json:"weeks_to_threshold,omitempty"`
	ProjectedDate     string               `json:"projected_date,omitempty"`
	Measurements      []storageMeasurement `json:"measurements"`
	Note              string               `json:"note,omitempty"`
}

// newStorageTrendCmd builds the storage-trend command: forecast when a client
// will need more backup storage. Reads the time series of per-backup size_bytes
// captured by `sync`, computes a linear regression on the last N weeks, and
// projects forward to a capacity threshold.
//
// History is sourced from snapshots tagged "storage-trend:<company_id>"; on
// first run (or with <2 snapshots) we emit the CURRENT total and recommend
// running periodically with --snapshot to build the trend line.
func newStorageTrendCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var thresholdStr string
	var engine string
	var saveSnapshot bool

	cmd := &cobra.Command{
		Use:   "storage-trend <company>",
		Short: "Forecast when a client will need more backup storage",
		Long: `Forecast when a specific client will need more backup storage. Reads the
time series of storage measurements captured by 'sync' (per-backup
size_bytes), computes a linear regression on the last N weeks, and projects
forward to a capacity threshold.

History is read from local snapshots tagged "storage-trend:<company_id>".
Pass --snapshot weekly (e.g. via cron) to build the series; subsequent runs
read those snapshots to compute the trend line.

The <company> argument is either a numeric company id or a case-insensitive
substring of the company name (exact id wins over substring on ambiguity).`,
		Example: `  # Forecast for company 4421 against the default 1TB threshold
  servosity-msp-cli storage-trend 4421

  # Use 8 weeks of history, project against a 500 GB threshold
  servosity-msp-cli storage-trend "Acme Corp" --weeks 8 --threshold 500GB

  # Periodic snapshot (run weekly via cron to build the series)
  servosity-msp-cli storage-trend 4421 --snapshot`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			engine = strings.ToLower(strings.TrimSpace(engine))
			switch engine {
			case "", "all", "classic", "restic", "dr":
			default:
				return usageErr(fmt.Errorf("invalid --engine %q (want classic|restic|dr|all)", engine))
			}
			if engine == "" {
				engine = "all"
			}
			if weeks < 2 {
				return usageErr(fmt.Errorf("--weeks must be >= 2, got %d", weeks))
			}

			thresholdBytes, err := parseBytesSuffix(thresholdStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --threshold %q: %w", thresholdStr, err))
			}

			ctx := cmd.Context()
			dbPath := defaultDBPath("servosity-msp-cli")
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'servosity-msp-cli sync' first.", err)
			}
			defer db.Close()

			companyID, companyName, err := resolveCompany(ctx, db, args[0])
			if err != nil {
				return err
			}

			currentBytes, err := computeCurrentStorageBytes(ctx, db, companyID, engine)
			if err != nil {
				return err
			}

			metric := fmt.Sprintf("storage-trend:%d", companyID)
			now := time.Now().UTC()

			// Persist a snapshot of the current total when requested. Idempotent
			// on (metric, taken_at) so back-to-back runs in the same nanosecond
			// don't double-write, but the human use case is one cron tick per
			// week so the same-instant collision is academic.
			if saveSnapshot {
				payload, mErr := json.Marshal(map[string]any{
					"company_id":   companyID,
					"company_name": companyName,
					"bytes":        currentBytes,
					"engine":       engine,
				})
				if mErr != nil {
					return fmt.Errorf("marshal snapshot payload: %w", mErr)
				}
				if sErr := snapshot.Save(ctx, db.DB(), metric, now, json.RawMessage(payload)); sErr != nil {
					return fmt.Errorf("saving storage-trend snapshot: %w", sErr)
				}
			}

			// Read history. limit=0 → unbounded; we trim to the requested weeks
			// window below.
			snaps, err := snapshot.List(ctx, db.DB(), metric, 0)
			if err != nil {
				return fmt.Errorf("listing storage-trend snapshots: %w", err)
			}

			measurements := collectMeasurements(snaps, currentBytes, now, weeks, saveSnapshot)

			result := storageTrendResult{
				CompanyID:      companyID,
				CompanyName:    companyName,
				CurrentBytes:   currentBytes,
				ThresholdBytes: thresholdBytes,
				Measurements:   measurements,
			}

			if len(measurements) < 2 {
				result.Note = "No historical data yet — run `storage-trend <company> --snapshot` periodically (weekly cron suggested) to build the trend line."
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, result)
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"%s (%d): current backup storage %s. No historical data yet — run `storage-trend <company> --snapshot` periodically (weekly cron suggested) to build the trend line.\n",
					companyName, companyID, formatBytes(currentBytes))
				return nil
			}

			// Linear regression on (week_offset, bytes).
			slope, intercept, ok := linearFitBytesPerWeek(measurements)
			if !ok {
				// Degenerate (e.g. all timestamps identical) — fall back to the
				// first-run path with a clarifying note.
				result.Note = "All snapshots share the same timestamp; cannot fit a trend line. Wait at least one week between --snapshot runs."
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, result)
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"%s (%d): current backup storage %s. %s\n",
					companyName, companyID, formatBytes(currentBytes), result.Note)
				return nil
			}
			slopeInt := int64(math.Round(slope))
			result.TrendBytesPerWeek = &slopeInt

			// Project to threshold. Only meaningful when growth is positive and
			// we're still below the threshold; otherwise leave projected_date
			// unset and let the renderer call out the state.
			if slope > 0 && float64(currentBytes) < float64(thresholdBytes) {
				lastT := measurements[0].TakenAt // measurements are newest-first
				lastWeekOffset := weeksSince(measurements[len(measurements)-1].TakenAt, lastT)
				// y = slope*x + intercept => x = (y - intercept)/slope
				xAtThreshold := (float64(thresholdBytes) - intercept) / slope
				weeksOut := xAtThreshold - lastWeekOffset
				if weeksOut > 0 {
					rounded := math.Round(weeksOut*10) / 10
					result.WeeksToThreshold = &rounded
					projected := lastT.Add(time.Duration(weeksOut * float64(7*24*time.Hour)))
					result.ProjectedDate = projected.Format("2006-01-02")
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}
			return renderStorageTrendHuman(cmd, result, weeks)
		},
	}

	cmd.Flags().IntVar(&weeks, "weeks", 12, "How many weeks of history to use for the regression")
	cmd.Flags().StringVar(&thresholdStr, "threshold", "1TB", "Capacity threshold to project against (e.g. 1TB, 500GB, 2.5T)")
	cmd.Flags().StringVar(&engine, "engine", "all", "Engine to include: classic|restic|dr|all")
	cmd.Flags().BoolVar(&saveSnapshot, "snapshot", false, "Persist the current total as a snapshot for future trend runs (weekly cron suggested)")

	return cmd
}

// collectMeasurements builds the (newest-first) measurement list driving the
// regression. Always seeds today's current total at the head so the freshest
// data point informs the fit even when --snapshot wasn't passed this run; if
// the most-recent snapshot is from "today" already, we keep the snapshot
// entry instead of duplicating. Trimmed to the requested weeks window.
func collectMeasurements(snaps []snapshot.Snapshot, currentBytes int64, now time.Time, weeks int, savedThisRun bool) []storageMeasurement {
	out := make([]storageMeasurement, 0, len(snaps)+1)

	// If we wrote a snapshot this run, the first entry in `snaps` IS the
	// current sample — don't duplicate.
	if !savedThisRun {
		out = append(out, storageMeasurement{TakenAt: now, Bytes: currentBytes})
	}

	for _, s := range snaps {
		var payload struct {
			Bytes int64 `json:"bytes"`
		}
		if err := json.Unmarshal(s.Data, &payload); err != nil {
			continue
		}
		out = append(out, storageMeasurement{TakenAt: s.TakenAt.UTC(), Bytes: payload.Bytes})
	}

	// Newest first (snapshot.List already returns newest-first; the prepended
	// `now` entry, when present, is at index 0 by construction).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].TakenAt.After(out[j].TakenAt)
	})

	// Trim to the requested window. weeks=12 ⇒ ~84 days. We keep every
	// measurement within `weeks` weeks of the newest sample.
	if len(out) > 0 {
		newest := out[0].TakenAt
		cutoff := newest.Add(-time.Duration(weeks) * 7 * 24 * time.Hour)
		trimmed := out[:0]
		for _, m := range out {
			if !m.TakenAt.Before(cutoff) {
				trimmed = append(trimmed, m)
			}
		}
		out = trimmed
	}
	return out
}

// linearFitBytesPerWeek returns slope (bytes/week), intercept (bytes), and ok.
// xs are weeks-since-oldest-measurement, ys are bytes. Returns ok=false when
// the time spread is zero (all xs identical → divide by zero in the normal
// equation).
func linearFitBytesPerWeek(measurements []storageMeasurement) (slope, intercept float64, ok bool) {
	n := len(measurements)
	if n < 2 {
		return 0, 0, false
	}
	oldest := measurements[n-1].TakenAt
	var sumX, sumY, sumXY, sumX2 float64
	for _, m := range measurements {
		x := weeksSince(oldest, m.TakenAt)
		y := float64(m.Bytes)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	fn := float64(n)
	denom := fn*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0, false
	}
	slope = (fn*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / fn
	return slope, intercept, true
}

// weeksSince returns the fractional weeks from `from` to `to` (positive when
// to is later).
func weeksSince(from, to time.Time) float64 {
	return to.Sub(from).Hours() / (24.0 * 7.0)
}

// computeCurrentStorageBytes sums size_bytes across the requested engines for
// one company. Reuses the json_extract pattern from backup_facts.go so column
// shape drift across engines doesn't fight us.
func computeCurrentStorageBytes(ctx context.Context, db *store.Store, companyID int64, engine string) (int64, error) {
	type leg struct {
		name string
		sql  string
	}
	legs := []leg{
		{"classic", `SELECT COALESCE(SUM(CAST(COALESCE(json_extract(data,'$.size_bytes'), json_extract(data,'$.size')) AS INTEGER)), 0) FROM backups WHERE CAST(json_extract(data,'$.company_id') AS INTEGER) = ?`},
		{"restic", `SELECT COALESCE(SUM(CAST(COALESCE(json_extract(data,'$.size_bytes'), json_extract(data,'$.size')) AS INTEGER)), 0) FROM restic_backups WHERE CAST(json_extract(data,'$.company_id') AS INTEGER) = ?`},
		{"dr", `SELECT COALESCE(SUM(CAST(COALESCE(json_extract(data,'$.size_bytes'), json_extract(data,'$.size')) AS INTEGER)), 0) FROM dr_backups WHERE CAST(json_extract(data,'$.company_id') AS INTEGER) = ?`},
	}
	var total int64
	for _, l := range legs {
		if engine != "all" && engine != l.name {
			continue
		}
		var v sql.NullInt64
		row := db.DB().QueryRowContext(ctx, l.sql, companyID)
		if err := row.Scan(&v); err != nil {
			// Table-missing or shape-mismatch on one engine shouldn't kill the
			// whole forecast; treat as zero and continue.
			if strings.Contains(err.Error(), "no such table") || strings.Contains(err.Error(), "no such column") {
				continue
			}
			return 0, fmt.Errorf("summing %s storage: %w", l.name, err)
		}
		if v.Valid {
			total += v.Int64
		}
	}
	return total, nil
}

// resolveCompany maps the user-supplied <company> arg to (id, name). Numeric
// args are matched as exact company id; otherwise we substring-match the name
// (case-insensitive). On ambiguous substring we pick the lexicographically
// smallest name so output stays deterministic across runs, and the JSON keeps
// the id so the caller can disambiguate.
func resolveCompany(ctx context.Context, db *store.Store, arg string) (int64, string, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return 0, "", usageErr(fmt.Errorf("<company> arg required"))
	}

	// Numeric id path.
	if id, err := strconv.ParseInt(arg, 10, 64); err == nil {
		var name sql.NullString
		row := db.DB().QueryRowContext(ctx, `SELECT COALESCE(name, '') FROM companies WHERE CAST(id AS INTEGER) = ?`, id)
		if err := row.Scan(&name); err == nil && name.String != "" {
			return id, name.String, nil
		}
		// Fall back to id-only when companies table isn't synced or the row is
		// missing — still useful for snapshot lookup.
		return id, fmt.Sprintf("Company %d", id), nil
	}

	// Substring path. Lower-case both sides so the comparison stays
	// case-insensitive across the union of synced fields.
	rows, err := db.DB().QueryContext(ctx,
		`SELECT CAST(id AS INTEGER), COALESCE(name, '') FROM companies WHERE lower(COALESCE(name,'')) LIKE ? ORDER BY name ASC LIMIT 2`,
		"%"+strings.ToLower(arg)+"%",
	)
	if err != nil {
		return 0, "", fmt.Errorf("looking up company by name: %w", err)
	}
	defer rows.Close()
	var matches []struct {
		ID   int64
		Name string
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return 0, "", fmt.Errorf("scanning company row: %w", err)
		}
		matches = append(matches, struct {
			ID   int64
			Name string
		}{id, name})
	}
	if err := rows.Err(); err != nil {
		return 0, "", fmt.Errorf("iterating company rows: %w", err)
	}
	if len(matches) == 0 {
		return 0, "", notFoundErr(fmt.Errorf("no company matches %q (try a numeric id or a more specific name substring)", arg))
	}
	// LIMIT 2 lets us flag ambiguity without scanning the whole table. Return
	// the first deterministically; the caller sees the id so disambiguation is
	// always possible.
	return matches[0].ID, matches[0].Name, nil
}

// bytesSuffixRE captures a positive number (with optional decimal) plus an
// optional case-insensitive unit suffix. The "B" is optional so "1T" parses
// the same as "1TB".
var bytesSuffixRE = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)\s*([kKmMgGtT]?[bB]?|)\s*$`)

// parseBytesSuffix converts strings like "1TB", "500GB", "2.5T" into bytes
// using binary units (1 KB = 1024 B). Bare numbers are treated as bytes.
func parseBytesSuffix(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	m := bytesSuffixRE.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("expected forms like 1TB, 500GB, 2.5T, or a bare byte count")
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("parsing numeric portion: %w", err)
	}
	unit := strings.ToUpper(strings.TrimSuffix(strings.TrimSpace(m[2]), "B"))
	var mult float64 = 1
	switch unit {
	case "":
		mult = 1
	case "K":
		mult = 1 << 10
	case "M":
		mult = 1 << 20
	case "G":
		mult = 1 << 30
	case "T":
		mult = 1 << 40
	default:
		return 0, fmt.Errorf("unknown size suffix %q (accept K/KB/M/MB/G/GB/T/TB)", m[2])
	}
	return int64(n * mult), nil
}

// formatBytes renders a byte count in the most appropriate binary unit with
// one decimal place. Matches the spec's human output (412.3 GB / 1.0 TB).
func formatBytes(b int64) string {
	const k = 1024.0
	f := float64(b)
	switch {
	case f >= k*k*k*k:
		return fmt.Sprintf("%.1f TB", f/(k*k*k*k))
	case f >= k*k*k:
		return fmt.Sprintf("%.1f GB", f/(k*k*k))
	case f >= k*k:
		return fmt.Sprintf("%.1f MB", f/(k*k))
	case f >= k:
		return fmt.Sprintf("%.1f KB", f/k)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// renderStorageTrendHuman writes the table+headline output documented in the
// spec. JSON callers use flags.printJSON directly.
func renderStorageTrendHuman(cmd *cobra.Command, r storageTrendResult, weeks int) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%d):\n", r.CompanyName, r.CompanyID)
	fmt.Fprintf(w, "  Current:   %s\n", formatBytes(r.CurrentBytes))
	if r.TrendBytesPerWeek != nil {
		trend := formatBytes(*r.TrendBytesPerWeek)
		// Force a leading sign so a flat/positive slope is unambiguous;
		// negatives already carry "-" from formatBytes via the float math.
		if *r.TrendBytesPerWeek >= 0 && !strings.HasPrefix(trend, "-") {
			trend = "+" + trend
		}
		fmt.Fprintf(w, "  Trend:     %s / week (linear fit over last %d weeks)\n", trend, weeks)
	}
	fmt.Fprintf(w, "  Threshold: %s\n", formatBytes(r.ThresholdBytes))
	switch {
	case r.ProjectedDate != "" && r.WeeksToThreshold != nil:
		fmt.Fprintf(w, "  Projected: %s (%.1f weeks out) at current pace\n", r.ProjectedDate, *r.WeeksToThreshold)
	case r.TrendBytesPerWeek != nil && *r.TrendBytesPerWeek <= 0:
		fmt.Fprintf(w, "  Projected: storage is flat or shrinking — no threshold crossing at current pace\n")
	case r.CurrentBytes >= r.ThresholdBytes:
		fmt.Fprintf(w, "  Projected: already at or past threshold\n")
	}
	if r.Note != "" {
		fmt.Fprintf(w, "  Note:      %s\n", r.Note)
	}
	if len(r.Measurements) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Recent measurements:")
		max := len(r.Measurements)
		if max > 10 {
			max = 10
		}
		for _, m := range r.Measurements[:max] {
			fmt.Fprintf(w, "  %s  %s\n", m.TakenAt.Format("2006-01-02"), formatBytes(m.Bytes))
		}
	}
	return nil
}
