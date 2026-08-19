// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared plumbing for the competitor-monitoring machine: workspace registry,
// API key ring, channel resolution, quota detection with key rotation, and
// analyst-databank access. Hand-authored in its own file so regeneration
// preserves it.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/store"
)

// ---------- Validation ----------

var workspaceNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// validWorkspaceName rejects path separators, traversal, and empty names so a
// registry entry can never point outside an intended directory by accident or
// injection.
func validWorkspaceName(name string) bool {
	return name != "" && name != ".." && name != "." && workspaceNameRe.MatchString(name)
}

// validVideoID gates API-supplied ids before they touch the filesystem.
// (Reuses the 11-char id shape; videos_links.go declares the canonical regex.)
var analystVideoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

func validVideoID(id string) bool {
	return analystVideoIDRe.MatchString(id)
}

// allowedThumbHost restricts thumbnail downloads to YouTube's image CDNs.
func allowedThumbHost(host string) bool {
	return strings.HasSuffix(host, ".ytimg.com") || host == "ytimg.com" ||
		strings.HasSuffix(host, ".ggpht.com") || host == "ggpht.com"
}

// ---------- Workspaces ----------

const workspaceEnvOptOut = "YOUTUBE_WORKSPACE" // set to "off" to disable auto-apply

type workspaceRegistry struct {
	Active     string            `json:"active"`
	Workspaces map[string]string `json:"workspaces"` // name -> home dir
}

// workspaceRegistryPath lives in the platform-default config dir on purpose:
// workspace switching relocates YOUTUBE_HOME, and the pointer must stay
// outside the relocated tree or switching becomes self-referential.
func workspaceRegistryPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "youtube-pp-cli", "workspaces.json"), nil
}

func loadWorkspaceRegistry() (*workspaceRegistry, string, error) {
	path, err := workspaceRegistryPath()
	if err != nil {
		return nil, "", err
	}
	reg := &workspaceRegistry{Active: "default", Workspaces: map[string]string{}}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return reg, path, nil
		}
		return nil, path, err
	}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, path, fmt.Errorf("parse %s: %w", path, err)
	}
	if reg.Workspaces == nil {
		reg.Workspaces = map[string]string{}
	}
	if reg.Active == "" {
		reg.Active = "default"
	}
	return reg, path, nil
}

func saveWorkspaceRegistry(reg *workspaceRegistry) error {
	path, err := workspaceRegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// applyActiveWorkspaceEnv points YOUTUBE_HOME at the active workspace's home
// directory. YOUTUBE_HOME is the lowest-priority path override (explicit
// per-kind env vars and the --home flag both beat it), so this can never
// shadow an operator's explicit choice. Runs from package init before any
// command resolves paths.
func applyActiveWorkspaceEnv() {
	if strings.EqualFold(os.Getenv(workspaceEnvOptOut), "off") {
		return
	}
	if os.Getenv("YOUTUBE_HOME") != "" {
		return // explicit env wins untouched
	}
	reg, _, err := loadWorkspaceRegistry()
	if err != nil || reg.Active == "" || reg.Active == "default" {
		return
	}
	home, ok := reg.Workspaces[reg.Active]
	if !ok || home == "" {
		return
	}
	if st, err := os.Stat(home); err != nil || !st.IsDir() {
		return
	}
	_ = os.Setenv("YOUTUBE_HOME", home)
}

func init() {
	applyActiveWorkspaceEnv()
}

// defaultWorkspacesBase returns the directory new named workspaces are
// created under.
func defaultWorkspacesBase() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "youtube-pp-cli", "workspaces"), nil
}

// ---------- API key ring ----------

type keyRing struct {
	Active string            `json:"active"`
	Keys   map[string]string `json:"keys"` // name -> api key value
	Order  []string          `json:"order,omitempty"`
}

// keyRingPath is deliberately shared across workspaces (default config dir):
// quota is per key, not per workspace, and re-adding keys per workspace would
// be pure friction.
func keyRingPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "youtube-pp-cli", "keyring.json"), nil
}

func loadKeyRing() (*keyRing, error) {
	path, err := keyRingPath()
	if err != nil {
		return nil, err
	}
	ring := &keyRing{Keys: map[string]string{}}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ring, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, ring); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if ring.Keys == nil {
		ring.Keys = map[string]string{}
	}
	return ring, nil
}

