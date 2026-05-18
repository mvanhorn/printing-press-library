// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH(upstream cli-printing-press#918): added long-form --help text
// clarifying the rollup is sourced from the webhook event stream, not
// the Video.views counter — empty results don't mean zero views.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newVideosViewedCmd rolls up webhook view-milestone events into a per-video
// summary over a window. Pulls the recent webhook message inbox live and
// filters to entries that look like view/milestone events.
func newVideosViewedCmd(flags *rootFlags) *cobra.Command {
	var since string
	var milestone int
	var limit int
	cmd := &cobra.Command{
		Use:   "viewed",
		Short: "Roll up webhook view-milestone events by video",
		Long: `Roll up viewer activity by reading the webhook inbox (video.viewed,
video.milestone) over a window. Returns one entry per video with hit counts.

Note: this is sourced from your webhook *event stream* — videos with views
recorded only in the API counter (Video.views) but no inbox events will not
appear here. Wire a webhook endpoint and let it accumulate to populate this.`,
		Example:     "  tella-pp-cli videos viewed --since 7d --milestone 75 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			windowStart, err := parseSinceWindow(since)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/v1/webhooks/messages", map[string]string{
				"limit": strconv.Itoa(limit),
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			messages := extractMessageObjects(data)
			type rollup struct {
				VideoID       string           `json:"video_id"`
				Name          string           `json:"name"`
				MilestoneHits int              `json:"milestone_hits"`
				ViewersCount  int              `json:"viewers_count"`
				Events        []map[string]any `json:"events,omitempty"`
			}
			byVideo := map[string]*rollup{}
			for _, m := range messages {
				if !isViewMilestoneEvent(m) {
					continue
				}
				ts := messageTime(m)
				if !windowStart.IsZero() && !ts.IsZero() && ts.Before(windowStart) {
					continue
				}
				ms := extractMilestone(m)
				if milestone > 0 && ms < milestone {
					continue
				}
				vid := extractVideoIDFromMessage(m)
				if vid == "" {
					continue
				}
				r, ok := byVideo[vid]
				if !ok {
					r = &rollup{VideoID: vid, Name: extractVideoNameFromMessage(m)}
					byVideo[vid] = r
				}
				r.MilestoneHits++
				if v := extractViewersCount(m); v > 0 {
					if v > r.ViewersCount {
						r.ViewersCount = v
					}
				}
			}
			out := make([]rollup, 0, len(byVideo))
			for _, r := range byVideo {
				out = append(out, *r)
			}
			result := map[string]any{
				"window_start": formatWindow(windowStart),
				"milestone":    milestone,
				"videos":       out,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: Go duration (1h30m) or shorthand <N><unit> with unit in m/h/d/w (15m, 1h, 7d, 2w); pass an empty string for no window")
	cmd.Flags().IntVar(&milestone, "milestone", 0, "Minimum milestone percentage threshold (0 = any)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max webhook messages to scan")
	return cmd
}

// parseSinceWindow converts strings like "7d", "1h", "15m" into a wall-clock
// start. Returns (zero time, nil) when the input is empty — that's the
// caller's "no window" sentinel. Returns a parse error for any other
// unrecognized value so a typo (`--since yesterday`, `--since 2 weeks`)
// doesn't silently widen the rollup to the entire event stream.
func parseSinceWindow(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	// Accept Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try suffixes like "7d", "30d", "2w"
	if len(s) >= 2 {
		unit := s[len(s)-1]
		num, err := strconv.Atoi(s[:len(s)-1])
		if err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(num) * 24 * time.Hour), nil
			case 'h':
				return time.Now().Add(-time.Duration(num) * time.Hour), nil
			case 'm':
				return time.Now().Add(-time.Duration(num) * time.Minute), nil
			case 'w':
				return time.Now().Add(-time.Duration(num) * 7 * 24 * time.Hour), nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q: expected a Go duration (e.g. 1h30m) or shorthand <N><unit> with unit in m/h/d/w (e.g. 15m, 1h, 7d, 2w); pass an empty value for no window", s)
}

func formatWindow(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// extractMessageObjects accepts either an array or an envelope `{messages|data: [...]}`.
func extractMessageObjects(data json.RawMessage) []map[string]any {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(data, &env); err == nil {
		for _, key := range []string{"messages", "data", "items", "results"} {
			if raw, ok := env[key]; ok {
				var inner []map[string]any
				if err := json.Unmarshal(raw, &inner); err == nil {
					return inner
				}
			}
		}
	}
	return nil
}

func isViewMilestoneEvent(m map[string]any) bool {
	for _, key := range []string{"eventType", "event_type", "type", "event"} {
		if v, ok := m[key].(string); ok {
			lower := strings.ToLower(v)
			if strings.Contains(lower, "view") || strings.Contains(lower, "milestone") {
				return true
			}
		}
	}
	// Look for a milestone field anywhere in the payload data.
	for _, key := range []string{"data", "payload"} {
		if nested, ok := m[key].(map[string]any); ok {
			if _, ok := nested["milestone"]; ok {
				return true
			}
			if _, ok := nested["viewMilestone"]; ok {
				return true
			}
		}
	}
	return false
}

func messageTime(m map[string]any) time.Time {
	for _, key := range []string{"createdAt", "created_at", "timestamp", "sentAt", "sent_at"} {
		if v, ok := m[key].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func extractMilestone(m map[string]any) int {
	candidates := []map[string]any{m}
	for _, key := range []string{"data", "payload"} {
		if n, ok := m[key].(map[string]any); ok {
			candidates = append(candidates, n)
		}
	}
	for _, c := range candidates {
		for _, key := range []string{"milestone", "viewMilestone", "view_milestone", "percentage"} {
			if v, ok := c[key]; ok {
				switch x := v.(type) {
				case float64:
					return int(x)
				case string:
					if n, err := strconv.Atoi(strings.TrimSuffix(x, "%")); err == nil {
						return n
					}
				}
			}
		}
	}
	return 0
}

func extractVideoIDFromMessage(m map[string]any) string {
	candidates := []map[string]any{m}
	for _, key := range []string{"data", "payload"} {
		if n, ok := m[key].(map[string]any); ok {
			candidates = append(candidates, n)
		}
	}
	for _, c := range candidates {
		for _, key := range []string{"videoId", "video_id", "videoID"} {
			if v, ok := c[key].(string); ok && v != "" {
				return v
			}
		}
		// Nested {video: {id: ...}}
		if vid, ok := c["video"].(map[string]any); ok {
			if id, ok := vid["id"].(string); ok && id != "" {
				return id
			}
		}
	}
	return ""
}

func extractVideoNameFromMessage(m map[string]any) string {
	candidates := []map[string]any{m}
	for _, key := range []string{"data", "payload"} {
		if n, ok := m[key].(map[string]any); ok {
			candidates = append(candidates, n)
		}
	}
	for _, c := range candidates {
		for _, key := range []string{"videoName", "video_name", "name", "title"} {
			if v, ok := c[key].(string); ok && v != "" {
				return v
			}
		}
		if vid, ok := c["video"].(map[string]any); ok {
			if n, ok := vid["name"].(string); ok && n != "" {
				return n
			}
		}
	}
	return ""
}

func extractViewersCount(m map[string]any) int {
	candidates := []map[string]any{m}
	for _, key := range []string{"data", "payload"} {
		if n, ok := m[key].(map[string]any); ok {
			candidates = append(candidates, n)
		}
	}
	for _, c := range candidates {
		for _, key := range []string{"viewersCount", "viewers_count", "uniqueViewers", "unique_viewers"} {
			if v, ok := c[key]; ok {
				if f, ok := v.(float64); ok {
					return int(f)
				}
			}
		}
	}
	return 0
}

// formatTimeRFC3339 is a small wrapper so callers can format a parsed time
// uniformly without dragging the time package into every command file.
func formatTimeRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
