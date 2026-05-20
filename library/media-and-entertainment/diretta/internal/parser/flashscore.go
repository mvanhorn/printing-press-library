// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

// Package parser decodes the FlashScore custom delimiter format used by
// https://local-global.flashscore.ninja/2/x/feed/* endpoints.
//
// Wire format:
//   - records separated by ~ (U+007E)
//   - fields within a record separated by ¬ (U+00AC, 2-byte UTF-8 \xC2\xAC)
//   - key and value within a field separated by ÷ (U+00F7, 2-byte UTF-8 \xC3\xB7)
package parser

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

const (
	recSep   = "~"
	fieldSep = "¬" // ¬
	kvSep    = "÷" // ÷
)

// Record is a single decoded FlashScore record: a map of raw code → value.
type Record map[string]string

// ParseFeed splits raw FlashScore response bytes into individual records.
// Empty records (no key-value pairs) are dropped.
func ParseFeed(data []byte) []Record {
	raw := string(data)
	parts := strings.Split(raw, recSep)
	out := make([]Record, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, fieldSep)
		rec := make(Record, len(fields))
		for _, f := range fields {
			f = strings.TrimSpace(f)
			idx := strings.IndexRune(f, '÷')
			if idx < 0 {
				continue
			}
			k := f[:idx]
			v := f[idx+len(kvSep):]
			if k != "" {
				rec[k] = v
			}
		}
		if len(rec) > 0 {
			out = append(out, rec)
		}
	}
	return out
}

// IsFlashScore reports whether data looks like FlashScore wire format
// (contains the ÷ key-value separator) rather than JSON.
func IsFlashScore(data []byte) bool {
	return strings.ContainsRune(string(data), '÷')
}

// statusText maps FlashScore AB status codes to human-readable strings.
func statusText(code string) string {
	switch code {
	case "1":
		return "NS" // Not Started
	case "2":
		return "1H" // First Half
	case "3":
		return "FT" // Full Time
	case "4":
		return "HT" // Half Time
	case "5":
		return "2H" // Second Half
	case "6":
		return "Postp." // Postponed
	case "7":
		return "Canc." // Cancelled
	case "8":
		return "Int." // Interrupted
	case "11":
		return "ET" // Extra Time
	case "12":
		return "Pen." // Penalty Shootout
	case "31":
		return "AET" // After Extra Time
	case "32":
		return "AP" // After Penalties
	default:
		return code
	}
}

// eventTypeText maps FlashScore IE event type codes to human-readable names.
// Confirmed from live data: 1=yellow_card, 3=goal, 7=substitution.
func eventTypeText(code string) string {
	switch code {
	case "1":
		return "yellow_card"
	case "2":
		return "yellow_red_card"
	case "3":
		return "goal"
	case "4":
		return "goal" // penalty goal
	case "5":
		return "own_goal"
	case "6":
		return "missed_penalty"
	case "7":
		return "substitution"
	case "8":
		return "red_card"
	case "9":
		return "var_decision"
	default:
		return "event"
	}
}

// ParseMatches parses a FlashScore matches feed (today/yesterday/tomorrow/live)
// and returns a JSON-serializable slice of match maps with human-readable keys.
// Matches are associated with the most recently seen tournament header.
func ParseMatches(data []byte) []map[string]any {
	if !IsFlashScore(data) {
		// Already JSON (from cache) — unwrap if it's our own array
		var cached []map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return nil
	}

	records := ParseFeed(data)
	var matches []map[string]any
	var curTournament, curCountry, curTournamentID string

	for _, rec := range records {
		// Tournament header records have ZA (name) but not AA (match id)
		if za, hasZA := rec["ZA"]; hasZA {
			if _, hasAA := rec["AA"]; !hasAA {
				curTournament = za
				curCountry = rec["ZY"]
				curTournamentID = rec["ZEE"]
				continue
			}
		}

		matchID, hasAA := rec["AA"]
		if !hasAA || matchID == "" {
			continue
		}

		// Away team: FK is the primary code; fall back to AF (alternate name field)
		awayTeam := rec["FK"]
		if awayTeam == "" {
			awayTeam = rec["AF"]
		}
		// Home/away scores: AT=home FT, AH=away FT
		// BC=home HT, BD=away HT
		m := map[string]any{
			"id":            matchID,
			"tournament":    curTournament,
			"tournament_id": curTournamentID,
			"country":       curCountry,
			"home_team":     rec["CX"],
			"away_team":     awayTeam,
			"home_score":    toIntField(rec["AT"]),
			"away_score":    toIntField(rec["AH"]),
			"status":        statusText(rec["AB"]),
		}

		if ts := rec["AD"]; ts != "" {
			if tsInt, err := strconv.ParseInt(ts, 10, 64); err == nil {
				m["timestamp"] = tsInt
				m["date"] = time.Unix(tsInt, 0).Local().Format("2006-01-02 15:04")
			}
		}
		if minute := rec["GD"]; minute != "" {
			if minuteInt, err := strconv.Atoi(minute); err == nil {
				m["minute"] = minuteInt
			} else {
				m["minute"] = minute
			}
		}
		// Half-time scores
		if hs := rec["BC"]; hs != "" {
			m["home_ht"] = toIntField(hs)
		}
		if as := rec["BD"]; as != "" {
			m["away_ht"] = toIntField(as)
		}

		matches = append(matches, m)
	}
	if matches == nil {
		return []map[string]any{}
	}
	return matches
}

