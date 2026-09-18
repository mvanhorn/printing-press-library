// Copyright 2026 Prashant Kamani and contributors. Licensed under Apache-2.0. See LICENSE.
//
// NOVEL — tests for internal/cli/garmin_insights.go.
//
// Every fixture below is synthetic and deterministic: no real account, no real
// heart rate, no real night. The numbers are chosen so an assertion can name
// the exact expected aggregate, which is the only way a test can tell a
// correct mean from a plausible one.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/garmin/internal/store"
)

// ---------------------------------------------------------------------------
// Fixture construction
// ---------------------------------------------------------------------------

// insightsFixtureEnd is the day every fixture window ends on. Pinning it makes
// the whole suite independent of the calendar it runs on.
const insightsFixtureEnd = "2026-03-10"

// pinInsightsToday pins the window's right-hand edge for one test.
func pinInsightsToday(t *testing.T, day string) {
	t.Helper()
	previous := insightsToday
	pinned := mustParseCivilDay(day)
	insightsToday = func() civilDay { return pinned }
	t.Cleanup(func() { insightsToday = previous })
}

// sleepNight builds one synthetic sleep_stats payload. Stage seconds sum to
// the total, matching the invariant observed on the live archive.
func sleepNight(day string, totalMinutes, deepMinutes, remMinutes, awakeMinutes, score, restingHR int) json.RawMessage {
	lightMinutes := totalMinutes - deepMinutes - remMinutes
	payload := map[string]any{
		"calendarDate": day,
		"values": map[string]any{
			"totalSleepTimeInSeconds": totalMinutes * 60,
			"deepTime":                deepMinutes * 60,
			"remTime":                 remMinutes * 60,
			"lightTime":               lightMinutes * 60,
			"awakeTime":               awakeMinutes * 60,
			"sleepScore":              score,
			"restingHeartRate":        restingHR,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}

// sleepNightUnmeasured is a night Garmin returned with every value null. It
// exists to prove that a null never becomes a zero in an average.
func sleepNightUnmeasured(day string) json.RawMessage {
	payload := map[string]any{
		"calendarDate": day,
		"values": map[string]any{
			"totalSleepTimeInSeconds": nil,
			"deepTime":                nil,
			"remTime":                 nil,
			"lightTime":               nil,
			"awakeTime":               nil,
			"sleepScore":              nil,
			"restingHeartRate":        nil,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}

func activityRow(id, startLocal, typeKey string, durationSeconds, distanceMetres, load, avgHR float64) json.RawMessage {
	payload := map[string]any{
		"activityId":           id,
		"activityName":         "synthetic activity " + id,
		"startTimeLocal":       startLocal,
		"activityType":         map[string]any{"typeKey": typeKey},
		"duration":             durationSeconds,
		"distance":             distanceMetres,
		"activityTrainingLoad": load,
		"averageHR":            avgHR,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}

func hrZonesRow(activityID string, secondsPerZone []float64) json.RawMessage {
	zones := make([]map[string]any, 0, len(secondsPerZone))
	for i, secs := range secondsPerZone {
		zones = append(zones, map[string]any{
			"zoneNumber":      i + 1,
			"secsInZone":      secs,
			"zoneLowBoundary": 90 + i*10,
		})
	}
	payload := map[string]any{
		"activityId":      activityID,
		"hrTimeInZones":   zones,
		"zoneCount":       len(zones),
		"garminWrappedBy": "history",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}

func putRows(t *testing.T, db *store.Store, resourceType string, rows map[string]json.RawMessage) {
	t.Helper()
	keyed := make([]store.KeyedRow, 0, len(rows))
	for id, data := range rows {
		keyed = append(keyed, store.KeyedRow{ID: id, Data: data})
	}
	stored, skipped, err := db.UpsertKeyedBatch(resourceType, keyed)
	if err != nil {
		t.Fatalf("seeding %s: %v", resourceType, err)
	}
	if skipped != 0 || stored != len(rows) {
		t.Fatalf("seeding %s: stored %d skipped %d, want stored %d skipped 0", resourceType, stored, skipped, len(rows))
	}
}

// seedSleepFixture writes fourteen nights ending on insightsFixtureEnd: seven
// in the reported window and seven in the prior window, plus one unmeasured
// night inside the reported window.
//
// Reported window (2026-03-04..2026-03-10): six measured nights of exactly
// 420 minutes with score 80, one unmeasured night.
// Prior window (2026-02-25..2026-03-03): seven measured nights of exactly
// 360 minutes with score 70.
func seedSleepFixture(t *testing.T, db *store.Store) {
	t.Helper()
	rows := map[string]json.RawMessage{}
	end := mustParseCivilDay(insightsFixtureEnd)
	for i := 0; i < 7; i++ {
		day := end.addDays(-i).String()
		if i == 3 {
			rows[day] = sleepNightUnmeasured(day)
			continue
		}
		rows[day] = sleepNight(day, 420, 60, 90, 30, 80, 50)
	}
	for i := 7; i < 14; i++ {
		day := end.addDays(-i).String()
		rows[day] = sleepNight(day, 360, 40, 70, 40, 70, 55)
	}
	putRows(t, db, "sleep_stats", rows)
}

// seedTrainingFixture writes four activities inside the reported window and
// one outside it, with heart-rate zones for three of the four inside.
func seedTrainingFixture(t *testing.T, db *store.Store) {
	t.Helper()
	putRows(t, db, "activities", map[string]json.RawMessage{
		"1001": activityRow("1001", "2026-03-10 07:00:00", "cycling", 3600, 30000, 100, 140),
		"1002": activityRow("1002", "2026-03-09 07:00:00", "cycling", 1800, 15000, 50, 130),
		"1003": activityRow("1003", "2026-03-05 18:00:00", "running", 1800, 5000, 60, 150),
		"1004": activityRow("1004", "2026-03-04 18:00:00", "yoga", 3600, 0, 10, 90),
		// Outside the seven-day window: must not appear in any total.
		"1005": activityRow("1005", "2026-01-01 09:00:00", "running", 7200, 20000, 200, 145),
	})
	putRows(t, db, "activity_hr_zones", map[string]json.RawMessage{
		"1001": hrZonesRow("1001", []float64{600, 1200, 1200, 600, 0}),
		"1002": hrZonesRow("1002", []float64{300, 600, 600, 300, 0}),
		"1003": hrZonesRow("1003", []float64{0, 300, 900, 600, 0}),
		// 1004 has none: an activity recorded without a heart-rate strap.
		// 1005 has zones but is outside the window; they must not be summed.
		"1005": hrZonesRow("1005", []float64{9999, 9999, 9999, 9999, 9999}),
	})
	putRows(t, db, "max_metrics", map[string]json.RawMessage{
		"2026-03-04": mustJSON(map[string]any{
			"generic": map[string]any{"calendarDate": "2026-03-04", "vo2MaxValue": 40.0},
		}),
		"2026-03-10": mustJSON(map[string]any{
			"generic": map[string]any{"calendarDate": "2026-03-10", "vo2MaxValue": 42.0},
		}),
	})
	putRows(t, db, "intensity_minutes", map[string]json.RawMessage{
		"2026-03-09": mustJSON(map[string]any{
			"calendarDate": "2026-03-09", "moderateValue": 120, "vigorousValue": 60, "weeklyGoal": 150,
		}),
	})
}

func mustJSON(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

// runInsights executes one recipe against a seeded store and returns the
// decoded JSON result plus whatever reached stderr.
func runInsights(t *testing.T, dbPath string, args ...string) (map[string]any, string) {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	root := newGarminInsightsCmd(flags)
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append(args, "--db", dbPath))
	if err := root.Execute(); err != nil {
		t.Fatalf("insights %v: %v (stderr %q)", args, err, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
		t.Fatalf("decoding output %q: %v", stdout.String(), err)
	}
	return result, stderr.String()
}

func seededStore(t *testing.T, seed func(*testing.T, *store.Store)) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.EnsureGarminSeriesState(); err != nil {
		t.Fatalf("ensure series state: %v", err)
	}
	if seed != nil {
		seed(t, db)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	return path
}

func nested(t *testing.T, m map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := m
	for _, key := range path {
		next, ok := cur[key].(map[string]any)
		if !ok {
			t.Fatalf("expected object at %s, got %T", strings.Join(path, "."), cur[key])
		}
		cur = next
	}
	return cur
}

func wantNumber(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()
	got, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%s: expected a number, got %#v", key, m[key])
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}

func wantNull(t *testing.T, m map[string]any, key string) {
	t.Helper()
	if m[key] != nil {
		t.Fatalf("%s = %#v, want null", key, m[key])
	}
}

// ---------------------------------------------------------------------------
// Functional — sleep
// ---------------------------------------------------------------------------

func TestInsightsSleep_ReportsDurationScoreAndStageSplit(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedSleepFixture)

	result, stderr := runInsights(t, path, "sleep", "--days", "7")

	if strings.Contains(stderr, "hint:") {
		t.Fatalf("a populated archive must not print the empty-archive hint; got %q", stderr)
	}
	coverage := nested(t, result, "coverage")
	wantNumber(t, coverage, "days_requested", 7)
	wantNumber(t, coverage, "nights_with_stats", 7)
	// The unmeasured night has a row but no duration: it must not be counted.
	wantNumber(t, coverage, "nights_with_duration", 6)
	wantNumber(t, coverage, "nights_with_score", 6)

	duration := nested(t, result, "duration_minutes")
	wantNumber(t, duration, "n", 6)
	wantNumber(t, duration, "avg", 420)
	wantNumber(t, duration, "min", 420)
	wantNumber(t, duration, "max", 420)

	score := nested(t, result, "score")
	wantNumber(t, score, "avg", 80)

	stages := nested(t, result, "stage_minutes")
	wantNumber(t, nested(t, stages, "deep"), "avg", 60)
	wantNumber(t, nested(t, stages, "rem"), "avg", 90)
	wantNumber(t, nested(t, stages, "light"), "avg", 270)
	wantNumber(t, nested(t, stages, "awake"), "avg", 30)
	// deep 60 of (60+270+90+30)=450 measured minutes.
	wantNumber(t, nested(t, stages, "deep"), "pct_of_measured", 13.3)
}

func TestInsightsSleep_TrendComparesAgainstThePriorWindow(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedSleepFixture)

	result, _ := runInsights(t, path, "sleep", "--days", "7")

	trend := nested(t, nested(t, result, "trend_vs_prior_period"), "duration_minutes")
	if trend["comparable"] != true {
		t.Fatalf("expected the trend to be comparable, got %#v", trend)
	}
	wantNumber(t, trend, "current", 420)
	wantNumber(t, trend, "prior", 360)
	wantNumber(t, trend, "delta", 60)
	wantNumber(t, trend, "prior_n", 7)

	scoreTrend := nested(t, nested(t, result, "trend_vs_prior_period"), "score")
	wantNumber(t, scoreTrend, "delta", 10)
}

func TestInsightsSleep_RollingAverageCountsOnlyMeasuredNights(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedSleepFixture)

	result, _ := runInsights(t, path, "sleep", "--days", "7")

	points, ok := nested(t, result, "rolling_7day")["duration_minutes"].([]any)
	if !ok {
		t.Fatalf("expected a rolling series, got %#v", nested(t, result, "rolling_7day")["duration_minutes"])
	}
	if len(points) != 7 {
		t.Fatalf("rolling series has %d points, want 7", len(points))
	}
	last, ok := points[len(points)-1].(map[string]any)
	if !ok {
		t.Fatalf("expected an object, got %#v", points[len(points)-1])
	}
	if last["date"] != insightsFixtureEnd {
		t.Fatalf("last rolling point is %v, want %s", last["date"], insightsFixtureEnd)
	}
	// Six measured nights in the trailing seven, all 420 minutes: the mean is
	// 420 and n is 6, not 7. A null counted as zero would give 360.
	wantNumber(t, last, "n", 6)
	wantNumber(t, last, "value", 420)
}

// ---------------------------------------------------------------------------
// Functional — training
// ---------------------------------------------------------------------------

func TestInsightsTraining_GroupsByTypeAndSumsZones(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedTrainingFixture)

	result, _ := runInsights(t, path, "training", "--days", "7")

	coverage := nested(t, result, "coverage")
	wantNumber(t, coverage, "activities", 4)
	wantNumber(t, coverage, "activities_with_zones", 3)
	wantNumber(t, coverage, "activity_types", 3)

	totals := nested(t, result, "totals")
	// 3600 + 1800 + 1800 + 3600 seconds = 180 minutes. The out-of-window
	// activity's 7200 seconds must not appear.
	wantNumber(t, totals, "duration_minutes", 180)
	wantNumber(t, totals, "distance_km", 50)
	wantNumber(t, totals, "training_load", 220)

	byType, ok := result["by_type"].([]any)
	if !ok || len(byType) != 3 {
		t.Fatalf("expected three activity types, got %#v", result["by_type"])
	}
	first, _ := byType[0].(map[string]any)
	if first["type"] != "cycling" {
		t.Fatalf("expected cycling to lead by duration, got %v", first["type"])
	}
	wantNumber(t, first, "count", 2)
	wantNumber(t, first, "duration_minutes", 90)
	wantNumber(t, first, "distance_km", 45)
	wantNumber(t, first, "avg_hr", 135)

	zones, ok := result["hr_zones"].([]any)
	if !ok || len(zones) != 5 {
		t.Fatalf("expected five zones, got %#v", result["hr_zones"])
	}
	zone3, _ := zones[2].(map[string]any)
	// 1200 + 600 + 900 = 2700 seconds of 7200 archived zone-seconds.
	wantNumber(t, zone3, "zone", 3)
	wantNumber(t, zone3, "minutes", 45)
	wantNumber(t, zone3, "pct", 37.5)

	warnings := result["warnings"].([]any)
	joined := fmt.Sprint(warnings...)
	if !strings.Contains(joined, "1 of 4 activities have no archived heart-rate zones") {
		t.Fatalf("expected the missing-zones warning, got %v", warnings)
	}
}

// N131.5.5 F-4: an activity whose archived activity_hr_zones row carries an
// empty hrTimeInZones list was recorded without a heart-rate strap. Garmin
// stores the row anyway (measured on the owner archive: every
// activity_hr_zones row is {"hrTimeInZones":[],"zoneCount":0}), so the
// presence of the row says nothing about coverage — only the entries do.
// Counting such a row made coverage.activities_with_zones equal the activity
// count on a strapless account and made the missing-zones warning unreachable,
// which is the exact misreading SKILL.md promises the field prevents.
func TestInsightsTraining_StraplessActivityIsNotCountedAsCovered(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, func(t *testing.T, db *store.Store) {
		t.Helper()
		putRows(t, db, "activities", map[string]json.RawMessage{
			"2001": activityRow("2001", "2026-03-10 07:00:00", "cycling", 3600, 30000, 100, 140),
			"2002": activityRow("2002", "2026-03-09 07:00:00", "cycling", 3600, 30000, 100, 0),
		})
		putRows(t, db, "activity_hr_zones", map[string]json.RawMessage{
			// Strapped: five zone entries, 3600 seconds in total.
			"2001": hrZonesRow("2001", []float64{600, 1200, 1200, 600, 0}),
			// Strapless: Garmin answered with an empty list, archived as-is.
			"2002": hrZonesRow("2002", nil),
		})
	})

	result, _ := runInsights(t, path, "training", "--days", "7")

	coverage := nested(t, result, "coverage")
	wantNumber(t, coverage, "activities", 2)
	wantNumber(t, coverage, "activities_with_zones", 1)

	warnings, ok := result["warnings"].([]any)
	if !ok {
		t.Fatalf("expected warnings, got %#v", result["warnings"])
	}
	joined := fmt.Sprint(warnings...)
	if !strings.Contains(joined, "1 of 2 activities have no archived heart-rate zones") {
		t.Fatalf("expected the missing-zones warning to name the strapless activity, got %v", warnings)
	}
}

func TestInsightsTraining_WeeklyBucketsAndFitnessTrend(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedTrainingFixture)

	result, _ := runInsights(t, path, "training", "--days", "7")

	weekly, ok := result["weekly"].([]any)
	if !ok || len(weekly) != 2 {
		t.Fatalf("expected two ISO weeks, got %#v", result["weekly"])
	}
	firstWeek, _ := weekly[0].(map[string]any)
	// 2026-03-04 and 2026-03-05 are both in the week starting Monday 2026-03-02.
	if firstWeek["week_start"] != "2026-03-02" {
		t.Fatalf("first bucket starts %v, want 2026-03-02", firstWeek["week_start"])
	}
	wantNumber(t, firstWeek, "activities", 2)
	secondWeek, _ := weekly[1].(map[string]any)
	if secondWeek["week_start"] != "2026-03-09" {
		t.Fatalf("second bucket starts %v, want 2026-03-09", secondWeek["week_start"])
	}
	wantNumber(t, secondWeek, "activities", 2)

	vo2 := nested(t, result, "vo2max")
	wantNumber(t, vo2, "days", 2)
	generic := nested(t, vo2, "generic")
	wantNumber(t, generic, "first", 40)
	wantNumber(t, generic, "last", 42)
	wantNumber(t, generic, "delta", 2)

	intensity := nested(t, result, "intensity_minutes")
	wantNumber(t, intensity, "weeks", 1)
	wantNumber(t, intensity, "moderate_total", 120)
}

// TestInsightsTraining_ReadinessWithNoRowsIsZeroNotAnError pins the behaviour
// agreed for the readiness discrepancy the archive walk reports: a series that
// returns nothing is reported as zero days with a warning, and this command
// does not substitute another endpoint to paper over it.
func TestInsightsTraining_ReadinessWithNoRowsIsZeroNotAnError(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedTrainingFixture)

	result, _ := runInsights(t, path, "training", "--days", "7")

	readiness := nested(t, result, "readiness")
	wantNumber(t, readiness, "days", 0)
	wantNumber(t, nested(t, readiness, "score"), "n", 0)
	wantNull(t, nested(t, readiness, "score"), "avg")
	joined := fmt.Sprint(result["warnings"].([]any)...)
	if !strings.Contains(joined, "no training readiness rows are archived") {
		t.Fatalf("expected the readiness warning, got %v", result["warnings"])
	}
}

