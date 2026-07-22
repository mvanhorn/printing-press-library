// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature support for the Wahoo CLI. Shared by the local
// training-analysis commands (load, digest, bests, ftp-history) and the offline
// route finder. Not generated — safe to edit.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
	"github.com/spf13/cobra"
)

// parseLooseFloat coerces a JSON value that may be a number, a numeric string,
// or null into a float64. The Wahoo Cloud API encodes workout-summary metrics
// (distance_accum, power_bike_avg, work_accum, duration_*_accum, ...) as
// STRINGS, and omits fields entirely on incomplete summaries, so every numeric
// read must tolerate "", null, and "12.5" alike. ok is false for
// missing/empty/unparseable input — never let a missing metric silently become
// a real zero in an aggregate.
func parseLooseFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case nil:
		return 0, false
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// idString renders an id value (int64 from JSON arrives as float64) without
// scientific notation, so workout ids like 1234567 don't become "1.234567e+06".
func idString(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }
func round2(x float64) float64 { return math.Round(x*100) / 100 }

// dayStart returns midnight-UTC for t, the bucket key for daily aggregation.
func dayStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// dbPathOrDefault resolves the --db flag, falling back to the canonical path.
func dbPathOrDefault(p string) string {
	if strings.TrimSpace(p) != "" {
		return p
	}
	return defaultDBPath("wahoo-pp-cli")
}

// parsedWorkout is the normalized numeric view of one workout plus its embedded
// summary, parsed NULL-safe from the local mirror. Bool flags record whether a
// metric was actually present so aggregates can exclude missing values from
// their denominators rather than averaging in phantom zeros.
type parsedWorkout struct {
	ID              string
	Name            string
	Starts          time.Time
	HasStarts       bool
	Minutes         float64
	DistanceM       float64
	HasDistance     bool
	AscentM         float64
	HasAscent       bool
	WorkKJ          float64
	HasWork         bool
	AvgPowerW       float64
	HasPower        bool
	NP              float64
	HasNP           bool
	TSS             float64
	HasTSS          bool
	AvgHR           float64
	HasHR           bool
	Calories        float64
	DurationActiveS float64
	DurationTotalS  float64
	HasSummary      bool
}

// DurationHours returns the best available ride duration in hours, preferring
// the summary's active duration, then total duration, then the workout's
// declared minutes.
func (w parsedWorkout) DurationHours() float64 {
	switch {
	case w.DurationActiveS > 0:
		return w.DurationActiveS / 3600
	case w.DurationTotalS > 0:
		return w.DurationTotalS / 3600
	case w.Minutes > 0:
		return w.Minutes / 60
	default:
		return 0
	}
}

// metricValue returns the requested summary metric and whether it was present,
// for the generic bests/digest paths.
func (w parsedWorkout) metricValue(metric string) (float64, bool) {
	switch metric {
	case "power":
		return w.AvgPowerW, w.HasPower
	case "distance":
		return w.DistanceM, w.HasDistance
	case "ascent":
		return w.AscentM, w.HasAscent
	case "work":
		return w.WorkKJ, w.HasWork
	case "duration":
		h := w.DurationHours()
		return h, h > 0
	case "heart_rate":
		return w.AvgHR, w.HasHR
	default:
		return 0, false
	}
}

