// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the transcendence commands (goat, biomarker trend,
// trending, oscillating, category trend, recommendations stale, bundle).
// Reads from the local SQLite store (resources table, resource_type =
// 'results-report' shape verified live 2026-05-28 against
// https://member-app-mid.functionhealth.com/api/v1/results-report):
//
//   {
//     "data": {
//       "biomarkerResultsRecord": [
//         {
//           "biomarker": { "id": "...", "name": "Iron", "questBiomarkerCode": "...", "oneLineDescription": "..." },
//           "units": "mcg/dL",
//           "rangeMin": "50",            // STRING — needs parseFloat()
//           "rangeMax": "180",           // STRING
//           "optimalRangeMin": "27",     // STRING
//           "optimalRangeMax": "159",    // STRING
//           "currentResult":  { "id":..., "dateOfService":"YYYY-MM-DD", "calculatedResult":"<numeric-string>", "displayResult":"<numeric-string>", "inRange":true, "requisitionId":"..." },
//           "previousResult": { same shape, the round before currentResult },
//           "pastResults":    [ same shape, every earlier round ],
//           "biomarkerResults": [           // FULL history flat array
//             { "id":..., "biomarkerId":..., "dateOfService":..., "testResult":"...", "measurementUnits":"...", "testResultOutOfRange":false, "requisitionId":..., "createdAt":... },
//             ...
//           ],
//           "categories": [ "<category uuid>", ... ],
//           "outOfRangeType": "in_range" | "above_range" | "below_range",
//           "improving": false,
//           "hasNewResults": false,
//         },
//         ... one entry per biomarker the member has results for
//       ],
//       "categories": [ { "category": { "id":..., "biomarkers": [...] } }, ... ]
//     },
//     "resultsHash": "..."
//   }

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/store"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// resultRow is the normalized row our transcendence queries consume.
type resultRow struct {
	BiomarkerID    string  `json:"biomarker_id"`
	QuestCode      string  `json:"quest_code"`
	BiomarkerName  string  `json:"biomarker"`
	Category       string  `json:"category"`
	RequisitionID  string  `json:"requisition_id"`
	DrawDate       string  `json:"draw_date"`
	Value          float64 `json:"value"`
	Unit           string  `json:"unit"`
	Status         string  `json:"status"`
	Direction      string  `json:"direction"`
	QuestRangeLow  float64 `json:"quest_range_low"`
	QuestRangeHigh float64 `json:"quest_range_high"`
	OptimalLow     float64 `json:"optimal_low"`
	OptimalHigh    float64 `json:"optimal_high"`
}

func openLocalStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("function-health-pp-cli")
	}
	return store.OpenWithContext(ctx, dbPath)
}

// loadAllResults walks the synced `results-report` payload(s) and returns
// every per-round biomarker measurement as a flat slice, sorted by
// (biomarker_name, draw_date).
func loadAllResults(ctx context.Context, s *store.Store) ([]resultRow, error) {
	rows, err := s.DB().QueryContext(ctx, `
		SELECT data
		FROM resources
		WHERE resource_type = 'results-report'
	`)
	if err != nil {
		return nil, fmt.Errorf("query results-report: %w", err)
	}
	defer rows.Close()

	// Build category UUID → name map from synced categories.
	catName := loadCategoryNames(ctx, s)

	var out []resultRow
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, extractResultRows(raw, catName)...)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BiomarkerName != out[j].BiomarkerName {
			return out[i].BiomarkerName < out[j].BiomarkerName
		}
		return out[i].DrawDate < out[j].DrawDate
	})
	return out, nil
}

