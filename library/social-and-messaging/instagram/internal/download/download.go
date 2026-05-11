// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// Package download fetches Instagram media (photos, reels, IGTV, carousel
// children) to the local filesystem and records each file in the SQLite
// catalog. It's a hand-written companion to the spec-driven sync — sync only
// pulls JSON metadata, this package pulls binary CDN bytes.

package download

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/igutil"
)

// Downloader is the bundle of dependencies a download command needs. Build
// once per command invocation and re-use for the whole walk so cookies, the
// rate limiter, and the DB handle stay shared.
type Downloader struct {
	OutputDir  string
	DryRun     bool
	HTTPClient *http.Client
	Limiter    *cliutil.AdaptiveLimiter
	DB         *sql.DB
}

// MediaResponse is a partial decoding of /api/v1/media/{id}/info/. Fields we
// don't read are kept as json.RawMessage so a response-shape change in
// fields we don't care about can't break the parse.
type MediaResponse struct {
	Items []MediaItem `json:"items"`
	// Some endpoints return a single item directly without the items wrapper;
	// callers can fall back to UnmarshalSingle.
}

type MediaItem struct {
	ID                   string          `json:"id"`
	Pk                   json.RawMessage `json:"pk"`
	Code                 string          `json:"code"`
	TakenAt              int64           `json:"taken_at"`
	MediaType            int             `json:"media_type"` // 1=image, 2=video, 8=carousel
	LikeCount            int             `json:"like_count"`
	CommentCount         int             `json:"comment_count"`
	Caption              *MediaCaption   `json:"caption"`
	AccessibilityCaption string          `json:"accessibility_caption"`
	ImageVersions2       *ImageVersions  `json:"image_versions2"`
	VideoVersions        []VideoVersion  `json:"video_versions"`
	CarouselMedia        []MediaItem     `json:"carousel_media"`
	User                 *MediaUser      `json:"user"`
	Location             json.RawMessage `json:"location"`
}

type MediaCaption struct {
	Text string `json:"text"`
}

type ImageVersions struct {
	Candidates []ImageCandidate `json:"candidates"`
}

type ImageCandidate struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type VideoVersion struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Type      int    `json:"type"`
	Bandwidth int    `json:"bandwidth"`
}

type MediaUser struct {
	Pk       json.RawMessage `json:"pk"`
	Username string          `json:"username"`
}

// DownloadOptions carries the per-call knobs. Most are settable via flags on
// the user-facing get/user commands; the package itself is policy-free —
// flags map onto fields here, the package executes.
type DownloadOptions struct {
	Shortcode       string
	OwnerID         string
	SlideKind       string // "" | "image" | "video"
	IncludeCaptions bool
	IncludeGeotag   bool
	IncludeComments bool
	CaptionsFormat  string // "txt" | "json"
	Slide           string // "1-3" or "" for all
	MaxBytes        int64
	DryRun          bool
}

// FileInfo describes one file produced (or that would be produced) by a
// download. EstimatedBytes is populated for dry-run output even when no
// bytes were transferred.
type FileInfo struct {
	Shortcode      string `json:"shortcode"`
	CarouselIndex  int    `json:"carousel_index"`
	Kind           string `json:"kind"`
	CDNURL         string `json:"cdn_url"`
	LocalPath      string `json:"local_path,omitempty"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	Bytes          int64  `json:"bytes,omitempty"`
	EstimatedBytes int64  `json:"estimated_bytes,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Skipped        bool   `json:"skipped,omitempty"`
	SkipReason     string `json:"skip_reason,omitempty"`
}

// DownloadResult aggregates the per-call outcome.
type DownloadResult struct {
	Shortcode       string     `json:"shortcode"`
	OwnerUsername   string     `json:"owner_username,omitempty"`
	Files           []FileInfo `json:"files"`
	BytesDownloaded int64      `json:"bytes_downloaded"`
	EstimatedBytes  int64      `json:"estimated_bytes,omitempty"`
}

// FetchMediaJSON calls /api/v1/media/<id>/info/ and returns the parsed
// response. mediaID accepts either a bare numeric (the kind ShortcodeToMediaID
// produces) or a "<numeric>_<owner>" full media ID.
func (d *Downloader) FetchMediaJSON(ctx context.Context, c *client.Client, mediaID string) (*MediaResponse, error) {
	path := fmt.Sprintf("/api/v1/media/%s/info/", mediaID)
	raw, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}
	var resp MediaResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		// Some endpoints return a bare item — try that path before giving up.
		var single MediaItem
		if err2 := json.Unmarshal(raw, &single); err2 == nil && (single.Code != "" || single.ID != "") {
			resp.Items = []MediaItem{single}
			return &resp, nil
		}
		return nil, fmt.Errorf("parsing media response: %w", err)
	}
	return &resp, nil
}

