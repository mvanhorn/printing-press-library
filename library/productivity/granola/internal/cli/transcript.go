// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

func newTranscriptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcript",
		Short: "Read a meeting transcript from the cache or live API",
	}
	cmd.AddCommand(newTranscriptGetCmd(flags))
	return cmd
}

func newTranscriptGetCmd(flags *rootFlags) *cobra.Command {
	var speaker bool
	var format, since string
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get the transcript for a meeting",
		Long: `Returns the cached transcript when available, falling back to the
live internal API. --format=json|text|srt. --speaker prefixes lines with
the source (microphone/system). --since 1:30 trims to segments after the
M:SS mark.`,
		Example: `  # Plain transcript text
  granola-pp-cli transcript get ff1186df-593b-4ce5-bb1d-70e265f4a811 --format text

  # Speaker-labeled (microphone vs system)
  granola-pp-cli transcript get ff1186df-593b-4ce5-bb1d-70e265f4a811 --format text --speaker

  # SRT for upload to a captioning tool
  granola-pp-cli transcript get ff1186df-593b-4ce5-bb1d-70e265f4a811 --format srt

  # Skip the first 90 seconds (intros)
  granola-pp-cli transcript get ff1186df-593b-4ce5-bb1d-70e265f4a811 --since 1:30 --format text`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			segs, source, err := loadTranscript(cmd.Context(), id, flags.dataSource)
			if err != nil {
				return err
			}
			if since != "" {
				cut, err := parseClock(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				segs = trimSegmentsAfter(segs, cut)
			}
			switch format {
			case "json", "":
				if flags.asJSON || flags.agent || format == "json" {
					out := map[string]any{
						"document_id": id,
						"source":      source,
						"segments":    segs,
					}
					return emitJSON(cmd, flags, out)
				}
				fallthrough
			case "text":
				w := cmd.OutOrStdout()
				for _, s := range segs {
					if speaker {
						fmt.Fprintf(w, "[%s] %s\n", s.Source, s.Text)
					} else {
						fmt.Fprintf(w, "%s\n", s.Text)
					}
				}
				return nil
			case "srt":
				w := cmd.OutOrStdout()
				for i, s := range segs {
					st, _ := granola.ParseISO(s.StartTimestamp)
					en, _ := granola.ParseISO(s.EndTimestamp)
					fmt.Fprintf(w, "%d\n%s --> %s\n", i+1, srtTime(st), srtTime(en))
					if speaker {
						fmt.Fprintf(w, "[%s] %s\n\n", s.Source, s.Text)
					} else {
						fmt.Fprintf(w, "%s\n\n", s.Text)
					}
				}
				return nil
			default:
				return usageErr(fmt.Errorf("invalid --format %q: must be json, text, or srt", format))
			}
		},
	}
	cmd.Flags().BoolVar(&speaker, "speaker", false, "Prefix each line with the source label")
	cmd.Flags().StringVar(&format, "format", "", "Output format: json | text | srt (default: json with --json, else text)")
	cmd.Flags().StringVar(&since, "since", "", "Trim to segments after M:SS from meeting start")
	return cmd
}

// loadTranscript returns segments + a string describing the source
// ("store", "cache", or "live"). Honors dataSource.
//
// PATCH(dual-path-store-read): the local read now goes through granolaRead,
// so segments the API sync hydrated into transcript_segments are reachable
// without a decryptable desktop cache and without an API key.
//
// The live fallback is skipped when the desktop cache proved unreadable: the
// internal API authenticates with the same desktop-owned credential the key
// migration invalidated, so attempting it there can only produce a
// safestorage refusal that tells the user nothing actionable. A readable
// cache means a legacy install where the internal API may still work, so the
// fallback is preserved there.
func loadTranscript(ctx context.Context, id, dataSource string) ([]granola.TranscriptSegment, string, error) {
	if dataSource != "live" {
		v, err := openGranolaRead(ctx)
		if err != nil {
			return nil, "", err
		}
		defer v.Close()
		if segs, src := v.transcriptWithSource(id); len(segs) > 0 {
			return segs, src, nil
		}
		if dataSource == "local" || !v.hasCache() {
			return nil, "", notFoundErr(fmt.Errorf("no transcript for %s in the local store; run `granola-pp-cli sync-api` (or `granola-pp-cli sync` for a desktop-cache install) first", id))
		}
	}
	ic, err := granola.NewInternalClient()
	if err != nil {
		return nil, "", authErr(err)
	}
	segs, err := ic.GetDocumentTranscript(id)
	if err != nil {
		return nil, "", apiErr(err)
	}
	return segs, "live", nil
}

// parseClock parses "M:SS" or "H:MM:SS" into a Duration.
func parseClock(s string) (time.Duration, error) {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		var m, sec int
		if _, err := fmt.Sscanf(s, "%d:%d", &m, &sec); err != nil {
			return 0, err
		}
		return time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	case 3:
		var h, m, sec int
		if _, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec); err != nil {
			return 0, err
		}
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
	}
	return 0, fmt.Errorf("expected M:SS or H:MM:SS")
}

func trimSegmentsAfter(segs []granola.TranscriptSegment, cut time.Duration) []granola.TranscriptSegment {
	if len(segs) == 0 || cut == 0 {
		return segs
	}
	first, err := granola.ParseISO(segs[0].StartTimestamp)
	if err != nil {
		return segs
	}
	threshold := first.Add(cut)
	out := segs[:0:len(segs)]
	for _, s := range segs {
		t, err := granola.ParseISO(s.StartTimestamp)
		if err == nil && !t.Before(threshold) {
			out = append(out, s)
		}
	}
	return out
}

func srtTime(t time.Time) string {
	if t.IsZero() {
		return "00:00:00,000"
	}
	h := t.Hour()
	m := t.Minute()
	s := t.Second()
	ms := t.Nanosecond() / 1_000_000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