// loadCategoryNames returns a map of category UUID → category name from the
// synced /categories endpoint.
func loadCategoryNames(ctx context.Context, s *store.Store) map[string]string {
	names := map[string]string{}
	rows, err := s.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'categories'`)
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var entry struct {
			Category struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				CategoryName string `json:"categoryName"`
			} `json:"category"`
			// Some shapes have name at top-level:
			ID           string `json:"id"`
			Name         string `json:"name"`
			CategoryName string `json:"categoryName"`
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			continue
		}
		// Try both top-level and nested shape.
		id := entry.ID
		name := entry.CategoryName
		if name == "" {
			name = entry.Name
		}
		if id == "" {
			id = entry.Category.ID
		}
		if name == "" {
			name = entry.Category.CategoryName
		}
		if name == "" {
			name = entry.Category.Name
		}
		if id != "" && name != "" {
			names[id] = name
		}
	}
	return names
}

// extractResultRows decodes the real /api/v1/results-report shape.
func extractResultRows(raw []byte, catName map[string]string) []resultRow {
	var envelope struct {
		Data struct {
			BiomarkerResultsRecord []struct {
				QuestBiomarkerID string   `json:"questBiomarkerId"`
				Units            string   `json:"units"`
				RangeMin         string   `json:"rangeMin"`
				RangeMax         string   `json:"rangeMax"`
				OptimalRangeMin  string   `json:"optimalRangeMin"`
				OptimalRangeMax  string   `json:"optimalRangeMax"`
				OutOfRangeType   string   `json:"outOfRangeType"`
				Categories       []string `json:"categories"`
				Biomarker        struct {
					ID                 string `json:"id"`
					Name               string `json:"name"`
					QuestBiomarkerCode string `json:"questBiomarkerCode"`
					Categories         []struct {
						ID           string `json:"id"`
						CategoryName string `json:"categoryName"`
					} `json:"categories"`
				} `json:"biomarker"`
				BiomarkerResults []struct {
					ID                   string `json:"id"`
					BiomarkerID          string `json:"biomarkerId"`
					DateOfService        string `json:"dateOfService"`
					TestResult           string `json:"testResult"`
					MeasurementUnits     string `json:"measurementUnits"`
					TestResultOutOfRange bool   `json:"testResultOutOfRange"`
					RequisitionID        string `json:"requisitionId"`
					QuestReferenceRange  string `json:"questReferenceRange"`
				} `json:"biomarkerResults"`
				CurrentResult *struct {
					DateOfService    string `json:"dateOfService"`
					CalculatedResult string `json:"calculatedResult"`
					DisplayResult    string `json:"displayResult"`
					InRange          bool   `json:"inRange"`
					RequisitionID    string `json:"requisitionId"`
				} `json:"currentResult"`
			} `json:"biomarkerResultsRecord"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil
	}

	var out []resultRow
	for _, rec := range envelope.Data.BiomarkerResultsRecord {
		biomarkerName := rec.Biomarker.Name
		biomarkerID := rec.Biomarker.ID
		// questCode is the join key used by `recommendations stale`: recommendation
		// payloads reference biomarkers by Quest code, not UUID or name. The
		// record-level questBiomarkerId matches; fall back to the nested code.
		questCode := rec.QuestBiomarkerID
		if questCode == "" {
			questCode = rec.Biomarker.QuestBiomarkerCode
		}
		categoryName := ""
		// Prefer the biomarker's embedded category metadata; fall back to the UUID-only categories list.
		if len(rec.Biomarker.Categories) > 0 {
			categoryName = rec.Biomarker.Categories[0].CategoryName
		}
		if categoryName == "" && len(rec.Categories) > 0 {
			if n, ok := catName[rec.Categories[0]]; ok {
				categoryName = n
			}
		}
		optimalLow := parseFloat(rec.OptimalRangeMin)
		optimalHigh := parseFloat(rec.OptimalRangeMax)
		rangeLow := parseFloat(rec.RangeMin)
		rangeHigh := parseFloat(rec.RangeMax)

		// Use biomarkerResults[] for the full history; it has every draw.
		// Fall back to currentResult when biomarkerResults is empty (rare).
		emit := func(date, value string, outOfRange bool, requisitionID string) {
			v := parseFloat(value)
			if v == 0 && value == "" {
				return
			}
			status := "in-range"
			direction := "in"
			if outOfRange {
				status = "out-of-range"
				// Classify THIS draw's direction from its own value rather than
				// the biomarker's current outOfRangeType. rec.OutOfRangeType
				// describes only the latest round, so stamping it onto every
				// historical draw mislabels any biomarker that has crossed sides
				// over time (e.g. below_range two draws ago, above_range now).
				direction = outOfRangeDirection(v, rangeLow, rangeHigh, optimalLow, optimalHigh, rec.OutOfRangeType)
			}
			out = append(out, resultRow{
				BiomarkerID: biomarkerID, QuestCode: questCode, BiomarkerName: biomarkerName, Category: categoryName,
				RequisitionID: requisitionID, DrawDate: formatDrawDate(date),
				Value: v, Unit: rec.Units, Status: status, Direction: direction,
				QuestRangeLow: rangeLow, QuestRangeHigh: rangeHigh,
				OptimalLow: optimalLow, OptimalHigh: optimalHigh,
			})
		}
		if len(rec.BiomarkerResults) > 0 {
			for _, br := range rec.BiomarkerResults {
				unit := br.MeasurementUnits
				if unit == "" {
					unit = rec.Units
				}
				// Apply per-row override of unit when present.
				_ = unit
				emit(br.DateOfService, br.TestResult, br.TestResultOutOfRange, br.RequisitionID)
			}
		} else if rec.CurrentResult != nil {
			emit(rec.CurrentResult.DateOfService, rec.CurrentResult.CalculatedResult,
				!rec.CurrentResult.InRange, rec.CurrentResult.RequisitionID)
		}
	}
	return out
}

