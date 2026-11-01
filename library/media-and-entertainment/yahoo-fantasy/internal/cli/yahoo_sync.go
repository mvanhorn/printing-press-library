// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
)

var (
	yahooPlayerUpsertMu  sync.Mutex
	yahooLegacyMigrateMu sync.Mutex
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
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if discErr != nil {
		recordErr(discErr)
	}
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
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawLg) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			recordErr(upsertLeagueJSON("leagues", lk, lk, rawLg))
			rawSet, setErr := c.Get(ctx, fmt.Sprintf("/league/%s/settings", lk), jsonParams)
			if setErr != nil {
				recordErr(setErr)
				continue
			}
			recordErr(upsertLeagueJSON("leagues", lk+":settings", lk, rawSet))
		}

	case "teams":
		for _, lk := range leagueKeys {
			rawTm, err := c.Get(ctx, fmt.Sprintf("/league/%s/teams", lk), jsonParams)
			if err != nil {
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawTm) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			teams := extractLeagueTeamsFromBlobs([][]byte{rawTm}, lk)
			if len(teams) > 0 {
				var writeErr error
				for _, t := range teams {
					if t.LeagueKey == "" {
						t.LeagueKey = lk
					}
					b, mErr := json.Marshal(t)
					if mErr != nil {
						writeErr = mErr
						recordErr(mErr)
						continue
					}
					if uErr := upsertLeagueJSON("teams", t.TeamKey, lk, b); uErr != nil {
						writeErr = uErr
						recordErr(uErr)
					}
				}
				if writeErr == nil {
					_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, lk+":teams")
				}
			} else {
				recordErr(upsertLeagueJSON("teams", lk+":teams", lk, rawTm))
			}
		}

	case "standings":
		for _, lk := range leagueKeys {
			rawSt, err := c.Get(ctx, fmt.Sprintf("/league/%s/standings", lk), jsonParams)
			if err != nil {
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawSt) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			teams := extractLeagueTeamsFromBlobs([][]byte{rawSt}, lk)
			if len(teams) > 0 {
				var writeErr error
				for _, t := range teams {
					if t.LeagueKey == "" {
						t.LeagueKey = lk
					}
					b, mErr := json.Marshal(t)
					if mErr != nil {
						writeErr = mErr
						recordErr(mErr)
						continue
					}
					if uErr := upsertLeagueJSON("standings", t.TeamKey+":standings", lk, b); uErr != nil {
						writeErr = uErr
						recordErr(uErr)
					}
				}
				if writeErr == nil {
					_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, lk+":standings")
				}
			} else {
				recordErr(upsertLeagueJSON("standings", lk+":standings", lk, rawSt))
			}
		}

	case "scoreboard", "matchups":
		for _, lk := range leagueKeys {
			startW, endW, wErr := resolveSyncWeekRange(ctx, c, db, lk, maxPages, latestOnly)
			if wErr != nil {
				recordErr(wErr)
				continue
			}
			for w := startW; w <= endW; w++ {
				sbPath := fmt.Sprintf("/league/%s/scoreboard;week=%s", lk, strconv.Itoa(w))
				rawSB, err := c.Get(ctx, sbPath, jsonParams)
				if err != nil {
					recordErr(err)
					continue
				}
				if isDryRunResponseForClient(c, rawSB) {
					if !humanFriendly {
						fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
					}
					return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
				}
				matchups := yahoofantasy.ExtractMatchups(rawSB)
				if len(matchups) > 0 {
					var writeErr error
					for _, m := range matchups {
						if m.LeagueKey == "" {
							m.LeagueKey = lk
						}
						if m.Week == 0 {
							m.Week = w
						}
						b, mErr := json.Marshal(m)
						if mErr != nil {
							writeErr = mErr
							recordErr(mErr)
							continue
						}
						if uErr := upsertLeagueJSON("matchups", fmt.Sprintf("%s.m.%d.%s.%s", lk, m.Week, m.TeamKey, m.OpponentKey), lk, b); uErr != nil {
							writeErr = uErr
							recordErr(uErr)
						}
					}
					if writeErr == nil {
						_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, fmt.Sprintf("%s.sb.%d", lk, w))
					}
				} else {
					recordErr(upsertLeagueJSON(resource, fmt.Sprintf("%s.sb.%d", lk, w), lk, rawSB))
				}
			}
		}

	case "draftresults", "draft":
		for _, lk := range leagueKeys {
			recordErr(migrateLegacyLeaguePlayerRows(ctx, db, lk))
			rawDR, err := c.Get(ctx, fmt.Sprintf("/league/%s/draftresults", lk), jsonParams)
			if err != nil {
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawDR) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			picks := yahoofantasy.ExtractDraftPicks(rawDR)
			var draftedKeys []string
			seenPK := map[string]bool{}
			if len(picks) > 0 {
				var writeErr error
				for _, pk := range picks {
					if pk.LeagueKey == "" {
						pk.LeagueKey = lk
					}
					b, mErr := json.Marshal(pk)
					if mErr != nil {
						writeErr = mErr
						recordErr(mErr)
						continue
					}
					if uErr := upsertLeagueJSON("draft", fmt.Sprintf("%s.pick.%d.%d", lk, pk.Round, pk.Pick), lk, b); uErr != nil {
						writeErr = uErr
						recordErr(uErr)
					}
					if pk.PlayerKey != "" && !seenPK[pk.PlayerKey] {
						seenPK[pk.PlayerKey] = true
						draftedKeys = append(draftedKeys, pk.PlayerKey)
					}
				}
				if writeErr == nil {
					_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, lk+".draft")
				}
			} else {
				recordErr(upsertLeagueJSON("draft", lk+".draft", lk, rawDR))
			}
			baseByKey := map[string]yahoofantasy.PlayerRecord{}
			for _, p := range yahoofantasy.ExtractPlayers(rawDR) {
				if p.PlayerKey != "" {
					p.LeagueKey = lk
					baseByKey[p.PlayerKey] = p
				}
			}
			recordErr(syncYahooPlayerStatsAndOwnership(ctx, c, db, lk, draftedKeys, baseByKey, &storedCount))
		}

	case "transactions":
		for _, lk := range leagueKeys {
			rawTx, err := c.Get(ctx, fmt.Sprintf("/league/%s/transactions", lk), jsonParams)
			if err != nil {
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawTx) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			txs := yahoofantasy.ExtractTransactions(rawTx)
			if len(txs) > 0 {
				var writeErr error
				for _, tx := range txs {
					if tx.LeagueKey == "" {
						tx.LeagueKey = lk
					}
					b, mErr := json.Marshal(tx)
					if mErr != nil {
						writeErr = mErr
						recordErr(mErr)
						continue
					}
					txID := tx.TransactionKey
					if txID == "" {
						txID = fmt.Sprintf("%s.tx.%s.%s.%d", lk, tx.TeamKey, tx.AddPlayerKey, tx.Timestamp)
					}
					if uErr := upsertLeagueJSON("transactions", txID, lk, b); uErr != nil {
						writeErr = uErr
						recordErr(uErr)
					}
				}
				if writeErr == nil {
					_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, lk+".transactions")
				}
			} else {
				recordErr(upsertLeagueJSON("transactions", lk+".transactions", lk, rawTx))
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
			recordErr(migrateLegacyLeaguePlayerRows(ctx, db, lk))
			var pKeys []string
			seenPK := map[string]bool{}
			baseByKey := map[string]yahoofantasy.PlayerRecord{}
			var firstRawFA []byte
			for page := 0; page < pages; page++ {
				startIdx := page * 25
				faPath := fmt.Sprintf("/league/%s/players;status=A;position=O;sort=PTS;start=%d;count=25/percent_owned", lk, startIdx)
				rawFA, err := c.Get(ctx, faPath, jsonParams)
				if err != nil {
					recordErr(err)
					break
				}
				if isDryRunResponseForClient(c, rawFA) {
					if !humanFriendly {
						fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
					}
					return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
				}
				if firstRawFA == nil {
					firstRawFA = rawFA
				}
				extracted := yahoofantasy.ExtractPlayers(rawFA)
				for _, p := range extracted {
					if p.PlayerKey != "" {
						p.LeagueKey = lk
						baseByKey[p.PlayerKey] = p
						if !seenPK[p.PlayerKey] {
							seenPK[p.PlayerKey] = true
							pKeys = append(pKeys, p.PlayerKey)
						}
					}
				}
				if len(extracted) < 25 {
					break
				}
			}
			if len(pKeys) > 0 {
				recordErr(syncYahooPlayerStatsAndOwnership(ctx, c, db, lk, pKeys, baseByKey, &storedCount))
			} else if firstRawFA != nil {
				recordErr(upsertLeagueJSON("players", lk+":players", lk, firstRawFA))
			}
		}

	case "rosters":
		if len(teamKeys) == 0 {
			for _, lk := range leagueKeys {
				rawTm, err := c.Get(ctx, fmt.Sprintf("/league/%s/teams", lk), jsonParams)
				if err != nil {
					recordErr(err)
					continue
				}
				for _, t := range yahoofantasy.ExtractTeams(rawTm) {
					if t.TeamKey != "" {
						teamKeys = append(teamKeys, t.TeamKey)
					}
				}
			}
		}
		leagueCurrentWeek := map[string]int{}
		for _, tk := range teamKeys {
			lk := yahoofantasy.YahooLeagueKeyFromEntityKey(tk)
			recordErr(migrateLegacyLeaguePlayerRows(ctx, db, lk))
			curW, ok := leagueCurrentWeek[lk]
			if !ok {
				_, cw, wErr := resolveSyncWeekRange(ctx, c, db, lk, 1, true)
				if wErr != nil {
					recordErr(wErr)
					continue
				}
				curW = cw
				leagueCurrentWeek[lk] = curW
			}
			rawR, err := c.Get(ctx, fmt.Sprintf("/team/%s/roster;week=%d", tk, curW), jsonParams)
			if err != nil {
				recordErr(err)
				continue
			}
			if isDryRunResponseForClient(c, rawR) {
				if !humanFriendly {
					fmt.Fprintf(syncEvents, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
				}
				return syncResult{Resource: resource, Count: 0, Duration: time.Since(started)}
			}
			rosterPlayers := yahoofantasy.ExtractPlayers(rawR)
			if len(rosterPlayers) > 0 {
				for i := range rosterPlayers {
					rosterPlayers[i].LeagueKey = lk
					rosterPlayers[i].OwnershipStatus = "T"
					if rosterPlayers[i].OwnerTeamKey == "" {
						rosterPlayers[i].OwnerTeamKey = tk
					}
				}
				if uErr := upsertYahooPlayerRecords(ctx, db, lk, rosterPlayers, &storedCount); uErr != nil {
					recordErr(uErr)
				} else {
					recordErr(reconcileRemovedTeamRosterPlayers(ctx, c, db, lk, tk, rosterPlayers))
					_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, tk+":roster")
				}
			} else {
				recordErr(upsertLeagueJSON("rosters", tk+":roster", lk, rawR))
			}
		}
	}

	if firstErr != nil {
		_ = db.SaveSyncProgressInScope(resource, scopeKey, "", storedCount)
		if !humanFriendly {
			fmt.Fprintln(syncEvents, syncErrorJSON(resource, "", firstErr))
		}
		return syncResult{Resource: resource, Count: storedCount, Err: fmt.Errorf("fetching %s: %w", resource, firstErr), Duration: time.Since(started)}
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

// migrateLegacyLeaguePlayerRows migrates any legacy batch rows (<leagueKey>:players:fa:*, <leagueKey>:stats:*, <leagueKey>:ownership:*)
// into stable per-player keys (<leagueKey>:player:<playerKey>) first, and removes each legacy batch row only after its records are safely saved.
func migrateLegacyLeaguePlayerRows(ctx context.Context, db *store.Store, leagueKey string) error {
	if leagueKey == "" {
		return nil
	}
	yahooLegacyMigrateMu.Lock()
	defer yahooLegacyMigrateMu.Unlock()

	rows, err := db.DB().QueryContext(ctx,
		`SELECT id, data FROM resources WHERE id LIKE ? OR id LIKE ? OR id LIKE ? ORDER BY synced_at ASC, rowid ASC`,
		leagueKey+":players:fa:%", leagueKey+":stats:%", leagueKey+":ownership:%")
	if err != nil {
		return nil
	}
	type legacyRow struct {
		id   string
		data string
	}
	var legacy []legacyRow
	for rows.Next() {
		var r legacyRow
		if rows.Scan(&r.id, &r.data) == nil {
			legacy = append(legacy, r)
		}
	}
	_ = rows.Close()

	var firstErr error
	for _, r := range legacy {
		extracted := yahoofantasy.ExtractPlayers([]byte(r.data))
		for i := range extracted {
			extracted[i].LeagueKey = leagueKey
		}
		if len(extracted) > 0 {
			if uErr := upsertYahooPlayerRecords(ctx, db, leagueKey, extracted, nil); uErr != nil {
				if firstErr == nil {
					firstErr = uErr
				}
				continue
			}
		}
		_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, r.id)
		_, _ = db.DB().ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, yahooPlayerFTSRowID(r.id))
	}
	return firstErr
}

