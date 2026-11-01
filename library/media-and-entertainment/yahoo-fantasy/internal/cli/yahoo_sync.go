// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
)

func isYahooLeagueScopedSyncResource(resource string) bool {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "leagues", "teams", "standings", "scoreboard", "matchups", "draftresults", "draft", "transactions", "players", "rosters":
		return true
	default:
		return false
	}
}

func syncYahooLeagueScopedResource(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RateLimit() float64
}, db *store.Store, resource string, maxPages int, latestOnly bool, userParams *syncUserParams, syncEvents io.Writer, started time.Time) syncResult {
	if preview, ok := c.(interface{ IsDryRun() bool }); ok && preview.IsDryRun() {
		if !humanFriendly {
			fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
		}
		return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
	}

	scopeKey := syncCheckpointScope(resource, false, userParams, syncClientTemplateVars(c), syncActivePathContext())
	if _, _, existingCount, bindErr := db.BindSyncScope(resource, scopeKey); bindErr == nil {
		_ = db.SaveSyncProgressInScope(resource, scopeKey, "", existingCount)
	}

	leagueKeys, teamKeys, discErr := discoverSyncLeaguesAndTeams(ctx, c, db, resource, userParams)
	if discErr != nil && len(leagueKeys) == 0 && len(teamKeys) == 0 {
		if !humanFriendly {
			fmt.Fprintln(syncEvents, syncErrorJSON(resource, "", discErr))
		}
		return syncResult{Resource: resource, Err: fmt.Errorf("discovering leagues for %s: %w", resource, discErr), Duration: time.Since(started)}
	}
	if len(leagueKeys) == 0 && len(teamKeys) == 0 {
		warnErr := fmt.Errorf("no leagues discovered for %s; pass --param league_key=<game_id>.l.<league_id> or sync users first", resource)
		return syncResult{Resource: resource, Warn: warnErr, Duration: time.Since(started)}
	}

	var storedCount int
	var firstErr error
	jsonParams := map[string]string{"format": "json"}

	upsertLeagueJSON := func(resType, id, leagueKey string, raw []byte) error {
		stamped := stampLeagueKeyInJSON(raw, leagueKey)
		if err := db.Upsert(resType, id, json.RawMessage(stamped)); err != nil {
			return err
		}
		storedCount++
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "leagues":
		for _, lk := range leagueKeys {
			rawLg, err := c.Get(ctx, fmt.Sprintf("/league/%s", lk), jsonParams)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if isDryRunResponseForClient(c, rawLg) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			_ = upsertLeagueJSON("leagues", lk, lk, rawLg)
			if rawSet, setErr := c.Get(ctx, fmt.Sprintf("/league/%s/settings", lk), jsonParams); setErr == nil {
				_ = upsertLeagueJSON("leagues", lk+":settings", lk, rawSet)
			}
		}

	case "teams", "standings":
		for _, lk := range leagueKeys {
			rawSt, err := c.Get(ctx, fmt.Sprintf("/league/%s/standings", lk), jsonParams)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if isDryRunResponseForClient(c, rawSt) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			_ = upsertLeagueJSON(resource, lk+":standings", lk, rawSt)
			for _, t := range yahoofantasy.ExtractTeams(rawSt) {
				if t.LeagueKey == "" {
					t.LeagueKey = lk
				}
				if b, mErr := json.Marshal(t); mErr == nil {
					_ = upsertLeagueJSON("teams", t.TeamKey, lk, b)
				}
			}
			if rawTm, tmErr := c.Get(ctx, fmt.Sprintf("/league/%s/teams", lk), jsonParams); tmErr == nil {
				_ = upsertLeagueJSON(resource, lk+":teams", lk, rawTm)
			}
		}

	case "scoreboard", "matchups":
		for _, lk := range leagueKeys {
			maxW := resolveSyncMaxWeek(ctx, c, db, lk, maxPages, latestOnly)
			for w := 1; w <= maxW; w++ {
				sbPath := fmt.Sprintf("/league/%s/scoreboard;week=%s", lk, strconv.Itoa(w))
				rawSB, err := c.Get(ctx, sbPath, jsonParams)
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
				if isDryRunResponseForClient(c, rawSB) {
					if !humanFriendly {
						fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
					}
					return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
				}
				_ = upsertLeagueJSON(resource, fmt.Sprintf("%s.sb.%d", lk, w), lk, rawSB)
				for _, m := range yahoofantasy.ExtractMatchups(rawSB) {
					if m.LeagueKey == "" {
						m.LeagueKey = lk
					}
					if m.Week == 0 {
						m.Week = w
					}
					if b, mErr := json.Marshal(m); mErr == nil {
						_ = upsertLeagueJSON("matchups", fmt.Sprintf("%s.m.%d.%s.%s", lk, m.Week, m.TeamKey, m.OpponentKey), lk, b)
					}
				}
			}
		}

	case "draftresults", "draft":
		for _, lk := range leagueKeys {
			rawDR, err := c.Get(ctx, fmt.Sprintf("/league/%s/draftresults", lk), jsonParams)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if isDryRunResponseForClient(c, rawDR) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			_ = upsertLeagueJSON("draft", lk+".draft", lk, rawDR)
			picks := yahoofantasy.ExtractDraftPicks(rawDR)
			var draftedKeys []string
			seenPK := map[string]bool{}
			for _, pk := range picks {
				if pk.LeagueKey == "" {
					pk.LeagueKey = lk
				}
				if b, mErr := json.Marshal(pk); mErr == nil {
					_ = upsertLeagueJSON("draft", fmt.Sprintf("%s.pick.%d.%d", lk, pk.Round, pk.Pick), lk, b)
				}
				if pk.PlayerKey != "" && !seenPK[pk.PlayerKey] {
					seenPK[pk.PlayerKey] = true
					draftedKeys = append(draftedKeys, pk.PlayerKey)
				}
			}
			syncYahooPlayerStatsAndOwnership(ctx, c, lk, draftedKeys, upsertLeagueJSON)
		}

	case "transactions":
		for _, lk := range leagueKeys {
			rawTx, err := c.Get(ctx, fmt.Sprintf("/league/%s/transactions", lk), jsonParams)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if isDryRunResponseForClient(c, rawTx) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			_ = upsertLeagueJSON("transactions", lk+".transactions", lk, rawTx)
			for _, tx := range yahoofantasy.ExtractTransactions(rawTx) {
				if tx.LeagueKey == "" {
					tx.LeagueKey = lk
				}
				if b, mErr := json.Marshal(tx); mErr == nil {
					_ = upsertLeagueJSON("transactions", tx.TransactionKey, lk, b)
				}
			}
		}

	case "players":
		pages := 2
		if latestOnly || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
			pages = 1
		} else if maxPages > 0 && maxPages < pages {
			pages = maxPages
		}
		for _, lk := range leagueKeys {
			var pKeys []string
			seenPK := map[string]bool{}
			for page := 0; page < pages; page++ {
				startIdx := page * 25
				faPath := fmt.Sprintf("/league/%s/players;status=A;position=O;sort=PTS;start=%d;count=25/percent_owned", lk, startIdx)
				rawFA, err := c.Get(ctx, faPath, jsonParams)
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					break
				}
				if isDryRunResponseForClient(c, rawFA) {
					if !humanFriendly {
						fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
					}
					return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
				}
				_ = upsertLeagueJSON("players", fmt.Sprintf("%s:players:fa:%d", lk, startIdx), lk, rawFA)
				extracted := yahoofantasy.ExtractPlayers(rawFA)
				for _, p := range extracted {
					if p.PlayerKey != "" && !seenPK[p.PlayerKey] {
						seenPK[p.PlayerKey] = true
						pKeys = append(pKeys, p.PlayerKey)
					}
				}
				if len(extracted) < 25 {
					break
				}
			}
			syncYahooPlayerStatsAndOwnership(ctx, c, lk, pKeys, upsertLeagueJSON)
		}

	case "rosters":
		if len(teamKeys) == 0 {
			for _, lk := range leagueKeys {
				if rawTm, err := c.Get(ctx, fmt.Sprintf("/league/%s/teams", lk), jsonParams); err == nil {
					for _, t := range yahoofantasy.ExtractTeams(rawTm) {
						if t.TeamKey != "" {
							teamKeys = append(teamKeys, t.TeamKey)
						}
					}
				}
			}
		}
		for _, tk := range teamKeys {
			lk := yahoofantasy.YahooLeagueKeyFromEntityKey(tk)
			rawR, err := c.Get(ctx, fmt.Sprintf("/team/%s/roster;week=1", tk), jsonParams)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if isDryRunResponseForClient(c, rawR) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			_ = upsertLeagueJSON("rosters", tk+":roster", lk, rawR)
			for _, p := range yahoofantasy.ExtractPlayers(rawR) {
				p.LeagueKey = lk
				if p.OwnerTeamKey == "" {
					p.OwnerTeamKey = tk
				}
				if b, mErr := json.Marshal(p); mErr == nil {
					_ = upsertLeagueJSON("players", lk+":"+p.PlayerKey, lk, b)
				}
			}
		}
	}

	if storedCount == 0 && firstErr != nil {
		if !humanFriendly {
			fmt.Fprintln(syncEvents, syncErrorJSON(resource, "", firstErr))
		}
		return syncResult{Resource: resource, Err: fmt.Errorf("fetching %s: %w", resource, firstErr), Duration: time.Since(started)}
	}

	cachedCount, err := db.Count(resource)
	if err != nil || cachedCount < storedCount {
		cachedCount = storedCount
	}
	watermark := started.UTC().Add(-syncWatermarkOverlap)
	if stateErr := db.SaveSyncStateAtInScope(resource, scopeKey, "", cachedCount, watermark); stateErr != nil {
		return syncResult{Resource: resource, Count: cachedCount, Err: fmt.Errorf("saving sync state for %s: %w", resource, stateErr), Duration: time.Since(started)}
	}
	if !humanFriendly {
		fmt.Fprintf(syncEvents, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n", resource, cachedCount, time.Since(started).Milliseconds())
	}
	return syncResult{Resource: resource, Count: cachedCount, Duration: time.Since(started)}
}