// outOfRangeDirection classifies a single out-of-range draw as "above" or
// "below" from that draw's own value, preferring the biomarker's reference
// (Quest) range, then the Function-optimal range, and only falling back to the
// record-level outOfRangeType when no numeric bounds are available. This keeps
// each historical draw's direction tied to that draw instead of the most recent
// round (which is all rec.OutOfRangeType reflects).
func outOfRangeDirection(v, rangeLow, rangeHigh, optimalLow, optimalHigh float64, recordType string) string {
	switch {
	case rangeHigh > 0 && v > rangeHigh:
		return "above"
	case rangeLow > 0 && v < rangeLow:
		return "below"
	case optimalHigh > 0 && v > optimalHigh:
		return "above"
	case optimalLow > 0 && v < optimalLow:
		return "below"
	case recordType != "" && recordType != "in_range":
		return strings.TrimSuffix(recordType, "_range")
	default:
		return "out"
	}
}

// parseFloat tolerates the Function Health convention of string-encoded
// numerics ("123", "45.6"). Returns 0 on empty/garbage.
func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return f
	}
	return 0
}

func filterByBiomarker(rows []resultRow, query string) []resultRow {
	if query == "" {
		return rows
	}
	q := strings.ToLower(query)
	var out []resultRow
	for _, r := range rows {
		name := strings.ToLower(r.BiomarkerName)
		if name == q || strings.Contains(name, q) || r.BiomarkerID == query {
			out = append(out, r)
		}
	}
	return out
}

func groupByBiomarker(rows []resultRow) map[string][]resultRow {
	g := make(map[string][]resultRow)
	for _, r := range rows {
		g[r.BiomarkerName] = append(g[r.BiomarkerName], r)
	}
	for k := range g {
		sort.Slice(g[k], func(i, j int) bool { return g[k][i].DrawDate < g[k][j].DrawDate })
	}
	return g
}

func optimalMidpoint(r resultRow) float64 {
	if r.OptimalLow > 0 && r.OptimalHigh > 0 {
		return (r.OptimalLow + r.OptimalHigh) / 2
	}
	if r.QuestRangeLow > 0 && r.QuestRangeHigh > 0 {
		return (r.QuestRangeLow + r.QuestRangeHigh) / 2
	}
	return 0
}

func distanceFromOptimal(r resultRow) float64 {
	m := optimalMidpoint(r)
	if m == 0 {
		return 0
	}
	return math.Abs(r.Value - m)
}

func slopePerRound(rows []resultRow, n int) float64 {
	if n <= 0 || n > len(rows) {
		n = len(rows)
	}
	if n < 2 {
		return 0
	}
	tail := rows[len(rows)-n:]
	var sumX, sumY, sumXY, sumXX float64
	for i, r := range tail {
		x := float64(i)
		sumX += x
		sumY += r.Value
		sumXY += x * r.Value
		sumXX += x * x
	}
	denom := float64(n)*sumXX - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (float64(n)*sumXY - sumX*sumY) / denom
}

func driftAway(rows []resultRow, n int) float64 {
	if len(rows) == 0 {
		return 0
	}
	slope := slopePerRound(rows, n)
	mid := optimalMidpoint(rows[len(rows)-1])
	if mid == 0 {
		return 0
	}
	currentValue := rows[len(rows)-1].Value
	if currentValue < mid {
		return -slope
	}
	return slope
}

func noStoreData(action string) error {
	return notFoundErr(fmt.Errorf("no synced lab results found locally; run `function-health-pp-cli sync` first (%s needs synced data)", action))
}

func sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	bars := []rune("▁▂▃▄▅▆▇█")
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	rng := max - min
	if rng == 0 {
		rng = 1
	}
	var b strings.Builder
	for _, v := range values {
		idx := int(((v - min) / rng) * float64(len(bars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(bars) {
			idx = len(bars) - 1
		}
		b.WriteRune(bars[idx])
	}
	return b.String()
}

func formatDrawDate(s string) string {
	if s == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05+00:00", "2006-01-02T15:04:05.000000+00:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

var errStoreEmpty = errors.New("no synced data")

func safeCloseStore(s *store.Store) {
	if s == nil {
		return
	}
	_ = s.Close()
}

func rowsExist(ctx context.Context, s *store.Store) (bool, error) {
	row := s.DB().QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM resources LIMIT 1)`)
	var x int
	if err := row.Scan(&x); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return x == 1, nil
}
