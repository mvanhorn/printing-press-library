// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package mobalytics

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Parsing strategy. Mobalytics ships an Apollo cache as inline JSON inside
// `__NEXT_DATA__` / `__APOLLO_STATE__`-style blobs. Cache keys are like:
//
//	"LolChampionBuild:{\"id\":\"...\",\"type\":\"MOST_POPULAR\",...}"
//
// We rely on stable substrings — typenames and field names — rather than
// the full cache topology. Each parser pulls just enough JSON to satisfy
// its return type. If Mobalytics changes a typename, the parser returns
// an empty slice rather than panicking.

// reTierRow extracts {role, skillLevel, tier} blocks per champion.
// The page emits objects of the form:
//
//	"slug":"jinx","riftTiers":[
//	   {"__typename":"ChampionTiersV1riftTiersChildDto",
//	    "role":"ADC","skillLevel":"low-elo","tags":null,"tier":"S"}, ...]
var reTierRow = regexp.MustCompile(
	`"slug":"([a-z0-9-]+)","riftTiers":\[(.*?)\]`)

var reTierEntry = regexp.MustCompile(
	`"role":"([A-Z_]+)","skillLevel":"([a-zA-Z-]+)"(?:[^}]*?)"tier":"([SABCD]\+?)"`)

// ParseTierList walks the embedded ChampionTiersV1 entries and returns
// a flat list of (slug, role, skillLevel, tier) rows.
func ParseTierList(html string) []TierRow {
	out := []TierRow{}
	seen := map[string]bool{}
	matches := reTierRow.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		slug := m[1]
		entries := reTierEntry.FindAllStringSubmatch(m[2], -1)
		for _, e := range entries {
			role := e[1]
			skill := e[2]
			tier := e[3]
			k := slug + "|" + role + "|" + skill
			if seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, TierRow{
				Slug:       slug,
				Role:       role,
				SkillLevel: skill,
				Tier:       tier,
			})
		}
	}
	return out
}

