package cli

import (
	"context"
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
		blobs = append(blobs, []byte(r.data))
	}
	return blobs, true, nil
}

func extractAllPlayersFromBlobs(blobs [][]byte) []yahoofantasy.PlayerRecord {
	var all []yahoofantasy.PlayerRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, p := range yahoofantasy.ExtractPlayers(b) {
			if idx, ok := seen[p.PlayerKey]; ok {
				if p.PointsTotal > all[idx].PointsTotal {
					all[idx].PointsTotal = p.PointsTotal
				}
				if p.PointsPerGame > all[idx].PointsPerGame {
					all[idx].PointsPerGame = p.PointsPerGame
				}
				if p.SelectedPosition != "" {
					all[idx].SelectedPosition = p.SelectedPosition
				}
				if p.Status != "" {
					all[idx].Status = p.Status
				}
				if p.OwnerTeamKey != "" {
					all[idx].OwnerTeamKey = p.OwnerTeamKey
				}
			} else {
				seen[p.PlayerKey] = len(all)
				all = append(all, p)
			}
		}
	}
	return all
}

func extractAllTeamsFromBlobs(blobs [][]byte) []yahoofantasy.TeamRecord {
	var all []yahoofantasy.TeamRecord
	seen := map[string]int{}
	for _, b := range blobs {
		for _, t := range yahoofantasy.ExtractTeams(b) {
			if idx, ok := seen[t.TeamKey]; ok {
				if t.PointsFor > all[idx].PointsFor {
					all[idx].PointsFor = t.PointsFor
				}
				if t.Wins > 0 || t.Losses > 0 {
					all[idx].Wins = t.Wins
					all[idx].Losses = t.Losses
					all[idx].Ties = t.Ties
				}
				if t.HasFaab || t.FaabBalance > 0 {
					all[idx].FaabBalance = t.FaabBalance
					all[idx].HasFaab = true
				}
			} else {
				seen[t.TeamKey] = len(all)
				all = append(all, t)
			}
		}
	}
	return all
}
