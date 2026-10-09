// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:novel-static-reference — sale rules, channels and prices below were read
// from the official sources on 2026-10-09; live commands re-check the rule text
// and report drift as a warning.

package tickets

import (
	"fmt"
	"sort"
	"strings"
)

// Kind selects how a sight's sources are read.
type Kind string

const (
	KindGhibli      Kind = "ghibli"
	KindShibuyaSky  Kind = "shibuya-sky"
	KindTeamLabJSON Kind = "teamlab-ticket-site"
	KindTeamLabDMM  Kind = "teamlab-dmm"
)

// Channel is one official place to buy.
type Channel struct {
	Name         string `json:"name"`
	NameJA       string `json:"name_ja,omitempty"`
	URL          string `json:"url"`
	Requirements string `json:"requirements,omitempty"`
}

// Price is one admission price from the official source.
type Price struct {
	Label string `json:"label"`
	JPY   *int   `json:"jpy"`
	Note  string `json:"note,omitempty"`
}

// Sight is a supported timed-entry sight.
type Sight struct {
	ID            string    `json:"id"`
	NameEN        string    `json:"name_en"`
	NameJA        string    `json:"name_ja"`
	City          string    `json:"city"`
	Kind          Kind      `json:"kind"`
	SaleRule      string    `json:"sale_rule"`
	SaleRuleJA    string    `json:"sale_rule_ja,omitempty"`
	EntryRule     string    `json:"entry_rule"`
	Availability  string    `json:"availability_coverage"`
	Channels      []Channel `json:"channels"`
	Prices        []Price   `json:"prices"`
	PriceNote     string    `json:"price_note,omitempty"`
	HandoffURL    string    `json:"handoff_url"`
	SourceURLs    []string  `json:"source_urls"`
	RuleCheckedOn string    `json:"rule_checked_on"`
	Host          string    `json:"-"`
	TicketSiteURL string    `json:"-"`
	SlotMinutes   int       `json:"slot_minutes"`
}

// IsTeamLab reports whether the sight is a teamLab venue (slot stock is readable).
func (s Sight) IsTeamLab() bool { return s.Kind == KindTeamLabJSON || s.Kind == KindTeamLabDMM }

// RuleCheckedOn is the date the stored rules, channels and prices were read
// from the official sources.
const RuleCheckedOn = "2026-10-09"

