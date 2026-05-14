package ytstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Channel represents a stored YouTube channel (own or watchlist).
type Channel struct {
	ID           string `json:"id"`
	Handle       string `json:"handle,omitempty"`
	Title        string `json:"title,omitempty"`
	Kind         string `json:"kind"`
	LastSyncedAt string `json:"last_synced_at,omitempty"`
}

// Video represents a stored YouTube video.
type Video struct {
	ID          string `json:"id"`
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	DurationS   int    `json:"duration_s,omitempty"`
}

// RetentionCurve is a 100-bucket curve with its recording timestamp.
type RetentionCurve struct {
	VideoID    string    `json:"video_id"`
	RecordedAt string    `json:"recorded_at"`
	Points     []float64 `json:"points"`
}

// VideoMetricsDay holds one day of metrics for a video.
type VideoMetricsDay struct {
	VideoID     string  `json:"video_id"`
	Day         string  `json:"day"`
	Views       int64   `json:"views"`
	Likes       int64   `json:"likes"`
	Comments    int64   `json:"comments"`
	WatchTimeS  int64   `json:"watch_time_s"`
	CTR         float64 `json:"ctr"`
	AvgViewPct  float64 `json:"avg_view_pct"`
	Impressions int64   `json:"impressions"`
}

// UpsertChannel inserts or updates a channel row.
func UpsertChannel(ctx context.Context, db *sql.DB, c Channel) error {
	if c.Kind == "" {
		c.Kind = "own"
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_channels(channel_id, handle, title, kind, last_synced_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(channel_id) DO UPDATE SET
			handle=excluded.handle,
			title=excluded.title,
			kind=COALESCE(NULLIF(excluded.kind,''), kind),
			last_synced_at=excluded.last_synced_at
	`, c.ID, c.Handle, c.Title, c.Kind, c.LastSyncedAt)
	return err
}

// UpsertVideo inserts or updates a video row.
func UpsertVideo(ctx context.Context, db *sql.DB, v Video) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_videos(video_id, channel_id, title, description, published_at, duration_s)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			channel_id=excluded.channel_id,
			title=excluded.title,
			description=excluded.description,
			published_at=excluded.published_at,
			duration_s=excluded.duration_s,
			synced_at=CURRENT_TIMESTAMP
	`, v.ID, v.ChannelID, v.Title, v.Description, v.PublishedAt, v.DurationS)
	return err
}

// SaveRetentionCurve stores a 100-bucket retention curve.
func SaveRetentionCurve(ctx context.Context, db *sql.DB, videoID string, points []float64) error {
	pts, err := json.Marshal(points)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO yt_retention_curves(video_id, recorded_at, points)
		VALUES (?, ?, ?)
		ON CONFLICT(video_id, recorded_at) DO UPDATE SET points=excluded.points
	`, videoID, time.Now().UTC().Format(time.RFC3339), string(pts))
	return err
}

// LatestRetentionCurve returns the most recent retention curve for a video.
func LatestRetentionCurve(ctx context.Context, db *sql.DB, videoID string) (*RetentionCurve, error) {
	row := db.QueryRowContext(ctx, `
		SELECT video_id, recorded_at, points
		FROM yt_retention_curves
		WHERE video_id = ?
		ORDER BY recorded_at DESC
		LIMIT 1
	`, videoID)
	var rc RetentionCurve
	var ptsRaw string
	if err := row.Scan(&rc.VideoID, &rc.RecordedAt, &ptsRaw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(ptsRaw), &rc.Points); err != nil {
		return nil, fmt.Errorf("decoding retention points: %w", err)
	}
	return &rc, nil
}

// VideosMatchingPattern returns video IDs whose titles match a SQL LIKE pattern
// or — when the input looks like a regex — uses Go regexp filtering after fetch.
func VideosMatchingTitleLike(ctx context.Context, db *sql.DB, like string, channelKind string) ([]Video, error) {
	q := `SELECT v.video_id, v.channel_id, v.title, COALESCE(v.description,''), COALESCE(v.published_at,'')
		FROM yt_videos v
		JOIN yt_channels c ON c.channel_id = v.channel_id
		WHERE v.title LIKE ?`
	args := []any{like}
	if channelKind != "" {
		q += ` AND c.kind = ?`
		args = append(args, channelKind)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.ChannelID, &v.Title, &v.Description, &v.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SaveDailyMetrics upserts a single day of metrics.
func SaveDailyMetrics(ctx context.Context, db *sql.DB, m VideoMetricsDay) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_video_metrics_daily(video_id, day, views, likes, comments, watch_time_s, ctr, avg_view_pct, impressions)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(video_id, day) DO UPDATE SET
			views=excluded.views,
			likes=excluded.likes,
			comments=excluded.comments,
			watch_time_s=excluded.watch_time_s,
			ctr=excluded.ctr,
			avg_view_pct=excluded.avg_view_pct,
			impressions=excluded.impressions
	`, m.VideoID, m.Day, m.Views, m.Likes, m.Comments, m.WatchTimeS, m.CTR, m.AvgViewPct, m.Impressions)
	return err
}

