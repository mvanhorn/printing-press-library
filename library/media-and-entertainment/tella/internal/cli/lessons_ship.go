// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/spf13/cobra"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

func exportSegment(id string) string {
	var b strings.Builder
	for _, c := range []byte(id) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	if id == "." || id == ".." {
		return strings.Repeat("%2E", len(id))
	}
	return b.String()
}
func checkedExport(raw json.RawMessage) (string, string, string, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", "", "", err
	}
	if e, ok := obj["export"].(map[string]any); ok {
		obj = e
	}
	if strings.TrimSpace(stringField(obj, "status", "state")) == "" {
		return "", "", "", fmt.Errorf("export response missing status")
	}
	id, status, location, _ := parseExportResponse(raw)
	if strings.EqualFold(status, "failed") || strings.EqualFold(status, "error") {
		return id, status, "", fmt.Errorf("export failed (%s)", status)
	}
	return id, status, location, nil
}
func waitLessonExport(ctx context.Context, c *client.Client, video string, options map[string]any, interval time.Duration) (string, error) {
	raw, _, err := c.Post(ctx, videoPath(video)+"/exports", options)
	if err != nil {
		return "", err
	}
	id, status, location, err := checkedExport(raw)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("export response missing exportId")
	}
	for !isExportTerminal(status) {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", fmt.Errorf("timed out waiting for export: %w", ctx.Err())
		case <-timer.C:
		}
		raw, err = c.Get(ctx, videoPath(video)+"/exports/"+exportSegment(id), nil)
		if err != nil {
			return "", err
		}
		nextID, nextStatus, nextURL, e := checkedExport(raw)
		if e != nil {
			return "", e
		}
		if nextID != "" && nextID != id {
			return "", fmt.Errorf("poll returned a different exportId")
		}
		status, location = nextStatus, nextURL
	}
	if location == "" {
		return "", fmt.Errorf("completed export missing download URL")
	}
	return location, nil
}
func validateLessonCaptions(raw []byte, format string) error {
	text := strings.TrimSpace(strings.TrimPrefix(string(raw), "\ufeff"))
	if !utf8.Valid(raw) || text == "" || json.Valid(raw) || strings.HasPrefix(text, "<") {
		return fmt.Errorf("captions response is not subtitle text")
	}
	if !strings.Contains(text, " --> ") {
		return fmt.Errorf("captions response has no timed cues")
	}
	if format == "vtt" && !strings.HasPrefix(text, "WEBVTT") {
		return fmt.Errorf("missing WEBVTT header")
	}
	if format == "srt" && (!strings.Contains(text, ",") || strings.HasPrefix(text, "WEBVTT")) {
		return fmt.Errorf("invalid SRT captions")
	}
	return nil
}
func newNovelLessonsShipCmd(flags *rootFlags) *cobra.Command {
	var outputDir, format string
	var poll, timeout time.Duration
	var overwrite bool
	cmd := &cobra.Command{Use: "ship <video>", Short: "Explicitly export, wait and download a reviewed lesson MP4 plus captions", Args: workflowArgs(1), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live"}, Example: "  tella-pp-cli lessons ship vid_example --output-dir ./delivery --format srt --wait-timeout 10m"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if format != "srt" && format != "vtt" {
			return usageErr(fmt.Errorf("--format must be srt or vtt"))
		}
		if poll <= 0 || poll > time.Minute || timeout <= 0 || timeout > time.Hour {
			return usageErr(fmt.Errorf("--poll-interval must be positive and <=1m; --wait-timeout must be positive and <=1h"))
		}
		if strings.TrimSpace(outputDir) == "" {
			return usageErr(fmt.Errorf("--output-dir must not be blank"))
		}
		video := args[0]
		mp4, err := checkLessonTarget(filepath.Join(outputDir, video+".mp4"), overwrite)
		if err != nil {
			return err
		}
		captions, err := checkLessonTarget(filepath.Join(outputDir, video+"."+format), overwrite)
		if err != nil {
			return err
		}
		options := map[string]any{"granularity": "video", "resolution": "1080p", "fps": "30", "speed": "1"}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "video_id": video, "mp4": mp4, "captions": captions, "export_options": options, "format": format, "poll_interval": poll.String(), "wait_timeout": timeout.String()}, flags)
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		if _, err = c.Get(cmd.Context(), videoPath(video), nil); err != nil {
			return err
		}
		waitCtx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		location, err := waitLessonExport(waitCtx, c, video, options, poll)
		if err != nil {
			return err
		}
		raw, err := c.GetWithHeadersNoCache(cmd.Context(), videoPath(video)+"/captions", map[string]string{"format": format}, map[string]string{client.BinaryResponseHeader: "true", "Accept": "text/vtt, application/x-subrip"})
		if err != nil {
			return err
		}
		captionBytes := []byte(raw)
		if b, _, ok := client.UnwrapBinaryResponse(raw); ok {
			captionBytes = b
		}
		if err = validateLessonCaptions(captionBytes, format); err != nil {
			return err
		}
		actualMP4, size, err := downloadLessonMedia(cmd.Context(), location, mp4, "mp4", overwrite, lessonMediaTimeout(flags))
		if err != nil {
			return err
		}
		actualCaptions, err := writeLessonAsset(captions, captionBytes, overwrite)
		if err != nil {
			return err
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"mp4": actualMP4, "mp4_bytes": size, "captions": actualCaptions, "caption_bytes": len(captionBytes), "format": format}, flags)
	}
	cmd.Flags().StringVar(&outputDir, "output-dir", "delivery", "Local folder for MP4 and caption files")
	cmd.Flags().StringVar(&format, "format", "srt", "Caption format: srt or vtt")
	cmd.Flags().DurationVar(&poll, "poll-interval", 5*time.Second, "Poll interval (positive, at most 1m)")
	cmd.Flags().DurationVar(&timeout, "wait-timeout", 10*time.Minute, "Export wait deadline (positive, at most 1h)")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace existing regular delivery files")
	return cmd
}