// FilterTierRows narrows a tier list by role/rank if non-empty. Role and
// rank are matched case-insensitively.
func FilterTierRows(rows []TierRow, role, rank string) []TierRow {
	role = strings.ToUpper(role)
	rank = strings.ToLower(rank)
	out := make([]TierRow, 0, len(rows))
	for _, r := range rows {
		if role != "" && !strings.EqualFold(r.Role, role) {
			continue
		}
		if rank != "" && !strings.EqualFold(r.SkillLevel, rank) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// SortTierRows sorts S+ < S < A < B < C < D, ties broken alphabetically.
func SortTierRows(rows []TierRow) {
	rank := func(t string) int {
		switch t {
		case "S+":
			return 0
		case "S":
			return 1
		case "A":
			return 2
		case "B":
			return 3
		case "C":
			return 4
		case "D":
			return 5
		}
		return 9
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ri, rj := rank(rows[i].Tier), rank(rows[j].Tier)
		if ri != rj {
			return ri < rj
		}
		return rows[i].Slug < rows[j].Slug
	})
}

// reStatsBlock pulls the LolChampionStats object for a champion. We pull
// patch + tier + the most recent win/pick/ban from *History arrays since
// the page does not expose flat "current" fields.
var reStatsBlock = regexp.MustCompile(
	`"LolChampionStats"(.*?)"totalMatchCount":(-?\d+)`)

var (
	reLastHistoryWR = regexp.MustCompile(`"winRateHistory":\[.*?\{"__typename":"LolChampionStatsHistoryPoint","value":([0-9.]+),"x":"([0-9.]+)"\}\]`)
	reLastHistoryPR = regexp.MustCompile(`"pickRateHistory":\[.*?\{"__typename":"LolChampionStatsHistoryPoint","value":([0-9.]+),"x":"([0-9.]+)"\}\]`)
	reLastHistoryBR = regexp.MustCompile(`"banRateHistory":\[.*?\{"__typename":"LolChampionStatsHistoryPoint","value":([0-9.]+),"x":"([0-9.]+)"\}\]`)
	reTierLetter    = regexp.MustCompile(`"tier":"([SABCD]\+?)"`)
)

// ParseChampionStats extracts the LolChampionStats block for `slug`. The
// page only carries one such block, so we anchor on the typename.
func ParseChampionStats(html string, slug string) ChampionStats {
	st := ChampionStats{Slug: slug}
	m := reStatsBlock.FindStringSubmatch(html)
	if m == nil {
		return st
	}
	block := m[0]
	if c, err := strconv.ParseInt(m[2], 10, 64); err == nil {
		st.TotalMatchCount = c
	}
	if t := reTierLetter.FindStringSubmatch(block); t != nil {
		st.Tier = t[1]
	}
	if w := reLastHistoryWR.FindStringSubmatch(block); w != nil {
		st.WinRate, _ = strconv.ParseFloat(w[1], 64)
		st.Patch = w[2]
	}
	if p := reLastHistoryPR.FindStringSubmatch(block); p != nil {
		st.PickRate, _ = strconv.ParseFloat(p[1], 64)
	}
	if b := reLastHistoryBR.FindStringSubmatch(block); b != nil {
		st.BanRate, _ = strconv.ParseFloat(b[1], 64)
	}
	return st
}

// buildEntry is the inline JSON shape Apollo writes for one build. We
// parse a subset and ignore the rest.
type buildEntry struct {
	Typename       string `json:"__typename"`
	ID             string `json:"id"`
	ChampionSlug   string `json:"championSlug"`
	Type           string `json:"type"`
	Queue          string `json:"queue"`
	VsChampionSlug string `json:"vsChampionSlug"`
	Role           string `json:"role"`
	Stats          struct {
		Wins       int64 `json:"wins"`
		MatchCount int64 `json:"matchCount"`
	} `json:"stats"`
	Perks struct {
		IDs      []int `json:"IDs"`
		Style    int   `json:"style"`
		SubStyle int   `json:"subStyle"`
	} `json:"perks"`
	Items []struct {
		Type         string `json:"type"`
		Items        []int  `json:"items"`
		Slots        []int  `json:"slots"`
		TimeToTarget int    `json:"timeToTarget"`
	} `json:"items"`
	SkillOrder    []int  `json:"skillOrder"`
	SkillMaxOrder []int  `json:"skillMaxOrder"`
	Spells        []int  `json:"spells"`
	Patch         string `json:"patch"`
}

// reBuildObj finds each {"__typename":"LolChampionBuild", ...} occurrence
// and lets us slice out a balanced JSON object.
var reBuildHead = regexp.MustCompile(`\{"__typename":"LolChampionBuild","id":"`)

// extractBalancedJSON returns the substring starting at the `{` immediately
// before index `start` and ending at the matching `}`. Assumes well-formed
// JSON inside a larger document.
func extractBalancedJSON(html string, openBrace int) string {
	if openBrace < 0 || openBrace >= len(html) || html[openBrace] != '{' {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := openBrace; i < len(html); i++ {
		c := html[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return html[openBrace : i+1]
			}
		}
	}
	return ""
}

// ParseBuilds returns every LolChampionBuild record on the page.
func ParseBuilds(html string) []ChampionBuild {
	out := []ChampionBuild{}
	idx := 0
	for {
		loc := reBuildHead.FindStringIndex(html[idx:])
		if loc == nil {
			break
		}
		start := idx + loc[0]
		// Look at the actual opening brace and slurp a balanced object.
		raw := extractBalancedJSON(html, start)
		if raw == "" {
			idx = start + 1
			continue
		}
		idx = start + len(raw)
		var be buildEntry
		if err := json.Unmarshal([]byte(raw), &be); err == nil && be.ID != "" {
			b := ChampionBuild{
				ID:            be.ID,
				Type:          be.Type,
				Queue:         be.Queue,
				Role:          be.Role,
				VsChampion:    be.VsChampionSlug,
				Wins:          be.Stats.Wins,
				MatchCount:    be.Stats.MatchCount,
				Perks:         PerkSelection{IDs: be.Perks.IDs, Style: be.Perks.Style, SubStyle: be.Perks.SubStyle},
				SkillOrder:    be.SkillOrder,
				SkillMaxOrder: be.SkillMaxOrder,
				Spells:        be.Spells,
				Patch:         be.Patch,
			}
			if be.Stats.MatchCount > 0 {
				b.WinRate = float64(be.Stats.Wins) * 100.0 / float64(be.Stats.MatchCount)
			}
			for _, it := range be.Items {
				b.Items = append(b.Items, ItemBlock{
					Type: it.Type, Items: it.Items, Slots: it.Slots, TimeToT: it.TimeToTarget,
				})
			}
			out = append(out, b)
		}
	}
	return out
}

// reCounter matches one LolChampionCounter inline JSON object.
var reCounterObj = regexp.MustCompile(
	`\{"__typename":"LolChampionCounter","matchupSlug":"([a-z0-9-]+)","matchupRole":"([A-Z_]+)","counterMetrics":\{"__typename":"LolChampionCounterMetrics","wins":(-?\d+),"looses":(-?\d+),"matchupDelta":(-?[0-9.]+)`)

// ParseCounters returns counter rows. ownSlug is filled in by the caller
// since the page does not carry it inline on each row.
func ParseCounters(html string, ownSlug string) []CounterRow {
	out := []CounterRow{}
	for _, m := range reCounterObj.FindAllStringSubmatch(html, -1) {
		opp := m[1]
		role := m[2]
		wins, _ := strconv.ParseInt(m[3], 10, 64)
		losses, _ := strconv.ParseInt(m[4], 10, 64)
		delta, _ := strconv.ParseFloat(m[5], 64)
		sample := wins + losses
		var wr float64
		if sample > 0 {
			wr = float64(wins) * 100.0 / float64(sample)
		}
		out = append(out, CounterRow{
			OwnSlug:      ownSlug,
			OpponentSlug: opp,
			Role:         role,
			Wins:         wins,
			Losses:       losses,
			Sample:       sample,
			WinRate:      wr,
			MatchupDelta: delta,
		})
	}
	return out
}

// SortCountersByDelta sorts ascending or descending by matchup delta. The
// "best counters" view wants ascending (most-negative-for-them = easiest);
// the "worst matchups" view wants descending.
func SortCountersByDelta(rows []CounterRow, descending bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		if descending {
			return rows[i].MatchupDelta > rows[j].MatchupDelta
		}
		return rows[i].MatchupDelta < rows[j].MatchupDelta
	})
}

