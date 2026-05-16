// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature command (Phase 3).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator/internal/cliutil"
)

func newBackupCmd(flags *rootFlags) *cobra.Command {
	var since, outDir, ytdlpPath, channelID, browser string
	var captions, thumbnails, infoJSON, dryList bool
	var concurrency int

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Archive own videos via yt-dlp (captions, thumbnails, info-json)",
		Long: `Wraps yt-dlp to back up your own (or any reachable) YouTube videos.
Enumerates your channel's uploads via playlistItems.list (cheap), constructs
a yt-dlp batch invocation with the chosen extras, and writes to --out.

Requires yt-dlp on PATH (or --yt-dlp /path/to/binary). For members-only or
private content, pair with --browser to import cookies (yt-dlp's
--cookies-from-browser).`,
		Example: "  youtube-creator-pp-cli backup --since 30d --captions --thumbnails --info-json --out ./archive\n" +
			"  youtube-creator-pp-cli backup --since 7d --browser firefox --out ./archive",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), `{"would":"archive channel uploads via yt-dlp","out":%q,"requires":"yt-dlp on PATH"}`+"\n", outDir)
				return nil
			}
			if outDir == "" {
				return usageErr(fmt.Errorf("--out is required"))
			}

			// Resolve yt-dlp
			if ytdlpPath == "" {
				p, err := exec.LookPath("yt-dlp")
				if err != nil {
					return configErr(fmt.Errorf("yt-dlp not on PATH; pass --yt-dlp or install with: pip install -U yt-dlp"))
				}
				ytdlpPath = p
			}
			if _, err := os.Stat(ytdlpPath); err != nil {
				return configErr(fmt.Errorf("yt-dlp binary not found at %s", ytdlpPath))
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating out dir: %w", err)
			}

			// Resolve channel (default: authenticated user)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var uploadsID string
			if channelID == "" {
				quotaLogCost("channels-list", 1)
				chData, err := c.Get("/youtube/v3/channels", map[string]string{
					"part": "contentDetails",
					"mine": "true",
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var chResp struct {
					Items []struct {
						ContentDetails struct {
							RelatedPlaylists struct {
								Uploads string `json:"uploads"`
							} `json:"relatedPlaylists"`
						} `json:"contentDetails"`
					} `json:"items"`
				}
				_ = json.Unmarshal(chData, &chResp)
				if len(chResp.Items) == 0 {
					return apiErr(fmt.Errorf("no authenticated channel"))
				}
				uploadsID = chResp.Items[0].ContentDetails.RelatedPlaylists.Uploads
			} else {
				// Derive from explicit channel ID: convert UCxxxx → UUxxxx (uploads playlist)
				if strings.HasPrefix(channelID, "UC") {
					uploadsID = "UU" + channelID[2:]
				} else {
					uploadsID = channelID
				}
			}

			// Enumerate uploads
			type videoEntry struct {
				ID          string    `json:"id"`
				Title       string    `json:"title"`
				PublishedAt time.Time `json:"published_at"`
			}
			var entries []videoEntry
			cutoff := time.Time{}
			if since != "" {
				if t, err := parseSince(since); err == nil {
					cutoff = t
				}
			}
			pageToken := ""
			for {
				params := map[string]string{
					"part":       "snippet,contentDetails",
					"playlistId": uploadsID,
					"maxResults": "50",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				quotaLogCost("playlist-items-list", 1)
				d, err := c.Get("/youtube/v3/playlistItems", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var page struct {
					Items []struct {
						Snippet struct {
							Title       string    `json:"title"`
							PublishedAt time.Time `json:"publishedAt"`
						} `json:"snippet"`
						ContentDetails struct {
							VideoID string `json:"videoId"`
						} `json:"contentDetails"`
					} `json:"items"`
					NextPageToken string `json:"nextPageToken"`
				}
				_ = json.Unmarshal(d, &page)
				for _, it := range page.Items {
					if !cutoff.IsZero() && it.Snippet.PublishedAt.Before(cutoff) {
						continue
					}
					entries = append(entries, videoEntry{
						ID:          it.ContentDetails.VideoID,
						Title:       it.Snippet.Title,
						PublishedAt: it.Snippet.PublishedAt,
					})
				}
				if page.NextPageToken == "" {
					break
				}
				pageToken = page.NextPageToken
			}

			if dryList || flags.dryRun {
				return flags.printJSON(cmd, map[string]any{
					"would_archive": len(entries),
					"target_dir":    outDir,
					"yt_dlp":        ytdlpPath,
					"entries":       entries,
					"flags": map[string]any{
						"captions":   captions,
						"thumbnails": thumbnails,
						"info_json":  infoJSON,
						"browser":    browser,
					},
				})
			}

			// Build batch file
			batchPath := filepath.Join(outDir, ".pp-backup-batch.txt")
			f, err := os.Create(batchPath)
			if err != nil {
				return fmt.Errorf("creating batch file: %w", err)
			}
			for _, e := range entries {
				fmt.Fprintf(f, "https://www.youtube.com/watch?v=%s\n", e.ID)
			}
			f.Close()

			ytArgs := []string{
				"--batch-file", batchPath,
				"-P", outDir,
				"-o", "%(upload_date)s-%(id)s/%(title)s.%(ext)s",
				"--download-archive", filepath.Join(outDir, ".yt-dlp-archive.txt"),
			}
			if captions {
				ytArgs = append(ytArgs, "--write-subs", "--write-auto-subs", "--sub-langs", "en")
			}
			if thumbnails {
				ytArgs = append(ytArgs, "--write-thumbnail")
			}
			if infoJSON {
				ytArgs = append(ytArgs, "--write-info-json")
			}
			if browser != "" {
				ytArgs = append(ytArgs, "--cookies-from-browser", browser)
			}
			if concurrency > 0 {
				ytArgs = append(ytArgs, "-N", fmt.Sprintf("%d", concurrency))
			}

			// In verify mode, don't actually run yt-dlp
			if cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{
					"would_run":  ytdlpPath,
					"args":       ytArgs,
					"batch_size": len(entries),
				})
			}

			runCmd := exec.Command(ytdlpPath, ytArgs...)
			runCmd.Stdout = cmd.OutOrStdout()
			runCmd.Stderr = cmd.ErrOrStderr()
			runErr := runCmd.Run()
			if runErr != nil {
				return apiErr(fmt.Errorf("yt-dlp exited with error: %w", runErr))
			}
			return flags.printJSON(cmd, map[string]any{
				"archived":   len(entries),
				"out_dir":    outDir,
				"batch_file": batchPath,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only archive videos newer than this duration (e.g. 30d)")
	cmd.Flags().StringVar(&outDir, "out", "", "Output directory (required)")
	cmd.Flags().StringVar(&ytdlpPath, "yt-dlp", "", "Path to yt-dlp binary (default: PATH lookup)")
	cmd.Flags().StringVar(&channelID, "channel", "", "Target channel UCxxxx (default: authenticated user)")
	cmd.Flags().StringVar(&browser, "browser", "", "Pass --cookies-from-browser <name> to yt-dlp (firefox, chrome, edge, ...)")
	cmd.Flags().BoolVar(&captions, "captions", false, "Also archive captions (auto + manual)")
	cmd.Flags().BoolVar(&thumbnails, "thumbnails", false, "Also archive thumbnails")
	cmd.Flags().BoolVar(&infoJSON, "info-json", false, "Also archive yt-dlp's info.json metadata")
	cmd.Flags().BoolVar(&dryList, "list-only", false, "List what would be archived without running yt-dlp")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Parallel downloads (yt-dlp -N)")
	return cmd
}
