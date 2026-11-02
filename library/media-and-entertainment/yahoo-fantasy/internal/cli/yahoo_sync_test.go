package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
)

type fakeYahooSyncClient struct {
	handlers map[string]func() (json.RawMessage, error)
	called   []string
}

func (f *fakeYahooSyncClient) Get(_ context.Context, path string, _ map[string]string) (json.RawMessage, error) {
	f.called = append(f.called, path)
	if h, ok := f.handlers[path]; ok {
		return h()
	}
	return nil, fmt.Errorf("unexpected path: %s", path)
}

func (f *fakeYahooSyncClient) RateLimit() float64 { return 0 }

func runTestRootCmd(t *testing.T, args ...string) string {
	t.Helper()
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("command %v failed: %v (stderr: %s)", args, err, stderr.String())
	}
	return stdout.String()
}

func TestYahooSyncNoDuplicateReportRecordsAndStablePlayerKeys(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sync.db")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	faPlayerKey := "449.p.39999"
	fc := &fakeYahooSyncClient{
		handlers: map[string]func() (json.RawMessage, error){
			"/league/449.l.365083/teams": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","teams":[{"team":{"team_key":"449.l.365083.t.1","name":"Alpha","faab_balance":"85"}}]}}}`), nil
			},
			"/league/449.l.365083/transactions": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","transactions":[{"transaction":{"transaction_key":"449.l.365083.tr.1","type":"add/drop","status":"successful","faab_bid":"15","timestamp":"1730000000","players":[{"player":{"player_key":"449.p.30123","name":{"full":"Bucky Irving"},"display_position":"RB","transaction_data":{"type":"add","destination_team_key":"449.l.365083.t.1"}}}]}}]}}}`), nil
			},
			"/league/449.l.365083/draftresults": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","draft_results":[{"draft_result":{"pick":1,"round":1,"team_key":"449.l.365083.t.1","player_key":"449.p.31002"}}]}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.31002/stats;type=season;week=1": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.31002","name":{"full":"Draft Star"},"display_position":"RB","player_points":{"total":"210.5"}}}]}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.31002/ownership": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.31002","ownership":{"ownership_type":"team","owner_team_key":"449.l.365083.t.1"}}}]}}}`), nil
			},
			"/league/449.l.365083/players;status=A;position=O;sort=PTS;start=0;count=25/percent_owned": func() (json.RawMessage, error) {
				return json.RawMessage(fmt.Sprintf(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"%s","name":{"full":"Free Agent"},"display_position":"WR","percent_owned":{"value":12}}}]}}}`, faPlayerKey)), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.39999/stats;type=season;week=1": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.39999","name":{"full":"Free Agent"},"display_position":"WR","player_points":{"total":"88.0"},"player_stats":{"stats":[{"stat":{"stat_id":"0","value":"8"}}]}}}]}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.39999/ownership": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.39999","ownership":{"ownership_type":"freeagents"}}}]}}}`), nil
			},
		},
	}

	up, err := parseSyncUserParams([]string{"league_key=449.l.365083"}, nil, nil)
	if err != nil {
		t.Fatalf("parseSyncUserParams: %v", err)
	}
	var events bytes.Buffer
	for _, res := range []string{"teams", "transactions", "draftresults", "players"} {
		sr := syncYahooLeagueScopedResource(ctx, fc, db, res, 1, false, up, &events, time.Now())
		if sr.Err != nil {
			t.Fatalf("sync %s error: %v", res, sr.Err)
		}
	}

	// Verify only 2 stable per-player rows exist (no batch rows piling up).
	playerCount, err := db.Count("players")
	if err != nil || playerCount != 2 {
		t.Fatalf("expected 2 stable per-player rows, got %d (err=%v)", playerCount, err)
	}
	_ = db.Close()

	// Verify faab-audit counts the $15 acquisition once, not twice.
	out := runTestRootCmd(t, "faab-audit", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(out, `"total_spent_faab": 15`) || !strings.Contains(out, `"paid_acquisitions": 1`) {
		t.Fatalf("expected single $15 acquisition in faab-audit, got: %s", out)
	}

	// Verify draft-roi retains the 210.5 points for 449.p.31002 even after players sync ran.
	drOut := runTestRootCmd(t, "draft-roi", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(drOut, `"season_points": 210.5`) {
		t.Fatalf("expected draft-roi to preserve 210.5 season_points after players sync, got: %s", drOut)
	}
}

func TestYahooSyncPartialFailureLatestOnlyAndDownwardScoreCorrection(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sync_scope.db")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Seed an unrelated historical league AND a stale week-1 league blob for 449.l.365083,
	// plus an old raw scoreboard with higher (pre-correction) week 3 score 130.0.
	_ = db.Upsert("leagues", "449.l.999999", json.RawMessage(`{"league_key":"449.l.999999","current_week":5}`))
	_ = db.Upsert("leagues", "449.l.365083", json.RawMessage(`{"league_key":"449.l.365083","current_week":1}`))
	_ = db.Upsert("scoreboard", "449.l.365083.sb.3", json.RawMessage(`{"league_key":"449.l.365083","week":3,"team_key":"449.l.365083.t.1","team_name":"Alpha","team_points":130.0,"team_projected_points":128.0,"opponent_key":"449.l.365083.t.2","opponent_name":"Beta","opponent_points":102.0,"opponent_projected_points":105.0}`))

	fc := &fakeYahooSyncClient{
		handlers: map[string]func() (json.RawMessage, error){
			"/league/449.l.365083": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","current_week":3}}}`), nil
			},
			"/league/449.l.365083/scoreboard;week=3": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","scoreboard":{"week":3,"matchups":[{"matchup":{"week":3,"teams":[{"team":{"team_key":"449.l.365083.t.1","name":"Alpha","team_points":{"total":"115.0"},"team_projected_points":{"total":"112.0"}}},{"team":{"team_key":"449.l.365083.t.2","name":"Beta","team_points":{"total":"102.0"},"team_projected_points":{"total":"100.0"}}}]}}]}}}}`), nil
			},
			"/league/449.l.365083/scoreboard;week=1": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","scoreboard":{"week":1,"matchups":[{"matchup":{"week":1,"teams":[{"team":{"team_key":"449.l.365083.t.1","name":"Alpha","team_points":{"total":"100.0"}}},{"team":{"team_key":"449.l.365083.t.2","name":"Beta","team_points":{"total":"95.0"}}}]}}]}}}}`), nil
			},
			"/league/449.l.365083/scoreboard;week=2": func() (json.RawMessage, error) {
				return nil, fmt.Errorf("503 week 2 scoreboard unavailable")
			},
		},
	}

	up, err := parseSyncUserParams([]string{"league_key=449.l.365083"}, nil, nil)
	if err != nil {
		t.Fatalf("parseSyncUserParams: %v", err)
	}

	// 1. --latest-only should query live /league/449.l.365083 and fetch week 3 even when DB had current_week=1.
	var events bytes.Buffer
	srLatest := syncYahooLeagueScopedResource(ctx, fc, db, "scoreboard", 1, true, up, &events, time.Now())
	if srLatest.Err != nil {
		t.Fatalf("latest-only sync failed: %v", srLatest.Err)
	}
	for _, p := range fc.called {
		if strings.Contains(p, "449.l.999999") {
			t.Fatalf("expected explicit league_key sync to ignore unrelated saved league, but called %s", p)
		}
		if strings.Contains(p, "scoreboard;week=1") {
			t.Fatalf("expected --latest-only to fetch current week 3, not week 1")
		}
	}

	// 2. Full scoreboard sync where week 1 succeeds and week 2 fails must return an error and NOT emit sync_complete.
	events.Reset()
	srFull := syncYahooLeagueScopedResource(ctx, fc, db, "matchups", 0, false, up, &events, time.Now())
	if srFull.Err == nil {
		t.Fatalf("expected partial week failure to return error, got nil")
	}
	if strings.Contains(events.String(), `"event":"sync_complete"`) {
		t.Fatalf("expected failed sync not to emit sync_complete, got: %s", events.String())
	}
	_ = db.Close()

	// 3. Downward score correction (130.0 -> 115.0) must report 115.0 in matchup-radar, not the old higher 130.0.
	mrOut := runTestRootCmd(t, "matchup-radar", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--week", "3", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(mrOut, `"team_points": 115`) || strings.Contains(mrOut, "130") {
		t.Fatalf("expected downward-corrected score 115.0 in matchup-radar, got: %s", mrOut)
	}
}

