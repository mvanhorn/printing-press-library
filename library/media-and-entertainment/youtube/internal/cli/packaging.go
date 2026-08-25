// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: packaging collector. Implemented body — regeneration preserves this file.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube/internal/cliutil"

	"github.com/spf13/cobra"
)

type packagingRow struct {
	VideoID         string `json:"videoId"`
	ChannelID       string `json:"channelId"`
	Title           string `json:"title"`
	ThumbURL        string `json:"thumbUrl,omitempty"`
	ThumbPath       string `json:"thumbPath,omitempty"`
	HookText        string `json:"hookText,omitempty"`
	DurationSeconds int64  `json:"durationSeconds"`
	ViewCount       int64  `json:"viewCount"`
	ThumbError      string `json:"thumbError,omitempty"`
	HookError       string `json:"hookError,omitempty"`
}

type packagingResult struct {
	Scope         string         `json:"scope"`
	Items         []packagingRow `json:"items"`
	ThumbsDir     string         `json:"thumbsDir,omitempty"`
	QuotaUnitsEst int            `json:"quotaUnitsEst"`
	RotatedKeys   []string       `json:"rotatedKeys,omitempty"`
	Note          string         `json:"note,omitempty"`
}

func newNovelPackagingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var topN, hookSeconds int
	var noThumbs, noHooks, rotate bool
	cmd := &cobra.Command{
		Use:     "packaging [@handle|channelId|videoId]",
		Short:   "Collect titles, thumbnails (downloaded as local image files), and hook text from transcript openings into a packaging table.",
		Long:    "Use this command to collect titles, thumbnails, and hook text for packaging analysis.\nDo NOT use it for full transcripts; use 'youtube videos-transcript' instead.",
		Example: "  youtube-pp-cli packaging @mkbhd --json\n  youtube-pp-cli packaging dQw4w9WgXcQ --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "target=@GoogleDevelopers;--top=2;--no-hooks",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "packaging collect")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a channel (@handle or UC… id) or a video id is required"))
			}
			target := strings.TrimSpace(args[0])
			top := clampInt(topN, 1, 200)
			hookWindowMs := int64(hookSeconds) * 1000
			if cliutil.IsDogfoodEnv() {
				if top > 2 {
					top = 2
				}
				noHooks = true // transcript fetches are slow; keep the matrix fast
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			g, err := newRotatingGetter(flags, rotate)
			if err != nil {
				return err
			}
			quota := 0
			var ids []string
			if looksLikeVideoID(target) {
				ids = []string{target}
			} else {
				ch, rerr := resolveChannelInput(ctx, g, target)
				if rerr != nil {
					return classifyAPIError(cmd.ErrOrStderr(), rerr, flags)
				}
				quota++
				walked, werr := walkUploadsPlaylist(ctx, g, ch.UploadsPlaylist, top)
				if werr != nil && len(walked) == 0 {
					return classifyAPIError(cmd.ErrOrStderr(), werr, flags)
				}
				quota += (len(walked) + 49) / 50
				ids = walked
			}
			rows, err := fetchVideoStats(ctx, g, ids)
			if err != nil && len(rows) == 0 {
				return classifyAPIError(cmd.ErrOrStderr(), err, flags)
			}
			partialErr := err
			quota += (len(ids) + 49) / 50

			thumbsDir := ""
			if !noThumbs {
				if dir, derr := cliutil.DataDir(); derr == nil {
					thumbsDir = filepath.Join(dir, "thumbs")
					if merr := os.MkdirAll(thumbsDir, 0o755); merr != nil {
						thumbsDir = ""
					}
				}
			}

			db, _, err := openAnalystStore(ctx, dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			now := nowUTC()
			if _, err := writeVideoRows(ctx, db, rows, now); err != nil {
				return apiErr(err)
			}

			httpClient := &http.Client{
				Timeout: 20 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if req.URL.Scheme != "https" || !allowedThumbHost(req.URL.Hostname()) {
						return fmt.Errorf("redirect to disallowed host %q", req.URL.Hostname())
					}
					return nil
				},
			}
			items := make([]packagingRow, 0, len(rows))
			for _, r := range rows {
				row := packagingRow{
					VideoID: r.VideoID, ChannelID: r.ChannelID, Title: r.Title,
					ThumbURL: r.ThumbURL, DurationSeconds: r.DurationSeconds, ViewCount: r.ViewCount,
				}
				if thumbsDir != "" && r.ThumbURL != "" {
					switch {
					case !validVideoID(r.VideoID):
						row.ThumbError = "refused: video id failed validation for filesystem use"
					default:
						path := filepath.Join(thumbsDir, r.VideoID+".jpg")
						if err := downloadFile(ctx, httpClient, r.ThumbURL, path); err != nil {
							row.ThumbError = err.Error()
						} else {
							row.ThumbPath = path
						}
					}
				}
				if !noHooks {
					tr, herr := fetchHookTranscript(ctx, r.VideoID)
					if herr != nil {
						row.HookError = herr.Error()
					} else {
						row.HookText = hookFromSegments(tr, hookWindowMs)
					}
				}
				if _, err := db.DB().ExecContext(ctx, `INSERT OR REPLACE INTO yt_packaging
					(video_id, channel_id, title, thumb_url, thumb_path, hook_text, duration_seconds, view_count, captured_at, hook_error, thumb_error)
					VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
					row.VideoID, row.ChannelID, row.Title, row.ThumbURL, row.ThumbPath, row.HookText,
					row.DurationSeconds, row.ViewCount, now, row.HookError, row.ThumbError); err != nil {
					return apiErr(err)
				}
				items = append(items, row)
			}
			res := packagingResult{Scope: target, Items: items, ThumbsDir: thumbsDir, QuotaUnitsEst: quota, RotatedKeys: g.Rotated}
			if partialErr != nil {
				res.Note = "partial: stats fetch errored mid-run (" + partialErr.Error() + "); rows cover what succeeded"
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+res.Note)
			}
			if noHooks {
				res.Note = "hook text skipped (--no-hooks or dogfood harness); rerun without it for transcript openings"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-60s  thumb:%s\n", it.VideoID, truncateStr(it.Title, 60), orDash(it.ThumbPath))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: active workspace databank)")
	cmd.Flags().IntVar(&topN, "top", 20, "Recent uploads to collect when the target is a channel")
	cmd.Flags().IntVar(&hookSeconds, "hook-seconds", 45, "Transcript opening window captured as hook text")
	cmd.Flags().BoolVar(&noThumbs, "no-thumbs", false, "Skip downloading thumbnail files")
	cmd.Flags().BoolVar(&noHooks, "no-hooks", false, "Skip transcript hook extraction")
	cmd.Flags().BoolVar(&rotate, "rotate", false, "Fail over to the next stored API key on quota exhaustion")
	return cmd
}

// fetchHookTranscript tries English first and falls back to the sole
// available language family, mirroring the transcript command's behavior.
func fetchHookTranscript(ctx context.Context, videoID string) (*transcriptResult, error) {
	tr, err := fetchTranscriptForVideo(ctx, videoID, "en")
	if err == nil {
		return tr, nil
	}
	tracks, terr := fetchCaptionTracks(ctx, videoID)
	if terr != nil || len(tracks) == 0 {
		return nil, err
	}
	fallback, fberr := pickSoleLanguageTrack(tracks)
	if fberr != nil || fallback == nil {
		return nil, err
	}
	return fetchTranscriptForVideo(ctx, videoID, fallback.LanguageCode)
}

func hookFromSegments(tr *transcriptResult, windowMs int64) string {
	if tr == nil || len(tr.Segments) == 0 {
		return ""
	}
	parts := make([]string, 0, 16)
	for _, seg := range tr.Segments {
		if seg.StartMs > windowMs {
			break
		}
		t := strings.TrimSpace(seg.Text)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func downloadFile(ctx context.Context, client *http.Client, rawURL, path string) error {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || !allowedThumbHost(u.Hostname()) {
		return fmt.Errorf("refused thumbnail host %q: only https YouTube image CDNs (ytimg.com, ggpht.com) are fetched", u.Hostname())
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("thumbnail fetch HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 10<<20)); err != nil {
		return err
	}
	return nil
}