// pickBestImage returns the highest-resolution image candidate from a slice.
// IG normally returns candidates ordered largest-first but we sort defensively
// by width descending so a future order change can't silently downgrade the
// downloaded asset.
func pickBestImage(c []ImageCandidate) *ImageCandidate {
	if len(c) == 0 {
		return nil
	}
	sorted := append([]ImageCandidate(nil), c...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Width > sorted[j].Width })
	return &sorted[0]
}

// pickBestVideo returns the highest-bandwidth video version. Falls through to
// the largest-pixel version when bandwidth is zero (some endpoints omit it).
func pickBestVideo(v []VideoVersion) *VideoVersion {
	if len(v) == 0 {
		return nil
	}
	sorted := append([]VideoVersion(nil), v...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Bandwidth != sorted[j].Bandwidth {
			return sorted[i].Bandwidth > sorted[j].Bandwidth
		}
		return sorted[i].Width > sorted[j].Width
	})
	return &sorted[0]
}

// parseSlideRange parses "1-3" / "2" / "" into an inclusive [lo, hi] range.
// 1-indexed (matches IG's user-facing "first slide is 1") with hi=0 meaning
// unbounded. Returns (0, 0) for empty input — meaning "all slides".
func parseSlideRange(spec string) (int, int, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, 0, nil
	}
	if !strings.Contains(spec, "-") {
		n, err := strconv.Atoi(spec)
		if err != nil || n < 1 {
			return 0, 0, fmt.Errorf("invalid --slide value %q (use N or N-M, 1-indexed)", spec)
		}
		return n, n, nil
	}
	parts := strings.SplitN(spec, "-", 2)
	lo, err1 := strconv.Atoi(parts[0])
	hi, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || lo < 1 || hi < lo {
		return 0, 0, fmt.Errorf("invalid --slide value %q (use N or N-M, 1-indexed)", spec)
	}
	return lo, hi, nil
}