func saveKeyRing(ring *keyRing) error {
	path, err := keyRingPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ring, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (r *keyRing) names() []string {
	if len(r.Order) > 0 {
		out := make([]string, 0, len(r.Order))
		for _, n := range r.Order {
			if _, ok := r.Keys[n]; ok {
				out = append(out, n)
			}
		}
		for n := range r.Keys {
			found := false
			for _, o := range out {
				if o == n {
					found = true
					break
				}
			}
			if !found {
				out = append(out, n)
			}
		}
		return out
	}
	out := make([]string, 0, len(r.Keys))
	for n := range r.Keys {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// activateRingKey writes the named key into the CLI's credential store (the
// same slot `auth set-token` uses) so every command picks it up.
func activateRingKey(ring *keyRing, name, configPath string) error {
	val, ok := ring.Keys[name]
	if !ok {
		return fmt.Errorf("no stored key named %q", name)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if err := cfg.SaveCredential(val); err != nil {
		return err
	}
	ring.Active = name
	return saveKeyRing(ring)
}

// nextRingKey returns the name after the active one, wrapping, or "" when the
// ring has fewer than two keys.
func nextRingKey(ring *keyRing) string {
	names := ring.names()
	if len(names) < 2 {
		return ""
	}
	for i, n := range names {
		if n == ring.Active {
			return names[(i+1)%len(names)]
		}
	}
	return names[0]
}

func maskKey(v string) string {
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "…" + v[len(v)-4:]
}

// ---------- Quota detection and rotation ----------

func isQuotaExhausted(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "quotaExceeded") ||
		strings.Contains(msg, "dailyLimitExceeded") ||
		strings.Contains(msg, "rateLimitExceeded") ||
		(strings.Contains(msg, "HTTP 403") && strings.Contains(msg, "quota"))
}

// rotatingGetter wraps live GETs with optional key-ring failover: when a call
// fails on exhausted quota and rotation is enabled, it activates the next
// ring key, rebuilds the client, and retries — at most once per remaining
// ring key.
type rotatingGetter struct {
	flags   *rootFlags
	c       *client.Client
	rotate  bool
	tried   int
	Rotated []string
}

func newRotatingGetter(flags *rootFlags, rotate bool) (*rotatingGetter, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return &rotatingGetter{flags: flags, c: c, rotate: rotate}, nil
}

func (g *rotatingGetter) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	for {
		data, err := g.c.GetWithHeaders(ctx, path, params, nil)
		if err == nil || !g.rotate || !isQuotaExhausted(err) {
			return data, err
		}
		ring, ringErr := loadKeyRing()
		if ringErr != nil || len(ring.Keys) < 2 || g.tried >= len(ring.Keys)-1 {
			return data, err
		}
		next := nextRingKey(ring)
		if next == "" || next == ring.Active {
			return data, err
		}
		if actErr := activateRingKey(ring, next, g.flags.configPath); actErr != nil {
			return data, err
		}
		g.tried++
		g.Rotated = append(g.Rotated, next)
		fresh, cErr := g.flags.newClient()
		if cErr != nil {
			return data, err
		}
		g.c = fresh
	}
}

// ---------- Channel resolution ----------

type resolvedChannel struct {
	ChannelID       string `json:"channelId"`
	Title           string `json:"title"`
	Handle          string `json:"handle,omitempty"`
	UploadsPlaylist string `json:"uploadsPlaylistId"`
	SubscriberCount int64  `json:"subscriberCount"`
	ViewCount       int64  `json:"viewCount"`
	VideoCount      int64  `json:"videoCount"`
}

// parseChannelListItems is the single decoder for channels.list responses;
// resolveChannelInput and fetchChannelsByIDs both consume it so a new
// resolvedChannel field only ever needs one struct edit.
func parseChannelListItems(data json.RawMessage) ([]*resolvedChannel, error) {
	var resp struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string `json:"title"`
				CustomURL string `json:"customUrl"`
			} `json:"snippet"`
			ContentDetails struct {
				RelatedPlaylists struct {
					Uploads string `json:"uploads"`
				} `json:"relatedPlaylists"`
			} `json:"contentDetails"`
			Statistics struct {
				SubscriberCount string `json:"subscriberCount"`
				ViewCount       string `json:"viewCount"`
				VideoCount      string `json:"videoCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse channels response: %w", err)
	}
	out := make([]*resolvedChannel, 0, len(resp.Items))
	for _, it := range resp.Items {
		out = append(out, &resolvedChannel{
			ChannelID:       it.ID,
			Title:           it.Snippet.Title,
			Handle:          it.Snippet.CustomURL,
			UploadsPlaylist: it.ContentDetails.RelatedPlaylists.Uploads,
			SubscriberCount: parseCount(it.Statistics.SubscriberCount),
			ViewCount:       parseCount(it.Statistics.ViewCount),
			VideoCount:      parseCount(it.Statistics.VideoCount),
		})
	}
	return out, nil
}

// looksLikeVideoID reports whether a bare argument should be treated as a
// video id rather than a channel handle. Documented ambiguity: an
// 11-character bare handle matching the id charset is indistinguishable and
// will be treated as a video id; prefix handles with @ to disambiguate.
func looksLikeVideoID(s string) bool {
	return validVideoID(s) && !strings.HasPrefix(s, "@") && !strings.HasPrefix(s, "UC")
}

// resolveChannelInput accepts @handle, bare handle, or UC… channel id and
// returns identity plus lifetime statistics. Costs 1 quota unit.
func resolveChannelInput(ctx context.Context, g *rotatingGetter, input string) (*resolvedChannel, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("channel handle or id is required")
	}
	params := map[string]string{"part": "snippet,contentDetails,statistics"}
	switch {
	case strings.HasPrefix(input, "UC") && len(input) == 24:
		params["id"] = input
	case strings.HasPrefix(input, "@"):
		params["forHandle"] = input
	default:
		params["forHandle"] = "@" + input
	}
	data, err := g.Get(ctx, "/youtube/v3/channels", params)
	if err != nil {
		return nil, err
	}
	items, err := parseChannelListItems(data)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("channel %q not found", input)
	}
	return items[0], nil
}

func parseCount(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

// ---------- ISO-8601 duration ----------

var isoDurationRe = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISODurationSeconds parses YouTube contentDetails.duration values like
// PT1H2M3S. Returns 0 for unparseable input.
func parseISODurationSeconds(s string) int64 {
	m := isoDurationRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0
	}
	days, _ := strconv.ParseInt(zeroIfEmpty(m[1]), 10, 64)
	h, _ := strconv.ParseInt(zeroIfEmpty(m[2]), 10, 64)
	mi, _ := strconv.ParseInt(zeroIfEmpty(m[3]), 10, 64)
	sec, _ := strconv.ParseInt(zeroIfEmpty(m[4]), 10, 64)
	return days*86400 + h*3600 + mi*60 + sec
}

func zeroIfEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// isShortDuration classifies Shorts by the <=180s convention.
func isShortDuration(seconds int64) bool {
	return seconds > 0 && seconds <= 180
}

// ---------- Analyst databank access ----------

// openAnalystStore resolves the DB path (explicit --db wins, then the active
// workspace via YOUTUBE_HOME which defaultDBPath already honors) and ensures
// the analyst schema exists.
func openAnalystStore(ctx context.Context, dbPath string) (*store.Store, string, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("youtube-pp-cli")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, dbPath, err
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening databank: %w", err)
	}
	if err := db.EnsureAnalystSchema(ctx); err != nil {
		db.Close()
		return nil, dbPath, err
	}
	return db, dbPath, nil
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ---------- Video meta + snapshot writers ----------

type videoStatsRow struct {
	VideoID         string
	ChannelID       string
	Title           string
	PublishedAt     string
	DurationSeconds int64
	Description     string
	ThumbURL        string
	ViewCount       int64
	LikeCount       int64
	CommentCount    int64
}

// fetchVideoStats batches videos.list for up to 50 ids per call (1 quota unit
// per call) and returns full meta + current statistics.
func fetchVideoStats(ctx context.Context, g *rotatingGetter, ids []string) ([]videoStatsRow, error) {
	out := make([]videoStatsRow, 0, len(ids))
	for start := 0; start < len(ids); start += 50 {
		end := start + 50
		if end > len(ids) {
			end = len(ids)
		}
		data, err := g.Get(ctx, "/youtube/v3/videos", map[string]string{
			"part":       "snippet,statistics,contentDetails",
			"id":         strings.Join(ids[start:end], ","),
			"maxResults": "50",
		})
		if err != nil {
			return out, err
		}
		var resp struct {
			Items []struct {
				ID      string `json:"id"`
				Snippet struct {
					ChannelID   string `json:"channelId"`
					Title       string `json:"title"`
					PublishedAt string `json:"publishedAt"`
					Description string `json:"description"`
					Thumbnails  map[string]struct {
						URL string `json:"url"`
					} `json:"thumbnails"`
				} `json:"snippet"`
				ContentDetails struct {
					Duration string `json:"duration"`
				} `json:"contentDetails"`
				Statistics struct {
					ViewCount    string `json:"viewCount"`
					LikeCount    string `json:"likeCount"`
					CommentCount string `json:"commentCount"`
				} `json:"statistics"`
			} `json:"items"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return out, fmt.Errorf("parse videos response: %w", err)
		}
		for _, it := range resp.Items {
			out = append(out, videoStatsRow{
				VideoID:         it.ID,
				ChannelID:       it.Snippet.ChannelID,
				Title:           it.Snippet.Title,
				PublishedAt:     it.Snippet.PublishedAt,
				DurationSeconds: parseISODurationSeconds(it.ContentDetails.Duration),
				Description:     it.Snippet.Description,
				ThumbURL:        bestThumbURL(it.Snippet.Thumbnails),
				ViewCount:       parseCount(it.Statistics.ViewCount),
				LikeCount:       parseCount(it.Statistics.LikeCount),
				CommentCount:    parseCount(it.Statistics.CommentCount),
			})
		}
	}
	return out, nil
}

