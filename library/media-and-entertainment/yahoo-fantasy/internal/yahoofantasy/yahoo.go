package yahoofantasy

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// RosterSlotUpdate represents a single player -> slot assignment in a Yahoo roster update.
type RosterSlotUpdate struct {
	PlayerKey string `json:"player_key"`
	Position  string `json:"position"`
}

// RosterUpdateRequest represents the payload for PUT /team/{team_key}/roster.
type RosterUpdateRequest struct {
	CoverageType string             `json:"coverage_type"`
	Week         int                `json:"week,omitempty"`
	Date         string             `json:"date,omitempty"`
	Players      []RosterSlotUpdate `json:"players"`
}

// TransactionCreateRequest represents the payload for POST /league/{league_key}/transactions.
type TransactionCreateRequest struct {
	Type          string `json:"type"`
	TeamKey       string `json:"team_key"`
	AddPlayerKey  string `json:"add_player_key,omitempty"`
	DropPlayerKey string `json:"drop_player_key,omitempty"`
	FaabBid       int    `json:"faab_bid,omitempty"`
	HasFaabBid    bool   `json:"-"`
}

// TransactionUpdateRequest represents the payload for PUT /transaction/{transaction_key}.
type TransactionUpdateRequest struct {
	TransactionKey string `json:"transaction_key,omitempty"`
	Type           string `json:"type"`
	Action         string `json:"action,omitempty"`
	FaabBid        int    `json:"faab_bid,omitempty"`
	HasFaabBid     bool   `json:"-"`
	TradeNote      string `json:"trade_note,omitempty"`
}

// BuildRosterUpdateXML constructs Yahoo Fantasy v2 XML for PUT /team/{team_key}/roster.
func BuildRosterUpdateXML(req RosterUpdateRequest) ([]byte, error) {
	cov := strings.ToLower(strings.TrimSpace(req.CoverageType))
	if cov == "" {
		if strings.TrimSpace(req.Date) != "" {
			cov = "date"
		} else {
			cov = "week"
		}
	}
	if cov != "week" && cov != "date" {
		return nil, fmt.Errorf("coverage_type must be \"week\" or \"date\", got %q", req.CoverageType)
	}
	if cov == "week" && req.Week <= 0 {
		return nil, fmt.Errorf("week must be > 0 when coverage_type is \"week\"")
	}
	if cov == "date" && strings.TrimSpace(req.Date) == "" {
		return nil, fmt.Errorf("date (YYYY-MM-DD) is required when coverage_type is \"date\"")
	}
	if len(req.Players) == 0 {
		return nil, fmt.Errorf("at least one player slot assignment is required")
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<fantasy_content>\n  <roster>\n")
	b.WriteString("    <coverage_type>" + xmlEscape(cov) + "</coverage_type>\n")
	if cov == "week" {
		b.WriteString(fmt.Sprintf("    <week>%d</week>\n", req.Week))
	} else {
		b.WriteString("    <date>" + xmlEscape(strings.TrimSpace(req.Date)) + "</date>\n")
	}
	b.WriteString("    <players>\n")
	for _, p := range req.Players {
		pk := strings.TrimSpace(p.PlayerKey)
		pos := strings.ToUpper(strings.TrimSpace(p.Position))
		if pk == "" || pos == "" {
			return nil, fmt.Errorf("each player update requires both player_key and position")
		}
		b.WriteString("      <player>\n")
		b.WriteString("        <player_key>" + xmlEscape(pk) + "</player_key>\n")
		b.WriteString("        <position>" + xmlEscape(pos) + "</position>\n")
		b.WriteString("      </player>\n")
	}
	b.WriteString("    </players>\n  </roster>\n</fantasy_content>")
	return []byte(b.String()), nil
}

// BuildTransactionCreateXML constructs Yahoo Fantasy v2 XML for POST /league/{league_key}/transactions.
func BuildTransactionCreateXML(req TransactionCreateRequest) ([]byte, error) {
	teamKey := strings.TrimSpace(req.TeamKey)
	if teamKey == "" {
		return nil, fmt.Errorf("team_key is required for transaction creation")
	}
	addKey := strings.TrimSpace(req.AddPlayerKey)
	dropKey := strings.TrimSpace(req.DropPlayerKey)
	txType := strings.ToLower(strings.TrimSpace(req.Type))
	if txType == "" {
		switch {
		case addKey != "" && dropKey != "":
			txType = "add/drop"
		case addKey != "":
			txType = "add"
		case dropKey != "":
			txType = "drop"
		default:
			return nil, fmt.Errorf("at least one of add_player_key or drop_player_key is required")
		}
	}
	switch txType {
	case "add":
		if addKey == "" {
			return nil, fmt.Errorf("add_player_key is required when type is \"add\"")
		}
	case "drop":
		if dropKey == "" {
			return nil, fmt.Errorf("drop_player_key is required when type is \"drop\"")
		}
	case "add/drop":
		if addKey == "" || dropKey == "" {
			return nil, fmt.Errorf("both add_player_key and drop_player_key are required when type is \"add/drop\"")
		}
	default:
		return nil, fmt.Errorf("unsupported transaction type %q (expected add, drop, or add/drop)", req.Type)
	}
	if req.FaabBid < 0 {
		return nil, fmt.Errorf("faab_bid cannot be negative (%d)", req.FaabBid)
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<fantasy_content>\n  <transaction>\n")
	b.WriteString("    <type>" + xmlEscape(txType) + "</type>\n")
	if req.HasFaabBid || req.FaabBid > 0 {
		b.WriteString(fmt.Sprintf("    <faab_bid>%d</faab_bid>\n", req.FaabBid))
	}
	b.WriteString("    <players>\n")
	if addKey != "" {
		b.WriteString("      <player>\n")
		b.WriteString("        <player_key>" + xmlEscape(addKey) + "</player_key>\n")
		b.WriteString("        <transaction_data>\n")
		b.WriteString("          <type>add</type>\n")
		b.WriteString("          <destination_team_key>" + xmlEscape(teamKey) + "</destination_team_key>\n")
		b.WriteString("        </transaction_data>\n")
		b.WriteString("      </player>\n")
	}
	if dropKey != "" {
		b.WriteString("      <player>\n")
		b.WriteString("        <player_key>" + xmlEscape(dropKey) + "</player_key>\n")
		b.WriteString("        <transaction_data>\n")
		b.WriteString("          <type>drop</type>\n")
		b.WriteString("          <source_team_key>" + xmlEscape(teamKey) + "</source_team_key>\n")
		b.WriteString("        </transaction_data>\n")
		b.WriteString("      </player>\n")
	}
	b.WriteString("    </players>\n  </transaction>\n</fantasy_content>")
	return []byte(b.String()), nil
}

// BuildTransactionUpdateXML constructs Yahoo Fantasy v2 XML for PUT /transaction/{transaction_key}.
func BuildTransactionUpdateXML(req TransactionUpdateRequest) ([]byte, error) {
	txType := strings.ToLower(strings.TrimSpace(req.Type))
	if txType == "" {
		txType = "waiver"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString("<fantasy_content>\n  <transaction>\n")
	if tk := strings.TrimSpace(req.TransactionKey); tk != "" {
		b.WriteString("    <transaction_key>" + xmlEscape(tk) + "</transaction_key>\n")
	}
	b.WriteString("    <type>" + xmlEscape(txType) + "</type>\n")
	if act := strings.TrimSpace(req.Action); act != "" {
		b.WriteString("    <action>" + xmlEscape(act) + "</action>\n")
	}
	if req.HasFaabBid || req.FaabBid > 0 {
		b.WriteString(fmt.Sprintf("    <faab_bid>%d</faab_bid>\n", req.FaabBid))
	}
	if note := strings.TrimSpace(req.TradeNote); note != "" {
		b.WriteString("    <trade_note>" + xmlEscape(note) + "</trade_note>\n")
	}
	b.WriteString("  </transaction>\n</fantasy_content>")
	return []byte(b.String()), nil
}

// TranslateJSONMutationToXML inspects an outgoing request's method, path, and JSON body.
// If it targets a Yahoo Fantasy v2 XML write endpoint, it converts the JSON body into
// Yahoo's required <fantasy_content> XML format.
func TranslateJSONMutationToXML(method, path string, body []byte) ([]byte, bool, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return body, false, nil
	}
	cleanPath := strings.SplitN(path, "?", 2)[0]
	var raw map[string]any
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return body, false, nil
	}

	methodUpper := strings.ToUpper(strings.TrimSpace(method))
	switch {
	case methodUpper == "PUT" && strings.HasSuffix(cleanPath, "/roster") && strings.Contains(cleanPath, "/team/"):
		req := RosterUpdateRequest{
			CoverageType: asString(raw["coverage_type"]),
			Week:         asInt(raw["week"]),
			Date:         asString(raw["date"]),
		}
		if pk := asString(raw["player_key"]); pk != "" {
			req.Players = append(req.Players, RosterSlotUpdate{
				PlayerKey: pk,
				Position:  asString(raw["position"]),
			})
		}
		if playersAny, ok := raw["players"].([]any); ok {
			for _, item := range playersAny {
				if m, ok := item.(map[string]any); ok {
					req.Players = append(req.Players, RosterSlotUpdate{
						PlayerKey: asString(m["player_key"]),
						Position:  asString(m["position"]),
					})
				}
			}
		}
		if len(req.Players) == 0 {
			return body, false, nil
		}
		if req.CoverageType == "" && req.Week == 0 && req.Date == "" {
			req.CoverageType = "week"
			req.Week = 1
		}
		xmlBytes, err := BuildRosterUpdateXML(req)
		return xmlBytes, true, err

	case methodUpper == "POST" && strings.HasSuffix(cleanPath, "/transactions") && strings.Contains(cleanPath, "/league/"):
		_, hasFaab := raw["faab_bid"]
		req := TransactionCreateRequest{
			Type:          asString(raw["type"]),
			TeamKey:       asString(raw["team_key"]),
			AddPlayerKey:  asString(raw["add_player_key"]),
			DropPlayerKey: asString(raw["drop_player_key"]),
			FaabBid:       asInt(raw["faab_bid"]),
			HasFaabBid:    hasFaab,
		}
		if req.TeamKey == "" || (req.AddPlayerKey == "" && req.DropPlayerKey == "") {
			return body, false, nil
		}
		xmlBytes, err := BuildTransactionCreateXML(req)
		return xmlBytes, true, err

	case methodUpper == "PUT" && strings.Contains(cleanPath, "/transaction/"):
		parts := strings.Split(strings.Trim(cleanPath, "/"), "/")
		txKey := ""
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "transaction" {
				txKey = parts[i+1]
				break
			}
		}
		_, hasFaab := raw["faab_bid"]
		req := TransactionUpdateRequest{
			TransactionKey: txKey,
			Type:           asString(raw["type"]),
			Action:         asString(raw["action"]),
			FaabBid:        asInt(raw["faab_bid"]),
			HasFaabBid:     hasFaab,
			TradeNote:      asString(raw["trade_note"]),
		}
		xmlBytes, err := BuildTransactionUpdateXML(req)
		return xmlBytes, true, err
	}
	return body, false, nil
}