// MetricsRange returns metrics for a video within [from, to] inclusive (YYYY-MM-DD).
func MetricsRange(ctx context.Context, db *sql.DB, videoID, from, to string) ([]VideoMetricsDay, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT video_id, day, COALESCE(views,0), COALESCE(likes,0), COALESCE(comments,0),
		       COALESCE(watch_time_s,0), COALESCE(ctr,0), COALESCE(avg_view_pct,0), COALESCE(impressions,0)
		FROM yt_video_metrics_daily
		WHERE video_id = ? AND day BETWEEN ? AND ?
		ORDER BY day
	`, videoID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VideoMetricsDay
	for rows.Next() {
		var m VideoMetricsDay
		if err := rows.Scan(&m.VideoID, &m.Day, &m.Views, &m.Likes, &m.Comments, &m.WatchTimeS, &m.CTR, &m.AvgViewPct, &m.Impressions); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LinkScript binds a script path to a video.
func LinkScript(ctx context.Context, db *sql.DB, videoID, scriptPath, signal, beliefShift, cta string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_script_videos(video_id, script_path, signal_line, belief_shift_line, cta_line)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET
			script_path=excluded.script_path,
			signal_line=excluded.signal_line,
			belief_shift_line=excluded.belief_shift_line,
			cta_line=excluded.cta_line,
			linked_at=CURRENT_TIMESTAMP
	`, videoID, scriptPath, signal, beliefShift, cta)
	return err
}

// ScriptBinding represents a video<->script binding.
type ScriptBinding struct {
	VideoID         string `json:"video_id"`
	ScriptPath      string `json:"script_path"`
	SignalLine      string `json:"signal_line,omitempty"`
	BeliefShiftLine string `json:"belief_shift_line,omitempty"`
	CTALine         string `json:"cta_line,omitempty"`
	LinkedAt        string `json:"linked_at"`
}

// GetScriptBinding returns the script binding for a video, or sql.ErrNoRows.
func GetScriptBinding(ctx context.Context, db *sql.DB, videoID string) (*ScriptBinding, error) {
	row := db.QueryRowContext(ctx, `
		SELECT video_id, script_path, COALESCE(signal_line,''), COALESCE(belief_shift_line,''), COALESCE(cta_line,''), COALESCE(linked_at,'')
		FROM yt_script_videos WHERE video_id = ?`, videoID)
	var b ScriptBinding
	if err := row.Scan(&b.VideoID, &b.ScriptPath, &b.SignalLine, &b.BeliefShiftLine, &b.CTALine, &b.LinkedAt); err != nil {
		return nil, err
	}
	return &b, nil
}

