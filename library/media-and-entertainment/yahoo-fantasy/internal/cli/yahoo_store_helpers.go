package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/store"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func hasLiveAuthConfigured(flags *rootFlags) bool {
	c, err := flags.newClient()
	if err != nil || c == nil || c.Config == nil {
		return false
	}
	if strings.TrimSpace(c.Config.AuthHeader()) != "" {
		return true
	}
	cid := strings.TrimSpace(os.Getenv("YAHOO_CLIENT_ID"))
	csec := strings.TrimSpace(os.Getenv("YAHOO_CLIENT_SECRET"))
	rtok := strings.TrimSpace(os.Getenv("YAHOO_REFRESH_TOKEN"))
	return cid != "" && csec != "" && rtok != ""
}

// loadLocalResourceBlobs reads raw JSON rows from the local SQLite resources table using the drain-first pattern,
// ordered from newest to oldest synced_at so the most recently synced record always takes precedence.
func loadLocalResourceBlobs(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string, resourceTypes ...string) ([][]byte, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = db.Close() }()

	if !hintIfUnsynced(cmd, db, "") {
		hintIfStale(cmd, db, "", flags.maxAge)
	}

	query := `SELECT id, resource_type, data FROM resources ORDER BY synced_at DESC, rowid DESC`
	rows, err := db.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, true, err
	}

	type rawEntry struct {
		id           string
		resourceType string
		data         string
	}
	rawList := make([]rawEntry, 0)
	for rows.Next() {
		var r rawEntry
		if scanErr := rows.Scan(&r.id, &r.resourceType, &r.data); scanErr != nil {
			_ = rows.Close()
			return nil, true, scanErr
		}
		rawList = append(rawList, r)
	}
	if iterErr := rows.Err(); iterErr != nil {
		_ = rows.Close()
		return nil, true, iterErr
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, true, closeErr
	}

	allow := make(map[string]bool, len(resourceTypes))
	for _, rt := range resourceTypes {
		allow[strings.ToLower(strings.TrimSpace(rt))] = true
	}

	blobs := make([][]byte, 0, len(rawList))
	for _, r := range rawList {
		if len(allow) > 0 {
			rt := strings.ToLower(strings.TrimSpace(r.resourceType))
			matched := allow[rt]
			if !matched {
				for k := range allow {
					if strings.Contains(rt, k) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}
		dataStr := strings.TrimSpace(r.data)
		if idLK := yahoofantasy.YahooLeagueKeyFromEntityKey(r.id); idLK != "" && strings.HasPrefix(dataStr, "{") && !strings.Contains(dataStr, `"league_key"`) {
			dataStr = `{"league_key":"` + idLK + `",` + dataStr[1:]
		}
		blobs = append(blobs, []byte(dataStr))
	}
	return blobs, true, nil
}

func extractAllPlayersFromBlobs(blobs [][]byte) []yahoofantasy.PlayerRecord {
	return extractLeaguePlayersFromBlobs(blobs, "")
}

// extractLeaguePlayersFromBlobs deduplicates player records across blobs ordered newest-first.
// The newest blob's values win; older blobs only backfill missing fields.
func extractLeaguePlayersFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.PlayerRecord {
	var all []yahoofantasy.PlayerRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, p := range yahoofantasy.ExtractPlayers(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, p.LeagueKey, p.OwnerTeamKey) {
				continue
			}
			key := p.LeagueKey + "|" + p.PlayerKey
			if idx, ok := seen[key]; ok {
				if all[idx].LeagueKey == "" && p.LeagueKey != "" {
					all[idx].LeagueKey = p.LeagueKey
				}
				if all[idx].FullName == "" && p.FullName != "" {
					all[idx].FullName = p.FullName
				}
				if all[idx].DisplayPosition == "" && p.DisplayPosition != "" {
					all[idx].DisplayPosition = p.DisplayPosition
				}
				if len(all[idx].EligiblePositions) == 0 && len(p.EligiblePositions) > 0 {
					all[idx].EligiblePositions = p.EligiblePositions
				}
				if all[idx].PointsTotal == 0 && p.PointsTotal > 0 {
					all[idx].PointsTotal = p.PointsTotal
				}
				if all[idx].PointsPerGame == 0 && p.PointsPerGame > 0 {
					all[idx].PointsPerGame = p.PointsPerGame
				}
				if all[idx].GamesPlayed == 0 && p.GamesPlayed > 0 {
					all[idx].GamesPlayed = p.GamesPlayed
				}
				if all[idx].PointsPerGame == 0 && all[idx].PointsTotal > 0 && all[idx].GamesPlayed > 0 {
					all[idx].PointsPerGame = all[idx].PointsTotal / float64(all[idx].GamesPlayed)
				}
				if all[idx].ProjectedPoints == 0 && p.ProjectedPoints > 0 {
					all[idx].ProjectedPoints = p.ProjectedPoints
				}
				if all[idx].PercentOwned == 0 && p.PercentOwned > 0 {
					all[idx].PercentOwned = p.PercentOwned
				}
				isAvailable := strings.EqualFold(all[idx].OwnershipStatus, "FA") || strings.EqualFold(all[idx].OwnershipStatus, "W")
				sameOrUnsetOwner := all[idx].OwnerTeamKey == "" || p.OwnerTeamKey == "" || strings.EqualFold(all[idx].OwnerTeamKey, p.OwnerTeamKey)
				if !isAvailable && sameOrUnsetOwner && all[idx].SelectedPosition == "" && p.SelectedPosition != "" {
					all[idx].SelectedPosition = p.SelectedPosition
				}
				if all[idx].Status == "" && p.Status != "" {
					all[idx].Status = p.Status
				}
				if all[idx].OwnershipStatus == "" && p.OwnershipStatus != "" {
					all[idx].OwnershipStatus = p.OwnershipStatus
				}
				if !isAvailable && all[idx].OwnerTeamKey == "" && p.OwnerTeamKey != "" {
					all[idx].OwnerTeamKey = p.OwnerTeamKey
				}
			} else {
				seen[key] = len(all)
				all = append(all, p)
			}
		}
	}
	return all
}

