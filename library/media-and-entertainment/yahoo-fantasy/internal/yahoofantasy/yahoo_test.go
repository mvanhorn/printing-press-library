package yahoofantasy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildRosterUpdateXML(t *testing.T) {
	tests := []struct {
		name    string
		req     RosterUpdateRequest
		wantSub []string
		wantErr bool
	}{
		{
			name: "week lineup update",
			req: RosterUpdateRequest{
				CoverageType: "week",
				Week:         9,
				Players: []RosterSlotUpdate{
					{PlayerKey: "449.p.31862", Position: "QB"},
					{PlayerKey: "449.p.30123", Position: "BN"},
				},
			},
			wantSub: []string{
				"<fantasy_content>",
				"<coverage_type>week</coverage_type>",
				"<week>9</week>",
				"<player_key>449.p.31862</player_key>",
				"<position>QB</position>",
				"<player_key>449.p.30123</player_key>",
				"<position>BN</position>",
			},
		},
		{
			name: "date lineup update",
			req: RosterUpdateRequest{
				CoverageType: "date",
				Date:         "2026-10-25",
				Players: []RosterSlotUpdate{
					{PlayerKey: "450.p.5001", Position: "PG"},
				},
			},
			wantSub: []string{
				"<coverage_type>date</coverage_type>",
				"<date>2026-10-25</date>",
				"<position>PG</position>",
			},
		},
		{
			name: "missing players fails",
			req: RosterUpdateRequest{
				CoverageType: "week",
				Week:         9,
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildRosterUpdateXML(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(got)
			for _, sub := range tc.wantSub {
				if !strings.Contains(s, sub) {
					t.Errorf("expected XML to contain %q, got:\n%s", sub, s)
				}
			}
		})
	}
}

func TestBuildTransactionCreateXML(t *testing.T) {
	tests := []struct {
		name    string
		req     TransactionCreateRequest
		wantSub []string
		wantErr bool
	}{
		{
			name: "atomic add/drop with FAAB bid",
			req: TransactionCreateRequest{
				Type:          "add/drop",
				TeamKey:       "449.l.365083.t.1",
				AddPlayerKey:  "449.p.31862",
				DropPlayerKey: "449.p.30123",
				FaabBid:       15,
				HasFaabBid:    true,
			},
			wantSub: []string{
				"<type>add/drop</type>",
				"<faab_bid>15</faab_bid>",
				"<player_key>449.p.31862</player_key>",
				"<destination_team_key>449.l.365083.t.1</destination_team_key>",
				"<player_key>449.p.30123</player_key>",
				"<source_team_key>449.l.365083.t.1</source_team_key>",
			},
		},
		{
			name: "single add without FAAB",
			req: TransactionCreateRequest{
				TeamKey:      "449.l.365083.t.1",
				AddPlayerKey: "449.p.31862",
			},
			wantSub: []string{
				"<type>add</type>",
				"<destination_team_key>449.l.365083.t.1</destination_team_key>",
			},
		},
		{
			name: "missing team key fails",
			req: TransactionCreateRequest{
				AddPlayerKey: "449.p.31862",
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildTransactionCreateXML(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			s := string(got)
			for _, sub := range tc.wantSub {
				if !strings.Contains(s, sub) {
					t.Errorf("expected XML to contain %q, got:\n%s", sub, s)
				}
			}
		})
	}
}

func TestBuildTransactionUpdateXMLAndTranslate(t *testing.T) {
	xmlBytes, err := BuildTransactionUpdateXML(TransactionUpdateRequest{
		TransactionKey: "449.l.365083.w.c.2_31862",
		Type:           "waiver",
		FaabBid:        25,
		HasFaabBid:     true,
	})
	if err != nil {
		t.Fatalf("BuildTransactionUpdateXML failed: %v", err)
	}
	if !strings.Contains(string(xmlBytes), "<faab_bid>25</faab_bid>") {
		t.Fatalf("expected faab_bid 25 in XML: %s", string(xmlBytes))
	}

	body := []byte(`{"coverage_type":"week","week":9,"player_key":"449.p.31862","position":"QB"}`)
	translated, converted, err := TranslateJSONMutationToXML("PUT", "/team/449.l.365083.t.1/roster", body)
	if err != nil || !converted {
		t.Fatalf("TranslateJSONMutationToXML roster failed: converted=%v err=%v", converted, err)
	}
	if !strings.Contains(string(translated), "<position>QB</position>") {
		t.Fatalf("expected <position>QB</position> in translated XML: %s", string(translated))
	}

	txBody := []byte(`{"type":"add/drop","team_key":"449.l.365083.t.1","add_player_key":"449.p.31862","drop_player_key":"449.p.30123","faab_bid":15}`)
	txTranslated, txConverted, err := TranslateJSONMutationToXML("POST", "/league/449.l.365083/transactions", txBody)
	if err != nil || !txConverted {
		t.Fatalf("TranslateJSONMutationToXML transaction failed: converted=%v err=%v", txConverted, err)
	}
	if !strings.Contains(string(txTranslated), "<type>add/drop</type>") {
		t.Fatalf("expected <type>add/drop</type> in translated XML: %s", string(txTranslated))
	}
}

func TestNormalizeYahooResponse(t *testing.T) {
	xmlResp := []byte(`<?xml version="1.0" encoding="UTF-8"?><fantasy_content><transaction><transaction_key>449.l.365083.tr.19</transaction_key><status>successful</status></transaction></fantasy_content>`)
	norm := NormalizeYahooResponse(xmlResp)
	var m map[string]any
	if err := json.Unmarshal(norm, &m); err != nil {
		t.Fatalf("expected valid JSON from NormalizeYahooResponse, got err=%v body=%s", err, string(norm))
	}
	if !strings.Contains(string(norm), "449.l.365083.tr.19") {
		t.Fatalf("expected transaction_key in normalized JSON: %s", string(norm))
	}
}

func TestParseSlotSwapsAndAnalyzeLineup(t *testing.T) {
	swaps, err := ParseSlotSwaps("QB=449.p.31862,BN=449.p.30123")
	if err != nil {
		t.Fatalf("ParseSlotSwaps failed: %v", err)
	}
	if len(swaps) != 2 || swaps[0].Position != "QB" || swaps[0].PlayerKey != "449.p.31862" {
		t.Fatalf("unexpected swaps: %+v", swaps)
	}

	roster := []PlayerRecord{
		{
			PlayerKey:         "449.p.30123",
			FullName:          "Injured Starter",
			DisplayPosition:   "QB",
			SelectedPosition:  "QB",
			Status:            "O",
			EligiblePositions: []string{"QB"},
			PointsPerGame:     18.2,
		},
		{
			PlayerKey:         "449.p.31862",
			FullName:          "Healthy Backup",
			DisplayPosition:   "QB",
			SelectedPosition:  "BN",
			Status:            "",
			EligiblePositions: []string{"QB"},
			PointsPerGame:     19.5,
		},
	}

	plan, err := AnalyzeLineup("449.l.365083.t.1", 9, "", roster, swaps)
	if err != nil {
		t.Fatalf("AnalyzeLineup failed: %v", err)
	}
	if len(plan.Alerts) != 1 || plan.Alerts[0].RecommendedBenchKey != "449.p.31862" {
		t.Fatalf("expected 1 alert recommending 449.p.31862, got %+v", plan.Alerts)
	}
	if !strings.Contains(plan.XMLPayload, "<player_key>449.p.31862</player_key>") {
		t.Fatalf("expected XML payload to include 449.p.31862, got %s", plan.XMLPayload)
	}
}

func TestPlanAddDropAndRankWaiverTargets(t *testing.T) {
	plan, err := PlanAddDrop("449.l.365083", "449.l.365083.t.1", "449.p.31862", "449.p.30123", 15, true, 75)
	if err != nil {
		t.Fatalf("PlanAddDrop failed: %v", err)
	}
	if plan.TransactionType != "add/drop" || plan.RemainingFaabPost != 60 {
		t.Fatalf("unexpected AddDropPlan: %+v", plan)
	}

	if _, err := PlanAddDrop("449.l.365083", "449.l.365083.t.1", "449.p.31862", "", 90, true, 75); err == nil {
		t.Fatalf("expected FAAB over-budget error")
	}

	candidates := []PlayerRecord{
		{PlayerKey: "449.p.40001", FullName: "Breakout RB", DisplayPosition: "RB", EligiblePositions: []string{"RB", "W/R/T"}, PointsPerGame: 16.4, PercentOwned: 42},
		{PlayerKey: "449.p.40002", FullName: "Depth RB", DisplayPosition: "RB", EligiblePositions: []string{"RB"}, PointsPerGame: 9.1, PercentOwned: 12},
	}
	roster := []PlayerRecord{
		{PlayerKey: "449.p.30001", FullName: "Bench RB", DisplayPosition: "RB", SelectedPosition: "BN", EligiblePositions: []string{"RB"}, PointsPerGame: 7.4},
	}
	ranked := RankWaiverTargets("449.l.365083", "449.l.365083.t.1", candidates, roster, "RB", 5)
	if len(ranked) != 2 || ranked[0].AddPlayerKey != "449.p.40001" || ranked[0].NetUpgradePPG != 9.0 {
		t.Fatalf("unexpected waiver rankings: %+v", ranked)
	}
}

func TestPowerRankingsFAABAndDraftROI(t *testing.T) {
	teams := []TeamRecord{
		{TeamKey: "449.l.365083.t.1", Name: "Alpha", Rank: 2, Wins: 5, Losses: 4, PointsFor: 1120.5, PointsAgainst: 1050.0, FaabBalance: 65, HasFaab: true},
		{TeamKey: "449.l.365083.t.2", Name: "Bravo", Rank: 1, Wins: 7, Losses: 2, PointsFor: 1010.0, PointsAgainst: 920.0, FaabBalance: 40, HasFaab: true},
	}
	weekly := map[int]map[string]float64{
		1: {"449.l.365083.t.1": 135.0, "449.l.365083.t.2": 110.0},
		2: {"449.l.365083.t.1": 128.0, "449.l.365083.t.2": 105.0},
	}
	pr := ComputePowerRankings(teams, weekly)
	if len(pr) != 2 || pr[0].TeamKey != "449.l.365083.t.1" || pr[0].AllPlayWins != 2 {
		t.Fatalf("unexpected power rankings: %+v", pr)
	}

	txs := []TransactionRecord{
		{TransactionKey: "tr.1", Status: "successful", TeamKey: "449.l.365083.t.1", FaabBid: 35, AddPlayerKey: "449.p.31862", AddPlayerName: "Puka Nacua", AddPlayerPos: "WR"},
		{TransactionKey: "tr.2", Status: "successful", TeamKey: "449.l.365083.t.2", FaabBid: 60, AddPlayerKey: "449.p.30123", AddPlayerName: "Kyren Williams", AddPlayerPos: "RB"},
	}
	faab := ComputeFAABAudit(100, teams, txs)
	if len(faab) != 2 || faab[0].BlockingBidOverNext != 41 {
		t.Fatalf("unexpected FAAB audit: %+v", faab)
	}

	picks := []DraftPickRecord{
		{Pick: 101, Round: 10, Cost: 5, TeamKey: "449.l.365083.t.1", PlayerKey: "449.p.31862", PlayerName: "Puka Nacua", Position: "WR"},
		{Pick: 102, Round: 10, Cost: 5, TeamKey: "449.l.365083.t.2", PlayerKey: "449.p.30123", PlayerName: "Bench WR", Position: "WR"},
	}
	players := map[string]PlayerRecord{
		"449.p.31862": {PlayerKey: "449.p.31862", FullName: "Puka Nacua", DisplayPosition: "WR", PointsTotal: 210.0},
		"449.p.30123": {PlayerKey: "449.p.30123", FullName: "Bench WR", DisplayPosition: "WR", PointsTotal: 70.0},
	}
	owners := map[string]string{"449.p.31862": "449.l.365083.t.1"}
	roi := ComputeDraftROI(picks, players, owners)
	if len(roi) != 2 || roi[0].Verdict != "steal" || !roi[0].RetainedByDrafter {
		t.Fatalf("unexpected draft ROI: %+v", roi)
	}
}

func TestBuildMatchupRadarAndExtractors(t *testing.T) {
	rawMatchups := []byte(`[{"week":9,"status":"midevent","team_key":"449.l.365083.t.1","team_name":"Alpha","team_points":88.4,"team_projected_points":124.5,"opponent_key":"449.l.365083.t.2","opponent_name":"Bravo","opponent_points":79.2,"opponent_projected_points":115.0,"team_starters":[{"player_key":"449.p.31862","full_name":"Josh Allen","selected_position":"QB","projected_points":24.5,"points_total":21.2}],"opponent_starters":[{"player_key":"449.p.30123","full_name":"Lamar Jackson","selected_position":"QB","projected_points":22.0,"points_total":18.4}]}]`)
	matchups := ExtractMatchups(rawMatchups)
	if len(matchups) != 1 {
		t.Fatalf("expected 1 matchup, got %d", len(matchups))
	}
	radar := BuildMatchupRadar("449.l.365083", matchups[0])
	if radar.ProjectedMargin != 9.5 || len(radar.SlotComparisons) != 1 {
		t.Fatalf("unexpected matchup radar: %+v", radar)
	}

	rawTeams := []byte(`[{"team_key":"449.l.365083.t.1","name":"Alpha","wins":6,"losses":3,"points_for":1100.5,"faab_balance":55}]`)
	teams := ExtractTeams(rawTeams)
	if len(teams) != 1 || teams[0].FaabBalance != 55 {
		t.Fatalf("unexpected teams: %+v", teams)
	}

	rawTxs := []byte(`[{"transaction_key":"449.l.365083.tr.1","type":"add/drop","status":"successful","team_key":"449.l.365083.t.1","faab_bid":12,"add_player_key":"449.p.31862"}]`)
	txs := ExtractTransactions(rawTxs)
	if len(txs) != 1 || txs[0].FaabBid != 12 {
		t.Fatalf("unexpected transactions: %+v", txs)
	}

	rawPicks := []byte(`[{"pick":1,"round":1,"team_key":"449.l.365083.t.1","player_key":"449.p.31862","cost":45}]`)
	picks := ExtractDraftPicks(rawPicks)
	if len(picks) != 1 || picks[0].Cost != 45 {
		t.Fatalf("unexpected draft picks: %+v", picks)
	}
}