// NormalizeYahooResponse converts an XML response body from Yahoo into JSON if needed,
// while leaving valid JSON responses untouched.
func NormalizeYahooResponse(body []byte) []byte {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] == '{' || trimmed[0] == '[' {
		return body
	}
	if !bytes.HasPrefix(trimmed, []byte("<?xml")) && !bytes.HasPrefix(trimmed, []byte("<fantasy_content")) && !bytes.HasPrefix(trimmed, []byte("<error")) {
		return body
	}
	parsed, err := xmlToMap(trimmed)
	if err != nil {
		return body
	}
	out, err := json.Marshal(parsed)
	if err != nil {
		return body
	}
	return out
}

type xmlNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Content  string     `xml:",chardata"`
	Children []xmlNode  `xml:",any"`
}

func xmlToMap(data []byte) (map[string]any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			var root xmlNode
			if err := dec.DecodeElement(&root, &se); err != nil && err != io.EOF {
				return nil, err
			}
			return map[string]any{
				root.XMLName.Local: collapseXMLNode(root),
			}, nil
		}
	}
}

func collapseXMLNode(n xmlNode) any {
	if len(n.Children) == 0 {
		return strings.TrimSpace(n.Content)
	}
	grouped := map[string][]any{}
	order := []string{}
	for _, child := range n.Children {
		name := child.XMLName.Local
		if _, exists := grouped[name]; !exists {
			order = append(order, name)
		}
		grouped[name] = append(grouped[name], collapseXMLNode(child))
	}
	if len(order) == 1 && (order[0] == "player" || order[0] == "team" || order[0] == "transaction" || order[0] == "matchup" || order[0] == "stat" || order[0] == "position") {
		return grouped[order[0]]
	}
	res := make(map[string]any, len(order))
	for _, k := range order {
		vals := grouped[k]
		if len(vals) == 1 {
			res[k] = vals[0]
		} else {
			res[k] = vals
		}
	}
	return res
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// ParseSlotSwaps parses comma-separated POSITION=PLAYER_KEY pairs (e.g. "QB=449.p.31862,BN=449.p.30123").
func ParseSlotSwaps(setSpec string) ([]RosterSlotUpdate, error) {
	trimmed := strings.TrimSpace(setSpec)
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, ",")
	updates := make([]RosterSlotUpdate, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eq := strings.IndexByte(part, '=')
		if eq <= 0 || eq >= len(part)-1 {
			return nil, fmt.Errorf("invalid --set assignment %q: expected POSITION=PLAYER_KEY (e.g. QB=449.p.31862)", part)
		}
		left := strings.TrimSpace(part[:eq])
		right := strings.TrimSpace(part[eq+1:])
		// Support both POS=PLAYER_KEY and PLAYER_KEY=POS gracefully.
		if strings.Contains(left, ".p.") && !strings.Contains(right, ".p.") {
			updates = append(updates, RosterSlotUpdate{
				PlayerKey: left,
				Position:  strings.ToUpper(right),
			})
		} else {
			updates = append(updates, RosterSlotUpdate{
				PlayerKey: right,
				Position:  strings.ToUpper(left),
			})
		}
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("no valid slot assignments found in %q", setSpec)
	}
	return updates, nil
}

// PlayerRecord represents a normalized Yahoo Fantasy player with roster and scoring context.
type PlayerRecord struct {
	PlayerKey         string   `json:"player_key"`
	PlayerID          string   `json:"player_id,omitempty"`
	FullName          string   `json:"full_name"`
	EditorialTeamAbbr string   `json:"editorial_team_abbr,omitempty"`
	DisplayPosition   string   `json:"display_position"`
	SelectedPosition  string   `json:"selected_position,omitempty"`
	Status            string   `json:"status,omitempty"`
	StatusFull        string   `json:"status_full,omitempty"`
	EligiblePositions []string `json:"eligible_positions,omitempty"`
	PercentOwned      float64  `json:"percent_owned,omitempty"`
	PointsTotal       float64  `json:"points_total,omitempty"`
	PointsPerGame     float64  `json:"points_per_game,omitempty"`
	ProjectedPoints   float64  `json:"projected_points,omitempty"`
	ByeWeek           int      `json:"bye_week,omitempty"`
	OwnershipStatus   string   `json:"ownership_status,omitempty"` // FA, W, T
	OwnerTeamKey      string   `json:"owner_team_key,omitempty"`
}

// LineupAlert describes a starter slot issue and recommended bench replacement.
type LineupAlert struct {
	Slot                   string  `json:"slot"`
	StarterPlayerKey       string  `json:"starter_player_key"`
	StarterName            string  `json:"starter_name"`
	StarterStatus          string  `json:"starter_status"`
	Reason                 string  `json:"reason"`
	RecommendedBenchKey    string  `json:"recommended_bench_player_key,omitempty"`
	RecommendedBenchName   string  `json:"recommended_bench_player_name,omitempty"`
	RecommendedBenchPoints float64 `json:"recommended_bench_ppg,omitempty"`
}

// LineupPlan is the structured output of the lineup command.
type LineupPlan struct {
	TeamKey          string             `json:"team_key"`
	CoverageType     string             `json:"coverage_type"`
	Week             int                `json:"week,omitempty"`
	Date             string             `json:"date,omitempty"`
	ActiveStarters   int                `json:"active_starters"`
	BenchCount       int                `json:"bench_count"`
	Alerts           []LineupAlert      `json:"alerts"`
	PlannedSwaps     []RosterSlotUpdate `json:"planned_swaps"`
	XMLPayload       string             `json:"xml_payload,omitempty"`
	Applied          bool               `json:"applied"`
	UpstreamResponse any                `json:"upstream_response,omitempty"`
}

