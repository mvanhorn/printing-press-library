// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — not generated.
//
// MyFitnessPal /food/diary HTML parser. Ported from python-myfitnesspal v2.0.4
// (github.com/coddingtonbear/python-myfitnesspal/blob/master/myfitnesspal/client.py)
// — specifically _get_meals, _get_fields, _get_goals, _get_completion,
// _extract_value, _get_full_name, and _get_numeric.
//
// Why a port and not blind invention: python-myfitnesspal is the canonical
// reverse-engineered MFP wrapper, 861★, actively maintained, and proven to
// parse current MFP markup as of v2.0.4 (2025-09). MyFitnessPal periodically
// rewrites its frontend (see issues #196, #198, #203, #205); when this parser
// breaks against the live site, the fix is usually one selector change in
// python-myfitnesspal upstream that we mirror here.
package parser

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Diary represents one day's parsed food diary.
type Diary struct {
	Date      string             `json:"date"`
	Username  string             `json:"username,omitempty"`
	Meals     []Meal             `json:"meals"`
	Totals    map[string]float64 `json:"totals,omitempty"`
	Goals     map[string]float64 `json:"goals,omitempty"`
	Complete  bool               `json:"complete"`
	Fields    []string           `json:"fields,omitempty"`
	RawErrors []string           `json:"errors,omitempty"`
}

// Meal is a single named meal (Breakfast, Lunch, Dinner, Snacks, or custom)
// with its food entries.
type Meal struct {
	Name    string  `json:"name"`
	Entries []Entry `json:"entries"`
}

// Entry is one food row inside a meal.
type Entry struct {
	Name      string             `json:"name"`
	Nutrients map[string]float64 `json:"nutrients"`
}

// abbreviations maps the abbreviated header names MFP renders into the
// canonical nutrient names used by every wrapper. Mirror of python-myfitnesspal's
// ABBREVIATIONS dict (today only one mapping; the upstream project has been
// adding mappings as MFP changes column headers).
var abbreviations = map[string]string{
	"carbs": "carbohydrates",
}

var numericPattern = regexp.MustCompile(`-?\d+(?:\.\d+)?`)

// ParseDiary parses one day's /food/diary HTML page. date is the YYYY-MM-DD
// the page was fetched for; username is optional context for friend's-diary
// detection.
//
// On a logged-in 200 OK that yields zero meals (or that contains the "diary is
// locked with a key" sentinel), the parser returns a Diary with empty Meals
// and a populated RawErrors slice — the caller decides whether that's a soft
// failure (e.g., empty diary day, valid) or a hard failure (parser broke).
func ParseDiary(r io.Reader, date, username string) (*Diary, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parsing diary HTML: %w", err)
	}

	d := &Diary{Date: date, Username: username}

	body := doc.Text()
	if strings.Contains(body, "diary is locked with a key") {
		return nil, fmt.Errorf("diary is locked with a key (private)")
	}
	if strings.Contains(body, "user maintains a private diary") {
		return nil, fmt.Errorf("user maintains a private diary")
	}
	if strings.Contains(body, "Sign In") && strings.Contains(body, "Forgot your password") && doc.Find("tr.meal_header").Length() == 0 {
		return nil, fmt.Errorf("session expired — re-run `auth login --chrome` after logging in to myfitnesspal.com again")
	}

	d.Fields = extractFields(doc)
	d.Meals = extractMeals(doc, d.Fields)
	d.Totals = extractTotals(doc, d.Fields)
	d.Goals = extractGoals(doc, d.Fields)
	d.Complete = extractCompletion(doc)

	if len(d.Meals) == 0 {
		d.RawErrors = append(d.RawErrors,
			"no meal_header rows found — page structure may have changed; check upstream python-myfitnesspal selectors")
	}
	return d, nil
}

// extractFields reads the column-name header from the first meal_header row.
// Mirrors python-myfitnesspal's _get_fields. Returns ["name", "calories",
// "carbohydrates", "fat", "protein", ...] with abbreviations expanded.
func extractFields(doc *goquery.Document) []string {
	first := doc.Find("tr.meal_header").First()
	if first.Length() == 0 {
		return nil
	}
	fields := []string{"name"}
	first.Find("td").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
			return // td[0] is the meal name column header
		}
		fields = append(fields, fullName(strings.TrimSpace(s.Text())))
	})
	return fields
}

