// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/store"
)

const dateOnly = "2006-01-02"

var localMoneyRE = regexp.MustCompile(`(?i)(US\$|CA\$|AU\$|NZ\$|HK\$|SG\$|[$€£¥₹])\s*([-+]?\d[\d,]*(?:\.\d+)?)|([-+]?\d[\d,]*(?:\.\d+)?)\s*(USD|EUR|GBP|CAD|AUD|NZD|HKD|SGD|JPY|CNY|INR)`)

func openBookingStore(ctx context.Context) (*store.Store, error) {
	st, err := store.OpenWithContext(ctx, defaultDBPath("booking-com-pp-cli"))
	if err != nil {
		return nil, err
	}
	if err := store.EnsureBookingTables(ctx, st.DB()); err != nil {
		st.Close()
		return nil, err
	}
	return st, nil
}

func dateWindow(s string) ([]time.Time, error) {
	parts := strings.Split(s, "..")
	if len(parts) != 2 {
		return nil, fmt.Errorf("window must be YYYY-MM-DD..YYYY-MM-DD")
	}
	start, err := time.Parse(dateOnly, strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(dateOnly, strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("window end must be on or after start")
	}
	out := make([]time.Time, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out, nil
}

func hotelPath(country, slug string) string { return fmt.Sprintf("/hotel/%s/%s.html", country, slug) }

func hotelParams(checkin, checkout time.Time, adults int) map[string]string {
	return map[string]string{"checkin": checkin.Format(dateOnly), "checkout": checkout.Format(dateOnly), "group_adults": strconv.Itoa(adults), "group_children": "0", "no_rooms": "1"}
}

func searchParams(query string, checkin, checkout time.Time, adults int, filters string) map[string]string {
	params := map[string]string{"ss": query, "checkin": checkin.Format(dateOnly), "checkout": checkout.Format(dateOnly), "group_adults": strconv.Itoa(adults), "group_children": "0", "no_rooms": "1"}
	for _, p := range strings.Split(filters, ",") {
		if kv := strings.SplitN(strings.TrimSpace(p), "=", 2); len(kv) == 2 && kv[0] != "" {
			params[kv[0]] = kv[1]
		}
	}
	return params
}

func parseHotel(data []byte) (booking.Property, error) {
	var prop booking.Property
	parsed, err := booking.ParseHotelDetail(data)
	if err != nil {
		return prop, err
	}
	return prop, json.Unmarshal(parsed, &prop)
}

func parseCards(data []byte) ([]booking.PropertyCard, error) {
	cards := make([]booking.PropertyCard, 0)
	parsed, err := booking.ParseSearchResults(data)
	if err != nil {
		return cards, err
	}
	return cards, json.Unmarshal(parsed, &cards)
}

func hotelPrice(p booking.Property) (float64, string) {
	if price, currency := parseLocalMoney(p.PriceRange); price > 0 {
		return price, firstNonEmptyString(currency, p.Currency)
	}
	return 0, p.Currency
}

func parseLocalMoney(text string) (float64, string) {
	m := localMoneyRE.FindStringSubmatch(text)
	if len(m) == 0 {
		return 0, ""
	}
	raw := firstNonEmptyString(m[2], m[3])
	price, _ := strconv.ParseFloat(strings.ReplaceAll(raw, ",", ""), 64)
	return price, firstNonEmptyString(m[1], m[4])
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func insertPrice(ctx context.Context, db *sql.DB, slug, country, checkin, checkout string, adults int, currency string, price float64) error {
	_, err := db.ExecContext(ctx, `INSERT OR REPLACE INTO price_history (slug,country,checkin,checkout,group_adults,currency,price,observed_at) VALUES (?,?,?,?,?,?,?,?)`,
		slug, country, checkin, checkout, adults, currency, price, time.Now().UTC().Format(time.RFC3339))
	return err
}

func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sort.Float64s(vals)
	m := len(vals) / 2
	if len(vals)%2 == 0 {
		return (vals[m-1] + vals[m]) / 2
	}
	return vals[m]
}