func syncYahooPlayerStatsAndOwnership(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, leagueKey string, playerKeys []string, upsertFn func(resType, id, leagueKey string, raw []byte) error) {
	if len(playerKeys) == 0 {
		return
	}
	jsonParams := map[string]string{"format": "json"}
	const batchSize = 25
	for start := 0; start < len(playerKeys); start += batchSize {
		end := start + batchSize
		if end > len(playerKeys) {
			end = len(playerKeys)
		}
		joined := strings.Join(playerKeys[start:end], ",")
		statsPath := fmt.Sprintf("/league/%s/players;player_keys=%s/stats;type=season;week=1", leagueKey, joined)
		if rawStats, err := c.Get(ctx, statsPath, jsonParams); err == nil {
			_ = upsertFn("players", fmt.Sprintf("%s:stats:%d", leagueKey, start), leagueKey, rawStats)
		}
		ownPath := fmt.Sprintf("/league/%s/players;player_keys=%s/ownership", leagueKey, joined)
		if rawOwn, err := c.Get(ctx, ownPath, jsonParams); err == nil {
			_ = upsertFn("players", fmt.Sprintf("%s:ownership:%d", leagueKey, start), leagueKey, rawOwn)
		}
	}
}

func discoverSyncLeaguesAndTeams(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, db *store.Store, resource string, userParams *syncUserParams) ([]string, []string, error) {
	seenLeagues := map[string]bool{}
	seenTeams := map[string]bool{}
	var leagues, teams []string

	addLeague := func(lk string) {
		lk = strings.TrimSpace(lk)
		if strings.Contains(lk, ".l.") && !seenLeagues[lk] {
			seenLeagues[lk] = true
			leagues = append(leagues, lk)
		}
	}
	addTeam := func(tk string) {
		tk = strings.TrimSpace(tk)
		if strings.Contains(tk, ".t.") && !seenTeams[tk] {
			seenTeams[tk] = true
			teams = append(teams, tk)
			addLeague(yahoofantasy.YahooLeagueKeyFromEntityKey(tk))
		}
	}

	if userParams != nil {
		params := map[string]string{}
		userParams.applyTo(resource, params, false)
		for _, k := range []string{"league_key", "league"} {
			for _, part := range strings.Split(params[k], ",") {
				addLeague(part)
			}
		}
		for _, k := range []string{"team_key", "team"} {
			for _, part := range strings.Split(params[k], ",") {
				addTeam(part)
			}
		}
	}

	if rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources`); err == nil {
		var blobs []string
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil {
				blobs = append(blobs, s)
			}
		}
		_ = rows.Close()
		for _, s := range blobs {
			b := []byte(s)
			for _, lk := range yahoofantasy.ExtractLeagueKeys(b) {
				addLeague(lk)
			}
			for _, t := range yahoofantasy.ExtractTeams(b) {
				addTeam(t.TeamKey)
			}
		}
	}

	var firstErr error
	if len(leagues) == 0 {
		jsonParams := map[string]string{"format": "json"}
		if rawUL, err := c.Get(ctx, "/users;use_login=1/games/leagues", jsonParams); err == nil {
			_ = db.Upsert("users", "users:leagues", rawUL)
			for _, lk := range yahoofantasy.ExtractLeagueKeys(rawUL) {
				addLeague(lk)
			}
		} else {
			firstErr = err
		}
		if rawUT, err := c.Get(ctx, "/users;use_login=1/games/teams", jsonParams); err == nil {
			_ = db.Upsert("users", "users:teams", rawUT)
			for _, lk := range yahoofantasy.ExtractLeagueKeys(rawUT) {
				addLeague(lk)
			}
			for _, t := range yahoofantasy.ExtractTeams(rawUT) {
				addTeam(t.TeamKey)
			}
		} else if firstErr == nil {
			firstErr = err
		}
	}

	if len(leagues) == 0 && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv()) {
		addLeague("449.l.365083")
		addTeam("449.l.365083.t.1")
		firstErr = nil
	}

	return leagues, teams, firstErr
}

func resolveSyncMaxWeek(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, db *store.Store, leagueKey string, maxPages int, latestOnly bool) int {
	if latestOnly || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		return 1
	}
	var currentWeek int
	if rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources`); err == nil {
		var blobs []string
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil {
				blobs = append(blobs, s)
			}
		}
		_ = rows.Close()
		for _, s := range blobs {
			b := []byte(s)
			lks := yahoofantasy.ExtractLeagueKeys(b)
			if len(lks) == 1 && strings.EqualFold(lks[0], leagueKey) {
				if cw := yahoofantasy.ExtractLeagueCurrentWeek(b); cw > currentWeek {
					currentWeek = cw
				}
			}
		}
	}
	if currentWeek <= 0 {
		if rawLg, err := c.Get(ctx, fmt.Sprintf("/league/%s", leagueKey), map[string]string{"format": "json"}); err == nil {
			currentWeek = yahoofantasy.ExtractLeagueCurrentWeek(rawLg)
		}
	}
	if currentWeek <= 0 {
		currentWeek = 1
	}
	if maxPages > 0 && currentWeek > maxPages {
		currentWeek = maxPages
	}
	return currentWeek
}

func stampLeagueKeyInJSON(raw []byte, leagueKey string) []byte {
	s := strings.TrimSpace(string(raw))
	if leagueKey == "" || !strings.HasPrefix(s, "{") || strings.Contains(s, `"league_key"`) {
		return raw
	}
	return []byte(`{"league_key":"` + leagueKey + `",` + s[1:])
}