// writeVideoRows upserts video meta and appends a dated statistics snapshot
// for each row inside one transaction.
func writeVideoRows(ctx context.Context, db *store.Store, rows []videoStatsRow, capturedAt string) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	metaStmt, err := tx.PrepareContext(ctx, `INSERT INTO yt_videos
		(video_id, channel_id, title, published_at, duration_seconds, is_short, description)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(video_id) DO UPDATE SET
			title=excluded.title, duration_seconds=excluded.duration_seconds,
			is_short=excluded.is_short, description=excluded.description`)
	if err != nil {
		return 0, err
	}
	defer metaStmt.Close()
	snapStmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO yt_video_snapshots
		(video_id, captured_at, view_count, like_count, comment_count) VALUES (?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer snapStmt.Close()
	n := 0
	for _, r := range rows {
		short := 0
		if isShortDuration(r.DurationSeconds) {
			short = 1
		}
		if _, err := metaStmt.ExecContext(ctx, r.VideoID, r.ChannelID, r.Title, r.PublishedAt, r.DurationSeconds, short, r.Description); err != nil {
			return n, err
		}
		if _, err := snapStmt.ExecContext(ctx, r.VideoID, capturedAt, r.ViewCount, r.LikeCount, r.CommentCount); err != nil {
			return n, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, err
	}
	return n, nil
}

// writeChannelSnapshot appends one dated channel statistics row.
func writeChannelSnapshot(ctx context.Context, db *store.Store, ch *resolvedChannel, capturedAt string) error {
	_, err := db.DB().ExecContext(ctx, `INSERT OR REPLACE INTO yt_channel_snapshots
		(channel_id, captured_at, subscriber_count, view_count, video_count) VALUES (?,?,?,?,?)`,
		ch.ChannelID, capturedAt, ch.SubscriberCount, ch.ViewCount, ch.VideoCount)
	return err
}

// walkUploadsPlaylist pages playlistItems.list (1 unit per page of 50) and
// returns video ids, newest first, up to maxItems.
func walkUploadsPlaylist(ctx context.Context, g *rotatingGetter, playlistID string, maxItems int) ([]string, error) {
	ids := make([]string, 0, 64)
	pageToken := ""
	for len(ids) < maxItems {
		params := map[string]string{
			"part":       "contentDetails",
			"playlistId": playlistID,
			"maxResults": "50",
		}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := g.Get(ctx, "/youtube/v3/playlistItems", params)
		if err != nil {
			return ids, err
		}
		var resp struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return ids, fmt.Errorf("parse playlistItems response: %w", err)
		}
		for _, it := range resp.Items {
			if it.ContentDetails.VideoID != "" {
				ids = append(ids, it.ContentDetails.VideoID)
			}
			if len(ids) >= maxItems {
				break
			}
		}
		if resp.NextPageToken == "" || len(resp.Items) == 0 {
			break
		}
		pageToken = resp.NextPageToken
	}
	return ids, nil
}