// AnalyzeLineup audits a team's roster for inactive starters, validates requested swaps, and builds the XML payload.
func AnalyzeLineup(teamKey string, week int, date string, roster []PlayerRecord, requestedSwaps []RosterSlotUpdate) (LineupPlan, error) {
	cov := "week"
	if strings.TrimSpace(date) != "" {
		cov = "date"
	} else if week <= 0 {
		week = 1
	}
	plan := LineupPlan{
		TeamKey:      strings.TrimSpace(teamKey),
		CoverageType: cov,
		Week:         week,
		Date:         strings.TrimSpace(date),
		Alerts:       make([]LineupAlert, 0),
		PlannedSwaps: make([]RosterSlotUpdate, 0),
	}

	byKey := make(map[string]PlayerRecord, len(roster))
	var starters []PlayerRecord
	var bench []PlayerRecord
	for _, p := range roster {
		byKey[p.PlayerKey] = p
		slot := strings.ToUpper(strings.TrimSpace(p.SelectedPosition))
		if slot == "" || slot == "BN" || slot == "IR" || slot == "IL" {
			bench = append(bench, p)
		} else {
			starters = append(starters, p)
		}
	}
	plan.ActiveStarters = len(starters)
	plan.BenchCount = len(bench)

	// Sort bench descending by PPG / PointsTotal for optimal replacement suggestions.
	sort.SliceStable(bench, func(i, j int) bool {
		return effectivePPG(bench[i]) > effectivePPG(bench[j])
	})

	usedBench := map[string]bool{}
	for _, st := range starters {
		reason := starterInactiveReason(st, week)
		if reason == "" {
			continue
		}
		alert := LineupAlert{
			Slot:             st.SelectedPosition,
			StarterPlayerKey: st.PlayerKey,
			StarterName:      st.FullName,
			StarterStatus:    st.Status,
			Reason:           reason,
		}
		for _, b := range bench {
			if usedBench[b.PlayerKey] {
				continue
			}
			if starterInactiveReason(b, week) != "" {
				continue
			}
			if isEligibleForSlot(b, st.SelectedPosition) {
				usedBench[b.PlayerKey] = true
				alert.RecommendedBenchKey = b.PlayerKey
				alert.RecommendedBenchName = b.FullName
				alert.RecommendedBenchPoints = effectivePPG(b)
				break
			}
		}
		plan.Alerts = append(plan.Alerts, alert)
	}

	// Validate explicit swaps if provided; otherwise if none provided and there are alerts with recommendations, keep PlannedSwaps as requestedSwaps (or suggested if caller wants).
	for _, sw := range requestedSwaps {
		if p, ok := byKey[sw.PlayerKey]; ok {
			if !isEligibleForSlot(p, sw.Position) {
				return plan, fmt.Errorf("player %s (%s, eligible: %s) is not eligible for slot %s",
					p.FullName, p.PlayerKey, strings.Join(p.EligiblePositions, "/"), sw.Position)
			}
		}
		plan.PlannedSwaps = append(plan.PlannedSwaps, sw)
	}

	if len(plan.PlannedSwaps) > 0 {
		xmlBytes, err := BuildRosterUpdateXML(RosterUpdateRequest{
			CoverageType: cov,
			Week:         week,
			Date:         date,
			Players:      plan.PlannedSwaps,
		})
		if err != nil {
			return plan, err
		}
		plan.XMLPayload = string(xmlBytes)
	}
	return plan, nil
}

func starterInactiveReason(p PlayerRecord, week int) string {
	st := strings.ToUpper(strings.TrimSpace(p.Status))
	switch st {
	case "O", "OUT":
		return "Player status is OUT"
	case "IR", "IL", "IR-R", "PUP", "NFI":
		return "Player is on Injured Reserve (" + st + ")"
	case "D", "DOUBTFUL":
		return "Player is Doubtful"
	case "SUSP":
		return "Player is Suspended"
	case "NA":
		return "Player is Inactive/Not Active"
	}
	if week > 0 && p.ByeWeek == week {
		return fmt.Sprintf("Player is on Week %d BYE", week)
	}
	return ""
}

func isEligibleForSlot(p PlayerRecord, slot string) bool {
	target := strings.ToUpper(strings.TrimSpace(slot))
	if target == "BN" || target == "BENCH" {
		return true
	}
	if target == "IR" || target == "IL" {
		st := strings.ToUpper(strings.TrimSpace(p.Status))
		return st == "IR" || st == "IL" || st == "O" || st == "OUT" || st == "PUP" || containsPos(p.EligiblePositions, target)
	}
	if len(p.EligiblePositions) == 0 && p.DisplayPosition == "" {
		return true
	}
	if containsPos(p.EligiblePositions, target) {
		return true
	}
	dispParts := strings.Split(strings.ToUpper(p.DisplayPosition), ",")
	for _, dp := range dispParts {
		dp = strings.TrimSpace(dp)
		if dp == target {
			return true
		}
		if (target == "W/R/T" || target == "FLEX") && (dp == "RB" || dp == "WR" || dp == "TE") {
			return true
		}
		if target == "W/R" && (dp == "RB" || dp == "WR") {
			return true
		}
		if target == "W/T" && (dp == "WR" || dp == "TE") {
			return true
		}
		if (target == "Q/W/R/T" || target == "SUPERFLEX" || target == "OP") && (dp == "QB" || dp == "RB" || dp == "WR" || dp == "TE") {
			return true
		}
	}
	return false
}

func containsPos(list []string, target string) bool {
	for _, item := range list {
		u := strings.ToUpper(strings.TrimSpace(item))
		if u == target {
			return true
		}
		if (target == "W/R/T" || target == "FLEX") && (u == "RB" || u == "WR" || u == "TE" || u == "W/R/T") {
			return true
		}
	}
	return false
}

func effectivePPG(p PlayerRecord) float64 {
	if p.PointsPerGame > 0 {
		return p.PointsPerGame
	}
	if p.ProjectedPoints > 0 {
		return p.ProjectedPoints
	}
	return p.PointsTotal
}

// AddDropPlan is the structured output of the add-drop command.
type AddDropPlan struct {
	LeagueKey         string `json:"league_key"`
	TeamKey           string `json:"team_key"`
	TransactionType   string `json:"transaction_type"`
	AddPlayerKey      string `json:"add_player_key,omitempty"`
	AddPlayerName     string `json:"add_player_name,omitempty"`
	DropPlayerKey     string `json:"drop_player_key,omitempty"`
	DropPlayerName    string `json:"drop_player_name,omitempty"`
	FaabBid           int    `json:"faab_bid,omitempty"`
	TeamFaabBalance   int    `json:"team_faab_balance,omitempty"`
	RemainingFaabPost int    `json:"remaining_faab_after,omitempty"`
	XMLPayload        string `json:"xml_payload"`
	Applied           bool   `json:"applied"`
	UpstreamResponse  any    `json:"upstream_response,omitempty"`
}

// PlanAddDrop validates an add/drop/FAAB request and constructs the Yahoo XML payload.
func PlanAddDrop(leagueKey, teamKey, addKey, dropKey string, faabBid int, hasFaab bool, currentFAAB int) (AddDropPlan, error) {
	lk := strings.TrimSpace(leagueKey)
	tk := strings.TrimSpace(teamKey)
	if lk == "" && strings.Contains(tk, ".t.") {
		lk = strings.SplitN(tk, ".t.", 2)[0]
	}
	if lk == "" {
		return AddDropPlan{}, fmt.Errorf("--league is required (e.g. 449.l.365083)")
	}
	if tk == "" {
		return AddDropPlan{}, fmt.Errorf("--team is required (e.g. 449.l.365083.t.1)")
	}
	ak := strings.TrimSpace(addKey)
	dk := strings.TrimSpace(dropKey)
	if ak == "" && dk == "" {
		return AddDropPlan{}, fmt.Errorf("at least one of --add or --drop is required")
	}
	if faabBid < 0 {
		return AddDropPlan{}, fmt.Errorf("--faab bid cannot be negative (%d)", faabBid)
	}
	if currentFAAB >= 0 && hasFaab && faabBid > currentFAAB {
		return AddDropPlan{}, fmt.Errorf("FAAB bid $%d exceeds team remaining FAAB balance $%d", faabBid, currentFAAB)
	}

	txType := "add"
	if ak != "" && dk != "" {
		txType = "add/drop"
	} else if dk != "" {
		txType = "drop"
	}
	xmlBytes, err := BuildTransactionCreateXML(TransactionCreateRequest{
		Type:          txType,
		TeamKey:       tk,
		AddPlayerKey:  ak,
		DropPlayerKey: dk,
		FaabBid:       faabBid,
		HasFaabBid:    hasFaab,
	})
	if err != nil {
		return AddDropPlan{}, err
	}

	rem := 0
	if currentFAAB >= 0 {
		rem = currentFAAB - faabBid
		if rem < 0 {
			rem = 0
		}
	}
	return AddDropPlan{
		LeagueKey:         lk,
		TeamKey:           tk,
		TransactionType:   txType,
		AddPlayerKey:      ak,
		DropPlayerKey:     dk,
		FaabBid:           faabBid,
		TeamFaabBalance:   maxInt(currentFAAB, 0),
		RemainingFaabPost: rem,
		XMLPayload:        string(xmlBytes),
	}, nil
}

