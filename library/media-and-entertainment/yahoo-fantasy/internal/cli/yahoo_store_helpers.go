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

// loadLocalResourceBlobs reads raw JSON rows from the local SQLite resources table using the drain-first pattern.
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

	query := `SELECT id, resource_type, data FROM resources`
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
				if p.LeagueKey != "" {
					all[idx].LeagueKey = p.LeagueKey
				}
				if p.FullName != "" {
					all[idx].FullName = p.FullName
				}
				if p.DisplayPosition != "" {
					all[idx].DisplayPosition = p.DisplayPosition
				}
				if len(p.EligiblePositions) > 0 && len(all[idx].EligiblePositions) == 0 {
					all[idx].EligiblePositions = p.EligiblePositions
				}
				if p.PointsTotal > all[idx].PointsTotal {
					all[idx].PointsTotal = p.PointsTotal
				}
				if p.PointsPerGame > all[idx].PointsPerGame {
					all[idx].PointsPerGame = p.PointsPerGame
				}
				if p.GamesPlayed > all[idx].GamesPlayed {
					all[idx].GamesPlayed = p.GamesPlayed
				}
				if all[idx].PointsPerGame == 0 && all[idx].PointsTotal > 0 && all[idx].GamesPlayed > 0 {
					all[idx].PointsPerGame = all[idx].PointsTotal / float64(all[idx].GamesPlayed)
				}
				if p.ProjectedPoints > all[idx].ProjectedPoints {
					all[idx].ProjectedPoints = p.ProjectedPoints
				}
				if p.PercentOwned > all[idx].PercentOwned {
					all[idx].PercentOwned = p.PercentOwned
				}
				if p.SelectedPosition != "" {
					all[idx].SelectedPosition = p.SelectedPosition
				}
				if p.Status != "" {
					all[idx].Status = p.Status
				}
				if p.OwnershipStatus != "" {
					all[idx].OwnershipStatus = p.OwnershipStatus
				}
				if p.OwnerTeamKey != "" {
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

func extractLeagueTeamsFromBlobs(blobs [][]byte, leagueKey string) []yahoofantasy.TeamRecord {
	var all []yahoofantasy.TeamRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, t := range yahoofantasy.ExtractTeams(b) {
			if leagueKey != "" && !yahoofantasy.BelongsToLeague(leagueKey, t.LeagueKey, t.TeamKey) {
				continue
			}
			if idx, ok := seen[t.TeamKey]; ok {
				if t.LeagueKey != "" {
					all[idx].LeagueKey = t.LeagueKey
				}
				if t.Name != "" {
					all[idx].Name = t.Name
				}
				if t.PointsFor > all[idx].PointsFor {
					all[idx].PointsFor = t.PointsFor
				}
				if t.ProjectedPoints > all[idx].ProjectedPoints {
					all[idx].ProjectedPoints = t.ProjectedPoints
				}
				if t.WinProbability > all[idx].WinProbability {
					all[idx].WinProbability = t.WinProbability
				}
				if t.Wins > 0 || t.Losses > 0 || t.Ties > 0 {
					all[idx].Wins = t.Wins
					all[idx].Losses = t.Losses
					all[idx].Ties = t.Ties
				}
				if t.HasFaab || t.FaabBalance > 0 {
					all[idx].FaabBalance = t.FaabBalance
					all[idx].HasFaab = true
				}
				if len(t.Roster) > 0 {
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
				if pk.LeagueKey != "" {
					all[idx].LeagueKey = pk.LeagueKey
				}
				if pk.TeamKey != "" {
					all[idx].TeamKey = pk.TeamKey
				}
				if pk.Cost > all[idx].Cost {
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
				if tx.LeagueKey != "" {
					all[idx].LeagueKey = tx.LeagueKey
				}
				if tx.TeamKey != "" {
					all[idx].TeamKey = tx.TeamKey
				}
				if tx.AddPlayerName != "" {
					all[idx].AddPlayerName = tx.AddPlayerName
				}
				if tx.AddPlayerPos != "" {
					all[idx].AddPlayerPos = tx.AddPlayerPos
				}
				if tx.FaabBid > all[idx].FaabBid {
					all[idx].FaabBid = tx.FaabBid
				}
			} else {
				seen[key] = len(all)
				all = append(all, tx)
			}
		}
	}
	return all
}

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
				if m.LeagueKey != "" {
					all[idx].LeagueKey = m.LeagueKey
				}
				if m.TeamName != "" {
					all[idx].TeamName = m.TeamName
				}
				if m.OpponentName != "" {
					all[idx].OpponentName = m.OpponentName
				}
				if m.TeamPoints > all[idx].TeamPoints {
					all[idx].TeamPoints = m.TeamPoints
				}
				if m.TeamProjected > all[idx].TeamProjected {
					all[idx].TeamProjected = m.TeamProjected
				}
				if m.OpponentPoints > all[idx].OpponentPoints {
					all[idx].OpponentPoints = m.OpponentPoints
				}
				if m.OpponentProjected > all[idx].OpponentProjected {
					all[idx].OpponentProjected = m.OpponentProjected
				}
				if m.TeamWinProb > all[idx].TeamWinProb {
					all[idx].TeamWinProb = m.TeamWinProb
				}
				if m.OpponentWinProb > all[idx].OpponentWinProb {
					all[idx].OpponentWinProb = m.OpponentWinProb
				}
				if len(m.TeamStarters) > len(all[idx].TeamStarters) {
					all[idx].TeamStarters = m.TeamStarters
				}
				if len(m.OpponentStarters) > len(all[idx].OpponentStarters) {
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
