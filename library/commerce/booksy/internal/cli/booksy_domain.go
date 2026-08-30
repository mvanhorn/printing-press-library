// Copyright 2026 Max Tomago and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Booksy domain helpers shared by the novel commands
// (services, availability, earliest, compare, cheapest, book).
// Regen preserves this whole file.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/booksy/internal/client"
)

// ---- API response shapes (only the fields the novel commands use) ----

type bkVariant struct {
	ID           int64   `json:"id"`
	Label        string  `json:"label"`
	Type         string  `json:"type"`
	Price        float64 `json:"price"`
	Duration     int     `json:"duration"`
	ServicePrice string  `json:"service_price"`
	StafferID    []int64 `json:"staffer_id"`
}

type bkService struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Active   bool        `json:"active"`
	IsOnline bool        `json:"is_online_service"`
	Variants []bkVariant `json:"variants"`
}

type bkServiceCategory struct {
	ID       int64       `json:"id"`
	Name     string      `json:"name"`
	Services []bkService `json:"services"`
}

type bkStaff struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type bkBusiness struct {
	ID                int64               `json:"id"`
	Name              string              `json:"name"`
	Slug              string              `json:"slug"`
	Phone             string              `json:"phone"`
	URL               string              `json:"url"`
	ReviewsStars      int                 `json:"reviews_stars"`
	ReviewsRank       float64             `json:"reviews_rank"`
	ReviewsCount      int                 `json:"reviews_count"`
	Location          json.RawMessage     `json:"location"`
	Regions           json.RawMessage     `json:"regions"`
	ServiceCategories []bkServiceCategory `json:"service_categories"`
	Staff             []bkStaff           `json:"staff"`
}

// bkServiceRow is the flattened, bookable unit surfaced by `services` and used
// by earliest/compare/cheapest/book.
type bkServiceRow struct {
	Category     string  `json:"category"`
	Service      string  `json:"service"`
	VariantID    int64   `json:"service_variant_id"`
	VariantLabel string  `json:"variant_label,omitempty"`
	Price        float64 `json:"price"`
	PriceLabel   string  `json:"price_label"`
	Duration     int     `json:"duration_minutes"`
	StafferIDs   []int64 `json:"staffer_ids,omitempty"`
}

type bkSlot struct {
	T string `json:"t"`
	P string `json:"p"`
}

type bkDaySlots struct {
	Date  string   `json:"date"`
	Slots []bkSlot `json:"slots"`
}

// ---- fetch helpers ----

// fetchBusiness loads a business profile. Business detail is public (no token
// required), so this powers services/compare/cheapest without auth.
func fetchBusiness(ctx context.Context, c *client.Client, businessID string) (*bkBusiness, error) {
	data, err := c.Get(ctx, "/core/v2/customer_api/businesses/"+businessID+"/", map[string]string{
		"with_combos":   "1",
		"with_markdown": "0",
	})
	if err != nil {
		return nil, err
	}
	var env struct {
		Business bkBusiness `json:"business"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing business %s: %w", businessID, err)
	}
	if env.Business.ID == 0 {
		// Some responses may already be the bare business object.
		var bare bkBusiness
		if err := json.Unmarshal(data, &bare); err == nil && bare.ID != 0 {
			return &bare, nil
		}
	}
	return &env.Business, nil
}

// flattenServices turns the nested service_categories -> services -> variants
// tree into flat bookable rows. When query is non-empty, only rows whose
// category/service/variant text matches (diacritic-insensitive) are kept.
func flattenServices(b *bkBusiness, query string) []bkServiceRow {
	rows := make([]bkServiceRow, 0)
	for _, cat := range b.ServiceCategories {
		for _, svc := range cat.Services {
			for _, v := range svc.Variants {
				row := bkServiceRow{
					Category:     cat.Name,
					Service:      svc.Name,
					VariantID:    v.ID,
					VariantLabel: v.Label,
					Price:        v.Price,
					PriceLabel:   strings.TrimSpace(cliutilCleanPrice(v.ServicePrice)),
					Duration:     v.Duration,
					StafferIDs:   v.StafferID,
				}
				if row.PriceLabel == "" {
					row.PriceLabel = fmt.Sprintf("%.2f zł", v.Price)
				}
				if query == "" || serviceMatches(cat.Name+" "+svc.Name+" "+v.Label, query) {
					rows = append(rows, row)
				}
			}
		}
	}
	return rows
}

// cheapestMatching returns the lowest-priced service row matching query, or nil.
func cheapestMatching(b *bkBusiness, query string) *bkServiceRow {
	rows := flattenServices(b, query)
	if len(rows) == 0 {
		return nil
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Price < rows[j].Price })
	best := rows[0]
	return &best
}

// fetchTimeSlots posts to the availability endpoint (requires token). staffer
// -1 means "any staffer". Returns day-grouped slots.
func fetchTimeSlots(ctx context.Context, c *client.Client, businessID string, variantID, stafferID int64, startDate, endDate string) ([]bkDaySlots, error) {
	body := map[string]any{
		"subbookings": []map[string]any{
			{
				"service_variant_id": variantID,
				"staffer_id":         stafferID,
				"combo_children":     []any{},
			},
		},
		"start_date": startDate,
		"end_date":   endDate,
	}
	data, status, err := c.Post(ctx, "/core/v2/customer_api/me/businesses/"+businessID+"/appointments/time_slots", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("availability HTTP %d: %s", status, bkTruncate(string(data), 300))
	}
	var env struct {
		TimeSlots      []bkDaySlots `json:"time_slots"`
		StaffTimeSlots []bkDaySlots `json:"staff_time_slots"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parsing time slots: %w", err)
	}
	if len(env.TimeSlots) > 0 {
		return env.TimeSlots, nil
	}
	return env.StaffTimeSlots, nil
}

// ---- text helpers ----

// serviceMatches does a diacritic-insensitive substring match, with a small
// English->Polish grooming stem map so an agent's "haircut" finds "strzyżenie".
func serviceMatches(text, query string) bool {
	t := deaccentLower(text)
	q := deaccentLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	if strings.Contains(t, q) {
		return true
	}
	for _, stem := range englishToPolishStems(q) {
		if strings.Contains(t, stem) {
			return true
		}
	}
	return false
}

func englishToPolishStems(q string) []string {
	switch {
	case strings.Contains(q, "haircut"), strings.Contains(q, "hair cut"), q == "cut", strings.Contains(q, "trim"):
		return []string{"strzyz", "strzyż", "wlos", "fryzj"}
	case strings.Contains(q, "beard"):
		return []string{"brod", "zarost"}
	case strings.Contains(q, "shave"):
		return []string{"golen", "goli"}
	case strings.Contains(q, "hair"):
		return []string{"wlos", "fryzj", "strzyz"}
	case strings.Contains(q, "color"), strings.Contains(q, "colour"), strings.Contains(q, "dye"):
		return []string{"kolor", "farb"}
	}
	return nil
}

// deaccentLower lowercases and strips common Polish diacritics so matches work
// regardless of accent input.
func deaccentLower(s string) string {
	s = strings.ToLower(s)
	repl := strings.NewReplacer(
		"ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n",
		"ó", "o", "ś", "s", "ź", "z", "ż", "z",
	)
	return repl.Replace(s)
}

// cliutilCleanPrice normalizes a Booksy formatted price ("170,00 zł+") to a
// trimmed display string, stripping the non-breaking space Booksy uses.
func cliutilCleanPrice(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(s)
}

func bkTruncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