// DownloadPost orchestrates a single-shortcode fetch: resolves the media ID,
// fetches the JSON, walks carousel children, and downloads each picked file.
// Honors DryRun (no IO), SlideKind filter, MaxBytes limit, and Slide range.
func (d *Downloader) DownloadPost(ctx context.Context, c *client.Client, opts DownloadOptions) (*DownloadResult, error) {
	mediaID, err := igutil.ShortcodeToMediaID(opts.Shortcode)
	if err != nil {
		return nil, err
	}
	resp, err := d.FetchMediaJSON(ctx, c, mediaID)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("media %s: no items in response", opts.Shortcode)
	}
	item := resp.Items[0]

	owner := ""
	if item.User != nil {
		owner = item.User.Username
	}
	captionText := ""
	if item.Caption != nil {
		captionText = item.Caption.Text
	}

	result := &DownloadResult{Shortcode: opts.Shortcode, OwnerUsername: owner}

	// Build the carousel slice. Single posts have no children — wrap the
	// item itself so the loop below handles single and carousel uniformly.
	children := item.CarouselMedia
	if len(children) == 0 {
		children = []MediaItem{item}
	}

	loSlide, hiSlide, err := parseSlideRange(opts.Slide)
	if err != nil {
		return nil, err
	}

	if d.OutputDir != "" && !opts.DryRun && !d.DryRun {
		if err := os.MkdirAll(d.OutputDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating output dir: %w", err)
		}
	}

	for idx, child := range children {
		slideOneIndexed := idx + 1
		if loSlide > 0 && (slideOneIndexed < loSlide || slideOneIndexed > hiSlide) {
			continue
		}

		kind := "image"
		var url string
		var width, height int
		var estBytes int64

		if len(child.VideoVersions) > 0 {
			kind = "video"
			vv := pickBestVideo(child.VideoVersions)
			if vv != nil {
				url = vv.URL
				width, height = vv.Width, vv.Height
				if vv.Bandwidth > 0 {
					estBytes = int64(vv.Bandwidth) / 8 // bps -> bytes/sec is rough
				}
			}
		} else if child.ImageVersions2 != nil {
			ic := pickBestImage(child.ImageVersions2.Candidates)
			if ic != nil {
				url = ic.URL
				width, height = ic.Width, ic.Height
			}
		}
		if url == "" {
			// Couldn't find a CDN URL we recognize — skip this child rather
			// than fail the whole post. A future API shape change shouldn't
			// nuke an entire archive run.
			result.Files = append(result.Files, FileInfo{
				Shortcode:     opts.Shortcode,
				CarouselIndex: idx,
				Kind:          kind,
				Skipped:       true,
				SkipReason:    "no recognized CDN URL",
			})
			continue
		}

		if opts.SlideKind != "" && opts.SlideKind != kind {
			result.Files = append(result.Files, FileInfo{
				Shortcode:     opts.Shortcode,
				CarouselIndex: idx,
				Kind:          kind,
				CDNURL:        url,
				Skipped:       true,
				SkipReason:    "slide-kind filter",
			})
			continue
		}

		ext := ".jpg"
		if kind == "video" {
			ext = ".mp4"
		}
		fileName := fmt.Sprintf("%s_%d%s", opts.Shortcode, idx, ext)
		localPath := filepath.Join(d.OutputDir, fileName)

		fi := FileInfo{
			Shortcode:     opts.Shortcode,
			CarouselIndex: idx,
			Kind:          kind,
			CDNURL:        url,
			LocalPath:     localPath,
			Width:         width,
			Height:        height,
		}
		// Heuristic estimate when the API didn't give us bandwidth: 500KB
		// for stills, 5MB for videos. Used for dry-run rollups only — never
		// trusted as a real byte count.
		if estBytes == 0 {
			if kind == "image" {
				estBytes = 500 * 1024
			} else {
				estBytes = 5 * 1024 * 1024
			}
		}
		fi.EstimatedBytes = estBytes
		result.EstimatedBytes += estBytes

		if opts.DryRun || d.DryRun {
			fi.Skipped = true
			fi.SkipReason = "dry-run"
			result.Files = append(result.Files, fi)
			continue
		}

		size, sha, err := d.fetchToFile(ctx, url, localPath, opts.MaxBytes)
		if err != nil {
			fi.Skipped = true
			fi.SkipReason = "fetch error: " + err.Error()
			result.Files = append(result.Files, fi)
			d.recordHistory(opts.Shortcode, owner, "error", err.Error())
			continue
		}
		fi.Bytes = size
		fi.SHA256 = sha
		result.BytesDownloaded += size
		result.Files = append(result.Files, fi)

		d.upsertMediaFile(idx, kind, url, localPath, width, height, size, sha,
			child.AccessibilityCaption, child.TakenAt, owner, captionText, opts.Shortcode)
	}

	if !opts.DryRun && !d.DryRun {
		d.recordHistory(opts.Shortcode, owner, "ok", "")
	}

	if opts.IncludeCaptions {
		if err := d.writeCaption(opts, item); err != nil {
			fmt.Fprintf(os.Stderr, "warning: caption write failed: %v\n", err)
		}
	}
	if opts.IncludeGeotag && len(item.Location) > 0 && string(item.Location) != "null" {
		if err := d.writeGeotag(opts, item.Location); err != nil {
			fmt.Fprintf(os.Stderr, "warning: geotag write failed: %v\n", err)
		}
	}
	if opts.IncludeComments {
		if err := d.writeComments(ctx, c, opts, mediaID); err != nil {
			// Non-fatal: log and move on. Comments aren't blocking the file
			// archive.
			fmt.Fprintf(os.Stderr, "comments fetch not available for this run: %v\n", err)
		}
	}

	return result, nil
}

