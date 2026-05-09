// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — diary sync helper. Iterates a date range, fetches each
// /food/diary page, runs it through the parser, and upserts per-food rows
// into the diary_entry table.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/parser"
	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/store"
)

// SyncDiaryRange pulls diary HTML for each day in [from, to], parses it, and
// upserts the rows into diary_entry. Pacing is conservative (1 req/sec) per
// the brief's rate-limiting guidance for MFP.
func SyncDiaryRange(ctx context.Context, cfg *config.Config, s *store.Store, w io.Writer, from, to time.Time, username string) (int, error) {
	if err := s.EnsureDiaryEntries(ctx); err != nil {
		return 0, fmt.Errorf("ensuring diary schema: %w", err)
	}

	totalEntries := 0
	day := from
	for !day.After(to) {
		select {
		case <-ctx.Done():
			return totalEntries, ctx.Err()
		default:
		}

		dateStr := day.Format("2006-01-02")
		count, err := syncOneDiaryDay(cfg, s, dateStr, username)
		if err != nil {
			fmt.Fprintf(w, "  %s: ERROR %v\n", dateStr, err)
			// keep going — one bad day shouldn't stop the run
		} else {
			fmt.Fprintf(w, "  %s: %d entries\n", dateStr, count)
			totalEntries += count
		}
		day = day.AddDate(0, 0, 1)

		// Conservative 1 req/sec pacing. The brief notes intermittent 403/429
		// under bursty access, and python-myfitnesspal serializes too.
		if !day.After(to) {
			time.Sleep(1 * time.Second)
		}
	}

	return totalEntries, nil
}

func syncOneDiaryDay(cfg *config.Config, s *store.Store, date, username string) (int, error) {
	endpoint, err := buildDiaryURL(date, username)
	if err != nil {
		return 0, err
	}
	body, err := fetchAuthenticatedHTML(cfg, endpoint)
	if err != nil {
		return 0, err
	}
	day, err := parser.ParseDiary(strings.NewReader(body), date, username)
	if err != nil {
		return 0, err
	}

	rows := flattenDiaryToRows(day)
	totalsJSON, _ := json.Marshal(day.Totals)
	goalsJSON, _ := json.Marshal(day.Goals)
	if err := s.UpsertDiaryDay(context.Background(), date, rows, totalsJSON, goalsJSON, day.Complete); err != nil {
		return 0, err
	}
	return len(rows), nil
}

func flattenDiaryToRows(d *parser.Diary) []store.DiaryEntryRow {
	var rows []store.DiaryEntryRow
	pos := 0
	for _, meal := range d.Meals {
		for _, e := range meal.Entries {
			r := store.DiaryEntryRow{
				Date:          d.Date,
				Meal:          meal.Name,
				Position:      pos,
				FoodName:      e.Name,
				Calories:      e.Nutrients["calories"],
				Carbohydrates: e.Nutrients["carbohydrates"],
				Fat:           e.Nutrients["fat"],
				Protein:       e.Nutrients["protein"],
				Sodium:        e.Nutrients["sodium"],
				Sugar:         e.Nutrients["sugar"],
				Fiber:         e.Nutrients["fiber"],
				Cholesterol:   e.Nutrients["cholesterol"],
				Extras:        map[string]float64{},
			}
			// Anything that's not in the named columns goes into extras.
			named := map[string]bool{
				"name": true, "calories": true, "carbohydrates": true, "fat": true,
				"protein": true, "sodium": true, "sugar": true, "fiber": true, "cholesterol": true,
			}
			for k, v := range e.Nutrients {
				if !named[k] {
					r.Extras[k] = v
				}
			}
			rows = append(rows, r)
			pos++
		}
	}
	return rows
}
