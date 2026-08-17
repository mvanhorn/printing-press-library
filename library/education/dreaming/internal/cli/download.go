// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// download: resolve a video's playable stream. Prints the signed HLS URL (and
// YouTube id) plus a ready ffmpeg command by default; --exec downloads via
// ffmpeg/yt-dlp when present. Print-by-default, verify-safe side effect.
// Hand-built feature (parity with yt-dlp-dreaming). Content is DRM-free.

package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/cliutil"

	"github.com/spf13/cobra"
)

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var doExec bool
	var outPath string

	cmd := &cobra.Command{
		Use:   "download <video-id>",
		Short: "Resolve a video's downloadable stream (prints the URL; --exec downloads via ffmpeg)",
		Long: "Resolve a video's playable HLS stream and print the signed master URL, the\n" +
			"YouTube id (for free videos), and a ready ffmpeg command. By default it only\n" +
			"prints — pass --exec to actually download with ffmpeg or yt-dlp when one is\n" +
			"on PATH. Dreaming content is DRM-free; stream segments are URL-signed.",
		Example: strings.Trim(`
  dreaming-pp-cli download 5f3a1b2c4d5e6f7a8b9c0d1e
  dreaming-pp-cli download 5f3a1b2c4d5e6f7a8b9c0d1e --exec --out lesson.mp4
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vd, err := fetchVideoDetail(cmd.Context(), c, id)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if vd.Sources.Bunny == "" && vd.Sources.Youtube == "" {
				return notFoundErr(fmt.Errorf("no stream source found for video %s", id))
			}
			out := outPath
			if out == "" {
				out = id + ".mp4"
			}
			ffmpegCmd := ""
			if vd.Sources.Bunny != "" {
				ffmpegCmd = fmt.Sprintf("ffmpeg -i %q -c copy %q", vd.Sources.Bunny, out)
			}

			// Default + verify path: print only, never invoke a downloader.
			if !doExec || cliutil.IsVerifyEnv() {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"video_id":      id,
						"title":         vd.Title,
						"hls_url":       vd.Sources.Bunny,
						"youtube_id":    vd.Sources.Youtube,
						"ffmpeg":        ffmpegCmd,
						"suggested_out": out,
					}, flags)
				}
				if vd.Sources.Bunny != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "HLS stream: %s\n", vd.Sources.Bunny)
					fmt.Fprintf(cmd.OutOrStdout(), "Download:   %s\n", ffmpegCmd)
				}
				if vd.Sources.Youtube != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "YouTube:    https://youtu.be/%s\n", vd.Sources.Youtube)
				}
				if doExec && cliutil.IsVerifyEnv() {
					fmt.Fprintln(cmd.OutOrStdout(), "(verify mode: not invoking ffmpeg)")
				}
				return nil
			}

			// --exec path: download with ffmpeg or yt-dlp if available.
			if vd.Sources.Bunny == "" {
				return apiErr(fmt.Errorf("no HLS source to download; this free video is on YouTube: https://youtu.be/%s", vd.Sources.Youtube))
			}
			tool, toolArgs := pickDownloader(vd.Sources.Bunny, out)
			if tool == "" {
				return configErr(fmt.Errorf("neither ffmpeg nor yt-dlp found on PATH; install one, or run without --exec and copy the printed command"))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Running: %s %s\n", tool, strings.Join(toolArgs, " "))
			ex := exec.CommandContext(cmd.Context(), tool, toolArgs...)
			ex.Stdout = cmd.OutOrStdout()
			ex.Stderr = cmd.ErrOrStderr()
			if err := ex.Run(); err != nil {
				return apiErr(fmt.Errorf("%s failed: %w", tool, err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %s\n", out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&doExec, "exec", false, "Actually download via ffmpeg/yt-dlp (default: print the URL and command only)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (default: <video-id>.mp4)")
	return cmd
}

func pickDownloader(url, out string) (string, []string) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, []string{"-i", url, "-c", "copy", out}
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, []string{url, "-o", out}
	}
	return "", nil
}