// ParseMatchDetail parses a match detail feed (dc_1_{matchId}).
// Returns a single map with basic match info.
func ParseMatchDetail(data []byte) map[string]any {
	if !IsFlashScore(data) {
		var cached map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return map[string]any{}
	}

	records := ParseFeed(data)
	result := map[string]any{}

	for _, rec := range records {
		if matchID, ok := rec["AA"]; ok && matchID != "" {
			result["id"] = matchID
		}
		for k, v := range rec {
			switch k {
			case "CX":
				result["home_team"] = v
			case "FK":
				result["away_team"] = v
			case "AF":
				if result["away_team"] == nil {
					result["away_team"] = v
				}
			case "AT":
				result["home_score"] = toIntField(v)
			case "AH":
				result["away_score"] = toIntField(v)
			case "BC":
				result["home_ht"] = toIntField(v)
			case "BD":
				result["away_ht"] = toIntField(v)
			case "AB":
				result["status"] = statusText(v)
			case "AD":
				if tsInt, err := strconv.ParseInt(v, 10, 64); err == nil {
					result["timestamp"] = tsInt
					result["date"] = time.Unix(tsInt, 0).Local().Format("2006-01-02 15:04")
				}
			case "GD":
				result["minute"] = v
			case "ZA":
				result["tournament"] = v
			case "ZEE":
				result["tournament_id"] = v
			case "ZY":
				result["country"] = v
			}
		}
	}
	return result
}

// ParseEvents parses a match events feed (df_sui_1_{matchId}).
// Returns a slice of event maps ordered as they appear in the feed.
func ParseEvents(data []byte) []map[string]any {
	if !IsFlashScore(data) {
		var cached []map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return nil
	}

	records := ParseFeed(data)
	var events []map[string]any
	var curPeriod string

	for _, rec := range records {
		// Period/half header: has AC (period name) but no III (event id)
		if period, hasAC := rec["AC"]; hasAC {
			if _, hasIII := rec["III"]; !hasIII {
				curPeriod = period
				continue
			}
		}

		evtID, hasIII := rec["III"]
		if !hasIII || evtID == "" {
			continue
		}

		team := "home"
		if rec["IA"] == "2" {
			team = "away"
		}

		evt := map[string]any{
			"id":          evtID,
			"period":      curPeriod,
			"team":        team,
			"minute":      rec["IB"],
			"type":        eventTypeText(rec["IE"]),
			"type_code":   rec["IE"],
			"player":      rec["IF"],
			"description": rec["IK"],
		}
		if assist := rec["IL"]; assist != "" {
			evt["assist"] = assist
		}
		events = append(events, evt)
	}
	if events == nil {
		return []map[string]any{}
	}
	return events
}

// ParseStats parses a match statistics feed (df_st_1_{matchId}).
// Returns a flat list of stat rows with section context.
func ParseStats(data []byte) []map[string]any {
	if !IsFlashScore(data) {
		var cached []map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return nil
	}

	records := ParseFeed(data)
	var stats []map[string]any
	var curSection, curSubsection string

	for _, rec := range records {
		// Section header: has SE, no SD
		if section, hasSE := rec["SE"]; hasSE {
			if _, hasSD := rec["SD"]; !hasSD {
				curSection = section
				continue
			}
		}
		// Subsection header: has SF, no SD
		if sub, hasSF := rec["SF"]; hasSF {
			if _, hasSD := rec["SD"]; !hasSD {
				curSubsection = sub
				continue
			}
		}

		statName, hasSG := rec["SG"]
		if !hasSG || statName == "" {
			continue
		}

		stat := map[string]any{
			"section":    curSection,
			"subsection": curSubsection,
			"name":       statName,
			"home_value": rec["SH"],
			"away_value": rec["SI"],
		}
		if id := rec["SD"]; id != "" {
			stat["stat_id"] = id
		}
		stats = append(stats, stat)
	}
	if stats == nil {
		return []map[string]any{}
	}
	return stats
}