// extractMeals walks each tr.meal_header and collects the entry rows that
// follow it until a row with a class attribute appears. Mirrors
// python-myfitnesspal's _get_meals.
func extractMeals(doc *goquery.Document, fields []string) []Meal {
	var meals []Meal
	doc.Find("tr.meal_header").Each(func(i int, header *goquery.Selection) {
		mealName := strings.ToLower(strings.TrimSpace(header.Find("td").First().Text()))
		entries := []Entry{}

		for row := header.Next(); row.Length() > 0; row = row.Next() {
			if cls, exists := row.Attr("class"); exists && cls != "" {
				break
			}
			cols := row.Find("td")
			if cols.Length() == 0 {
				continue
			}
			entry := parseEntryRow(cols, fields)
			if entry.Name != "" {
				entries = append(entries, entry)
			}
		}
		meals = append(meals, Meal{Name: mealName, Entries: entries})
	})
	return meals
}

// parseEntryRow extracts one food entry row. The first td holds the food
// name (sometimes inside <a>, sometimes inside <div><a>, sometimes raw text);
// subsequent tds are nutrient values aligned to fields[1..].
func parseEntryRow(cols *goquery.Selection, fields []string) Entry {
	entry := Entry{Nutrients: map[string]float64{}}

	first := cols.First()
	if a := first.Find("a"); a.Length() > 0 {
		entry.Name = strings.TrimSpace(a.First().Text())
	}
	if entry.Name == "" {
		if div := first.Find("div"); div.Length() > 0 {
			if da := div.Find("a"); da.Length() > 0 {
				entry.Name = strings.TrimSpace(da.First().Text())
			}
			if entry.Name == "" {
				entry.Name = strings.TrimSpace(div.First().Text())
			}
		}
	}
	if entry.Name == "" {
		entry.Name = strings.TrimSpace(first.Text())
	}

	cols.Each(func(i int, td *goquery.Selection) {
		if i == 0 || i >= len(fields) {
			return
		}
		entry.Nutrients[fields[i]] = extractCellValue(td)
	})
	return entry
}

// extractTotals reads the tr.total row's nutrient columns.
func extractTotals(doc *goquery.Document, fields []string) map[string]float64 {
	totalRow := doc.Find("tr.total").First()
	if totalRow.Length() == 0 {
		return nil
	}
	out := map[string]float64{}
	totalRow.Find("td").Each(func(i int, td *goquery.Selection) {
		if i == 0 || i >= len(fields) {
			return
		}
		out[fields[i]] = extractCellValue(td)
	})
	return out
}

// extractGoals reads the row immediately following tr.total. Mirrors
// python-myfitnesspal's _get_goals.
func extractGoals(doc *goquery.Document, fields []string) map[string]float64 {
	totalRow := doc.Find("tr.total").First()
	if totalRow.Length() == 0 {
		return nil
	}
	goalRow := totalRow.Next()
	if goalRow.Length() == 0 {
		return nil
	}
	out := map[string]float64{}
	goalRow.Find("td").Each(func(i int, td *goquery.Selection) {
		if i == 0 || i >= len(fields) {
			return
		}
		out[fields[i]] = extractCellValue(td)
	})
	return out
}

// extractCompletion reads div#complete_day. Mirrors python-myfitnesspal's
// _get_completion.
func extractCompletion(doc *goquery.Document) bool {
	completion := doc.Find("div#complete_day").First()
	if completion.Length() == 0 {
		return false
	}
	first := completion.Children().First()
	cls, _ := first.Attr("class")
	return strings.Contains(cls, "day_complete_message")
}

// extractCellValue is the Go port of _extract_value: numeric value from
// element.text, falling back to span.macro-value when the cell wraps the
// number in a span. Returns 0 when no number can be extracted (matches
// python-myfitnesspal's _get_numeric behavior on parse failure).
func extractCellValue(td *goquery.Selection) float64 {
	if span := td.Find("span.macro-value"); span.Length() > 0 {
		return parseNumeric(strings.TrimSpace(span.First().Text()))
	}
	return parseNumeric(strings.TrimSpace(td.Text()))
}

// parseNumeric extracts the first numeric run from a string. Mirrors the
// permissive parsing python-myfitnesspal does in _get_numeric: strips commas,
// units, weight notation, etc.
func parseNumeric(s string) float64 {
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	match := numericPattern.FindString(s)
	if match == "" {
		return 0
	}
	v, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0
	}
	return v
}

// fullName resolves abbreviations (e.g. "carbs" → "carbohydrates"). Mirrors
// python-myfitnesspal's _get_full_name.
func fullName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if mapped, ok := abbreviations[name]; ok {
		return mapped
	}
	return name
}