// WaiverUpgrade represents a ranked free-agent or waiver pickup recommendation.
type WaiverUpgrade struct {
	Rank                int     `json:"rank"`
	AddPlayerKey        string  `json:"add_player_key"`
	AddPlayerName       string  `json:"add_player_name"`
	Position            string  `json:"position"`
	TeamAbbr            string  `json:"team_abbr,omitempty"`
	OwnershipStatus     string  `json:"ownership_status"`
	PercentOwned        float64 `json:"percent_owned"`
	CandidatePPG        float64 `json:"candidate_ppg"`
	CandidateTotalPts   float64 `json:"candidate_total_points"`
	DropPlayerKey       string  `json:"drop_player_key,omitempty"`
	DropPlayerName      string  `json:"drop_player_name,omitempty"`
	DropPlayerPPG       float64 `json:"drop_player_ppg"`
	NetUpgradePPG       float64 `json:"net_upgrade_ppg"`
	SuggestedAddDropCmd string  `json:"suggested_command,omitempty"`
}

// RankWaiverTargets joins available free agents/waivers against a team's roster and ranks by net PPG upgrade.
func RankWaiverTargets(leagueKey, teamKey string, candidates []PlayerRecord, roster []PlayerRecord, posFilter string, limit int) []WaiverUpgrade {
	posFilter = strings.ToUpper(strings.TrimSpace(posFilter))
	rosteredKeys := make(map[string]bool, len(roster))
	weakestByPos := make(map[string]PlayerRecord)

	for _, rp := range roster {
		rosteredKeys[rp.PlayerKey] = true
		positions := rp.EligiblePositions
		if len(positions) == 0 && rp.DisplayPosition != "" {
			positions = strings.Split(rp.DisplayPosition, ",")
		}
		for _, pos := range positions {
			pos = strings.ToUpper(strings.TrimSpace(pos))
			if pos == "" || pos == "BN" || pos == "IR" || pos == "W/R/T" {
				continue
			}
			cur, exists := weakestByPos[pos]
			// Prefer bench players as drop candidates over active starters.
			rpBench := strings.EqualFold(rp.SelectedPosition, "BN") || rp.SelectedPosition == ""
			curBench := strings.EqualFold(cur.SelectedPosition, "BN") || cur.SelectedPosition == ""
			if !exists || (rpBench && !curBench) || (rpBench == curBench && effectivePPG(rp) < effectivePPG(cur)) {
				weakestByPos[pos] = rp
			}
		}
	}

	upgrades := make([]WaiverUpgrade, 0, len(candidates))
	for _, cand := range candidates {
		if rosteredKeys[cand.PlayerKey] {
			continue
		}
		if cand.OwnershipStatus == "T" && cand.OwnerTeamKey != "" {
			continue
		}
		primaryPos := strings.ToUpper(strings.TrimSpace(strings.Split(cand.DisplayPosition, ",")[0]))
		if posFilter != "" && primaryPos != posFilter && !containsPos(cand.EligiblePositions, posFilter) {
			continue
		}
		matchPos := primaryPos
		if posFilter != "" {
			matchPos = posFilter
		}
		weakest, hasWeakest := weakestByPos[matchPos]
		candPPG := round2(effectivePPG(cand))
		dropPPG := 0.0
		dropKey := ""
		dropName := ""
		if hasWeakest {
			dropPPG = round2(effectivePPG(weakest))
			dropKey = weakest.PlayerKey
			dropName = weakest.FullName
		}
		netDelta := round2(candPPG - dropPPG)
		ownStatus := cand.OwnershipStatus
		if ownStatus == "" {
			ownStatus = "FA"
		}
		cmdStr := fmt.Sprintf("yahoo-fantasy-pp-cli add-drop --league %s --team %s --add %s", leagueKey, teamKey, cand.PlayerKey)
		if dropKey != "" {
			cmdStr += fmt.Sprintf(" --drop %s", dropKey)
		}
		cmdStr += " --dry-run"
		upgrades = append(upgrades, WaiverUpgrade{
			AddPlayerKey:        cand.PlayerKey,
			AddPlayerName:       cand.FullName,
			Position:            matchPos,
			TeamAbbr:            cand.EditorialTeamAbbr,
			OwnershipStatus:     ownStatus,
			PercentOwned:        round2(cand.PercentOwned),
			CandidatePPG:        candPPG,
			CandidateTotalPts:   round2(cand.PointsTotal),
			DropPlayerKey:       dropKey,
			DropPlayerName:      dropName,
			DropPlayerPPG:       dropPPG,
			NetUpgradePPG:       netDelta,
			SuggestedAddDropCmd: cmdStr,
		})
	}

	sort.SliceStable(upgrades, func(i, j int) bool {
		if upgrades[i].NetUpgradePPG != upgrades[j].NetUpgradePPG {
			return upgrades[i].NetUpgradePPG > upgrades[j].NetUpgradePPG
		}
		if upgrades[i].CandidatePPG != upgrades[j].CandidatePPG {
			return upgrades[i].CandidatePPG > upgrades[j].CandidatePPG
		}
		return upgrades[i].PercentOwned > upgrades[j].PercentOwned
	})

	if limit > 0 && len(upgrades) > limit {
		upgrades = upgrades[:limit]
	}
	for i := range upgrades {
		upgrades[i].Rank = i + 1
	}
	return upgrades
}

// MatchupSlotComparison compares a single starting lineup slot between two head-to-head opponents.
type MatchupSlotComparison struct {
	Slot             string  `json:"slot"`
	TeamPlayerName   string  `json:"team_player_name"`
	TeamPlayerStatus string  `json:"team_player_status,omitempty"`
	TeamProjectedPts float64 `json:"team_projected_points"`
	TeamActualPts    float64 `json:"team_actual_points"`
	OppPlayerName    string  `json:"opponent_player_name"`
	OppPlayerStatus  string  `json:"opponent_player_status,omitempty"`
	OppProjectedPts  float64 `json:"opponent_projected_points"`
	OppActualPts     float64 `json:"opponent_actual_points"`
	ProjectedEdge    float64 `json:"projected_edge"`
}

// MatchupRecord holds normalized matchup data for a single head-to-head pairing.
type MatchupRecord struct {
	Week              int            `json:"week"`
	Status            string         `json:"status"`
	TeamKey           string         `json:"team_key"`
	TeamName          string         `json:"team_name"`
	TeamPoints        float64        `json:"team_points"`
	TeamProjected     float64        `json:"team_projected_points"`
	TeamWinProb       float64        `json:"team_win_probability"`
	TeamStarters      []PlayerRecord `json:"team_starters,omitempty"`
	OpponentKey       string         `json:"opponent_key"`
	OpponentName      string         `json:"opponent_name"`
	OpponentPoints    float64        `json:"opponent_points"`
	OpponentProjected float64        `json:"opponent_projected_points"`
	OpponentWinProb   float64        `json:"opponent_win_probability"`
	OpponentStarters  []PlayerRecord `json:"opponent_starters,omitempty"`
}

// MatchupRadarView is the structured response for the matchup-radar command.
type MatchupRadarView struct {
	LeagueKey         string                  `json:"league_key"`
	Week              int                     `json:"week"`
	Status            string                  `json:"status"`
	TeamKey           string                  `json:"team_key"`
	TeamName          string                  `json:"team_name"`
	TeamPoints        float64                 `json:"team_points"`
	TeamProjected     float64                 `json:"team_projected_points"`
	TeamWinProb       float64                 `json:"team_win_probability"`
	OpponentKey       string                  `json:"opponent_key"`
	OpponentName      string                  `json:"opponent_name"`
	OpponentPoints    float64                 `json:"opponent_points"`
	OpponentProjected float64                 `json:"opponent_projected_points"`
	ProjectedMargin   float64                 `json:"projected_margin"`
	CurrentMargin     float64                 `json:"current_margin"`
	InjuryAlerts      []string                `json:"injury_alerts"`
	SlotComparisons   []MatchupSlotComparison `json:"slot_comparisons"`
}

