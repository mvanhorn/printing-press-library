// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	skyBase      = "https://www.shibuya-scramble-square.com"
	skyTicketURL = skyBase + "/sky/ticket/"
	skyFAQURL    = skyBase + "/sky/faq/"
	// skyWebketURL is the official web store; it sits behind a Queue-it waiting room the CLI never joins.
	skyWebketURL  = "https://webket.jp/pc/ticket/index?fc=00396&ac=8001"
	skyLeadDays   = 14
	skyPriceSplit = 15 // 15:00 JST: entries from this hour use the higher tier
)

// SHIBUYA SKY adult prices (JPY) and slot facts, read from the official
// ticket page and FAQ on 2026-10-09. Live runs re-check the web prices.
const (
	SkyWebAdultEarly     = 2700 // web, entry until 14:59
	SkyWebAdultLate      = 3400 // web, entry from 15:00
	SkyCounterAdultEarly = 3000 // 14F counter, entry until 14:59
	SkyCounterAdultLate  = 3700 // 14F counter, entry from 15:00
	SkySlotMinutes       = 20
	sunsetLeadMax        = 60 // earliest target slot start, minutes before sunset
	sunsetLeadMin        = 20 // latest target slot start, minutes before sunset
)

var (
	reNextChunk   = regexp.MustCompile(`/sky/_next/static/chunks/pages/(ticket|faq)-[a-f0-9]+\.js`)
	reUnicodeEsc  = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	skyRuleText   = "入場日の2週間前の日本時間午前0時から販売"
	skySlotText   = "20分ごとの入場時間指定制"
	skyPriceTexts = []string{yen(SkyWebAdultEarly), yen(SkyWebAdultLate), "15:00以降の入場"}
)

// DecodeJSEscapes turns \uXXXX escapes in a JavaScript bundle into text.
func DecodeJSEscapes(s string) string {
	return reUnicodeEsc.ReplaceAllStringFunc(s, func(m string) string {
		// strconv.Unquote decodes the 4-hex-digit escape without an integer-to-rune
		// conversion; an invalid escape stays as it is.
		r, err := strconv.Unquote(`"` + m + `"`)
		if err != nil {
			return m
		}
		return r
	})
}

// yen formats 2700 as "2,700円", the way the official page writes prices.
func yen(v int) string { return fmt.Sprintf("%d,%03d円", v/1000, v%1000) }

// CheckSkyFAQ reports whether the FAQ bundle still states the sale rule and slot length.
func CheckSkyFAQ(bundle string) (rule, slot bool) {
	text := DecodeJSEscapes(bundle)
	return strings.Contains(text, skyRuleText), strings.Contains(text, skySlotText)
}

// CheckSkyPrices reports whether the ticket bundle still shows the web prices.
func CheckSkyPrices(bundle string) bool {
	text := DecodeJSEscapes(bundle)
	for _, t := range skyPriceTexts {
		if !strings.Contains(text, t) {
			return false
		}
	}
	return true
}

// SkySaleMoment is 00:00 JST, 14 days before the entry date.
func SkySaleMoment(visit Date) time.Time { return visit.AddDays(-skyLeadDays).Time(0, 0) }

// SunsetPlan is the SHIBUYA SKY sunset targeting for one date.
type SunsetPlan struct {
	SunsetJST   string       `json:"sunset_jst"`
	SunsetLocal string       `json:"sunset_local,omitempty"`
	Method      string       `json:"method"`
	TargetSlots []TargetSlot `json:"target_slots"`
	SlotGrid    string       `json:"slot_grid"`
}

// TargetSlot is one 20-minute entry slot before sunset with its web price tier.
type TargetSlot struct {
	Start           string `json:"start"`
	End             string `json:"end"`
	WebAdultJPY     int    `json:"web_adult_price_jpy"`
	CounterAdultJPY int    `json:"counter_adult_price_jpy"`
	PriceTier       string `json:"price_tier"`
	MinutesBefore   int    `json:"minutes_before_sunset"`
}

// PlanSunset computes sunset at SHIBUYA SKY and the 20-minute slots that start
// between 60 and 20 minutes before it.
func PlanSunset(d Date, loc *time.Location) (*SunsetPlan, bool) {
	sunset, ok := Sunset(d, ShibuyaLat, ShibuyaLon)
	if !ok {
		return nil, false
	}
	plan := &SunsetPlan{
		SunsetJST: FormatJST(sunset),
		Method:    "computed (NOAA solar equations, Shibuya Scramble Square coordinates, about ±1 minute)",
		SlotGrid:  "assumed :00/:20/:40 starts; the source states 20-minute slots but does not publish the grid",
	}
	if loc != nil {
		plan.SunsetLocal = sunset.In(loc).Format(time.RFC3339)
	}
	plan.TargetSlots = make([]TargetSlot, 0)
	slot := SkySlotMinutes * time.Minute
	earliest := sunset.Add(-sunsetLeadMax * time.Minute)
	latest := sunset.Add(-sunsetLeadMin * time.Minute)
	start := time.Date(earliest.Year(), earliest.Month(), earliest.Day(), earliest.Hour(), (earliest.Minute()/SkySlotMinutes)*SkySlotMinutes, 0, 0, JST)
	for t := start; !t.After(latest); t = t.Add(slot) {
		if t.Before(earliest) {
			continue
		}
		web, counter, tier := SkyWebAdultEarly, SkyCounterAdultEarly, "before 15:00"
		if t.Hour() >= skyPriceSplit {
			web, counter, tier = SkyWebAdultLate, SkyCounterAdultLate, "15:00 and later"
		}
		plan.TargetSlots = append(plan.TargetSlots, TargetSlot{
			Start:           t.Format("15:04"),
			End:             t.Add(slot).Format("15:04"),
			WebAdultJPY:     web,
			CounterAdultJPY: counter,
			PriceTier:       tier,
			MinutesBefore:   int(sunset.Sub(t).Minutes()),
		})
	}
	return plan, true
}

func readSky(ctx context.Context, f *Fetcher, sd *SightData, now func() time.Time) {
	// bundle reads a Next.js page and then its page bundle, where the text lives.
	bundle := func(pageURL string) (string, bool) {
		page, err := sd.read(ctx, f, pageURL, false, now)
		if err != nil {
			sd.warn("SHIBUYA SKY page unavailable: " + err.Error())
			return "", false
		}
		path := reNextChunk.FindString(string(page))
		if path == "" {
			sd.warn("page bundle not found on " + pageURL + "; the site layout may have changed")
			return "", false
		}
		body, err := sd.read(ctx, f, skyBase+path, false, now)
		if err != nil {
			sd.warn("SHIBUYA SKY page bundle unavailable: " + err.Error())
			return "", false
		}
		return string(body), true
	}
	if faq, ok := bundle(skyFAQURL); ok {
		rule, slot := CheckSkyFAQ(faq)
		sd.RuleConfirmed = rule
		if !rule {
			sd.warn("the SHIBUYA SKY FAQ no longer shows '" + skyRuleText + "'; sale moments are estimated")
		}
		if !slot {
			sd.warn("the SHIBUYA SKY FAQ no longer shows '" + skySlotText + "'; target slots assume 20-minute entries")
		}
	}
	if ticket, ok := bundle(skyTicketURL); ok && !CheckSkyPrices(ticket) {
		sd.warn(fmt.Sprintf("the SHIBUYA SKY ticket page no longer shows the %s/%s web prices; price tiers may be stale", yen(SkyWebAdultEarly), yen(SkyWebAdultLate)))
	}
}