var registry = []Sight{
	{
		ID:         "ghibli-museum",
		NameEN:     "Ghibli Museum, Mitaka",
		NameJA:     "三鷹の森ジブリ美術館",
		City:       "Mitaka, Tokyo",
		Kind:       KindGhibli,
		SaleRule:   "Tickets for a month go on sale at 10:00 JST on the 10th of the previous month.",
		SaleRuleJA: "毎月10日の10時から、翌月入場分を発売",
		EntryRule:  "Advance reservation only; no sale at the museum. Entry times 10:00-16:00 on the hour; enter within one hour of the ticket time.",
		Availability: "Closed days come from the museum calendar. Per-date stock is behind a Lawson Ticket member login and is reported as unknown. " +
			"Days with no general sale (for example 市民デー citizens' days) come from the Lawson Ticket Ghibli page.",
		Channels: []Channel{
			{Name: "Lawson Ticket", NameJA: "ローチケ", URL: lawsonGhibliURL, Requirements: "Free Lawson WEB member account; Japanese mobile number (090/080/070) for SMS verification for e-tickets; ticket shows the buyer's name; one purchase per account per month, up to 6 tickets."},
			{Name: "Lawson Ticket (English, overseas)", URL: "https://l-tike.com/st1/ghibli-en/Tt/Ttg010agreement/index", Requirements: "Overseas sales channel listed on the museum's English ticket page."},
			{Name: "Sunrise Tours JTB bus tour", URL: ghibliTicketsURL, Requirements: "Tour that includes the museum among several stops."},
		},
		Prices: []Price{
			{Label: "Ages 19 and over", JPY: intPtr(1000)},
			{Label: "Ages 13 to 18", JPY: intPtr(700)},
			{Label: "Ages 7 to 12", JPY: intPtr(400)},
			{Label: "Ages 4 to 6", JPY: intPtr(100)},
			{Label: "Ages 3 and under", JPY: intPtr(0), Note: "No ticket needed."},
		},
		HandoffURL:    lawsonGhibliURL,
		SourceURLs:    []string{ghibliTicketsURL, "https://www.ghibli-museum.jp/ticket/", lawsonGhibliURL},
		RuleCheckedOn: RuleCheckedOn,
		SlotMinutes:   60,
	},
	{
		ID:         "shibuya-sky",
		NameEN:     "SHIBUYA SKY",
		NameJA:     "渋谷スカイ",
		City:       "Shibuya, Tokyo",
		Kind:       KindShibuyaSky,
		SaleRule:   "Tickets for an entry date go on sale at 00:00 JST two weeks (14 days) before that date.",
		SaleRuleJA: "入場日の2週間前の日本時間午前0時から販売",
		EntryRule:  "20-minute timed entry slots. Elementary-school children and infants are sold only at the 14F counter on the day. If adult web tickets sell out, the counter is sold out too. The rooftop can close for weather.",
		Availability: "Slot availability is on the Webket store, which sits behind a virtual waiting room (Queue-it). The CLI never joins it, so availability is reported as unknown. " +
			"The sale moment, sunset time and price tier are reported for every date.",
		Channels: []Channel{
			{Name: "Webket (official web store)", URL: skyWebketURL, Requirements: "Waiting room (Queue-it) may hold you before the store opens."},
			{Name: "14F ticket counter", NameJA: "14階チケットカウンター", URL: skyTicketURL, Requirements: "Counter price is higher; children's tickets are counter-only on the day."},
		},
		Prices: []Price{
			{Label: "Web adult, entry until 14:59", JPY: intPtr(SkyWebAdultEarly)},
			{Label: "Web adult, entry from 15:00", JPY: intPtr(SkyWebAdultLate)},
			{Label: "Counter adult, entry until 14:59", JPY: intPtr(SkyCounterAdultEarly)},
			{Label: "Counter adult, entry from 15:00", JPY: intPtr(SkyCounterAdultLate)},
		},
		PriceNote:     "Adult means age 12 and over (12-year-old elementary pupils buy children's tickets at the counter).",
		HandoffURL:    skyWebketURL,
		SourceURLs:    []string{skyTicketURL, skyFAQURL},
		RuleCheckedOn: RuleCheckedOn,
		SlotMinutes:   SkySlotMinutes,
	},
	{
		ID:            "teamlab-planets",
		NameEN:        "teamLab Planets TOKYO",
		NameJA:        "チームラボプラネッツ TOKYO",
		City:          "Toyosu, Tokyo",
		Kind:          KindTeamLabDMM,
		SaleRule:      "Calendar months are released in batches; the store calendar announces the next release (for example \"1月のチケットは10月末頃販売予定\").",
		EntryRule:     "30-minute timed entry. Dynamic adult pricing.",
		Availability:  "Day status and 30-minute slot stock come from the official DMM store calendar.",
		Channels:      []Channel{{Name: "Official Ticket Store (DMM)", URL: dmmTicketENURL}},
		Prices:        []Price{{Label: "Adult (18+)", JPY: intPtr(3800), Note: "From; dynamic pricing."}, {Label: "Junior high / high school", JPY: intPtr(2800)}, {Label: "Children (4-12)", JPY: intPtr(1500)}, {Label: "Under 3", JPY: intPtr(0)}},
		HandoffURL:    dmmTicketENURL,
		SourceURLs:    []string{dmmEntryURL, "https://www.teamlab.art/e/planets/"},
		RuleCheckedOn: RuleCheckedOn,
		Host:          "teamlabplanets.dmm.com",
		SlotMinutes:   30,
	},
	teamLabJSONSight("teamlab-borderless", "teamLab Borderless: MORI Building DIGITAL ART MUSEUM", "森ビル デジタルアート ミュージアム：エプソン チームラボボーダレス", "Azabudai Hills, Tokyo", "borderless-azabudai.ticket.teamlab.art", "https://www.teamlab.art/e/tokyo/"),
	teamLabJSONSight("teamlab-kyoto", "teamLab Biovortex Kyoto", "チームラボ バイオヴォルテックス 京都", "Kyoto", "kyoto.tickets.teamlab.art", "https://www.teamlab.art/e/kyoto/"),
	teamLabJSONSight("teamlab-botanical-osaka", "teamLab Botanical Garden Osaka", "チームラボ ボタニカルガーデン 大阪", "Nagai Botanical Garden, Osaka", "botanicalgarden.ticket.teamlab.art", "https://www.teamlab.art/e/botanicalgarden/"),
	teamLabJSONSight("teamlab-okinawa", "teamLab Future Park Okinawa", "チームラボ 学ぶ！未来の遊園地 沖縄", "T Galleria Okinawa, Naha", "dfs-okinawa.ticket.teamlab.art", "https://www.teamlab.art/e/futurepark-okinawa/"),
}