// reconcileRemovedTeamRosterPlayers checks any saved league player rows that previously belonged to teamKey
// but are absent from the team's freshly synced roster, fetching their current league ownership so dropped
// players become FA/W while traded players update to their new owner team.
func reconcileRemovedTeamRosterPlayers(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, db *store.Store, leagueKey, teamKey string, currentRoster []yahoofantasy.PlayerRecord) error {
	if leagueKey == "" || teamKey == "" {
		return nil
	}
	onRoster := make(map[string]bool, len(currentRoster))
	for _, p := range currentRoster {
		if p.PlayerKey != "" {
			onRoster[p.PlayerKey] = true
		}
	}

	rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE id LIKE ?`, leagueKey+":player:%")
	if err != nil {
		return err
	}
	var removedKeys []string
	removedBase := map[string]yahoofantasy.PlayerRecord{}
	for rows.Next() {
		var data string
		if rows.Scan(&data) != nil {
			continue
		}
		extracted := yahoofantasy.ExtractPlayers([]byte(data))
		if len(extracted) == 0 {
			continue
		}
		p := extracted[0]
		if p.PlayerKey != "" && strings.EqualFold(p.OwnerTeamKey, teamKey) && !onRoster[p.PlayerKey] {
			p.LeagueKey = leagueKey
			p.SelectedPosition = ""
			p.OwnerTeamKey = ""
			p.OwnershipStatus = "FA"
			removedKeys = append(removedKeys, p.PlayerKey)
			removedBase[p.PlayerKey] = p
		}
	}
	_ = rows.Close()

	if len(removedKeys) == 0 {
		return nil
	}

	jsonParams := map[string]string{"format": "json"}
	const batchSize = 25
	var firstErr error
	for start := 0; start < len(removedKeys); start += batchSize {
		end := start + batchSize
		if end > len(removedKeys) {
			end = len(removedKeys)
		}
		batchKeys := removedKeys[start:end]
		ownPath := fmt.Sprintf("/league/%s/players;player_keys=%s/ownership", leagueKey, strings.Join(batchKeys, ","))
		rawOwn, ownErr := c.Get(ctx, ownPath, jsonParams)
		if ownErr != nil {
			if firstErr == nil {
				firstErr = ownErr
			}
			continue
		}
		byKey := make(map[string]yahoofantasy.PlayerRecord, len(batchKeys))
		for _, pk := range batchKeys {
			byKey[pk] = removedBase[pk]
		}
		seenInOwn := make(map[string]bool, len(batchKeys))
		for _, op := range yahoofantasy.ExtractPlayers(rawOwn) {
			if op.PlayerKey == "" {
				continue
			}
			seenInOwn[op.PlayerKey] = true
			cur := byKey[op.PlayerKey]
			cur = mergePlayerRecordRefresh(cur, op, leagueKey)
			byKey[op.PlayerKey] = cur
		}
		updates := make([]yahoofantasy.PlayerRecord, 0, len(batchKeys))
		for _, pk := range batchKeys {
			if rec, ok := byKey[pk]; ok && rec.PlayerKey != "" && seenInOwn[pk] {
				updates = append(updates, rec)
			}
		}
		if uErr := upsertYahooPlayerRecords(ctx, db, leagueKey, updates, nil); uErr != nil && firstErr == nil {
			firstErr = uErr
		}
	}
	return firstErr
}

// syncYahooPlayerStatsAndOwnership fetches season stats and ownership in batches of 25 and stores
// merged player details under stable per-player keys (<league_key>:player:<player_key>).
func syncYahooPlayerStatsAndOwnership(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, db *store.Store, leagueKey string, playerKeys []string, baseByKey map[string]yahoofantasy.PlayerRecord, storedCount *int) error {
	if len(playerKeys) == 0 {
		return nil
	}
	var firstErr error
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	jsonParams := map[string]string{"format": "json"}
	const batchSize = 25
	for start := 0; start < len(playerKeys); start += batchSize {
		end := start + batchSize
		if end > len(playerKeys) {
			end = len(playerKeys)
		}
		batchKeys := playerKeys[start:end]
		joined := strings.Join(batchKeys, ",")
		mergedByKey := make(map[string]yahoofantasy.PlayerRecord, len(batchKeys))
		for _, pk := range batchKeys {
			if base, ok := baseByKey[pk]; ok {
				base.LeagueKey = leagueKey
				mergedByKey[pk] = base
			} else {
				mergedByKey[pk] = yahoofantasy.PlayerRecord{LeagueKey: leagueKey, PlayerKey: pk}
			}
		}

		statsPath := fmt.Sprintf("/league/%s/players;player_keys=%s/stats;type=season;week=1", leagueKey, joined)
		rawStats, statsErr := c.Get(ctx, statsPath, jsonParams)
		if statsErr != nil {
			recordErr(statsErr)
		} else {
			for _, sp := range yahoofantasy.ExtractPlayers(rawStats) {
				if sp.PlayerKey == "" {
					continue
				}
				cur := mergedByKey[sp.PlayerKey]
				cur = mergePlayerRecordRefresh(cur, sp, leagueKey)
				mergedByKey[sp.PlayerKey] = cur
			}
		}

		ownPath := fmt.Sprintf("/league/%s/players;player_keys=%s/ownership", leagueKey, joined)
		rawOwn, ownErr := c.Get(ctx, ownPath, jsonParams)
		if ownErr != nil {
			recordErr(ownErr)
		} else {
			for _, op := range yahoofantasy.ExtractPlayers(rawOwn) {
				if op.PlayerKey == "" {
					continue
				}
				cur := mergedByKey[op.PlayerKey]
				cur = mergePlayerRecordRefresh(cur, op, leagueKey)
				mergedByKey[op.PlayerKey] = cur
			}
		}

		batchRecords := make([]yahoofantasy.PlayerRecord, 0, len(batchKeys))
		for _, pk := range batchKeys {
			if rec, ok := mergedByKey[pk]; ok && rec.PlayerKey != "" {
				batchRecords = append(batchRecords, rec)
			}
		}
		recordErr(upsertYahooPlayerRecords(ctx, db, leagueKey, batchRecords, storedCount))
	}
	return firstErr
}

// upsertYahooPlayerRecords holds a mutex and SQLite transaction across reading, merging, and writing
// each stable player row (<leagueKey>:player:<playerKey>), replacing the full JSON column so cleared
// fields (like OwnerTeamKey and SelectedPosition when a player becomes FA/W) are actually removed.
func upsertYahooPlayerRecords(ctx context.Context, db *store.Store, leagueKey string, players []yahoofantasy.PlayerRecord, storedCount *int) error {
	if len(players) == 0 {
		return nil
	}
	yahooPlayerUpsertMu.Lock()
	defer yahooPlayerUpsertMu.Unlock()

	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, incoming := range players {
		if incoming.PlayerKey == "" {
			continue
		}
		rowID := fmt.Sprintf("%s:player:%s", leagueKey, incoming.PlayerKey)
		legacyID := leagueKey + ":" + incoming.PlayerKey
		merged := incoming
		merged.LeagueKey = leagueKey

		var existingJSON string
		if qErr := tx.QueryRowContext(ctx, `SELECT data FROM resources WHERE id = ?`, rowID).Scan(&existingJSON); qErr == nil && existingJSON != "" {
			if existingPlayers := yahoofantasy.ExtractPlayers([]byte(existingJSON)); len(existingPlayers) > 0 {
				merged = mergePlayerRecordRefresh(existingPlayers[0], incoming, leagueKey)
			}
		} else if qErr := tx.QueryRowContext(ctx, `SELECT data FROM resources WHERE id = ?`, legacyID).Scan(&existingJSON); qErr == nil && existingJSON != "" {
			if existingPlayers := yahoofantasy.ExtractPlayers([]byte(existingJSON)); len(existingPlayers) > 0 {
				merged = mergePlayerRecordRefresh(existingPlayers[0], incoming, leagueKey)
			}
		}

		b, mErr := json.Marshal(merged)
		if mErr != nil {
			return mErr
		}
		stamped := stampLeagueKeyInJSON(b, leagueKey)
		if _, execErr := tx.ExecContext(ctx,
			`INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
			 VALUES (?, 'players', ?, strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
			 ON CONFLICT(resource_type, id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, updated_at = excluded.updated_at`,
			rowID, string(stamped)); execErr != nil {
			return execErr
		}
		ftsID := yahooPlayerFTSRowID(rowID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsID)
		_, _ = tx.ExecContext(ctx,
			`INSERT INTO resources_fts (rowid, id, resource_type, content) VALUES (?, ?, 'players', ?)`,
			ftsID, rowID, yahooPlayerSearchableContent(merged))
		_, _ = tx.ExecContext(ctx, `DELETE FROM resources WHERE id = ?`, legacyID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, yahooPlayerFTSRowID(legacyID))
		if storedCount != nil {
			*storedCount++
		}
	}

	return tx.Commit()
}

func yahooPlayerFTSRowID(id string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("players"))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(id))
	return int64(h.Sum64() & 0x7FFFFFFFFFFFFFFF)
}

func yahooPlayerSearchableContent(p yahoofantasy.PlayerRecord) string {
	var parts []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len(s) >= 2 {
			parts = append(parts, s)
		}
	}
	add(p.LeagueKey)
	add(p.PlayerKey)
	add(p.FullName)
	add(p.DisplayPosition)
	for _, ep := range p.EligiblePositions {
		add(ep)
	}
	add(p.EditorialTeamAbbr)
	add(p.SelectedPosition)
	add(p.Status)
	add(p.OwnershipStatus)
	add(p.OwnerTeamKey)
	return strings.Join(parts, " ")
}

func mergePlayerRecordRefresh(base, update yahoofantasy.PlayerRecord, leagueKey string) yahoofantasy.PlayerRecord {
	out := base
	out.LeagueKey = leagueKey
	if update.PlayerKey != "" {
		out.PlayerKey = update.PlayerKey
	}
	if update.FullName != "" {
		out.FullName = update.FullName
	}
	if update.DisplayPosition != "" {
		out.DisplayPosition = update.DisplayPosition
	}
	if len(update.EligiblePositions) > 0 {
		out.EligiblePositions = update.EligiblePositions
	}
	if update.EditorialTeamAbbr != "" {
		out.EditorialTeamAbbr = update.EditorialTeamAbbr
	}
	if update.SelectedPosition != "" {
		out.SelectedPosition = update.SelectedPosition
	}
	if update.Status != "" {
		out.Status = update.Status
	}
	if update.ByeWeek > 0 {
		out.ByeWeek = update.ByeWeek
	}
	if update.PointsTotal > 0 {
		out.PointsTotal = update.PointsTotal
	}
	if update.GamesPlayed > 0 {
		out.GamesPlayed = update.GamesPlayed
	}
	if update.PointsPerGame > 0 {
		out.PointsPerGame = update.PointsPerGame
	} else if out.PointsTotal > 0 && out.GamesPlayed > 0 {
		out.PointsPerGame = out.PointsTotal / float64(out.GamesPlayed)
	}
	if update.ProjectedPoints > 0 {
		out.ProjectedPoints = update.ProjectedPoints
	}
	if update.PercentOwned > 0 {
		out.PercentOwned = update.PercentOwned
	}
	if update.OwnershipStatus != "" {
		out.OwnershipStatus = update.OwnershipStatus
	}
	if update.OwnerTeamKey != "" && !strings.EqualFold(out.OwnershipStatus, "FA") && !strings.EqualFold(out.OwnershipStatus, "W") {
		if base.OwnerTeamKey != "" && !strings.EqualFold(base.OwnerTeamKey, update.OwnerTeamKey) && update.SelectedPosition == "" {
			out.SelectedPosition = ""
		}
		out.OwnerTeamKey = update.OwnerTeamKey
	} else if strings.EqualFold(out.OwnershipStatus, "FA") || strings.EqualFold(out.OwnershipStatus, "W") {
		out.OwnerTeamKey = ""
		out.SelectedPosition = ""
	}
	return out
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

	// If explicit league or team parameters were supplied, respect that selection without adding unrelated saved leagues.
	if len(leagues) > 0 || len(teams) > 0 {
		if len(teams) == 0 && strings.EqualFold(strings.TrimSpace(resource), "rosters") {
			if rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources ORDER BY synced_at DESC, rowid DESC`); err == nil {
				var blobs []string
				for rows.Next() {
					var s string
					if rows.Scan(&s) == nil {
						blobs = append(blobs, s)
					}
				}
				_ = rows.Close()
				for _, s := range blobs {
					for _, t := range yahoofantasy.ExtractTeams([]byte(s)) {
						for _, lk := range leagues {
							if yahoofantasy.BelongsToLeague(lk, t.LeagueKey, t.TeamKey) {
								addTeam(t.TeamKey)
							}
						}
					}
				}
			}
		}
		return leagues, teams, nil
	}

	if rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources ORDER BY synced_at DESC, rowid DESC`); err == nil {
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

func resolveSyncWeekRange(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, db *store.Store, leagueKey string, maxPages int, latestOnly bool) (int, int, error) {
	if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		return 1, 1, nil
	}
	// Always query the live league endpoint first so --latest-only and full syncs use the live current_week
	// rather than a stale week cached from an earlier sync.
	rawLg, err := c.Get(ctx, fmt.Sprintf("/league/%s", leagueKey), map[string]string{"format": "json"})
	if err != nil {
		return 0, 0, err
	}
	currentWeek := yahoofantasy.ExtractLeagueCurrentWeek(rawLg)
	if currentWeek <= 0 {
		if rows, qErr := db.DB().QueryContext(ctx, `SELECT data FROM resources ORDER BY synced_at DESC, rowid DESC`); qErr == nil {
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
					if cw := yahoofantasy.ExtractLeagueCurrentWeek(b); cw > 0 {
						currentWeek = cw
						break
					}
				}
			}
		}
	}
	if currentWeek <= 0 {
		currentWeek = 1
	}
	startWeek := 1
	if latestOnly {
		startWeek = currentWeek
	} else if maxPages > 0 && currentWeek > maxPages {
		startWeek = currentWeek - maxPages + 1
	}
	return startWeek, currentWeek, nil
}

func stampLeagueKeyInJSON(raw []byte, leagueKey string) []byte {
	s := strings.TrimSpace(string(raw))
	if leagueKey == "" || !strings.HasPrefix(s, "{") || strings.Contains(s, `"league_key"`) {
		return raw
	}
	return []byte(`{"league_key":"` + leagueKey + `",` + s[1:])
}
