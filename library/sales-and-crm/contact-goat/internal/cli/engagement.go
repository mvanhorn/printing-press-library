// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// engagement <person-id-or-url>: score last-touch engagement across LinkedIn,
// Happenstance, and Deepline. Modeled after hubspot-pp-cli's engagement
// scoring — a single number 0-100 plus the raw event log that justifies it.
//
// The event log is materialized from the person_touches table (populated by
// sync and by individual `contact-goat linkedin ...` / `happenstance
// research ...` commands). If no touches are recorded we still surface a
// useful result: a score of 0 plus instructions for how to start logging.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/store"

	"github.com/spf13/cobra"
)

// EngagementResult is the flat output of `engagement`.
type EngagementResult struct {
	Person     string            `json:"person"`
	PersonKey  string            `json:"person_key"`
	Score      int               `json:"score"`
	LastTouch  *time.Time        `json:"last_touch,omitempty"`
	Events     []EngagementEvent `json:"events"`
	WindowDays int               `json:"window_days"`
	Note       string            `json:"note,omitempty"`
}

// EngagementEvent is one materialized touch event.
type EngagementEvent struct {
	Source    string    `json:"source"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
}

func newEngagementCmd(flags *rootFlags) *cobra.Command {
	var sinceWindow string

	cmd := &cobra.Command{
		Use:   "engagement <person-id-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Score last-touch engagement with a person across all sources",
		Long: `Compute a 0-100 engagement score based on the most recent interaction with
a person across LinkedIn (messages, profile views), Happenstance (research,
feed appearances), and Deepline (enrich calls).

Scoring thresholds:
  100  touched in the last 24h
   50  touched in the last 7 days
   20  touched in the last 30 days
    5  older than 30 days`,
		Example: `  contact-goat-pp-cli engagement https://www.linkedin.com/in/patrickcollison/
  contact-goat-pp-cli engagement patrick-collison-uuid --json
  contact-goat-pp-cli engagement satyanadella --since 30d`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			winDur, err := parseExtDuration(sinceWindow)
			if err != nil {
				return usageErr(err)
			}
			hours := int(winDur.Hours())
			if hours <= 0 {
				hours = 24 * 90
			}

			s, err := openP2Store()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if s == nil {
				return fmt.Errorf("no local store. Run `contact-goat-pp-cli sync` first to populate data")
			}
			defer s.Close()

			key := canonicalPersonKey(args[0])
			touches, err := s.ListTouches(key, hours)
			if err != nil {
				return fmt.Errorf("listing touches: %w", err)
			}

			result := EngagementResult{
				Person:     args[0],
				PersonKey:  key,
				WindowDays: hours / 24,
			}
			if len(touches) == 0 {
				result.Note = "no recorded touches in the given window. Touches are logged by " +
					"sync and by top-level linkedin / research / deepline commands. " +
					"Run a sync or any of those commands to start populating engagement history."
				return emitEngagement(cmd, flags, &result)
			}
			result.Events = make([]EngagementEvent, 0, len(touches))
			for _, t := range touches {
				result.Events = append(result.Events, EngagementEvent{
					Source:    t.Source,
					Type:      t.EventType,
					Timestamp: t.EventTime,
				})
			}
			// Events already come back DESC by event_time.
			latest := result.Events[0].Timestamp
			result.LastTouch = &latest
			result.Score = scoreLastTouch(latest)

			// Ensure deterministic order for JSON consumers.
			sort.Slice(result.Events, func(i, j int) bool {
				return result.Events[i].Timestamp.After(result.Events[j].Timestamp)
			})
			return emitEngagement(cmd, flags, &result)
		},
	}

	cmd.Flags().StringVar(&sinceWindow, "since", "90d", "Analysis window (e.g. 24h, 7d, 90d)")
	return cmd
}

// scoreLastTouch is intentionally simple — a lookup table that anyone can
// tune. Anthropic-style: explicit, readable, easy to test.
func scoreLastTouch(t time.Time) int {
	age := time.Since(t)
	switch {
	case age < 24*time.Hour:
		return 100
	case age < 7*24*time.Hour:
		return 50
	case age < 30*24*time.Hour:
		return 20
	default:
		return 5
	}
}

// canonicalPersonKey normalizes the input to the form we use in person_touches.
// For URLs we strip trailing slashes and query strings; for bare slugs we keep
// them lowercase. Agents can use either a LinkedIn URL or a Happenstance UUID.
func canonicalPersonKey(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "linkedin.com/") {
		// Strip query and hash.
		if i := strings.IndexAny(s, "?#"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimRight(s, "/")
	}
	return strings.ToLower(s)
}

func emitEngagement(cmd *cobra.Command, flags *rootFlags, res *EngagementResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Engagement for %s\n", res.Person)
	fmt.Fprintf(w, "  score       %d\n", res.Score)
	if res.LastTouch != nil {
		fmt.Fprintf(w, "  last touch  %s\n", res.LastTouch.Format(time.RFC3339))
	}
	fmt.Fprintf(w, "  window      %dd\n", res.WindowDays)
	fmt.Fprintf(w, "  events      %d\n", len(res.Events))
	for _, e := range res.Events {
		fmt.Fprintf(w, "    %s  %-14s %s\n", e.Timestamp.Format(time.RFC3339), e.Source+"/"+e.Type, "")
	}
	if res.Note != "" {
		fmt.Fprintf(w, "  note: %s\n", res.Note)
	}
	return nil
}

// recordTouchSafely is a shared helper other commands can use to log a touch
// without aborting on store errors. Keeps this package's top-level commands
// DRY; the store layer already guards against missing tables.
func recordTouchSafely(personKey, source, eventType string, data any) {
	s, err := openP2Store()
	if err != nil || s == nil {
		return
	}
	defer s.Close()
	var raw json.RawMessage
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			raw = b
		}
	}
	_ = s.RecordTouch(personKey, source, eventType, time.Now().UTC(), raw)
}

// Ensure unused-var guard is off.
var _ = (*store.Store)(nil)
