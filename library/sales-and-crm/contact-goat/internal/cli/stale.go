// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// stale: find warm intros going cold — people we recently researched or
// looked up on LinkedIn but haven't actually talked to in a while. Useful for
// "who should I follow up with this week?" queries.
//
// Heuristic:
//   1. Scan person_touches for all persons with any touch in the last 90d.
//   2. For each, find the newest touch. If the newest touch is older than
//      --days, the person is "stale".
//   3. Rank by recency of FIRST interest (earliest touch) — the oldest
//      expressed interest that has cooled the longest rises to the top.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// StaleItem is the shape emitted for each stale warm intro.
type StaleItem struct {
	Name            string    `json:"name,omitempty"`
	PersonKey       string    `json:"person_key"`
	LinkedInURL     string    `json:"linkedin_url,omitempty"`
	LastResearched  time.Time `json:"last_researched"`
	DaysStale       int       `json:"days_stale"`
	LastTouchSource string    `json:"last_touch_source"`
	FirstTouch      time.Time `json:"first_touch"`
	TotalTouches    int       `json:"total_touches"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var staleDays, limit, lookbackDays int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find warm intros going cold - researched or looked up but not followed up on",
		Long: `Find people you've recently shown interest in (via LinkedIn lookup or
Happenstance research) but haven't touched in at least --days.

This surfaces "warm intros going cold" — names to revisit before they lose
context, ranked by how long ago you first expressed interest.`,
		Example: `  contact-goat-pp-cli stale
  contact-goat-pp-cli stale --days 7 --limit 50
  contact-goat-pp-cli stale --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if staleDays <= 0 {
				staleDays = 14
			}
			if lookbackDays <= 0 {
				lookbackDays = 90
			}

			s, err := openP2Store()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if s == nil {
				return fmt.Errorf("no local store. Run `contact-goat-pp-cli sync` first")
			}
			defer s.Close()

			keys, err := s.DistinctTouchPersonsSince(lookbackDays * 24)
			if err != nil {
				return fmt.Errorf("listing touch persons: %w", err)
			}
			if len(keys) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no recorded touches in the last 90 days. Run some `linkedin get-person` / `research` commands first, then try again.")
				if flags.asJSON {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode([]StaleItem{})
				}
				return nil
			}

			staleCutoff := time.Now().Add(-time.Duration(staleDays) * 24 * time.Hour)
			var items []StaleItem
			for _, k := range keys {
				touches, err := s.ListTouches(k, lookbackDays*24)
				if err != nil || len(touches) == 0 {
					continue
				}
				// ListTouches already returns newest-first.
				newest := touches[0]
				oldest := touches[len(touches)-1]
				if newest.EventTime.After(staleCutoff) {
					continue
				}
				item := StaleItem{
					PersonKey:       k,
					LinkedInURL:     extractLinkedInURLFromKey(k),
					LastResearched:  newest.EventTime,
					DaysStale:       int(time.Since(newest.EventTime).Hours() / 24),
					LastTouchSource: newest.Source,
					FirstTouch:      oldest.EventTime,
					TotalTouches:    len(touches),
					Name:            extractNameFromTouch(newest.Data),
				}
				items = append(items, item)
			}
			// Rank by how recently we first showed interest — fresh-interest-
			// then-silence (newest FirstTouch) goes to the top because that's
			// where the context is still loadable.
			sort.Slice(items, func(i, j int) bool {
				return items[i].FirstTouch.After(items[j].FirstTouch)
			})
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			return emitStale(cmd, flags, items)
		},
	}

	cmd.Flags().IntVar(&staleDays, "days", 14, "Stale threshold in days (newest touch must be older than this)")
	cmd.Flags().IntVar(&lookbackDays, "lookback", 90, "How far back to look for interest signals (days)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	return cmd
}

func extractLinkedInURLFromKey(key string) string {
	if len(key) >= 12 && key[:12] == "https://www." {
		return key
	}
	if len(key) >= 8 && key[:8] == "https://" {
		return key
	}
	return ""
}

func extractNameFromTouch(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	for _, k := range []string{"name", "full_name", "fullName", "display_name", "title"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func emitStale(cmd *cobra.Command, flags *rootFlags, items []StaleItem) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no stale warm intros — you're on top of your follow-ups!)")
		return nil
	}
	rows := make([]map[string]any, 0, len(items))
	for _, it := range items {
		rows = append(rows, map[string]any{
			"name":          it.Name,
			"person_key":    it.PersonKey,
			"days_stale":    it.DaysStale,
			"last_source":   it.LastTouchSource,
			"total_touches": it.TotalTouches,
			"first_touch":   it.FirstTouch.Format(time.RFC3339),
		})
	}
	return printAutoTable(cmd.OutOrStdout(), rows)
}
