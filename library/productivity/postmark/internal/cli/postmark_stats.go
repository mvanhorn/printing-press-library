// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

// ratePct returns part/whole as a percentage rounded to 3 decimals, or 0
// when whole is zero.
func ratePct(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return roundTo(float64(part)/float64(whole)*100, 3)
}

func roundTo(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// postmarkDay is one Eastern-time day of outbound stats.
type postmarkDay struct {
	Date    string `json:"date"`
	Sent    int    `json:"sent"`
	Bounced int    `json:"bounced"`
	Spam    int    `json:"spam"`
}

// postmarkStatsDays decodes a /stats/outbound/* time series and returns the
// per-day value. sumAll adds every numeric field except Date (the bounces
// series splits counts across HardBounce, SoftBounce, Transient, and
// SMTPApiError); otherwise only field is read.
func postmarkStatsDays(raw json.RawMessage, field string, sumAll bool) (map[string]int, error) {
	var doc struct {
		Days []map[string]json.RawMessage `json:"Days"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing stats series: %w", err)
	}
	out := make(map[string]int, len(doc.Days))
	for _, day := range doc.Days {
		var date string
		if err := json.Unmarshal(day["Date"], &date); err != nil || date == "" {
			continue
		}
		date, _, _ = strings.Cut(date, "T")
		total := 0
		if sumAll {
			for k := range day {
				if k == "Date" {
					continue
				}
				if n, ok := cliutil.ExtractInt(day, k); ok {
					total += int(n)
				}
			}
		} else if n, ok := cliutil.ExtractInt(day, field); ok {
			total = int(n)
		}
		out[date] += total
	}
	return out, nil
}

// fetchPostmarkDailyStats pulls the sends, bounces, and spam series for one
// server (optionally one stream or tag) and merges them by date. Days with
// no activity are absent; callers zero-fill with zeroFillDays.
func fetchPostmarkDailyStats(ctx context.Context, c *client.Client, from, to time.Time, stream, tag string) (map[string]*postmarkDay, error) {
	params := map[string]string{
		"fromdate": from.In(postmarkEastern()).Format(postmarkDateLayout),
		"todate":   to.In(postmarkEastern()).Format(postmarkDateLayout),
	}
	if stream != "" {
		params["messagestream"] = stream
	}
	if tag != "" {
		params["tag"] = tag
	}
	merged := map[string]*postmarkDay{}
	get := func(date string) *postmarkDay {
		d, ok := merged[date]
		if !ok {
			d = &postmarkDay{Date: date}
			merged[date] = d
		}
		return d
	}
	series := []struct {
		path   string
		field  string
		sumAll bool
		apply  func(d *postmarkDay, n int)
	}{
		{"/stats/outbound/sends", "Sent", false, func(d *postmarkDay, n int) { d.Sent += n }},
		{"/stats/outbound/bounces", "", true, func(d *postmarkDay, n int) { d.Bounced += n }},
		{"/stats/outbound/spam", "SpamComplaint", false, func(d *postmarkDay, n int) { d.Spam += n }},
	}
	for _, s := range series {
		raw, err := c.Get(ctx, s.path, params)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.path, classifyAPIErrorOnly(err))
		}
		days, err := postmarkStatsDays(raw, s.field, s.sumAll)
		if err != nil {
			return nil, err
		}
		for date, n := range days {
			s.apply(get(date), n)
		}
	}
	return merged, nil
}

// zeroFillDays returns exactly n consecutive days starting at from, taking
// counts from days and filling missing dates with zeros.
func zeroFillDays(days map[string]*postmarkDay, from time.Time, n int) []postmarkDay {
	start := postmarkDayStart(from)
	out := make([]postmarkDay, 0, n)
	for i := 0; i < n; i++ {
		date := start.AddDate(0, 0, i).Format(postmarkDateLayout)
		if d, ok := days[date]; ok && d != nil {
			row := *d
			row.Date = date
			out = append(out, row)
			continue
		}
		out = append(out, postmarkDay{Date: date})
	}
	return out
}
