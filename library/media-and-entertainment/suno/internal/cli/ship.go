// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add one-shot publishing bundle export for clips.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/cliutil"
)

// PATCH(greptile #577 P2): bound CDN downloads. Stale or hung URLs would block
// `ship` indefinitely on the bare http.Get default (no timeout). 120s covers
// the longest legitimate audio/video render (Suno MP4 ~5MB, MP3 ~5MB) with
// generous slack for slow networks.
var shipDownloadClient = &http.Client{Timeout: 120 * time.Second}

func newShipCmd(flags *rootFlags) *cobra.Command {
	var toDir, format string
	var skipVideo bool
	cmd := &cobra.Command{
		Use:   "ship <clip-id>",
		Short: "Create an editor-ready publishing bundle for a clip",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() || os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				fmt.Fprintln(cmd.OutOrStdout(), "verify mode: ship file writes skipped")
				return nil
			}
			if format != "mp3" && format != "wav" {
				return usageErr(fmt.Errorf("invalid --format %q: expected mp3 or wav", format))
			}
			if toDir == "" {
				toDir = "."
			}
			raw, err := readClipRaw(cmd.Context(), args[0])
			if err != nil {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				raw, err = c.Get("/api/clip/"+args[0], nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			obj := unmarshalObject(raw)
			base := sanitizeFilename(clipTitle(obj))
			audioPath := filepath.Join(toDir, base+"."+format)
			videoPath := filepath.Join(toDir, base+".mp4")
			coverPath := filepath.Join(toDir, base+".cover.png")
			lrcPath := filepath.Join(toDir, base+".lrc")
			sidecarPath := filepath.Join(toDir, base+".json")
			audioURL := stringAtAny(obj, []string{"audio_url"}, []string{"audioUrl"})
			videoURL := stringAtAny(obj, []string{"video_url"}, []string{"videoUrl"})
			imageURL := stringAtAny(obj, []string{"image_url"}, []string{"imageUrl"})
			// Only list files in the plan when the corresponding source URL is
			// present. A missing source URL means the file would be a zero-byte
			// placeholder, which silently breaks downstream importers (CapCut etc).
			files := map[string]string{
				"lrc":  lrcPath,
				"json": sidecarPath,
			}
			warnings := []string{}
			if audioURL != "" {
				files["audio"] = audioPath
			} else {
				warnings = append(warnings, "no audio_url on clip; audio file skipped")
			}
			if imageURL != "" {
				files["cover"] = coverPath
			} else {
				warnings = append(warnings, "no image_url on clip; cover.png skipped")
			}
			if !skipVideo {
				if videoURL != "" {
					files["video"] = videoPath
				} else {
					warnings = append(warnings, "no video_url on clip; .mp4 skipped (pass --skip-video to suppress)")
				}
			}
			plan := map[string]any{
				"clip_id": args[0],
				"files":   files,
				"source_urls": map[string]string{
					"audio": audioURL,
					"video": videoURL,
					"image": imageURL,
				},
			}
			if len(warnings) > 0 {
				plan["warnings"] = warnings
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			if err := os.MkdirAll(toDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}
			// TODO(upstream): preserve ID3/USLT/SYLT metadata after the downloader grows tagging support.
			if audioURL != "" {
				if err := downloadOrPlaceholder(audioURL, audioPath); err != nil {
					return err
				}
			}
			if !skipVideo && videoURL != "" {
				if err := downloadOrPlaceholder(videoURL, videoPath); err != nil {
					return err
				}
			}
			if imageURL != "" {
				if err := downloadOrPlaceholder(imageURL, coverPath); err != nil {
					return err
				}
			}
			lrc := ""
			if c, err := flags.newClient(); err == nil {
				if data, err := c.Get("/api/gen/"+args[0]+"/aligned_lyrics/v2/", nil); err == nil {
					lrc = strings.TrimSpace(string(data))
				}
			}
			if err := os.WriteFile(lrcPath, []byte(lrc), 0o644); err != nil {
				return fmt.Errorf("writing LRC: %w", err)
			}
			sidecar := map[string]any{"clip": obj, "ship": plan}
			b, err := json.MarshalIndent(sidecar, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling sidecar: %w", err)
			}
			if err := os.WriteFile(sidecarPath, b, 0o644); err != nil {
				return fmt.Errorf("writing sidecar: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&toDir, "to", ".", "Output directory")
	cmd.Flags().BoolVar(&skipVideo, "skip-video", false, "Skip MP4 output")
	cmd.Flags().StringVar(&format, "format", "mp3", "Audio format: mp3 or wav")
	return cmd
}

func downloadOrPlaceholder(url, path string) error {
	if url == "" {
		return os.WriteFile(path, nil, 0o644)
	}
	resp, err := shipDownloadClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("downloading %s returned HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