// ParseH2H parses a head-to-head feed (df_hh_1_{matchId}).
// Returns a slice of historical match maps grouped by section.
func ParseH2H(data []byte) []map[string]any {
	if !IsFlashScore(data) {
		var cached []map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return nil
	}

	records := ParseFeed(data)
	var h2h []map[string]any
	var curSection string

	for _, rec := range records {
		// Section header: has KA but no KC (timestamp)
		if section, hasKA := rec["KA"]; hasKA {
			if _, hasKC := rec["KC"]; !hasKC {
				curSection = section
				continue
			}
		}
		// Text header: has KB
		if _, hasKB := rec["KB"]; hasKB {
			if _, hasKC := rec["KC"]; !hasKC {
				continue
			}
		}

		_, hasKC := rec["KC"]
		_, hasKP := rec["KP"]
		if !hasKC && !hasKP {
			continue
		}

		m := map[string]any{
			"section":    curSection,
			"id":         rec["KP"],
			"tournament": rec["KF"],
			"home_team":  rec["KJ"],
			"away_team":  rec["KK"],
			"score":      rec["KL"],
			"home_score": toIntField(rec["KU"]),
			"away_score": toIntField(rec["KT"]),
			"venue":      venueText(rec["KN"]),
		}
		if ts := rec["KC"]; ts != "" {
			if tsInt, err := strconv.ParseInt(ts, 10, 64); err == nil {
				m["timestamp"] = tsInt
				m["date"] = time.Unix(tsInt, 0).UTC().Format("2006-01-02")
			}
		}
		h2h = append(h2h, m)
	}
	if h2h == nil {
		return []map[string]any{}
	}
	return h2h
}

// venueText converts venue codes to readable strings.
func venueText(code string) string {
	switch code {
	case "lo":
		return "home"
	case "aw":
		return "away"
	case "ne":
		return "neutral"
	default:
		return code
	}
}

// ParseLineups parses a match lineups feed (df_li_1_{matchId}).
// Returns a map with home/away formations and player lists.
func ParseLineups(data []byte) map[string]any {
	if !IsFlashScore(data) {
		var cached map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return map[string]any{}
	}

	records := ParseFeed(data)
	result := map[string]any{
		"home_formation": "",
		"away_formation": "",
		"home_players":   []map[string]any{},
		"away_players":   []map[string]any{},
	}

	homePlayers := []map[string]any{}
	awayPlayers := []map[string]any{}
	var lastRating string

	for _, rec := range records {
		// Formation record: has LD (formation string) and LH (side)
		if formation, hasLD := rec["LD"]; hasLD {
			side := rec["LH"]
			if side == "0" || side == "" {
				result["home_formation"] = formation
			} else {
				result["away_formation"] = formation
			}
			// Capture rating for next player
			if r, hasLRH := rec["LRH"]; hasLRH {
				lastRating = r
			}
			// This record may also be a player record
			if playerID := rec["LP"]; playerID != "" {
				p := playerFromRecord(rec, lastRating)
				if rec["LH"] == "0" || rec["LH"] == "" {
					homePlayers = append(homePlayers, p)
				} else {
					awayPlayers = append(awayPlayers, p)
				}
				lastRating = ""
			}
			continue
		}

		// Rating pre-record
		if r, hasLRH := rec["LRH"]; hasLRH {
			lastRating = r
			// May also be a player record
			if playerID := rec["LP"]; playerID != "" {
				p := playerFromRecord(rec, lastRating)
				if rec["LH"] == "0" || rec["LH"] == "" {
					homePlayers = append(homePlayers, p)
				} else {
					awayPlayers = append(awayPlayers, p)
				}
				lastRating = ""
			}
			continue
		}

		// Player record: has LP (player id)
		if playerID := rec["LP"]; playerID != "" {
			p := playerFromRecord(rec, lastRating)
			if rec["LH"] == "0" || rec["LH"] == "" {
				homePlayers = append(homePlayers, p)
			} else {
				awayPlayers = append(awayPlayers, p)
			}
			lastRating = ""
		}
	}

	result["home_players"] = homePlayers
	result["away_players"] = awayPlayers
	return result
}

