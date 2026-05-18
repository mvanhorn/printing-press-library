package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

// videoRow is the minimal shape decoded from the resources blob for novel-feature joins.
type videoRow struct {
	ID           string
	Title        string
	PublishedAt  time.Time
	ChannelID    string
	ChannelTitle string
	Tags         []string
	Description  string
	ViewCount    int64
	LikeCount    int64
	CommentCount int64
	DurationSec  int64
}

// loadVideos reads every cached video resource (resource_type IN ('videos','channels_videos'))
// and decodes the relevant fields. NULL-safe scans.
func loadVideos(db *store.Store, channelID string, limit int) ([]videoRow, error) {
	q := `SELECT id, data FROM resources WHERE resource_type IN ('videos','channels_videos')`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.DB().Query(q)
	if err != nil {
		return nil, fmt.Errorf("query videos: %w", err)
	}
	defer rows.Close()
	var out []videoRow
	for rows.Next() {
		var id string
		var data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		v, ok := decodeVideo(id, data.String)
		if !ok {
			continue
		}
		if channelID != "" && v.ChannelID != channelID {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeVideo(id, raw string) (videoRow, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return videoRow{}, false
	}
	v := videoRow{ID: id}
	var snippet map[string]json.RawMessage
	if r, ok := obj["snippet"]; ok {
		_ = json.Unmarshal(r, &snippet)
	}
	if snippet != nil {
		_ = json.Unmarshal(snippet["title"], &v.Title)
		_ = json.Unmarshal(snippet["channelId"], &v.ChannelID)
		_ = json.Unmarshal(snippet["channelTitle"], &v.ChannelTitle)
		_ = json.Unmarshal(snippet["description"], &v.Description)
		_ = json.Unmarshal(snippet["tags"], &v.Tags)
		var s string
		if err := json.Unmarshal(snippet["publishedAt"], &s); err == nil && s != "" {
			v.PublishedAt, _ = time.Parse(time.RFC3339, s)
		}
	}
	var stats map[string]json.RawMessage
	if r, ok := obj["statistics"]; ok {
		_ = json.Unmarshal(r, &stats)
	}
	if stats != nil {
		v.ViewCount = parseInt64(stats["viewCount"])
		v.LikeCount = parseInt64(stats["likeCount"])
		v.CommentCount = parseInt64(stats["commentCount"])
	}
	var details map[string]json.RawMessage
	if r, ok := obj["contentDetails"]; ok {
		_ = json.Unmarshal(r, &details)
	}
	if details != nil {
		var dur string
		_ = json.Unmarshal(details["duration"], &dur)
		v.DurationSec = parseISO8601Duration(dur)
	}
	return v, true
}

func parseInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var x int64
		_, _ = fmt.Sscanf(s, "%d", &x)
		return x
	}
	return 0
}

// parseISO8601Duration handles PT#H#M#S — good enough for video lengths.
func parseISO8601Duration(s string) int64 {
	s = strings.TrimPrefix(s, "PT")
	if s == "" {
		return 0
	}
	var total int64
	cur := ""
	for _, r := range s {
		switch r {
		case 'H':
			var n int64
			_, _ = fmt.Sscanf(cur, "%d", &n)
			total += n * 3600
			cur = ""
		case 'M':
			var n int64
			_, _ = fmt.Sscanf(cur, "%d", &n)
			total += n * 60
			cur = ""
		case 'S':
			var n int64
			_, _ = fmt.Sscanf(cur, "%d", &n)
			total += n
			cur = ""
		default:
			cur += string(r)
		}
	}
	return total
}

// resolveChannel returns the channel id to operate on: explicit --channel, or
// the locally-cached "mine" channel id if available.
func resolveChannel(db *store.Store, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	row := db.DB().QueryRow(`SELECT id FROM resources WHERE resource_type='channels' ORDER BY synced_at DESC LIMIT 1`)
	var id string
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("no channel cached; pass --channel <id> or run sync first")
	}
	return id, nil
}