// BuildMatchupRadar builds a slot-by-slot comparison view for a head-to-head matchup.
func BuildMatchupRadar(leagueKey string, m MatchupRecord) MatchupRadarView {
	view := MatchupRadarView{
		LeagueKey:         leagueKey,
		Week:              m.Week,
		Status:            m.Status,
		TeamKey:           m.TeamKey,
		TeamName:          m.TeamName,
		TeamPoints:        round2(m.TeamPoints),
		TeamProjected:     round2(m.TeamProjected),
		TeamWinProb:       round2(m.TeamWinProb),
		OpponentKey:       m.OpponentKey,
		OpponentName:      m.OpponentName,
		OpponentPoints:    round2(m.OpponentPoints),
		OpponentProjected: round2(m.OpponentProjected),
		ProjectedMargin:   round2(m.TeamProjected - m.OpponentProjected),
		CurrentMargin:     round2(m.TeamPoints - m.OpponentPoints),
		InjuryAlerts:      make([]string, 0),
		SlotComparisons:   make([]MatchupSlotComparison, 0),
	}

	maxLen := len(m.TeamStarters)
	if len(m.OpponentStarters) > maxLen {
		maxLen = len(m.OpponentStarters)
	}
	for i := 0; i < maxLen; i++ {
		var tp, op PlayerRecord
		if i < len(m.TeamStarters) {
			tp = m.TeamStarters[i]
		}
		if i < len(m.OpponentStarters) {
			op = m.OpponentStarters[i]
		}
		slot := tp.SelectedPosition
		if slot == "" {
			slot = op.SelectedPosition
		}
		if slot == "" {
			slot = tp.DisplayPosition
		}
		if reason := starterInactiveReason(tp, m.Week); reason != "" {
			view.InjuryAlerts = append(view.InjuryAlerts, fmt.Sprintf("%s (%s): %s", tp.FullName, slot, reason))
		}
		tProj := tp.ProjectedPoints
		if tProj == 0 {
			tProj = tp.PointsPerGame
		}
		oProj := op.ProjectedPoints
		if oProj == 0 {
			oProj = op.PointsPerGame
		}
		view.SlotComparisons = append(view.SlotComparisons, MatchupSlotComparison{
			Slot:             slot,
			TeamPlayerName:   tp.FullName,
			TeamPlayerStatus: tp.Status,
			TeamProjectedPts: round2(tProj),
			TeamActualPts:    round2(tp.PointsTotal),
			OppPlayerName:    op.FullName,
			OppPlayerStatus:  op.Status,
			OppProjectedPts:  round2(oProj),
			OppActualPts:     round2(op.PointsTotal),
			ProjectedEdge:    round2(tProj - oProj),
		})
	}
	return view
}

// TeamRecord holds normalized team standings and budget metadata.
type TeamRecord struct {
	TeamKey        string  `json:"team_key"`
	TeamID         string  `json:"team_id,omitempty"`
	Name           string  `json:"name"`
	ManagerName    string  `json:"manager_name,omitempty"`
	Rank           int     `json:"rank,omitempty"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	Ties           int     `json:"ties"`
	PointsFor      float64 `json:"points_for"`
	PointsAgainst  float64 `json:"points_against"`
	FaabBalance    int     `json:"faab_balance"`
	HasFaab        bool    `json:"-"`
	WaiverPriority int     `json:"waiver_priority,omitempty"`
	NumberOfMoves  int     `json:"number_of_moves,omitempty"`
}

// PowerRankingRow represents a single team's composite power ranking and all-play metrics.
type PowerRankingRow struct {
	PowerRank           int     `json:"power_rank"`
	StandingRank        int     `json:"standing_rank"`
	TeamKey             string  `json:"team_key"`
	TeamName            string  `json:"team_name"`
	ActualRecord        string  `json:"actual_record"`
	AllPlayWins         int     `json:"all_play_wins"`
	AllPlayLosses       int     `json:"all_play_losses"`
	AllPlayTies         int     `json:"all_play_ties"`
	AllPlayWinPct       float64 `json:"all_play_win_pct"`
	ExpectedWins        float64 `json:"expected_all_play_wins"`
	ScheduleLuckDelta   float64 `json:"schedule_luck_delta"`
	PointsFor           float64 `json:"points_for"`
	PointsAgainst       float64 `json:"points_against"`
	WeeklyScoringStdDev float64 `json:"weekly_scoring_stddev"`
	CompositePowerScore float64 `json:"composite_power_score"`
}

// ComputePowerRankings computes true weekly all-play W-L-T records, schedule luck delta, and composite power score.
func ComputePowerRankings(teams []TeamRecord, weeklyScores map[int]map[string]float64) []PowerRankingRow {
	rows := make([]PowerRankingRow, 0, len(teams))
	allPlayW := make(map[string]int, len(teams))
	allPlayL := make(map[string]int, len(teams))
	allPlayT := make(map[string]int, len(teams))
	teamWeeklyPts := make(map[string][]float64, len(teams))

	weeks := make([]int, 0, len(weeklyScores))
	for w := range weeklyScores {
		weeks = append(weeks, w)
	}
	sort.Ints(weeks)

	for _, w := range weeks {
		scores := weeklyScores[w]
		keys := make([]string, 0, len(scores))
		for k := range scores {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k1 := range keys {
			s1 := scores[k1]
			teamWeeklyPts[k1] = append(teamWeeklyPts[k1], s1)
			for j := i + 1; j < len(keys); j++ {
				k2 := keys[j]
				s2 := scores[k2]
				switch {
				case s1 > s2:
					allPlayW[k1]++
					allPlayL[k2]++
				case s2 > s1:
					allPlayW[k2]++
					allPlayL[k1]++
				default:
					allPlayT[k1]++
					allPlayT[k2]++
				}
			}
		}
	}

	// If no weekly scoreboard breakdown is present, derive synthetic all-play ranking from PointsFor across teams.
	if len(weeks) == 0 && len(teams) > 1 {
		for i := 0; i < len(teams); i++ {
			for j := i + 1; j < len(teams); j++ {
				t1, t2 := teams[i], teams[j]
				switch {
				case t1.PointsFor > t2.PointsFor:
					allPlayW[t1.TeamKey]++
					allPlayL[t2.TeamKey]++
				case t2.PointsFor > t1.PointsFor:
					allPlayW[t2.TeamKey]++
					allPlayL[t1.TeamKey]++
				default:
					allPlayT[t1.TeamKey]++
					allPlayT[t2.TeamKey]++
				}
			}
		}
	}

	var maxPF float64
	for _, t := range teams {
		if t.PointsFor > maxPF {
			maxPF = t.PointsFor
		}
	}
	if maxPF <= 0 {
		maxPF = 1
	}

	for idx, t := range teams {
		apW := allPlayW[t.TeamKey]
		apL := allPlayL[t.TeamKey]
		apT := allPlayT[t.TeamKey]
		apTotal := apW + apL + apT
		apPct := 0.0
		if apTotal > 0 {
			apPct = (float64(apW) + 0.5*float64(apT)) / float64(apTotal)
		}
		actualGames := t.Wins + t.Losses + t.Ties
		expectedWins := apPct * float64(actualGames)
		actualWinEquiv := float64(t.Wins) + 0.5*float64(t.Ties)
		luckDelta := actualWinEquiv - expectedWins
		stdDev := stddev(teamWeeklyPts[t.TeamKey])

		pfNorm := (t.PointsFor / maxPF) * 100.0
		actualPct := 0.0
		if actualGames > 0 {
			actualPct = (actualWinEquiv / float64(actualGames)) * 100.0
		}
		composite := 0.50*(apPct*100.0) + 0.35*pfNorm + 0.15*actualPct

		stRank := t.Rank
		if stRank <= 0 {
			stRank = idx + 1
		}
		rows = append(rows, PowerRankingRow{
			StandingRank:        stRank,
			TeamKey:             t.TeamKey,
			TeamName:            t.Name,
			ActualRecord:        fmt.Sprintf("%d-%d-%d", t.Wins, t.Losses, t.Ties),
			AllPlayWins:         apW,
			AllPlayLosses:       apL,
			AllPlayTies:         apT,
			AllPlayWinPct:       round3(apPct),
			ExpectedWins:        round2(expectedWins),
			ScheduleLuckDelta:   round2(luckDelta),
			PointsFor:           round2(t.PointsFor),
			PointsAgainst:       round2(t.PointsAgainst),
			WeeklyScoringStdDev: round2(stdDev),
			CompositePowerScore: round2(composite),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CompositePowerScore != rows[j].CompositePowerScore {
			return rows[i].CompositePowerScore > rows[j].CompositePowerScore
		}
		return rows[i].PointsFor > rows[j].PointsFor
	})
	for i := range rows {
		rows[i].PowerRank = i + 1
	}
	return rows
}

// TransactionRecord represents a normalized league transaction with FAAB and player details.
type TransactionRecord struct {
	TransactionKey string `json:"transaction_key"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	Timestamp      int64  `json:"timestamp,omitempty"`
	FaabBid        int    `json:"faab_bid,omitempty"`
	TeamKey        string `json:"team_key,omitempty"`
	AddPlayerKey   string `json:"add_player_key,omitempty"`
	AddPlayerName  string `json:"add_player_name,omitempty"`
	AddPlayerPos   string `json:"add_player_position,omitempty"`
	DropPlayerKey  string `json:"drop_player_key,omitempty"`
	DropPlayerName string `json:"drop_player_name,omitempty"`
}