// ---------------------------------------------------------------------------
// Functional — output conventions
// ---------------------------------------------------------------------------

func TestInsights_TableOutputNamesItsDenominators(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedSleepFixture)

	flags := &rootFlags{}
	root := newGarminInsightsCmd(flags)
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"sleep", "--days", "7", "--db", path})
	if err := root.Execute(); err != nil {
		t.Fatalf("table run: %v", err)
	}
	text := stdout.String()
	for _, want := range []string{"sleep insights", "Coverage", "nights_with_duration", "Stage", "deep", "Trend vs prior period"} {
		if !strings.Contains(text, want) {
			t.Fatalf("table output is missing %q:\n%s", want, text)
		}
	}
}

func TestInsights_CompactAndAgentFlagsProduceMachineOutput(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedTrainingFixture)

	// The root command's pre-run turns --agent into --json --compact before a
	// subcommand ever sees it (root.go:409-414), so the agent case is modelled
	// with the flags the root would have set rather than with `agent` alone.
	for name, flags := range map[string]*rootFlags{
		"compact": {compact: true},
		"agent":   {agent: true, asJSON: true, compact: true},
		"json":    {asJSON: true},
	} {
		t.Run(name, func(t *testing.T) {
			root := newGarminInsightsCmd(flags)
			var stdout, stderr strings.Builder
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs([]string{"training", "--days", "7", "--db", path})
			if err := root.Execute(); err != nil {
				t.Fatalf("%s run: %v", name, err)
			}
			var decoded any
			if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
				t.Fatalf("%s output is not JSON: %v\n%s", name, err, stdout.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Negative
// ---------------------------------------------------------------------------

func TestInsights_EmptyArchiveReturnsHonestZerosAndAHint(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, nil)

	for _, recipe := range []string{"sleep", "training"} {
		t.Run(recipe, func(t *testing.T) {
			result, stderr := runInsights(t, path, recipe, "--days", "28")

			wantNumber(t, result, "rows_read", 0)
			if !strings.Contains(stderr, insightsSyncCommand) {
				t.Fatalf("expected a hint naming %q, got %q", insightsSyncCommand, stderr)
			}
			switch recipe {
			case "sleep":
				wantNumber(t, nested(t, result, "coverage"), "nights_with_stats", 0)
				wantNull(t, nested(t, result, "duration_minutes"), "avg")
				wantNull(t, nested(t, result, "score"), "avg")
			case "training":
				wantNumber(t, nested(t, result, "totals"), "activities", 0)
				wantNumber(t, nested(t, result, "totals"), "duration_minutes", 0)
				if rows, ok := result["by_type"].([]any); !ok || len(rows) != 0 {
					t.Fatalf("by_type on an empty archive = %#v, want []", result["by_type"])
				}
			}
		})
	}
}

// TestInsights_MissingResourcesTableReadsAsEmpty covers a database file that
// exists but was never migrated — the shape a hand-copied or truncated archive
// takes. Reading it must be an empty answer, not a crash.
func TestInsights_MissingResourcesTableReadsAsEmpty(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	dir := t.TempDir()
	path := filepath.Join(dir, "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := db.DB().Exec(`DROP TABLE resources`); err != nil {
		t.Fatalf("dropping resources: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	raw := openRawArchive(t, path)
	defer raw.Close() //nolint:errcheck // test cleanup

	from := mustParseCivilDay("2026-03-01")
	to := mustParseCivilDay("2026-03-10")
	rows, err := readDayRange(raw, "sleep_stats", from, to)
	if err != nil {
		t.Fatalf("readDayRange on a database with no resources table: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
	byType, err := readResource(raw, "activities")
	if err != nil {
		t.Fatalf("readResource on a database with no resources table: %v", err)
	}
	if len(byType) != 0 {
		t.Fatalf("expected no rows, got %d", len(byType))
	}
}

func TestInsights_RejectsANonPositiveWindow(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, nil)

	flags := &rootFlags{asJSON: true}
	root := newGarminInsightsCmd(flags)
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"sleep", "--days", "0", "--db", path})
	if err := root.Execute(); err == nil {
		t.Fatal("expected --days 0 to be rejected")
	}
}

// ---------------------------------------------------------------------------
// Performance — the query shape, not the wall clock
//
// A recipe over a six-year archive must cost the days it asks for, not the
// rows the archive holds. That is a property of the query plan, so it is
// asserted directly: a plan that says SCAN has regressed even if the test
// still passes quickly on a small fixture.
// ---------------------------------------------------------------------------

func TestInsights_DayRangeQueryUsesTheResourcesPrimaryKey(t *testing.T) {
	path := seededStore(t, seedSleepFixture)
	raw := openRawArchive(t, path)
	defer raw.Close() //nolint:errcheck // test cleanup

	plan := queryPlan(t, raw, `SELECT id, data FROM resources
	           WHERE resource_type = ? AND id >= ? AND id <= ?
	           ORDER BY id`, "sleep_stats", "2026-03-04", "2026-03-10")

	if strings.Contains(plan, "SCAN resources") {
		t.Fatalf("the day-range query degraded to a table scan:\n%s", plan)
	}
	if !strings.Contains(plan, "SEARCH resources") {
		t.Fatalf("expected an indexed search over resources, got:\n%s", plan)
	}
	// The primary key is (resource_type, id), so the range must be bounded on
	// both columns rather than filtered after the fact.
	for _, want := range []string{"resource_type=?", "id>?", "id<?"} {
		if !strings.Contains(strings.ReplaceAll(plan, " ", ""), want) {
			t.Fatalf("query plan does not bound %s:\n%s", want, plan)
		}
	}
}

func TestInsights_ResourceTypeQueryUsesTheTypeIndex(t *testing.T) {
	path := seededStore(t, seedTrainingFixture)
	raw := openRawArchive(t, path)
	defer raw.Close() //nolint:errcheck // test cleanup

	plan := queryPlan(t, raw, `SELECT id, data FROM resources WHERE resource_type = ?`, "activities")

	if strings.Contains(plan, "SCAN resources") {
		t.Fatalf("the activities query degraded to a table scan:\n%s", plan)
	}
	if !strings.Contains(plan, "SEARCH resources") {
		t.Fatalf("expected an indexed search over resources, got:\n%s", plan)
	}
}

// TestInsights_WindowSizeBoundsRowsRead proves the cost claim behind the query
// shape: the same archive read over a narrower window returns proportionally
// fewer rows, so a short recipe does not pay for a long archive.
func TestInsights_WindowSizeBoundsRowsRead(t *testing.T) {
	path := seededStore(t, seedSleepFixture)
	raw := openRawArchive(t, path)
	defer raw.Close() //nolint:errcheck // test cleanup

	end := mustParseCivilDay(insightsFixtureEnd)
	wide, err := readDayRange(raw, "sleep_stats", end.addDays(-13), end)
	if err != nil {
		t.Fatalf("wide read: %v", err)
	}
	narrow, err := readDayRange(raw, "sleep_stats", end.addDays(-6), end)
	if err != nil {
		t.Fatalf("narrow read: %v", err)
	}
	if len(wide) != 14 {
		t.Fatalf("fourteen-day window read %d rows, want 14", len(wide))
	}
	if len(narrow) != 7 {
		t.Fatalf("seven-day window read %d rows, want 7", len(narrow))
	}
	// Rows come back in chronological order, which the rolling average relies on.
	for i := 1; i < len(wide); i++ {
		if wide[i-1].day >= wide[i].day {
			t.Fatalf("rows are not in ascending day order: %s then %s", wide[i-1].day, wide[i].day)
		}
	}
}

// ---------------------------------------------------------------------------
// Idempotence
// ---------------------------------------------------------------------------

// TestInsights_IsIdempotentAndReadOnly runs the same recipe twice against the
// same archive and requires byte-identical output and an unchanged database
// file, which is what "reads only the local archive" has to mean in practice.
func TestInsights_IsIdempotentAndReadOnly(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, seedSleepFixture)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	first, _ := runInsights(t, path, "sleep", "--days", "7")
	second, _ := runInsights(t, path, "sleep", "--days", "7")

	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("two runs disagree:\n%s\n%s", firstJSON, secondJSON)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-reading fixture: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("the archive file changed size, %d then %d bytes", len(before), len(after))
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func openRawArchive(t *testing.T, path string) *store.Store {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopening archive: %v", err)
	}
	return db
}

func queryPlan(t *testing.T, db *store.Store, query string, args ...any) string {
	t.Helper()
	rows, err := db.DB().Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close() //nolint:errcheck // test helper
	var lines []string
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("explain columns: %v", err)
	}
	for rows.Next() {
		cells := make([]any, len(cols))
		holders := make([]any, len(cols))
		for i := range cells {
			holders[i] = &cells[i]
		}
		if err := rows.Scan(holders...); err != nil {
			t.Fatalf("explain scan: %v", err)
		}
		parts := make([]string, 0, len(cells))
		for _, cell := range cells {
			parts = append(parts, fmt.Sprint(cell))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}
	return strings.Join(lines, "\n")
}

// TestInsightsSleep_ScoresWithoutStatsAreWarnedAbout covers a half-populated
// archive: sleep_score rows present, sleep_stats rows absent. The score block
// still answers, but duration and every stage are null, and a reader must be
// told that is a gap in the archive rather than a measured zero.
func TestInsightsSleep_ScoresWithoutStatsAreWarnedAbout(t *testing.T) {
	pinInsightsToday(t, insightsFixtureEnd)
	path := seededStore(t, func(t *testing.T, db *store.Store) {
		t.Helper()
		rows := map[string]json.RawMessage{}
		end := mustParseCivilDay(insightsFixtureEnd)
		for i := 0; i < 3; i++ {
			day := end.addDays(-i).String()
			rows[day] = mustJSON(map[string]any{"calendarDate": day, "value": 75})
		}
		putRows(t, db, "sleep_score", rows)
	})

	result, stderr := runInsights(t, path, "sleep", "--days", "7")

	if strings.Contains(stderr, "hint:") {
		t.Fatalf("rows were read, so the empty-archive hint must not fire; got %q", stderr)
	}
	wantNumber(t, nested(t, result, "coverage"), "nights_with_stats", 0)
	wantNumber(t, nested(t, result, "score"), "n", 3)
	wantNumber(t, nested(t, result, "score"), "avg", 75)
	wantNull(t, nested(t, result, "duration_minutes"), "avg")
	wantNull(t, nested(t, stagesOf(t, result), "deep"), "avg")

	joined := fmt.Sprint(result["warnings"].([]any)...)
	if !strings.Contains(joined, "no sleep_stats rows") {
		t.Fatalf("expected a half-populated-archive warning, got %v", result["warnings"])
	}
}

func stagesOf(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	return nested(t, result, "stage_minutes")
}

// ---------------------------------------------------------------------------
// N131.4g audit fixes
// ---------------------------------------------------------------------------

// F12: plan section 3 shapes the recipes as `insights <recipe> --from --to`.
// --days stays the default vocabulary; the two do not silently mix.
func TestResolveInsightsWindow(t *testing.T) {
	end := mustParseCivilDay("2026-09-08")

	t.Run("days is the default rolling window", func(t *testing.T) {
		w, err := resolveInsightsWindow(end, 28, "", "", false)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.from.String() != "2026-08-12" || w.to.String() != "2026-09-08" || w.days != 28 {
			t.Fatalf("window = %s..%s (%d days)", w.from, w.to, w.days)
		}
		if w.priorFrom.String() != "2026-07-15" || w.priorTo.String() != "2026-08-11" {
			t.Fatalf("prior = %s..%s", w.priorFrom, w.priorTo)
		}
	})

	t.Run("from and to name an explicit range", func(t *testing.T) {
		w, err := resolveInsightsWindow(end, 28, "2026-06-01", "2026-06-30", false)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.from.String() != "2026-06-01" || w.to.String() != "2026-06-30" || w.days != 30 {
			t.Fatalf("window = %s..%s (%d days)", w.from, w.to, w.days)
		}
		if w.priorFrom.String() != "2026-05-02" || w.priorTo.String() != "2026-05-31" {
			t.Fatalf("prior = %s..%s, want the equally long window before it", w.priorFrom, w.priorTo)
		}
	})

	t.Run("from alone runs through today", func(t *testing.T) {
		w, err := resolveInsightsWindow(end, 28, "2026-09-01", "", false)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.from.String() != "2026-09-01" || w.to.String() != "2026-09-08" || w.days != 8 {
			t.Fatalf("window = %s..%s (%d days)", w.from, w.to, w.days)
		}
	})

	t.Run("to alone ends a days-long window on that day", func(t *testing.T) {
		w, err := resolveInsightsWindow(end, 7, "", "2026-06-30", false)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.from.String() != "2026-06-24" || w.to.String() != "2026-06-30" || w.days != 7 {
			t.Fatalf("window = %s..%s (%d days)", w.from, w.to, w.days)
		}
	})

	t.Run("a single day is one day", func(t *testing.T) {
		w, err := resolveInsightsWindow(end, 28, "2026-06-01", "2026-06-01", false)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if w.days != 1 {
			t.Fatalf("days = %d, want 1", w.days)
		}
	})

	for name, tc := range map[string]struct {
		days         int
		from, to     string
		daysChanged  bool
		wantContains string
	}{
		"days below one":      {0, "", "", false, "--days must be at least 1"},
		"from after to":       {28, "2026-07-01", "2026-06-01", false, "is after"},
		"days beside a range": {90, "2026-06-01", "2026-06-30", true, "cannot be combined"},
		"unparseable from":    {28, "june", "", false, "--from"},
		"unparseable to":      {28, "", "2026-13-40", false, "--to"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := resolveInsightsWindow(end, tc.days, tc.from, tc.to, tc.daysChanged)
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), tc.wantContains) {
				t.Fatalf("error %q does not name %q", err, tc.wantContains)
			}
		})
	}

	t.Run("an explicit range without --days is accepted", func(t *testing.T) {
		if _, err := resolveInsightsWindow(end, 28, "2026-06-01", "2026-06-30", false); err != nil {
			t.Fatalf("resolve: %v", err)
		}
	})
}