// fetchToFile streams url -> localPath, sha-256-hashing as it goes. Honors
// the configured rate limiter and respects MaxBytes by aborting the copy
// without writing anything if Content-Length exceeds the cap.
//
// On HTTP 429, returns a typed *cliutil.RateLimitError so callers can
// distinguish throttling from empty results — silently swallowing a 429
// looks identical to "no data" and corrupts downstream queries.
func (d *Downloader) fetchToFile(ctx context.Context, url, localPath string, maxBytes int64) (int64, string, error) {
	if d.Limiter != nil {
		d.Limiter.Wait()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.instagram.com/")
	hc := d.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		// CDN throttling — surface as a typed error so the caller can choose
		// to back off rather than silently treating the missing bytes as
		// "no asset available".
		if d.Limiter != nil {
			d.Limiter.OnRateLimit()
		}
		return 0, "", &cliutil.RateLimitError{
			URL:        url,
			RetryAfter: cliutil.RetryAfter(resp),
		}
	}
	if resp.StatusCode >= 400 {
		return 0, "", fmt.Errorf("CDN HTTP %d", resp.StatusCode)
	}
	if maxBytes > 0 && resp.ContentLength > 0 && resp.ContentLength > maxBytes {
		return 0, "", fmt.Errorf("content-length %d exceeds --max-bytes %d", resp.ContentLength, maxBytes)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return 0, "", err
	}
	f, err := os.Create(localPath)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	var written int64
	if maxBytes > 0 {
		written, err = io.CopyN(mw, resp.Body, maxBytes+1)
		if err == nil && written > maxBytes {
			os.Remove(localPath)
			return 0, "", fmt.Errorf("downloaded body exceeded --max-bytes %d", maxBytes)
		}
		if err == io.EOF {
			err = nil
		}
	} else {
		written, err = io.Copy(mw, resp.Body)
	}
	if err != nil {
		os.Remove(localPath)
		return 0, "", err
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (d *Downloader) upsertMediaFile(idx int, kind, url, localPath string, width, height int, bytes int64, sha, alt string, takenAt int64, owner, caption, shortcode string) {
	if d.DB == nil {
		return
	}
	_, err := d.DB.Exec(`INSERT INTO media_files (
		shortcode, carousel_index, kind, cdn_url, local_path, width, height, bytes, sha256, alt_text, taken_at, owner_username, caption, downloaded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(shortcode, carousel_index, cdn_url) DO UPDATE SET
		local_path = excluded.local_path,
		width = excluded.width,
		height = excluded.height,
		bytes = excluded.bytes,
		sha256 = excluded.sha256,
		alt_text = excluded.alt_text,
		taken_at = excluded.taken_at,
		owner_username = excluded.owner_username,
		caption = excluded.caption,
		downloaded_at = excluded.downloaded_at`,
		shortcode, idx, kind, url, localPath, width, height, bytes, sha, alt, takenAt, owner, caption, time.Now().Unix(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: media_files upsert: %v\n", err)
		return
	}
	// Best-effort FTS sync. The FTS5 contentless trigger isn't auto-wired
	// because we created the FTS table with content='media_files'; we keep
	// FTS in sync with an explicit INSERT so search works end-to-end.
	_, _ = d.DB.Exec(`INSERT INTO media_files_fts(rowid, shortcode, owner_username, caption, alt_text)
		SELECT id, shortcode, owner_username, caption, alt_text FROM media_files
		WHERE shortcode = ? AND carousel_index = ? AND cdn_url = ?
		AND id NOT IN (SELECT rowid FROM media_files_fts WHERE rowid = id)`,
		shortcode, idx, url)
}

func (d *Downloader) recordHistory(shortcode, owner, status, lastErr string) {
	if d.DB == nil {
		return
	}
	now := time.Now().Unix()
	_, err := d.DB.Exec(`INSERT INTO download_history (shortcode, owner_username, status, attempts, last_error, last_attempt_at, first_seen_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(shortcode) DO UPDATE SET
			owner_username = excluded.owner_username,
			status = excluded.status,
			attempts = download_history.attempts + 1,
			last_error = excluded.last_error,
			last_attempt_at = excluded.last_attempt_at`,
		shortcode, owner, status, lastErr, now, now,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: download_history upsert: %v\n", err)
	}
}

func (d *Downloader) writeCaption(opts DownloadOptions, item MediaItem) error {
	format := opts.CaptionsFormat
	if format == "" {
		format = "txt"
	}
	switch format {
	case "json":
		fp := filepath.Join(d.OutputDir, opts.Shortcode+".json")
		data, err := json.MarshalIndent(item, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(fp, data, 0o644)
	default:
		fp := filepath.Join(d.OutputDir, opts.Shortcode+".txt")
		text := ""
		if item.Caption != nil {
			text = item.Caption.Text
		}
		return os.WriteFile(fp, []byte(text), 0o644)
	}
}

func (d *Downloader) writeGeotag(opts DownloadOptions, loc json.RawMessage) error {
	fp := filepath.Join(d.OutputDir, opts.Shortcode+".location.json")
	return os.WriteFile(fp, []byte(loc), 0o644)
}

// writeComments fetches /api/v1/media/<id>/comments/ and writes the raw JSON
// payload alongside the post. We don't try to walk pagination here — the
// first page is what most archive tools surface; users who want full threads
// can use IG's web UI.
func (d *Downloader) writeComments(ctx context.Context, c *client.Client, opts DownloadOptions, mediaID string) error {
	apiPath := fmt.Sprintf("/api/v1/media/%s/comments/", mediaID)
	raw, err := c.Get(apiPath, nil)
	if err != nil {
		return err
	}
	fp := filepath.Join(d.OutputDir, opts.Shortcode+".comments.json")
	return os.WriteFile(fp, []byte(raw), 0o644)
}

// CDNExtFromURL is a utility callers (e.g. stories/highlights) can use to
// pick a sensible extension when they have a URL but no MediaResponse to
// inspect. Stays in this package so the download package owns all extension
// logic.
func CDNExtFromURL(url string) string {
	ext := strings.ToLower(path.Ext(url))
	if ext == "" || len(ext) > 5 {
		return ""
	}
	if i := strings.Index(ext, "?"); i >= 0 {
		ext = ext[:i]
	}
	return ext
}
