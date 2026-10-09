// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

// Package parks holds the pure, network-free logic for japan-theme-parks-pp-cli:
// the park registry, source parsers, the published TDR sale-open rule and
// wait-time statistics.
package parks

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Tokyo is the time zone of every park in the registry.
var Tokyo = mustLoadTokyo()

func mustLoadTokyo() *time.Location {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		// Japan has no DST; a fixed zone is exact when tzdata is missing.
		return time.FixedZone("JST", 9*60*60)
	}
	return loc
}

// Source names used in coverage and meta blocks.
const (
	SourceQueueTimes = "queue-times.com"
	SourceTDR        = "tokyodisneyresort.jp"
	SourceThemeParks = "api.themeparks.wiki"

	QueueTimesAttribution = "Powered by Queue-Times.com"
	QueueTimesURL         = "https://queue-times.com/"
	ThemeParksAttribution = "Powered by ThemeParks.wiki"
	ThemeParksURL         = "https://themeparks.wiki"
)

// Park is one Japan park known to the CLI.
type Park struct {
	Key               string   `json:"key"`
	QueueTimesID      int      `json:"queue_times_id"`
	NameEN            string   `json:"name_en"`
	NameJA            string   `json:"name_ja"`
	Aliases           []string `json:"aliases"`
	Timezone          string   `json:"timezone"`
	OfficialURL       string   `json:"official_url"`
	CrowdCalendarURL  string   `json:"crowd_calendar_url"`
	TDRCode           string   `json:"-"`
	ThemeParksID      string   `json:"-"`
	HoursSource       string   `json:"-"`
	TicketsSource     string   `json:"-"`
	TicketsUnknownWhy string   `json:"-"`
}

// Registry lists the five Japan parks that Queue-Times covers.
var Registry = []Park{
	{
		Key: "tdl", QueueTimesID: 274, NameEN: "Tokyo Disneyland", NameJA: "東京ディズニーランド",
		Aliases: []string{"tdl", "274", "tokyo-disneyland", "disneyland"}, Timezone: "Asia/Tokyo",
		OfficialURL: "https://www.tokyodisneyresort.jp/tdl/", CrowdCalendarURL: "https://queue-times.com/parks/274/calendar",
		TDRCode: "tdl", HoursSource: SourceTDR, TicketsSource: SourceTDR,
	},
	{
		Key: "tds", QueueTimesID: 275, NameEN: "Tokyo DisneySea", NameJA: "東京ディズニーシー",
		Aliases: []string{"tds", "275", "tokyo-disneysea", "disneysea", "disney-sea"}, Timezone: "Asia/Tokyo",
		OfficialURL: "https://www.tokyodisneyresort.jp/tds/", CrowdCalendarURL: "https://queue-times.com/parks/275/calendar",
		TDRCode: "tds", HoursSource: SourceTDR, TicketsSource: SourceTDR,
	},
	{
		Key: "usj", QueueTimesID: 284, NameEN: "Universal Studios Japan", NameJA: "ユニバーサル・スタジオ・ジャパン",
		Aliases: []string{"usj", "284", "universal", "universal-studios-japan"}, Timezone: "Asia/Tokyo",
		OfficialURL: "https://www.usj.co.jp/web/ja/jp", CrowdCalendarURL: "https://queue-times.com/parks/284/calendar",
		ThemeParksID: "47f61fac-7586-41ac-ae80-61c9257cf33e", HoursSource: SourceThemeParks,
		TicketsUnknownWhy: "USJ dated Studio Pass price and Express Pass availability are only shown in the USJ WEB ticket store, which sits behind a waiting room and robot check; this CLI does not read it. The official site publishes only \"from\" prices.",
	},
	{
		Key: "fujiq", QueueTimesID: 337, NameEN: "Fuji-Q Highland", NameJA: "富士急ハイランド",
		Aliases: []string{"fujiq", "fuji-q", "337", "fuji-q-highland"}, Timezone: "Asia/Tokyo",
		OfficialURL: "https://www.fujiq.jp/", CrowdCalendarURL: "https://queue-times.com/parks/337/calendar",
		ThemeParksID: "ae527507-b1d0-4d40-83ea-143d87bef989", HoursSource: SourceThemeParks,
		TicketsUnknownWhy: "Fuji-Q Highland ticket data is not in this CLI's sources.",
	},
	{
		Key: "legoland", QueueTimesID: 285, NameEN: "Legoland Japan", NameJA: "レゴランド・ジャパン",
		Aliases: []string{"legoland", "285", "legoland-japan"}, Timezone: "Asia/Tokyo",
		OfficialURL: "https://www.legoland.jp/", CrowdCalendarURL: "https://queue-times.com/parks/285/calendar",
		TicketsUnknownWhy: "Legoland Japan ticket data is not in this CLI's sources.",
	},
}

// HoursUnknownWhy explains missing hours coverage for a park.
func (p Park) HoursUnknownWhy() string {
	if p.HoursSource != "" {
		return ""
	}
	return p.NameEN + " hours are not in this CLI's sources."
}

// Lookup resolves a park key, Queue-Times id, alias or name (EN or JA).
func Lookup(s string) (Park, bool) {
	q := normalize(s)
	if q == "" {
		return Park{}, false
	}
	for _, p := range Registry {
		if q == p.Key || q == strconv.Itoa(p.QueueTimesID) || q == normalize(p.NameEN) || q == normalize(p.NameJA) {
			return p, true
		}
		for _, a := range p.Aliases {
			if q == normalize(a) {
				return p, true
			}
		}
	}
	return Park{}, false
}

// LookupList resolves a comma-separated park list. Empty input returns def.
func LookupList(csv string, def []string) ([]Park, error) {
	items := SplitCSV(csv)
	if len(items) == 0 {
		items = def
	}
	out := make([]Park, 0, len(items))
	seen := map[string]bool{}
	for _, it := range items {
		p, ok := Lookup(it)
		if !ok {
			return nil, fmt.Errorf("unknown park %q (use one of: %s)", it, strings.Join(Keys(), ", "))
		}
		if !seen[p.Key] {
			seen[p.Key] = true
			out = append(out, p)
		}
	}
	return out, nil
}

// Keys returns the registry keys in display order.
func Keys() []string {
	out := make([]string, 0, len(Registry))
	for _, p := range Registry {
		out = append(out, p.Key)
	}
	return out
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// SplitCSV splits a comma-separated flag value, trimming blanks.
func SplitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