// FAABAuditRow summarizes a team's FAAB budget, historical spend, and blocking bid threshold.
type FAABAuditRow struct {
	Rank                int            `json:"rank"`
	TeamKey             string         `json:"team_key"`
	TeamName            string         `json:"team_name"`
	RemainingFAAB       int            `json:"remaining_faab"`
	TotalSpentFAAB      int            `json:"total_spent_faab"`
	BudgetSharePct      float64        `json:"budget_share_pct"`
	PaidAcquisitions    int            `json:"paid_acquisitions"`
	FreeAcquisitions    int            `json:"free_acquisitions"`
	AvgWinningBid       float64        `json:"avg_winning_bid"`
	LargestBid          int            `json:"largest_bid"`
	LargestBidPlayer    string         `json:"largest_bid_player,omitempty"`
	BlockingBidOverNext int            `json:"blocking_bid_over_next_rival"`
	SpendByPosition     map[string]int `json:"spend_by_position,omitempty"`
}

// ComputeFAABAudit joins team FAAB balances with league transactions to produce a budget intelligence table.
func ComputeFAABAudit(defaultBudget int, teams []TeamRecord, transactions []TransactionRecord) []FAABAuditRow {
	if defaultBudget <= 0 {
		defaultBudget = 100
	}
	byTeam := make(map[string]*FAABAuditRow, len(teams))
	order := make([]string, 0, len(teams))

	for _, t := range teams {
		rem := t.FaabBalance
		if !t.HasFaab && rem == 0 {
			rem = defaultBudget
		}
		byTeam[t.TeamKey] = &FAABAuditRow{
			TeamKey:         t.TeamKey,
			TeamName:        t.Name,
			RemainingFAAB:   rem,
			SpendByPosition: map[string]int{},
		}
		order = append(order, t.TeamKey)
	}

	for _, tx := range transactions {
		if tx.TeamKey == "" || (tx.Status != "" && !strings.EqualFold(tx.Status, "successful")) {
			continue
		}
		row, exists := byTeam[tx.TeamKey]
		if !exists {
			row = &FAABAuditRow{
				TeamKey:         tx.TeamKey,
				TeamName:        tx.TeamKey,
				RemainingFAAB:   defaultBudget,
				SpendByPosition: map[string]int{},
			}
			byTeam[tx.TeamKey] = row
			order = append(order, tx.TeamKey)
		}
		if tx.FaabBid > 0 {
			row.TotalSpentFAAB += tx.FaabBid
			row.PaidAcquisitions++
			pos := strings.ToUpper(strings.TrimSpace(tx.AddPlayerPos))
			if pos == "" {
				pos = "FLEX"
			}
			row.SpendByPosition[pos] += tx.FaabBid
			if tx.FaabBid > row.LargestBid {
				row.LargestBid = tx.FaabBid
				row.LargestBidPlayer = tx.AddPlayerName
				if row.LargestBidPlayer == "" {
					row.LargestBidPlayer = tx.AddPlayerKey
				}
			}
		} else if tx.AddPlayerKey != "" {
			row.FreeAcquisitions++
		}
	}

	totalRemainingPool := 0
	rows := make([]FAABAuditRow, 0, len(order))
	for _, tk := range order {
		r := byTeam[tk]
		if r.PaidAcquisitions > 0 {
			r.AvgWinningBid = round2(float64(r.TotalSpentFAAB) / float64(r.PaidAcquisitions))
		}
		totalRemainingPool += r.RemainingFAAB
		rows = append(rows, *r)
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].RemainingFAAB != rows[j].RemainingFAAB {
			return rows[i].RemainingFAAB > rows[j].RemainingFAAB
		}
		return rows[i].TotalSpentFAAB < rows[j].TotalSpentFAAB
	})

	for i := range rows {
		rows[i].Rank = i + 1
		if totalRemainingPool > 0 {
			rows[i].BudgetSharePct = round2((float64(rows[i].RemainingFAAB) / float64(totalRemainingPool)) * 100.0)
		}
		if i+1 < len(rows) {
			nextRem := rows[i+1].RemainingFAAB
			if rows[i].RemainingFAAB > nextRem {
				rows[i].BlockingBidOverNext = nextRem + 1
			} else {
				rows[i].BlockingBidOverNext = rows[i].RemainingFAAB
			}
		} else {
			rows[i].BlockingBidOverNext = 1
		}
	}
	return rows
}

// DraftPickRecord represents a normalized league draft pick.
type DraftPickRecord struct {
	Pick       int    `json:"pick"`
	Round      int    `json:"round"`
	Cost       int    `json:"cost,omitempty"`
	TeamKey    string `json:"team_key"`
	PlayerKey  string `json:"player_key"`
	PlayerName string `json:"player_name,omitempty"`
	Position   string `json:"position,omitempty"`
}

// DraftROIRow evaluates the return on investment of a single draft pick.
type DraftROIRow struct {
	Rank                int     `json:"rank"`
	Pick                int     `json:"pick"`
	Round               int     `json:"round"`
	Cost                int     `json:"cost,omitempty"`
	DraftedByTeamKey    string  `json:"drafted_by_team_key"`
	PlayerKey           string  `json:"player_key"`
	PlayerName          string  `json:"player_name"`
	Position            string  `json:"position,omitempty"`
	SeasonPoints        float64 `json:"season_points"`
	RoundExpectedPoints float64 `json:"round_expected_points"`
	SurplusPoints       float64 `json:"surplus_points"`
	PointsPerDollar     float64 `json:"points_per_dollar,omitempty"`
	RetainedByDrafter   bool    `json:"retained_by_drafter"`
	Verdict             string  `json:"verdict"` // steal, value, par, bust
}