// watchlistEntries returns the watchlist, oldest first.
func watchlistEntries(ctx context.Context, db *store.Store) ([]watchEntry, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT channel_id, handle, title, note, added_at, last_monitored_at
		FROM yt_watchlist ORDER BY added_at ASC`)
	if err != nil {
		return nil, err
	}
	out := make([]watchEntry, 0)
	for rows.Next() {
		var e watchEntry
		if err := rows.Scan(&e.ChannelID, &e.Handle, &e.Title, &e.Note, &e.AddedAt, &e.LastMonitoredAt); err != nil {
			_ = rows.Close()
			return out, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	_ = rows.Close()
	return out, nil
}

type watchEntry struct {
	ChannelID       string `json:"channelId"`
	Handle          string `json:"handle,omitempty"`
	Title           string `json:"title"`
	Note            string `json:"note,omitempty"`
	AddedAt         string `json:"addedAt"`
	LastMonitoredAt string `json:"lastMonitoredAt,omitempty"`
}

// bestThumbURL prefers the largest available thumbnail variant.
func bestThumbURL(thumbs map[string]struct {
	URL string `json:"url"`
}) string {
	for _, k := range []string{"maxres", "standard", "high", "medium", "default"} {
		if t, ok := thumbs[k]; ok && t.URL != "" {
			return t.URL
		}
	}
	return ""
}
