// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/rocket-league-tracker/internal/store"
)

// platformAliases canonicalizes platform tokens entered as --platform flags.
// The collector imports always pass the lowercase canonical token.
var platformAliases = map[string]string{
	"epic": "epic", "epicgames": "epic", "epic-games": "epic",
	"steam": "steam",
	"psn":   "psn", "playstation": "psn", "ps": "psn", "ps4": "psn", "ps5": "psn",
	"xbox": "xbox", "xboxlive": "xbox", "xbl": "xbox", "xbox-live": "xbox",
	"switch": "switch", "nintendo": "switch",
}

// normalizePlatform returns the canonical platform token, or "" if unknown.
func normalizePlatform(in string) string {
	if v, ok := platformAliases[strings.ToLower(strings.TrimSpace(in))]; ok {
		return v
	}
	return ""
}

// storageKey concatenates platform + player id into a stable lookup key.
// Mirrors the rocket-league-stats web app's player-identity convention.
func storageKey(platform, playerID string) string {
	return strings.ToLower(strings.TrimSpace(platform)) + ":" + strings.TrimSpace(playerID)
}

// playlistAlias is the user-friendly playlist name in --playlist flags. The
// API and collector both speak the same set of canonical strings (1v1, 2v2,
// 3v3, hoops, rumble, dropshot, snowday, tournament).
var validPlaylists = map[string]bool{
	"1v1": true, "2v2": true, "3v3": true,
	"hoops": true, "rumble": true, "dropshot": true,
	"snowday": true, "tournament": true,
}

// validatePlaylist returns nil if k is one of the eight competitive playlists.
func validatePlaylist(k string) error {
	if validPlaylists[strings.ToLower(strings.TrimSpace(k))] {
		return nil
	}
	keys := make([]string, 0, len(validPlaylists))
	for p := range validPlaylists {
		keys = append(keys, p)
	}
	sort.Strings(keys)
	return fmt.Errorf("--playlist must be one of: %s", strings.Join(keys, ", "))
}

// rankSnapshot is the parsed shape of a single rank-row JSON blob.
// Field names tolerate the most common API spellings (mmr/rating, tier/tierName,
// games/matchesPlayed, streak/winStreak). Unknown fields are kept in Raw.
type rankSnapshot struct {
	Playlist   string          `json:"playlist"`
	Tier       string          `json:"tier"`
	Division   string          `json:"division"`
	MMR        int             `json:"mmr"`
	Games      int             `json:"games"`
	Streak     int             `json:"streak"`
	CapturedAt time.Time       `json:"captured_at"`
	Raw        json.RawMessage `json:"-"`
}

// parsePlaylistEntry pulls common rank fields out of a single playlist
// object. The API is RapidAPI's "Rocket League" provider whose exact
// response shape is undocumented in the absorb manifest, so we fall back
// across the most plausible field names.
func parsePlaylistEntry(raw json.RawMessage) rankSnapshot {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return rankSnapshot{Raw: raw}
	}
	snap := rankSnapshot{Raw: raw}
	snap.Playlist = firstStr(obj, "playlist", "name", "key", "playlist_name", "mode")
	snap.Tier = firstStr(obj, "tier", "tierName", "tier_name", "rank")
	snap.Division = firstStr(obj, "division", "div")
	snap.MMR = firstInt(obj, "mmr", "rating", "skill", "score")
	snap.Games = firstInt(obj, "games", "matchesPlayed", "matches", "matchesplayed")
	snap.Streak = firstInt(obj, "streak", "winStreak", "win_streak")
	if ts := firstStr(obj, "captured_at", "capturedAt", "updated_at", "updatedAt"); ts != "" {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			snap.CapturedAt = t
		}
	}
	return snap
}

