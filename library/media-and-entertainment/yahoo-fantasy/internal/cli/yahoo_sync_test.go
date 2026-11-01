package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
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