func TestYahooSyncLegacyBatchMigrationDroppedOwnerClearingAndConcurrentUpserts(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy_and_drop.db")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	// Seed legacy batch rows for a drafted player (449.p.31002) and a draft pick row,
	// a rostered player (449.p.32005) who will be dropped to free agency,
	// and a rostered player (449.p.33009) who was traded from t.1 to t.2.
	_ = db.Upsert("leagues", "449.l.365083", json.RawMessage(`{"league_key":"449.l.365083","current_week":1}`))
	_ = db.Upsert("teams", "449.l.365083.t.1", json.RawMessage(`{"league_key":"449.l.365083","team_key":"449.l.365083.t.1","name":"Alpha"}`))
	_ = db.Upsert("teams", "449.l.365083.t.2", json.RawMessage(`{"league_key":"449.l.365083","team_key":"449.l.365083.t.2","name":"Beta"}`))
	_ = db.Upsert("draft", "449.l.365083.pick.1.1", json.RawMessage(`{"league_key":"449.l.365083","round":1,"pick":1,"team_key":"449.l.365083.t.1","player_key":"449.p.31002"}`))
	_ = db.Upsert("draft", "449.l.365083.pick.2.1", json.RawMessage(`{"league_key":"449.l.365083","round":2,"pick":13,"team_key":"449.l.365083.t.1","player_key":"449.p.32005"}`))
	_ = db.Upsert("players", "449.l.365083:stats:0", json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.31002","name":{"full":"Legacy Drafted"},"display_position":"RB","player_points":{"total":"195.0"}}}]}}}`))
	_ = db.Upsert("players", "449.l.365083:ownership:0", json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.31002","ownership":{"ownership_type":"team","owner_team_key":"449.l.365083.t.1"}}}]}}}`))
	_ = db.Upsert("players", "449.l.365083:player:449.p.32005", json.RawMessage(`{"league_key":"449.l.365083","player_key":"449.p.32005","full_name":"Dropped WR","display_position":"WR","selected_position":"WR","ownership_status":"T","owner_team_key":"449.l.365083.t.1","points_total":120.0,"games_played":8,"points_per_game":15.0}`))
	_ = db.Upsert("players", "449.l.365083:player:449.p.33009", json.RawMessage(`{"league_key":"449.l.365083","player_key":"449.p.33009","full_name":"Traded Star","display_position":"WR","selected_position":"BN","ownership_status":"T","owner_team_key":"449.l.365083.t.1","points_total":180.0,"games_played":8,"points_per_game":22.5}`))

	fc := &fakeYahooSyncClient{
		handlers: map[string]func() (json.RawMessage, error){
			"/league/449.l.365083": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","current_week":1}}}`), nil
			},
			"/league/449.l.365083/players;status=A;position=O;sort=PTS;start=0;count=25/percent_owned": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.32005","name":{"full":"Dropped WR"},"display_position":"WR","percent_owned":{"value":35}}}]}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.32005/stats;type=season;week=1": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.32005","name":{"full":"Dropped WR"},"display_position":"WR","player_points":{"total":"120.0"},"player_stats":{"stats":[{"stat":{"stat_id":"0","value":"8"}}]}}}]}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.32005/ownership": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.32005","ownership":{"ownership_type":"freeagents"}}}]}}}`), nil
			},
			"/team/449.l.365083.t.1/roster;week=1": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"team":{"team_key":"449.l.365083.t.1","roster":{"players":[{"player":{"player_key":"449.p.31002","name":{"full":"Legacy Drafted"},"display_position":"RB","selected_position":{"position":"RB"}}}]}}}}`), nil
			},
			"/league/449.l.365083/players;player_keys=449.p.33009/ownership": func() (json.RawMessage, error) {
				return json.RawMessage(`{"fantasy_content":{"league":{"league_key":"449.l.365083","players":[{"player":{"player_key":"449.p.33009","ownership":{"ownership_type":"team","owner_team_key":"449.l.365083.t.2"}}}]}}}`), nil
			},
		},
	}

	up, err := parseSyncUserParams([]string{"league_key=449.l.365083"}, nil, nil)
	if err != nil {
		t.Fatalf("parseSyncUserParams: %v", err)
	}
	var events bytes.Buffer
	// Run ONLY "players" sync (without re-running "draftresults") to verify legacy batch rows for 449.p.31002 are migrated, not deleted.
	sr := syncYahooLeagueScopedResource(ctx, fc, db, "players", 1, true, up, &events, time.Now())
	if sr.Err != nil {
		t.Fatalf("sync players failed: %v", sr.Err)
	}

	// Sync only team t.1's roster: 449.p.33009 left t.1 via trade to t.2, so ownership check must assign it to t.2 (not FA).
	upTeam1, err := parseSyncUserParams([]string{"team_key=449.l.365083.t.1"}, nil, nil)
	if err != nil {
		t.Fatalf("parseSyncUserParams team1: %v", err)
	}
	srRoster := syncYahooLeagueScopedResource(ctx, fc, db, "rosters", 1, true, upTeam1, &events, time.Now())
	if srRoster.Err != nil {
		t.Fatalf("sync rosters failed: %v", srRoster.Err)
	}

	// Verify FTS search finds players written by upsertYahooPlayerRecords.
	matches, sErr := db.Search("Dropped WR", 10, "players")
	if sErr != nil || len(matches) == 0 {
		t.Fatalf("expected FTS search to find 'Dropped WR', got %d matches (err=%v)", len(matches), sErr)
	}

	// Verify traded player 449.p.33009 had its old team's BN slot cleared when ownership moved from t.1 to t.2.
	tradedRaw, gErr := db.Get("players", "449.l.365083:player:449.p.33009")
	if gErr != nil || strings.Contains(string(tradedRaw), `"selected_position"`) {
		t.Fatalf("expected traded player 449.p.33009 to clear old team's selected_position, got: %s (err=%v)", string(tradedRaw), gErr)
	}

	// Verify concurrent upserts on the same player preserve merged fields from both writers.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = upsertYahooPlayerRecords(ctx, db, "449.l.365083", []yahoofantasy.PlayerRecord{{
			LeagueKey:   "449.l.365083",
			PlayerKey:   "449.p.31002",
			PointsTotal: 205.0,
			GamesPlayed: 10,
		}}, nil)
	}()
	go func() {
		defer wg.Done()
		_ = upsertYahooPlayerRecords(ctx, db, "449.l.365083", []yahoofantasy.PlayerRecord{{
			LeagueKey:       "449.l.365083",
			PlayerKey:       "449.p.31002",
			PercentOwned:    98.0,
			OwnershipStatus: "T",
			OwnerTeamKey:    "449.l.365083.t.1",
		}}, nil)
	}()
	wg.Wait()
	_ = db.Close()

	// 1. Verify draft-roi retained migrated player 449.p.31002 (205.0 pts, retained_by_drafter: true)
	// AND cleared owner_team_key on dropped player 449.p.32005 (retained_by_drafter: false).
	drOut := runTestRootCmd(t, "draft-roi", "--league", "449.l.365083", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(drOut, `"season_points": 205`) {
		t.Fatalf("expected migrated + concurrently updated player 449.p.31002 to have 205 season_points, got: %s", drOut)
	}
	if !strings.Contains(drOut, `"retained_by_drafter": false`) {
		t.Fatalf("expected dropped player 449.p.32005 to have retained_by_drafter=false in draft-roi, got: %s", drOut)
	}

	// 2. Verify waiver-wire recommends the dropped FA (449.p.32005) and does NOT recommend the traded player (449.p.33009).
	wwOut := runTestRootCmd(t, "waiver-wire", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(wwOut, "449.p.32005") {
		t.Fatalf("expected dropped player 449.p.32005 in waiver-wire candidates, got: %s", wwOut)
	}
	if strings.Contains(wwOut, "449.p.33009") {
		t.Fatalf("expected traded player 449.p.33009 (now owned by t.2) NOT to appear in waiver-wire candidates, got: %s", wwOut)
	}

	// 3. Verify player-name resolution in lineup (--set "RB=Legacy Drafted") and add-drop (--add "Dropped WR" --drop "Legacy Drafted").
	luOut := runTestRootCmd(t, "lineup", "--team", "449.l.365083.t.1", "--week", "1", "--set", "RB=Legacy Drafted", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(luOut, `"player_key": "449.p.31002"`) {
		t.Fatalf("expected lineup --set 'RB=Legacy Drafted' to resolve to 449.p.31002, got: %s", luOut)
	}
	adOut := runTestRootCmd(t, "add-drop", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--add", "Dropped WR", "--drop", "Legacy Drafted", "--db", dbPath, "--data-source", "local", "--json")
	if !strings.Contains(adOut, `"add_player_key": "449.p.32005"`) || !strings.Contains(adOut, `"drop_player_key": "449.p.31002"`) {
		t.Fatalf("expected add-drop player names to resolve to 449.p.32005 and 449.p.31002, got: %s", adOut)
	}
}