// ComputeDraftROI joins draft results with player season points and current team rosters.
func ComputeDraftROI(picks []DraftPickRecord, players map[string]PlayerRecord, currentOwnerByPlayer map[string]string) []DraftROIRow {
	roundTotals := map[int]float64{}
	roundCounts := map[int]int{}

	for _, pk := range picks {
		p := players[pk.PlayerKey]
		roundTotals[pk.Round] += p.PointsTotal
		roundCounts[pk.Round]++
	}

	rows := make([]DraftROIRow, 0, len(picks))
	for _, pk := range picks {
		p := players[pk.PlayerKey]
		name := pk.PlayerName
		if name == "" {
			name = p.FullName
		}
		if name == "" {
			name = pk.PlayerKey
		}
		pos := pk.Position
		if pos == "" {
			pos = p.DisplayPosition
		}
		expected := 0.0
		if cnt := roundCounts[pk.Round]; cnt > 0 {
			expected = roundTotals[pk.Round] / float64(cnt)
		}
		surplus := p.PointsTotal - expected
		ppd := 0.0
		if pk.Cost > 0 {
			ppd = round2(p.PointsTotal / float64(pk.Cost))
		}
		owner, hasOwner := currentOwnerByPlayer[pk.PlayerKey]
		retained := hasOwner && owner == pk.TeamKey
		if !hasOwner && p.OwnerTeamKey != "" {
			retained = p.OwnerTeamKey == pk.TeamKey
		}
		verdict := "par"
		switch {
		case surplus >= 30.0:
			verdict = "steal"
		case surplus >= 10.0:
			verdict = "value"
		case surplus <= -25.0:
			verdict = "bust"
		}
		rows = append(rows, DraftROIRow{
			Pick:                pk.Pick,
			Round:               pk.Round,
			Cost:                pk.Cost,
			DraftedByTeamKey:    pk.TeamKey,
			PlayerKey:           pk.PlayerKey,
			PlayerName:          name,
			Position:            pos,
			SeasonPoints:        round2(p.PointsTotal),
			RoundExpectedPoints: round2(expected),
			SurplusPoints:       round2(surplus),
			PointsPerDollar:     ppd,
			RetainedByDrafter:   retained,
			Verdict:             verdict,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SurplusPoints != rows[j].SurplusPoints {
			return rows[i].SurplusPoints > rows[j].SurplusPoints
		}
		return rows[i].SeasonPoints > rows[j].SeasonPoints
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	return rows
}

// --- Flexible JSON Extractors (support both Yahoo nested fantasy_content and flat JSON records) ---

// ExtractPlayers extracts PlayerRecord items from any Yahoo JSON or flat JSON payload.
func ExtractPlayers(data []byte) []PlayerRecord {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var out []PlayerRecord
	seen := map[string]int{}
	walkJSON(root, func(m map[string]any) {
		pk := asString(m["player_key"])
		if pk == "" {
			return
		}
		rec := parsePlayerMap(m)
		if idx, ok := seen[pk]; ok {
			out[idx] = mergePlayerRecord(out[idx], rec)
		} else {
			seen[pk] = len(out)
			out = append(out, rec)
		}
	})
	return out
}

func parsePlayerMap(m map[string]any) PlayerRecord {
	rec := PlayerRecord{
		PlayerKey:         asString(m["player_key"]),
		PlayerID:          asString(m["player_id"]),
		FullName:          asString(m["full_name"]),
		EditorialTeamAbbr: asString(m["editorial_team_abbr"]),
		DisplayPosition:   asString(m["display_position"]),
		SelectedPosition:  asString(m["selected_position"]),
		Status:            asString(m["status"]),
		StatusFull:        asString(m["status_full"]),
		PercentOwned:      asFloat(m["percent_owned"]),
		PointsTotal:       asFloat(m["points_total"]),
		PointsPerGame:     asFloat(m["points_per_game"]),
		ProjectedPoints:   asFloat(m["projected_points"]),
		ByeWeek:           asInt(m["bye_week"]),
		OwnershipStatus:   asString(m["ownership_status"]),
		OwnerTeamKey:      asString(m["owner_team_key"]),
	}
	if rec.FullName == "" {
		if nameMap, ok := m["name"].(map[string]any); ok {
			rec.FullName = asString(nameMap["full"])
		} else if nameStr := asString(m["name"]); nameStr != "" {
			rec.FullName = nameStr
		}
	}
	if selMap, ok := m["selected_position"].(map[string]any); ok {
		rec.SelectedPosition = asString(selMap["position"])
	}
	if bwMap, ok := m["bye_weeks"].(map[string]any); ok && rec.ByeWeek == 0 {
		rec.ByeWeek = asInt(bwMap["week"])
	}
	if poMap, ok := m["percent_owned"].(map[string]any); ok && rec.PercentOwned == 0 {
		rec.PercentOwned = asFloat(poMap["value"])
	}
	if ptsMap, ok := m["player_points"].(map[string]any); ok && rec.PointsTotal == 0 {
		rec.PointsTotal = asFloat(ptsMap["total"])
	}
	if projMap, ok := m["player_projected_points"].(map[string]any); ok && rec.ProjectedPoints == 0 {
		rec.ProjectedPoints = asFloat(projMap["total"])
	}
	if ownMap, ok := m["ownership"].(map[string]any); ok {
		if rec.OwnershipStatus == "" {
			ot := asString(ownMap["ownership_type"])
			switch ot {
			case "freeagents":
				rec.OwnershipStatus = "FA"
			case "waivers":
				rec.OwnershipStatus = "W"
			case "team":
				rec.OwnershipStatus = "T"
			default:
				rec.OwnershipStatus = ot
			}
		}
		if rec.OwnerTeamKey == "" {
			rec.OwnerTeamKey = asString(ownMap["owner_team_key"])
		}
	}
	if eligAny, ok := m["eligible_positions"]; ok {
		switch v := eligAny.(type) {
		case []any:
			for _, item := range v {
				switch sub := item.(type) {
				case string:
					rec.EligiblePositions = appendUnique(rec.EligiblePositions, sub)
				case map[string]any:
					rec.EligiblePositions = appendUnique(rec.EligiblePositions, asString(sub["position"]))
				}
			}
		case map[string]any:
			rec.EligiblePositions = appendUnique(rec.EligiblePositions, asString(v["position"]))
		case string:
			for _, p := range strings.Split(v, ",") {
				rec.EligiblePositions = appendUnique(rec.EligiblePositions, strings.TrimSpace(p))
			}
		}
	}
	if len(rec.EligiblePositions) == 0 && rec.DisplayPosition != "" {
		for _, p := range strings.Split(rec.DisplayPosition, ",") {
			rec.EligiblePositions = appendUnique(rec.EligiblePositions, strings.TrimSpace(p))
		}
	}
	return rec
}

func mergePlayerRecord(a, b PlayerRecord) PlayerRecord {
	if b.PlayerID != "" {
		a.PlayerID = b.PlayerID
	}
	if b.FullName != "" {
		a.FullName = b.FullName
	}
	if b.EditorialTeamAbbr != "" {
		a.EditorialTeamAbbr = b.EditorialTeamAbbr
	}
	if b.DisplayPosition != "" {
		a.DisplayPosition = b.DisplayPosition
	}
	if b.SelectedPosition != "" {
		a.SelectedPosition = b.SelectedPosition
	}
	if b.Status != "" {
		a.Status = b.Status
	}
	if b.StatusFull != "" {
		a.StatusFull = b.StatusFull
	}
	for _, ep := range b.EligiblePositions {
		a.EligiblePositions = appendUnique(a.EligiblePositions, ep)
	}
	if b.PercentOwned > 0 {
		a.PercentOwned = b.PercentOwned
	}
	if b.PointsTotal > 0 {
		a.PointsTotal = b.PointsTotal
	}
	if b.PointsPerGame > 0 {
		a.PointsPerGame = b.PointsPerGame
	}
	if b.ProjectedPoints > 0 {
		a.ProjectedPoints = b.ProjectedPoints
	}
	if b.ByeWeek > 0 {
		a.ByeWeek = b.ByeWeek
	}
	if b.OwnershipStatus != "" {
		a.OwnershipStatus = b.OwnershipStatus
	}
	if b.OwnerTeamKey != "" {
		a.OwnerTeamKey = b.OwnerTeamKey
	}
	return a
}

// ExtractTeams extracts TeamRecord items from any Yahoo JSON or flat JSON payload.
func ExtractTeams(data []byte) []TeamRecord {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var out []TeamRecord
	seen := map[string]int{}
	walkJSON(root, func(m map[string]any) {
		tk := asString(m["team_key"])
		if tk == "" || !strings.Contains(tk, ".t.") {
			return
		}
		// Skip transaction data maps that only reference source_team_key/destination_team_key
		if _, isPlayer := m["player_key"]; isPlayer {
			return
		}
		if _, isPick := m["pick"]; isPick {
			return
		}
		if _, isTx := m["transaction_key"]; isTx {
			return
		}
		rec := parseTeamMap(m)
		if idx, ok := seen[tk]; ok {
			out[idx] = mergeTeamRecord(out[idx], rec)
		} else {
			seen[tk] = len(out)
			out = append(out, rec)
		}
	})
	return out
}

func parseTeamMap(m map[string]any) TeamRecord {
	_, hasFaab := m["faab_balance"]
	rec := TeamRecord{
		TeamKey:        asString(m["team_key"]),
		TeamID:         asString(m["team_id"]),
		Name:           asString(m["name"]),
		ManagerName:    asString(m["manager_nickname"]),
		Rank:           asInt(m["rank"]),
		Wins:           asInt(m["wins"]),
		Losses:         asInt(m["losses"]),
		Ties:           asInt(m["ties"]),
		PointsFor:      asFloat(m["points_for"]),
		PointsAgainst:  asFloat(m["points_against"]),
		FaabBalance:    asInt(m["faab_balance"]),
		HasFaab:        hasFaab,
		WaiverPriority: asInt(m["waiver_priority"]),
		NumberOfMoves:  asInt(m["number_of_moves"]),
	}
	if st, ok := m["team_standings"].(map[string]any); ok {
		if rec.Rank == 0 {
			rec.Rank = asInt(st["rank"])
		}
		if rec.PointsFor == 0 {
			rec.PointsFor = asFloat(st["points_for"])
		}
		if rec.PointsAgainst == 0 {
			rec.PointsAgainst = asFloat(st["points_against"])
		}
		if ot, ok := st["outcome_totals"].(map[string]any); ok {
			rec.Wins = asInt(ot["wins"])
			rec.Losses = asInt(ot["losses"])
			rec.Ties = asInt(ot["ties"])
		}
	}
	if tp, ok := m["team_points"].(map[string]any); ok && rec.PointsFor == 0 {
		rec.PointsFor = asFloat(tp["total"])
	}
	return rec
}

func mergeTeamRecord(a, b TeamRecord) TeamRecord {
	if b.TeamID != "" {
		a.TeamID = b.TeamID
	}
	if b.Name != "" {
		a.Name = b.Name
	}
	if b.ManagerName != "" {
		a.ManagerName = b.ManagerName
	}
	if b.Rank > 0 {
		a.Rank = b.Rank
	}
	if b.Wins > 0 || b.Losses > 0 || b.Ties > 0 {
		a.Wins = b.Wins
		a.Losses = b.Losses
		a.Ties = b.Ties
	}
	if b.PointsFor > 0 {
		a.PointsFor = b.PointsFor
	}
	if b.PointsAgainst > 0 {
		a.PointsAgainst = b.PointsAgainst
	}
	if b.HasFaab || b.FaabBalance > 0 {
		a.FaabBalance = b.FaabBalance
		a.HasFaab = true
	}
	if b.WaiverPriority > 0 {
		a.WaiverPriority = b.WaiverPriority
	}
	if b.NumberOfMoves > 0 {
		a.NumberOfMoves = b.NumberOfMoves
	}
	return a
}

// ExtractTransactions extracts TransactionRecord items from Yahoo JSON or flat JSON.
func ExtractTransactions(data []byte) []TransactionRecord {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var out []TransactionRecord
	seen := map[string]int{}
	walkJSON(root, func(m map[string]any) {
		tk := asString(m["transaction_key"])
		if tk == "" {
			return
		}
		rec := TransactionRecord{
			TransactionKey: tk,
			Type:           asString(m["type"]),
			Status:         asString(m["status"]),
			Timestamp:      int64(asInt(m["timestamp"])),
			FaabBid:        asInt(m["faab_bid"]),
			TeamKey:        asString(m["team_key"]),
			AddPlayerKey:   asString(m["add_player_key"]),
			AddPlayerName:  asString(m["add_player_name"]),
			AddPlayerPos:   asString(m["add_player_position"]),
			DropPlayerKey:  asString(m["drop_player_key"]),
			DropPlayerName: asString(m["drop_player_name"]),
		}
		if rec.TeamKey == "" {
			rec.TeamKey = asString(m["trader_team_key"])
		}
		// Inspect nested players if present
		walkJSON(m["players"], func(pm map[string]any) {
			pk := asString(pm["player_key"])
			if pk == "" {
				return
			}
			pRec := parsePlayerMap(pm)
			td, _ := pm["transaction_data"].(map[string]any)
			if td == nil {
				if tdArr, ok := pm["transaction_data"].([]any); ok && len(tdArr) > 0 {
					td, _ = tdArr[0].(map[string]any)
				}
			}
			if td != nil {
				tType := strings.ToLower(asString(td["type"]))
				if tType == "add" {
					rec.AddPlayerKey = pk
					rec.AddPlayerName = pRec.FullName
					rec.AddPlayerPos = pRec.DisplayPosition
					if dest := asString(td["destination_team_key"]); dest != "" {
						rec.TeamKey = dest
					}
				} else if tType == "drop" {
					rec.DropPlayerKey = pk
					rec.DropPlayerName = pRec.FullName
					if src := asString(td["source_team_key"]); src != "" && rec.TeamKey == "" {
						rec.TeamKey = src
					}
				}
			}
		})
		if idx, ok := seen[tk]; ok {
			if rec.FaabBid > 0 {
				out[idx].FaabBid = rec.FaabBid
			}
			if rec.TeamKey != "" {
				out[idx].TeamKey = rec.TeamKey
			}
			if rec.AddPlayerKey != "" {
				out[idx].AddPlayerKey = rec.AddPlayerKey
				out[idx].AddPlayerName = rec.AddPlayerName
				out[idx].AddPlayerPos = rec.AddPlayerPos
			}
		} else {
			seen[tk] = len(out)
			out = append(out, rec)
		}
	})
	return out
}

// ExtractDraftPicks extracts DraftPickRecord items from Yahoo JSON or flat JSON.
func ExtractDraftPicks(data []byte) []DraftPickRecord {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var out []DraftPickRecord
	seen := map[string]bool{}
	walkJSON(root, func(m map[string]any) {
		pick := asInt(m["pick"])
		round := asInt(m["round"])
		pk := asString(m["player_key"])
		tk := asString(m["team_key"])
		if pick <= 0 || round <= 0 || pk == "" {
			return
		}
		key := fmt.Sprintf("%d:%d:%s", round, pick, pk)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, DraftPickRecord{
			Pick:       pick,
			Round:      round,
			Cost:       asInt(m["cost"]),
			TeamKey:    tk,
			PlayerKey:  pk,
			PlayerName: asString(m["player_name"]),
			Position:   asString(m["position"]),
		})
	})
	return out
}

// ExtractMatchups extracts MatchupRecord items from Yahoo JSON or flat JSON.
func ExtractMatchups(data []byte) []MatchupRecord {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	var out []MatchupRecord
	seen := map[string]bool{}
	walkJSON(root, func(m map[string]any) {
		tk := asString(m["team_key"])
		okKey := asString(m["opponent_key"])
		if tk != "" && okKey != "" {
			week := asInt(m["week"])
			key := fmt.Sprintf("%d:%s:%s", week, tk, okKey)
			if seen[key] {
				return
			}
			seen[key] = true
			rec := MatchupRecord{
				Week:              week,
				Status:            asString(m["status"]),
				TeamKey:           tk,
				TeamName:          asString(m["team_name"]),
				TeamPoints:        asFloat(m["team_points"]),
				TeamProjected:     asFloat(m["team_projected_points"]),
				TeamWinProb:       asFloat(m["team_win_probability"]),
				OpponentKey:       okKey,
				OpponentName:      asString(m["opponent_name"]),
				OpponentPoints:    asFloat(m["opponent_points"]),
				OpponentProjected: asFloat(m["opponent_projected_points"]),
				OpponentWinProb:   asFloat(m["opponent_win_probability"]),
			}
			if stRaw, err := json.Marshal(m["team_starters"]); err == nil {
				rec.TeamStarters = ExtractPlayers(stRaw)
			}
			if opRaw, err := json.Marshal(m["opponent_starters"]); err == nil {
				rec.OpponentStarters = ExtractPlayers(opRaw)
			}
			out = append(out, rec)
			return
		}
		// Also support nested Yahoo matchup objects containing a "teams" sub-collection
		if teamsVal, hasTeams := m["teams"]; hasTeams && m["week"] != nil {
			teamsRaw, err := json.Marshal(teamsVal)
			if err != nil {
				return
			}
			teams := ExtractTeams(teamsRaw)
			if len(teams) >= 2 {
				week := asInt(m["week"])
				key := fmt.Sprintf("%d:%s:%s", week, teams[0].TeamKey, teams[1].TeamKey)
				if seen[key] {
					return
				}
				seen[key] = true
				out = append(out, MatchupRecord{
					Week:           week,
					Status:         asString(m["status"]),
					TeamKey:        teams[0].TeamKey,
					TeamName:       teams[0].Name,
					TeamPoints:     teams[0].PointsFor,
					OpponentKey:    teams[1].TeamKey,
					OpponentName:   teams[1].Name,
					OpponentPoints: teams[1].PointsFor,
				})
			}
		}
	})
	return out
}

// walkJSON traverses arbitrary decoded JSON values, collapsing Yahoo's array-of-single-key-maps convention into merged maps before invoking fn.
func walkJSON(v any, fn func(map[string]any)) {
	switch val := v.(type) {
	case map[string]any:
		fn(val)
		for _, child := range val {
			walkJSON(child, fn)
		}
	case []any:
		// Yahoo Fantasy JSON frequently encodes an object as an array of maps/slices (e.g. [ [{team_key:...}, {name:...}], {team_standings:...} ]).
		merged := map[string]any{}
		for _, elem := range val {
			switch sub := elem.(type) {
			case map[string]any:
				for k, v2 := range sub {
					merged[k] = v2
				}
			case []any:
				for _, inner := range sub {
					if im, ok := inner.(map[string]any); ok {
						for k, v2 := range im {
							merged[k] = v2
						}
					}
				}
			}
		}
		if len(merged) > 0 {
			fn(merged)
		}
		for _, elem := range val {
			walkJSON(elem, fn)
		}
	}
}

func appendUnique(list []string, val string) []string {
	val = strings.ToUpper(strings.TrimSpace(val))
	if val == "" {
		return list
	}
	for _, item := range list {
		if strings.EqualFold(item, val) {
			return list
		}
	}
	return append(list, val)
}

func asString(v any) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case float64:
		if val == math.Trunc(val) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	default:
		return ""
	}
}

func asInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case json.Number:
		i, _ := val.Int64()
		return int(i)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(val))
		return i
	default:
		return 0
	}
}

func asFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	default:
		return 0
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func stddev(vals []float64) float64 {
	if len(vals) <= 1 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var sqDiff float64
	for _, v := range vals {
		d := v - mean
		sqDiff += d * d
	}
	return math.Sqrt(sqDiff / float64(len(vals)))
}