// reSynergy pulls (slug, role, winRate) triples for same-team teammates.
var reSynergy = regexp.MustCompile(
	`"__typename":"LolChampionMatchupSynergy","championSlug":"([a-z0-9-]+)","role":"([A-Z_]+)","winRate":([0-9.]+)`)

// ParseSynergies returns synergy rows for the given champion.
func ParseSynergies(html string, ownSlug string) []SynergyRow {
	out := []SynergyRow{}
	for _, m := range reSynergy.FindAllStringSubmatch(html, -1) {
		wr, _ := strconv.ParseFloat(m[3], 64)
		out = append(out, SynergyRow{
			OwnSlug:     ownSlug,
			PartnerSlug: m[1],
			PartnerRole: m[2],
			WinRate:     wr,
		})
	}
	return out
}

// comboEntry mirrors the ChampionCombosV1 record Mobalytics SSRs onto the
// /lol/champions/<slug>/combos page. Difficulty and tags are flat arrays in
// the wire format.
type comboEntry struct {
	Typename string `json:"__typename"`
	FlatData struct {
		Slug             string   `json:"slug"`
		ChampionSlug     string   `json:"championSlug"`
		ShortDescription string   `json:"shortDescription"`
		ExecutionText    string   `json:"executionText"`
		Notes            string   `json:"notes"`
		VideoURL         string   `json:"videoUrl"`
		ThumbnailID      string   `json:"thumbnailId"`
		Tags             []string `json:"tags"`
		Sequence         []struct {
			Items []string `json:"items"`
		} `json:"sequence"`
		Difficulty []struct {
			FlatData struct {
				Slug  string `json:"slug"`
				Name  string `json:"name"`
				Index int    `json:"index"`
				Color string `json:"color"`
			} `json:"flatData"`
		} `json:"difficulty"`
	} `json:"flatData"`
}