func firstStr(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func firstInt(obj map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case json.Number:
				i, err := t.Int64()
				if err == nil {
					return int(i)
				}
			case string:
				var n int
				if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// extractRankPlaylists pulls the per-playlist entries out of a /ranks/{playerId}
// API response. The API returns a JSON envelope already unwrapped by
// extractResponseData; the inner shape is either a JSON array of playlist
// objects or a JSON object with a "playlists" / "ranks" / "tiers" array.
func extractRankPlaylists(data json.RawMessage) []rankSnapshot {
	if len(data) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		out := make([]rankSnapshot, 0, len(arr))
		for _, it := range arr {
			snap := parsePlaylistEntry(it)
			if snap.Playlist != "" {
				out = append(out, snap)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, key := range []string{"playlists", "ranks", "tiers", "data", "results", "items"} {
			if raw, ok := obj[key]; ok {
				var inner []json.RawMessage
				if json.Unmarshal(raw, &inner) == nil {
					out := make([]rankSnapshot, 0, len(inner))
					for _, it := range inner {
						snap := parsePlaylistEntry(it)
						if snap.Playlist != "" {
							out = append(out, snap)
						}
					}
					if len(out) > 0 {
						return out
					}
				}
			}
		}
		// Fallback: treat the object itself as a single playlist entry.
		if snap := parsePlaylistEntry(data); snap.Playlist != "" {
			return []rankSnapshot{snap}
		}
	}
	return nil
}

// rankSnapshotRow is one row of the local `rank` table parsed for analytics.
type rankSnapshotRow struct {
	ID       string
	PlayerID string
	SyncedAt time.Time
	Data     json.RawMessage
}

// queryRecentRankRows reads up to `limit` rows from the local `rank` table for
// the given player_id, ordered by synced_at DESC. Empty `playerID` matches all.
func queryRecentRankRows(ctx context.Context, db *store.Store, playerID string, limit int) ([]rankSnapshotRow, error) {
	if db == nil {
		return nil, fmt.Errorf("local store not open")
	}
	q := `SELECT id, COALESCE(player_id, ''), synced_at, data
	      FROM rank
	      WHERE (? = '' OR player_id = ?)
	      ORDER BY synced_at DESC
	      LIMIT ?`
	if limit <= 0 {
		limit = 1000
	}
	rows, err := db.DB().QueryContext(ctx, q, playerID, playerID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying rank rows: %w", err)
	}
	defer rows.Close()
	var out []rankSnapshotRow
	for rows.Next() {
		var r rankSnapshotRow
		var ts sql.NullString
		var data sql.NullString
		if err := rows.Scan(&r.ID, &r.PlayerID, &ts, &data); err != nil {
			return nil, fmt.Errorf("scan rank row: %w", err)
		}
		if ts.Valid {
			r.SyncedAt = parseAnyTime(ts.String)
		}
		if data.Valid {
			r.Data = json.RawMessage(data.String)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// queryRankRowsSince reads rank rows for player_id whose synced_at is on or
// after `since`. Ordered DESC.
func queryRankRowsSince(ctx context.Context, db *store.Store, playerID string, since time.Time) ([]rankSnapshotRow, error) {
	if db == nil {
		return nil, fmt.Errorf("local store not open")
	}
	q := `SELECT id, COALESCE(player_id, ''), synced_at, data
	      FROM rank
	      WHERE (? = '' OR player_id = ?)
	        AND synced_at >= ?
	      ORDER BY synced_at DESC`
	rows, err := db.DB().QueryContext(ctx, q, playerID, playerID, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("querying rank rows: %w", err)
	}
	defer rows.Close()
	var out []rankSnapshotRow
	for rows.Next() {
		var r rankSnapshotRow
		var ts sql.NullString
		var data sql.NullString
		if err := rows.Scan(&r.ID, &r.PlayerID, &ts, &data); err != nil {
			return nil, fmt.Errorf("scan rank row: %w", err)
		}
		if ts.Valid {
			r.SyncedAt = parseAnyTime(ts.String)
		}
		if data.Valid {
			r.Data = json.RawMessage(data.String)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseAnyTime parses common SQLite datetime formats (RFC3339 and
// 'YYYY-MM-DD HH:MM:SS') into a Time. Returns the zero value on failure.
func parseAnyTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// flattenRankRows turns a list of rank rows into a flat per-row playlist
// snapshot list, keyed by (synced_at, playlist).
func flattenRankRows(rows []rankSnapshotRow) []rankSnapshot {
	var out []rankSnapshot
	for _, r := range rows {
		for _, snap := range extractRankPlaylists(r.Data) {
			if snap.CapturedAt.IsZero() {
				snap.CapturedAt = r.SyncedAt
			}
			out = append(out, snap)
		}
	}
	return out
}

// pathEscapePlayer URL-escapes a player ID for use in API paths. Mirrors the
// pattern used by every absorbed-API command in this CLI.
func pathEscapePlayer(id string) string {
	return url.PathEscape(id)
}
