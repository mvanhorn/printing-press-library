// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func newCaptionsFetchCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "fetch [nasa_id]",
		Short: "Fetch the actual caption text for a video asset (not just the URL)",
		Long: `Follow the /captions/{nasa_id} indirection: fetch the location URL,
GET the .srt or .vtt body, and print it.

Format:
  srt   Subrip subtitles (sequence + timecodes + text).
  vtt   WebVTT subtitles (HTTP Live Streaming text track).
  text  Strip cue numbers and timecodes, keep only the spoken text.

Errors out with status 400 when the asset is not a video (NASA enforces this);
status 404 when the asset has no captions.`,
		Example:     "  nasa-images-pp-cli captions fetch jsc2022m000123 --format text",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			nasaID := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch captions for %q\n", nasaID)
				return nil
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/captions/"+nasaID, nil)
			if err != nil {
				return fmt.Errorf("calling /captions/%s: %w", nasaID, err)
			}
			var loc struct {
				Location string `json:"location"`
			}
			if err := json.Unmarshal(raw, &loc); err != nil {
				return fmt.Errorf("parsing captions indirection: %w", err)
			}
			if loc.Location == "" {
				return fmt.Errorf("captions response has no location URL")
			}
			body, err := httpGetBody(ctx, flags, loc.Location)
			if err != nil {
				return fmt.Errorf("fetching captions body: %w", err)
			}
			text := string(body)
			switch strings.ToLower(format) {
			case "srt", "vtt":
				// pass through
			case "text":
				text = stripCueArtifacts(text)
			default:
				return fmt.Errorf("invalid --format %q: must be srt, vtt, or text", format)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet && !flags.plain && !flags.csv) {
				result := map[string]any{
					"nasa_id":  nasaID,
					"format":   format,
					"location": upgradeToHTTPS(loc.Location),
					"content":  text,
				}
				return flags.printJSON(cmd, result)
			}
			fmt.Fprint(cmd.OutOrStdout(), text)
			if !strings.HasSuffix(text, "\n") {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "srt", "Output format: srt, vtt, or text (strips cue numbers/timecodes)")
	return cmd
}

// srtCueNumber matches the standalone sequence-number lines in a SubRip file.
var srtCueNumberRE = regexp.MustCompile(`(?m)^\d+\s*$`)

// timecode matches SRT and WebVTT timecode lines.
var timecodeRE = regexp.MustCompile(`(?m)^\d{1,2}:\d{2}:\d{2}[.,]\d{3}\s*-->\s*\d{1,2}:\d{2}:\d{2}[.,]\d{3}.*$`)

// vttHeader matches the WebVTT magic header.
var vttHeaderRE = regexp.MustCompile(`(?m)^WEBVTT.*$`)

func stripCueArtifacts(s string) string {
	s = vttHeaderRE.ReplaceAllString(s, "")
	s = timecodeRE.ReplaceAllString(s, "")
	s = srtCueNumberRE.ReplaceAllString(s, "")
	// Collapse runs of blank lines that the strips above leave behind.
	blankRE := regexp.MustCompile(`\n{3,}`)
	s = blankRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s) + "\n"
}