// reComboHead matches the start of a top-level ChampionCombosV1 record.
// We anchor on `{"__typename":"ChampionCombosV1"` rather than the nested
// flatData typename so we don't double-pick the inner sequence/difficulty
// records.
var reComboHead = regexp.MustCompile(`\{"__typename":"ChampionCombosV1"`)

// ParseCombos returns every named combo Mobalytics rendered for the
// champion. The combos page server-renders the full Apollo cache; we
// balanced-brace slice each ChampionCombosV1 record and project it to
// the Combo struct.
func ParseCombos(html string) []Combo {
	out := []Combo{}
	idx := 0
	for {
		loc := reComboHead.FindStringIndex(html[idx:])
		if loc == nil {
			break
		}
		start := idx + loc[0]
		raw := extractBalancedJSON(html, start)
		if raw == "" {
			idx = start + 1
			continue
		}
		idx = start + len(raw)
		var ce comboEntry
		if err := json.Unmarshal([]byte(raw), &ce); err != nil || ce.FlatData.Slug == "" {
			continue
		}
		c := Combo{
			Slug:             ce.FlatData.Slug,
			ChampionSlug:     ce.FlatData.ChampionSlug,
			ShortDescription: ce.FlatData.ShortDescription,
			ExecutionText:    ce.FlatData.ExecutionText,
			Notes:            ce.FlatData.Notes,
			VideoURL:         ce.FlatData.VideoURL,
			ThumbnailID:      ce.FlatData.ThumbnailID,
			Tags:             ce.FlatData.Tags,
		}
		for _, s := range ce.FlatData.Sequence {
			c.Sequence = append(c.Sequence, ComboStep{Items: s.Items})
		}
		if len(ce.FlatData.Difficulty) > 0 {
			c.Difficulty = ce.FlatData.Difficulty[0].FlatData.Name
			if c.Difficulty == "" {
				c.Difficulty = ce.FlatData.Difficulty[0].FlatData.Slug
			}
		}
		out = append(out, c)
	}
	return out
}

// ItemsetBlock is the LoL client item-set JSON block shape.
type ItemsetBlock struct {
	Type  string                   `json:"type"`
	Items []map[string]interface{} `json:"items"`
}

// Itemset is the LoL client item-set JSON shape.
type Itemset struct {
	Title               string         `json:"title"`
	Type                string         `json:"type"`
	Map                 string         `json:"map"`
	Mode                string         `json:"mode"`
	PriorityItems       []int          `json:"priority_items"`
	Sortrank            int            `json:"sortrank"`
	StartedFrom         string         `json:"startedFrom"`
	AssociatedChampions []int          `json:"associatedChampions"`
	AssociatedMaps      []int          `json:"associatedMaps"`
	Blocks              []ItemsetBlock `json:"blocks"`
}

// BuildToItemset packages a Mobalytics ChampionBuild as a LoL client item set.
// championID is the riot numeric id (use Data Dragon list-champions to look up).
func BuildToItemset(b ChampionBuild, championID int, slug string, mode string) Itemset {
	title := fmt.Sprintf("Mobalytics %s (%s)", strings.ToUpper(slug[:1])+slug[1:], b.Type)
	blocks := make([]ItemsetBlock, 0, len(b.Items))
	for _, blk := range b.Items {
		entry := ItemsetBlock{Type: blk.Type}
		for _, id := range blk.Items {
			entry.Items = append(entry.Items, map[string]interface{}{
				"id":    fmt.Sprintf("%d", id),
				"count": 1,
			})
		}
		blocks = append(blocks, entry)
	}
	mapHint := "any"
	if strings.EqualFold(mode, "aram") {
		mapHint = "HA"
	}
	return Itemset{
		Title:               title,
		Type:                "custom",
		Map:                 mapHint,
		Mode:                "any",
		Sortrank:            1,
		AssociatedChampions: []int{championID},
		AssociatedMaps:      []int{},
		Blocks:              blocks,
	}
}