func extractAllTeamsFromBlobs(blobs [][]byte) []yahoofantasy.TeamRecord {
	return extractLeagueTeamsFromBlobs(blobs, "")
}

// extractLeagueTeamsFromBlobs deduplicates team records across blobs ordered newest-first.
// The newest record wins for each field; older or complementary blobs (e.g. teams vs standings) only fill unset fields.
func extractLeagueTeamsFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.TeamRecord {
	var all []yahoofantasy.TeamRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, t := range yahoofantasy.ExtractTeams(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, t.LeagueKey, t.TeamKey) {
				continue
			}
			if idx, ok := seen[t.TeamKey]; ok {
				if all[idx].LeagueKey == "" && t.LeagueKey != "" {
					all[idx].LeagueKey = t.LeagueKey
				}
				if all[idx].Name == "" && t.Name != "" {
					all[idx].Name = t.Name
				}
				if all[idx].PointsFor == 0 && t.PointsFor > 0 {
					all[idx].PointsFor = t.PointsFor
				}
				if all[idx].ProjectedPoints == 0 && t.ProjectedPoints > 0 {
					all[idx].ProjectedPoints = t.ProjectedPoints
				}
				if all[idx].WinProbability == 0 && t.WinProbability > 0 {
					all[idx].WinProbability = t.WinProbability
				}
				if all[idx].Wins == 0 && all[idx].Losses == 0 && all[idx].Ties == 0 && (t.Wins > 0 || t.Losses > 0 || t.Ties > 0) {
					all[idx].Wins = t.Wins
					all[idx].Losses = t.Losses
					all[idx].Ties = t.Ties
				}
				if !all[idx].HasFaab && all[idx].FaabBalance == 0 && (t.HasFaab || t.FaabBalance > 0) {
					all[idx].FaabBalance = t.FaabBalance
					all[idx].HasFaab = true
				}
				if len(all[idx].Roster) == 0 && len(t.Roster) > 0 {
					all[idx].Roster = t.Roster
				}
			} else {
				seen[t.TeamKey] = len(all)
				all = append(all, t)
			}
		}
	}
	return all
}

func extractLeagueDraftPicksFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.DraftPickRecord {
	var all []yahoofantasy.DraftPickRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, pk := range yahoofantasy.ExtractDraftPicks(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, pk.LeagueKey, pk.TeamKey) {
				continue
			}
			key := fmt.Sprintf("%s|%d|%d|%s", pk.LeagueKey, pk.Round, pk.Pick, pk.PlayerKey)
			if idx, ok := seen[key]; ok {
				if all[idx].LeagueKey == "" && pk.LeagueKey != "" {
					all[idx].LeagueKey = pk.LeagueKey
				}
				if all[idx].TeamKey == "" && pk.TeamKey != "" {
					all[idx].TeamKey = pk.TeamKey
				}
				if all[idx].Cost == 0 && pk.Cost > 0 {
					all[idx].Cost = pk.Cost
				}
			} else {
				seen[key] = len(all)
				all = append(all, pk)
			}
		}
	}
	return all
}

func extractLeagueTransactionsFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.TransactionRecord {
	var all []yahoofantasy.TransactionRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, tx := range yahoofantasy.ExtractTransactions(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, tx.LeagueKey, tx.TransactionKey, tx.TeamKey) {
				continue
			}
			key := tx.TransactionKey
			if key == "" {
				key = fmt.Sprintf("%s|%s|%s|%s|%d|%d", tx.LeagueKey, tx.TeamKey, tx.AddPlayerKey, tx.DropPlayerKey, tx.FaabBid, tx.Timestamp)
			}
			if idx, ok := seen[key]; ok {
				if all[idx].LeagueKey == "" && tx.LeagueKey != "" {
					all[idx].LeagueKey = tx.LeagueKey
				}
				if all[idx].TeamKey == "" && tx.TeamKey != "" {
					all[idx].TeamKey = tx.TeamKey
				}
				if all[idx].AddPlayerName == "" && tx.AddPlayerName != "" {
					all[idx].AddPlayerName = tx.AddPlayerName
				}
				if all[idx].AddPlayerPos == "" && tx.AddPlayerPos != "" {
					all[idx].AddPlayerPos = tx.AddPlayerPos
				}
			} else {
				seen[key] = len(all)
				all = append(all, tx)
			}
		}
	}
	return all
}

// extractLeagueMatchupsFromBlobs selects the newest saved record for each weekly matchup
// (since blobs are ordered by synced_at DESC) without combining field maxima across stale copies.
func extractLeagueMatchupsFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.MatchupRecord {
	var all []yahoofantasy.MatchupRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, m := range yahoofantasy.ExtractMatchups(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, m.LeagueKey, m.TeamKey, m.OpponentKey) {
				continue
			}
			key := fmt.Sprintf("%d|%s|%s", m.Week, m.TeamKey, m.OpponentKey)
			if idx, ok := seen[key]; ok {
				if all[idx].LeagueKey == "" && m.LeagueKey != "" {
					all[idx].LeagueKey = m.LeagueKey
				}
				if all[idx].TeamName == "" && m.TeamName != "" {
					all[idx].TeamName = m.TeamName
				}
				if all[idx].OpponentName == "" && m.OpponentName != "" {
					all[idx].OpponentName = m.OpponentName
				}
				if len(all[idx].TeamStarters) == 0 && len(m.TeamStarters) > 0 {
					all[idx].TeamStarters = m.TeamStarters
				}
				if len(all[idx].OpponentStarters) == 0 && len(m.OpponentStarters) > 0 {
					all[idx].OpponentStarters = m.OpponentStarters
				}
			} else {
				seen[key] = len(all)
				all = append(all, m)
			}
		}
	}
	return all
}