func playerFromRecord(rec Record, rating string) map[string]any {
	p := map[string]any{
		"id":       rec["LP"],
		"name":     rec["LI"],
		"jersey":   rec["LK"],
		"position": rec["LJ"],
		"role":     rec["LS"],
	}
	if rating != "" {
		p["rating"] = rating
	}
	return p
}

// ParseStandings parses a tournament standings feed.
// Returns a slice of standing row maps.
func ParseStandings(data []byte) []map[string]any {
	if !IsFlashScore(data) {
		var cached []map[string]any
		if json.Unmarshal(data, &cached) == nil {
			return cached
		}
		return nil
	}

	records := ParseFeed(data)
	var rows []map[string]any

	for _, rec := range records {
		// Standing rows have TA (team name) and TB (position)
		teamName, hasTEA := rec["TEA"]
		if !hasTEA {
			teamName = rec["TA"]
			if teamName == "" {
				continue
			}
		}

		row := map[string]any{
			"team":          teamName,
			"team_id":       rec["TB"],
			"position":      parseInt(rec["TC"]),
			"played":        parseInt(rec["TJ"]),
			"wins":          parseInt(rec["TK"]),
			"draws":         parseInt(rec["TL"]),
			"losses":        parseInt(rec["TM"]),
			"goals_for":     parseInt(rec["TN"]),
			"goals_against": parseInt(rec["TO"]),
			"goal_diff":     parseInt(rec["TP"]),
			"points":        parseInt(rec["TQ"]),
		}
		// Cleanup zero-value fields that were not in the response
		for k, v := range row {
			if v == 0 {
				// Only keep zero if the key was in the record
				code := standingsFieldCode(k)
				if code != "" {
					if _, ok := rec[code]; !ok {
						delete(row, k)
					}
				}
			}
		}
		rows = append(rows, row)
	}
	if rows == nil {
		return []map[string]any{}
	}
	return rows
}

func standingsFieldCode(field string) string {
	m := map[string]string{
		"position": "TC", "played": "TJ", "wins": "TK",
		"draws": "TL", "losses": "TM", "goals_for": "TN",
		"goals_against": "TO", "goal_diff": "TP", "points": "TQ",
	}
	return m[field]
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

// toIntField converts a string field to int when non-empty, nil when empty.
// This ensures numeric fields appear as JSON numbers rather than strings.
func toIntField(s string) any {
	if s == "" {
		return nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return s
}

// ParseSports parses the multi-sport feed (f_2_0_3_it_1).
// Returns a slice of match maps across all sports.
func ParseSports(data []byte) []map[string]any {
	return ParseMatches(data)
}

// MarshalMatches converts a parsed matches slice to JSON.
func MarshalMatches(matches []map[string]any) ([]byte, error) {
	return json.Marshal(matches)
}

// MarshalEvents converts a parsed events slice to JSON.
func MarshalEvents(events []map[string]any) ([]byte, error) {
	return json.Marshal(events)
}

// MarshalStats converts a parsed stats slice to JSON.
func MarshalStats(stats []map[string]any) ([]byte, error) {
	return json.Marshal(stats)
}

// MarshalH2H converts a parsed h2h slice to JSON.
func MarshalH2H(h2h []map[string]any) ([]byte, error) {
	return json.Marshal(h2h)
}

// MarshalLineups converts parsed lineups to JSON.
func MarshalLineups(lineups map[string]any) ([]byte, error) {
	return json.Marshal(lineups)
}

// MarshalStandings converts a parsed standings slice to JSON.
func MarshalStandings(rows []map[string]any) ([]byte, error) {
	return json.Marshal(rows)
}

// FilterLive returns only live (ongoing) matches from a parsed matches slice.
func FilterLive(matches []map[string]any) []map[string]any {
	var live []map[string]any
	for _, m := range matches {
		s, _ := m["status"].(string)
		switch s {
		case "1H", "HT", "2H", "ET", "Pen.", "AET", "AP":
			live = append(live, m)
		}
	}
	if live == nil {
		return []map[string]any{}
	}
	return live
}