func parseWorkoutObj(obj map[string]any) parsedWorkout {
	w := parsedWorkout{ID: idString(obj["id"])}
	if n, ok := obj["name"].(string); ok {
		w.Name = n
	}
	if s, ok := obj["starts"].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			w.Starts = t
			w.HasStarts = true
		}
	}
	if m, ok := parseLooseFloat(obj["minutes"]); ok {
		w.Minutes = m
	}
	if sum, ok := obj["workout_summary"].(map[string]any); ok && sum != nil {
		w.HasSummary = true
		w.DistanceM, w.HasDistance = parseLooseFloat(sum["distance_accum"])
		w.AscentM, w.HasAscent = parseLooseFloat(sum["ascent_accum"])
		// work_accum is reported in JOULES by the live API (the OpenAPI spec was
		// unit-less); convert to kilojoules. Treating it as kJ over-reports work
		// ~1000x and blows up the load estimate.
		if j, ok := parseLooseFloat(sum["work_accum"]); ok {
			w.WorkKJ, w.HasWork = j/1000, true
		}
		// Average power: the live API field is "power_avg". The spec's
		// "power_bike_avg" never appears in real responses, so read power_avg
		// first and keep the spec name as a fallback.
		if p, ok := parseLooseFloat(sum["power_avg"]); ok {
			w.AvgPowerW, w.HasPower = p, true
		} else if p, ok := parseLooseFloat(sum["power_bike_avg"]); ok {
			w.AvgPowerW, w.HasPower = p, true
		}
		w.NP, w.HasNP = parseLooseFloat(sum["power_bike_np_last"])
		w.TSS, w.HasTSS = parseLooseFloat(sum["power_bike_tss_last"])
		w.AvgHR, w.HasHR = parseLooseFloat(sum["heart_rate_avg"])
		w.Calories, _ = parseLooseFloat(sum["calories_accum"])
		w.DurationActiveS, _ = parseLooseFloat(sum["duration_active_accum"])
		w.DurationTotalS, _ = parseLooseFloat(sum["duration_total_accum"])
	}
	return w
}

// loadWorkouts reads and parses every workout from the local mirror.
func loadWorkouts(db *store.Store) ([]parsedWorkout, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'workouts'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []parsedWorkout
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if !raw.Valid || raw.String == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		out = append(out, parseWorkoutObj(obj))
	}
	return out, rows.Err()
}

// estimateRideLoad returns a per-ride training-stress score. It prefers the
// real per-ride TSS the Wahoo head unit recorded (power_bike_tss_last), present
// on most rides including HR-derived TSS. When TSS is absent it falls back to a
// power-vs-FTP approximation (normalized power when available, else average
// power: IF = power/FTP, load = durationHours * IF^2 * 100), and finally to a
// duration-only proxy at a nominal moderate intensity. Returns 0 when there is
// no usable signal.
func estimateRideLoad(w parsedWorkout, ftp float64) float64 {
	// Prefer the real per-ride TSS the Wahoo head unit recorded
	// (power_bike_tss_last) — present on most rides, including HR-derived TSS.
	if w.HasTSS && w.TSS > 0 {
		return w.TSS
	}
	hours := w.DurationHours()
	if hours <= 0 {
		return 0
	}
	// Power-vs-FTP approximation, normalized power preferred over average.
	power := w.AvgPowerW
	if w.HasNP && w.NP > 0 {
		power = w.NP
	}
	if power > 0 && ftp > 0 {
		intensity := power / ftp
		return hours * intensity * intensity * 100
	}
	// Last resort: duration at a nominal moderate intensity.
	const nominalIF = 0.65
	return hours * nominalIF * nominalIF * 100
}

// loadPoint is one day of the Performance Management Chart.
type loadPoint struct {
	Date string  `json:"date"`
	Load float64 `json:"load"`
	CTL  float64 `json:"ctl"`
	ATL  float64 `json:"atl"`
	TSB  float64 `json:"tsb"`
}