// AddToWatchlist inserts a channel into the watchlist.
func AddToWatchlist(ctx context.Context, db *sql.DB, channelID, handle, title, niche string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_watchlist(channel_id, handle, title, niche)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(channel_id) DO UPDATE SET
			handle=excluded.handle,
			title=excluded.title,
			niche=excluded.niche
	`, channelID, handle, title, niche)
	if err != nil {
		return err
	}
	// Also mark in yt_channels with kind='competitor'
	return UpsertChannel(ctx, db, Channel{ID: channelID, Handle: handle, Title: title, Kind: "competitor"})
}

// RemoveFromWatchlist removes a channel from the watchlist.
func RemoveFromWatchlist(ctx context.Context, db *sql.DB, channelID string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM yt_watchlist WHERE channel_id = ?`, channelID); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, `UPDATE yt_channels SET kind = 'archived' WHERE channel_id = ?`, channelID)
	return err
}

// WatchlistEntry represents one row in the watchlist.
type WatchlistEntry struct {
	ChannelID string `json:"channel_id"`
	Handle    string `json:"handle,omitempty"`
	Title     string `json:"title,omitempty"`
	Niche     string `json:"niche,omitempty"`
	AddedAt   string `json:"added_at"`
}

// ListWatchlist returns all watchlist entries.
func ListWatchlist(ctx context.Context, db *sql.DB) ([]WatchlistEntry, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT channel_id, COALESCE(handle,''), COALESCE(title,''), COALESCE(niche,''), COALESCE(added_at,'')
		FROM yt_watchlist ORDER BY added_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchlistEntry
	for rows.Next() {
		var e WatchlistEntry
		if err := rows.Scan(&e.ChannelID, &e.Handle, &e.Title, &e.Niche, &e.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LogQuota records quota usage for the day.
func LogQuota(ctx context.Context, db *sql.DB, endpoint string, units int) error {
	day := time.Now().UTC().Format("2006-01-02")
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_quota_log(day, endpoint, units_used) VALUES (?, ?, ?)
	`, day, endpoint, units)
	return err
}

// QuotaUsedToday returns total units used today across all endpoints.
func QuotaUsedToday(ctx context.Context, db *sql.DB) (int, map[string]int, error) {
	day := time.Now().UTC().Format("2006-01-02")
	rows, err := db.QueryContext(ctx, `
		SELECT endpoint, SUM(units_used) FROM yt_quota_log
		WHERE day = ? GROUP BY endpoint
	`, day)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	total := 0
	byEndpoint := map[string]int{}
	for rows.Next() {
		var ep string
		var u int
		if err := rows.Scan(&ep, &u); err != nil {
			return 0, nil, err
		}
		byEndpoint[ep] = u
		total += u
	}
	return total, byEndpoint, rows.Err()
}

// AddIdeaGapEntry records a competitor video for idea-gap analysis.
func AddIdeaGapEntry(ctx context.Context, db *sql.DB, competitorID, videoID, title, topicSignal string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO yt_search_idea_gap(competitor_channel_id, video_id, topic_signal, title)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(competitor_channel_id, video_id) DO UPDATE SET
			topic_signal=excluded.topic_signal,
			title=excluded.title
	`, competitorID, videoID, topicSignal, title)
	return err
}

// IdeaGap is one identified gap: a competitor topic the own channel hasn't covered.
type IdeaGap struct {
	CompetitorChannelID string `json:"competitor_channel_id"`
	VideoID             string `json:"video_id"`
	Title               string `json:"title"`
	TopicSignal         string `json:"topic_signal"`
	SeenAt              string `json:"seen_at"`
}

// FindIdeaGaps returns competitor topics not present in own channel videos.
// Comparison is via case-insensitive substring on a simple stop-worded token.
func FindIdeaGaps(ctx context.Context, db *sql.DB, sinceDays int, ownChannelIDs []string) ([]IdeaGap, error) {
	// pull recent competitor entries
	rows, err := db.QueryContext(ctx, `
		SELECT competitor_channel_id, video_id, COALESCE(title,''), COALESCE(topic_signal,''), COALESCE(seen_at,'')
		FROM yt_search_idea_gap
		WHERE seen_at >= datetime('now', ?)
	`, fmt.Sprintf("-%d days", sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ownTitles, err := titlesForChannels(ctx, db, ownChannelIDs)
	if err != nil {
		return nil, err
	}

	var gaps []IdeaGap
	for rows.Next() {
		var g IdeaGap
		if err := rows.Scan(&g.CompetitorChannelID, &g.VideoID, &g.Title, &g.TopicSignal, &g.SeenAt); err != nil {
			return nil, err
		}
		if !coveredBy(g.Title, ownTitles) {
			gaps = append(gaps, g)
		}
	}
	return gaps, rows.Err()
}

func titlesForChannels(ctx context.Context, db *sql.DB, channelIDs []string) ([]string, error) {
	if len(channelIDs) == 0 {
		// fall back to all 'own' kind channels
		rows, err := db.QueryContext(ctx, `
			SELECT COALESCE(v.title,'') FROM yt_videos v
			JOIN yt_channels c ON c.channel_id = v.channel_id
			WHERE c.kind = 'own'`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var titles []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				return nil, err
			}
			titles = append(titles, t)
		}
		return titles, rows.Err()
	}
	placeholders := strings.Repeat("?,", len(channelIDs))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]any, len(channelIDs))
	for i, id := range channelIDs {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(title,'') FROM yt_videos WHERE channel_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		titles = append(titles, t)
	}
	return titles, rows.Err()
}

func coveredBy(title string, corpus []string) bool {
	tokens := significantTokens(title)
	if len(tokens) == 0 {
		return false
	}
	// title is "covered" when at least 2 significant tokens appear in any
	// own-channel title (case-insensitive). Single-token overlap is too loose.
	for _, own := range corpus {
		lowered := strings.ToLower(own)
		hits := 0
		for _, tok := range tokens {
			if strings.Contains(lowered, tok) {
				hits++
				if hits >= 2 {
					return true
				}
			}
		}
	}
	return false
}

var stopWords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "and": {}, "or": {}, "but": {}, "to": {}, "of": {},
	"in": {}, "on": {}, "at": {}, "is": {}, "it": {}, "with": {}, "for": {}, "by": {},
	"as": {}, "this": {}, "that": {}, "be": {}, "are": {}, "was": {}, "were": {},
	"vs": {}, "you": {}, "your": {}, "my": {}, "i": {},
}