func teamLabJSONSight(id, en, ja, city, host, page string) Sight {
	site := "https://" + host + "/#/?lang=en"
	return Sight{
		ID:            id,
		NameEN:        en,
		NameJA:        ja,
		City:          city,
		Kind:          KindTeamLabJSON,
		SaleRule:      "The official ticket site sells up to its published last calendar month and announces the next release (for example \"On sale at 12:00 PM on November 2 (tentative)\").",
		EntryRule:     "Timed entry (30-minute slots for most products). Adult price is dynamic.",
		Availability:  "Day status and slot stock come from the official ticket site's public calendar data.",
		Channels:      []Channel{{Name: "Official ticket site", URL: site}},
		HandoffURL:    site,
		SourceURLs:    []string{"https://" + host + "/", page},
		RuleCheckedOn: RuleCheckedOn,
		Host:          host,
		TicketSiteURL: site,
		SlotMinutes:   30,
	}
}

// All returns every supported sight in registry order.
func All() []Sight {
	out := make([]Sight, len(registry))
	copy(out, registry)
	return out
}

// IDs returns the supported sight IDs.
func IDs() []string {
	ids := make([]string, 0, len(registry))
	for _, s := range registry {
		ids = append(ids, s.ID)
	}
	return ids
}

// aliases accept short forms agents type.
var aliases = map[string]string{
	"ghibli":               "ghibli-museum",
	"ghibli-museum-mitaka": "ghibli-museum",
	"shibuya":              "shibuya-sky",
	"sky":                  "shibuya-sky",
	"planets":              "teamlab-planets",
	"borderless":           "teamlab-borderless",
	"kyoto":                "teamlab-kyoto",
	"biovortex":            "teamlab-kyoto",
	"botanical":            "teamlab-botanical-osaka",
	"osaka":                "teamlab-botanical-osaka",
	"okinawa":              "teamlab-okinawa",
}

// Lookup resolves a sight ID or alias.
func Lookup(id string) (Sight, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	if a, ok := aliases[key]; ok {
		key = a
	}
	for _, s := range registry {
		if s.ID == key {
			return s, nil
		}
	}
	return Sight{}, fmt.Errorf("unknown sight %q; use one of: %s", id, strings.Join(IDs(), ", "))
}

// Select resolves a comma-separated list. "teamlab" expands to every teamLab
// venue; an empty list returns every sight.
func Select(csv string) ([]Sight, error) {
	if strings.TrimSpace(csv) == "" {
		return All(), nil
	}
	seen := map[string]bool{}
	out := make([]Sight, 0)
	for _, part := range strings.Split(csv, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(part, "teamlab") {
			for _, s := range registry {
				if s.IsTeamLab() && !seen[s.ID] {
					seen[s.ID] = true
					out = append(out, s)
				}
			}
			continue
		}
		s, err := Lookup(part)
		if err != nil {
			return nil, err
		}
		if !seen[s.ID] {
			seen[s.ID] = true
			out = append(out, s)
		}
	}
	order := map[string]int{}
	for i, s := range registry {
		order[s.ID] = i
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no sight IDs in %q; use one of: %s", csv, strings.Join(IDs(), ", "))
	}
	sort.SliceStable(out, func(i, j int) bool { return order[out[i].ID] < order[out[j].ID] })
	return out, nil
}
