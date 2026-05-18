// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newTalktimeCmd(flags *rootFlags) *cobra.Command {
	var by, since, until, last string
	cmd := &cobra.Command{
		Use:   "talktime [<meeting-id>]",
		Short: "Per-source talk time for one meeting or rolled up across meetings",
		Long: `Without args: returns microphone vs system seconds for one meeting.
With --by participant --since DATE: aggregates across meetings, attributing
to source for 2-attendee meetings and rolling up otherwise.`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				id := args[0]
				segs := c.TranscriptByID(id)
				if len(segs) == 0 {
					return notFoundErr(fmt.Errorf("no cached transcript for %s", id))
				}
				agg := aggregateBySources(segs)
				return emitJSON(cmd, flags, agg)
			}
			from, to, err := parseTimeWindow(last, since, until)
			if err != nil {
				return usageErr(err)
			}
			ids := selectDocsInWindow(c, from, to, 0)
			if by == "" || by == "participant" {
				// Per-email aggregation. For 2-attendee meetings, microphone =
				// the "creator" email; system = the other attendee. For meetings
				// with >2 attendees we cannot disambiguate system-source speakers
				// from the transcript alone, so system seconds split evenly across
				// non-creator attendees and the key is prefixed "rollup:" so the
				// user knows attribution is heuristic, not from speaker diarization.
				type row struct {
					micSec float64
					sysSec float64
				}
				out := map[string]*row{}
				bump := func(key string, mic, sys float64) {
					r, ok := out[key]
					if !ok {
						r = &row{}
						out[key] = r
					}
					r.micSec += mic
					r.sysSec += sys
				}
				for _, id := range ids {
					segs := c.TranscriptByID(id)
					if len(segs) == 0 {
						continue
					}
					md := c.MeetingMetadataByID(id)
					micOwner := ""
					if md != nil && md.Creator != nil {
						micOwner = strings.ToLower(md.Creator.Email)
					}
					var others []string
					if md != nil {
						for _, a := range md.Attendees {
							e := strings.ToLower(a.Email)
							if e != "" && e != micOwner {
								others = append(others, e)
							}
						}
					}
					micSec, sysSec := sourceSeconds(segs)
					switch {
					case micOwner != "" && len(others) == 1:
						bump(micOwner, micSec, 0)
						bump(others[0], 0, sysSec)
					case micOwner != "" && len(others) > 1:
						bump(micOwner, micSec, 0)
						share := sysSec / float64(len(others))
						for _, e := range others {
							bump("rollup:"+e, 0, share)
						}
					default:
						bump("rollup:source:microphone", micSec, 0)
						bump("rollup:source:system", 0, sysSec)
					}
				}
				resp := []map[string]any{}
				for k, r := range out {
					total := r.micSec + r.sysSec
					resp = append(resp, map[string]any{
						"key":            k,
						"mic_seconds":    r.micSec,
						"system_seconds": r.sysSec,
						"total_seconds":  total,
						"total_minutes":  total / 60.0,
					})
				}
				return emitJSON(cmd, flags, resp)
			}
			return usageErr(fmt.Errorf("invalid --by %q", by))
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "Aggregate by: participant (default)")
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	cmd.Flags().StringVar(&until, "until", "", "End date")
	cmd.Flags().StringVar(&last, "last", "", "Time window (e.g. 7d)")
	return cmd
}

// aggregateBySources returns per-source totals for one transcript.
func aggregateBySources(segs []granola.TranscriptSegment) map[string]any {
	micSec, sysSec := sourceSeconds(segs)
	avgConf := 0.0
	if len(segs) > 0 {
		sum := 0.0
		for _, s := range segs {
			sum += s.Confidence
		}
		avgConf = sum / float64(len(segs))
	}
	return map[string]any{
		"microphone_seconds": micSec,
		"system_seconds":     sysSec,
		"total_seconds":      micSec + sysSec,
		"microphone_minutes": micSec / 60.0,
		"system_minutes":     sysSec / 60.0,
		"segment_count":      len(segs),
		"confidence_avg":     avgConf,
	}
}

// sourceSeconds totals seconds per source. Returns (micSec, sysSec).
func sourceSeconds(segs []granola.TranscriptSegment) (float64, float64) {
	var mic, sys float64
	for _, s := range segs {
		dur := segSeconds(s)
		switch strings.ToLower(s.Source) {
		case "microphone", "mic":
			mic += dur
		case "system", "speakers":
			sys += dur
		}
	}
	return mic, sys
}

func segSeconds(s granola.TranscriptSegment) float64 {
	st, err1 := granola.ParseISO(s.StartTimestamp)
	en, err2 := granola.ParseISO(s.EndTimestamp)
	if err1 != nil || err2 != nil || st.IsZero() || en.IsZero() {
		return 0
	}
	d := en.Sub(st).Seconds()
	if d < 0 {
		return 0
	}
	return d
}

// Ensure time imported.
var _ = time.Now