// computeLoadSeries builds the daily CTL (42-day) / ATL (7-day) / TSB series
// using the standard exponentially-weighted PMC recurrence, iterating every
// calendar day from the first ride through `now` so rest days correctly decay
// fitness. Only the trailing lastNDays points are returned (0 = all).
func computeLoadSeries(ws []parsedWorkout, ftp float64, lastNDays int, now time.Time) []loadPoint {
	daily := map[string]float64{}
	var minDay time.Time
	have := false
	for _, w := range ws {
		if !w.HasStarts {
			continue
		}
		ds := dayStart(w.Starts)
		daily[ds.Format("2006-01-02")] += estimateRideLoad(w, ftp)
		if !have || ds.Before(minDay) {
			minDay = ds
		}
		have = true
	}
	if !have {
		return nil
	}
	end := dayStart(now)
	if end.Before(minDay) {
		end = minDay
	}
	var pts []loadPoint
	ctl, atl := 0.0, 0.0
	for d := minDay; !d.After(end); d = d.AddDate(0, 0, 1) {
		load := daily[d.Format("2006-01-02")]
		ctl += (load - ctl) / 42
		atl += (load - atl) / 7
		pts = append(pts, loadPoint{
			Date: d.Format("2006-01-02"),
			Load: round1(load),
			CTL:  round1(ctl),
			ATL:  round1(atl),
			TSB:  round1(ctl - atl),
		})
	}
	if lastNDays > 0 && len(pts) > lastNDays {
		pts = pts[len(pts)-lastNDays:]
	}
	return pts
}

// formLabel interprets a Training Stress Balance (Form) value the way cyclists
// read a Performance Management Chart.
func formLabel(tsb float64) string {
	switch {
	case tsb > 15:
		return "very fresh (tapered / detraining risk)"
	case tsb > 5:
		return "fresh"
	case tsb >= -10:
		return "neutral (gray zone)"
	case tsb >= -30:
		return "building (productive fatigue)"
	default:
		return "high fatigue (overreaching)"
	}
}

// haversineKm returns the great-circle distance in kilometers between two
// lat/lng points.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// latestFTP returns the most recent FTP (watts) from the power-zones mirror,
// chosen by the newest updated_at/created_at among records that carry an FTP.
// Returns 0 when no power-zone record has an FTP.
func latestFTP(db *store.Store) (float64, error) {
	recs, err := loadPowerZones(db)
	if err != nil {
		return 0, err
	}
	best := 0.0
	var bestTime time.Time
	for _, r := range recs {
		if r.FTP <= 0 {
			continue
		}
		if best == 0 || r.Updated.After(bestTime) {
			best = r.FTP
			bestTime = r.Updated
		}
	}
	return best, nil
}

// powerZoneRecord is the normalized view of one power-zones record.
type powerZoneRecord struct {
	ID            string
	FTP           float64
	CriticalPower float64
	FamilyID      string
	Created       time.Time
	Updated       time.Time
	UpdatedRaw    string
}

func loadPowerZones(db *store.Store) ([]powerZoneRecord, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type IN ('power-zones','power_zones')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []powerZoneRecord
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		rec := powerZoneRecord{ID: idString(obj["id"]), FamilyID: idString(obj["workout_type_family_id"])}
		rec.FTP, _ = parseLooseFloat(obj["ftp"])
		rec.CriticalPower, _ = parseLooseFloat(obj["critical_power"])
		if s, ok := obj["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				rec.Created = t
			}
		}
		if s, ok := obj["updated_at"].(string); ok {
			rec.UpdatedRaw = s
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				rec.Updated = t
			}
		}
		if rec.Updated.IsZero() {
			rec.Updated = rec.Created
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// userWeightKg returns the authenticated user's weight in kilograms from the
// local mirror, or 0 when unknown (the API stores weight as a string).
func userWeightKg(db *store.Store) (float64, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'user'`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		if kg, ok := parseLooseFloat(obj["weight"]); ok && kg > 0 {
			return kg, nil
		}
	}
	return 0, rows.Err()
}

// mirrorMissing reports whether the local SQLite database file is absent, and
// emits the standard "run sync first" hint (plus an empty machine payload when
// machine output is requested). Callers return nil after a true result: a
// missing mirror is an empty local-cache state, not an error.
func mirrorMissing(cmd *cobra.Command, flags *rootFlags, dbPath, resource, emptyJSON string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"no local mirror at %s\nrun: wahoo-pp-cli sync --resources %s --db %s\n",
			dbPath, resource, dbPath)
		if flags != nil && (flags.asJSON || flags.agent) {
			fmt.Fprintln(cmd.OutOrStdout(), emptyJSON)
		}
		return true
	}
	return false
}