func significantTokens(s string) []string {
	lower := strings.ToLower(s)
	// crude tokenize: keep [a-z0-9]+, drop short and stopwords
	var tokens []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := b.String()
		b.Reset()
		if len(t) < 3 {
			return
		}
		if _, stop := stopWords[t]; stop {
			return
		}
		tokens = append(tokens, t)
	}
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// ListOwnChannelIDs returns channel IDs marked 'own'.
func ListOwnChannelIDs(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT channel_id FROM yt_channels WHERE kind = 'own'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// TitleCTRStat is a per-video title with its mean CTR across all daily metric rows.
type TitleCTRStat struct {
	Title string
	CTR   float64
}

// OwnChannelTitleCTRs returns mean-CTR per video (with title) for all videos
// belonging to own channels that have at least one non-zero CTR row.
func OwnChannelTitleCTRs(ctx context.Context, db *sql.DB) ([]TitleCTRStat, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT v.title, AVG(NULLIF(m.ctr, 0)) AS mean_ctr
		FROM yt_videos v
		JOIN yt_video_metrics_daily m ON m.video_id = v.video_id
		JOIN yt_channels c ON c.channel_id = v.channel_id
		WHERE c.kind = 'own' AND m.ctr > 0
		GROUP BY v.video_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TitleCTRStat
	for rows.Next() {
		var title string
		var ctr sql.NullFloat64
		if err := rows.Scan(&title, &ctr); err != nil {
			return nil, err
		}
		if ctr.Valid && ctr.Float64 > 0 {
			out = append(out, TitleCTRStat{Title: title, CTR: ctr.Float64})
		}
	}
	return out, rows.Err()
}
