// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// since: time-windowed diff across LinkedIn + Happenstance. Answers the
// question "what has changed for me in the last <duration>?". The output is
// grouped by source and then by entity type so agents can pick the slices
// they care about.
//
// Because there is no official endpoint for "new LinkedIn 1st-degree
// connections since T", we diff against whatever is currently in the local
// `resources` store. For Happenstance we use /api/feed?unseen=true for feed,
// and recent research/search listings filtered client-side by timestamp.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"

	"github.com/spf13/cobra"
)

// SinceResult is the flat output of `since`.
type SinceResult struct {
	Window                  string            `json:"window"`
	WindowSince             time.Time         `json:"window_since"`
	Sources                 []string          `json:"sources"`
	LinkedInNewConnections  []json.RawMessage `json:"linkedin_new_connections"`
	HappenstanceNewFeed     []json.RawMessage `json:"happenstance_new_feed"`
	HappenstanceNewResearch []json.RawMessage `json:"happenstance_new_research"`
	HappenstanceNewSearches []json.RawMessage `json:"happenstance_new_searches"`
	LinkedInNote            string            `json:"linkedin_note,omitempty"`
	HappenstanceNote        string            `json:"happenstance_note,omitempty"`
	Counts                  map[string]int    `json:"counts"`
}

func newSinceCmd(flags *rootFlags) *cobra.Command {
	var sourcesCSV string
	var perSourceLimit int

	cmd := &cobra.Command{
		Use:   "since <duration>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Time-windowed diff of new items across LinkedIn + Happenstance",
		Long: `Show what's changed for you in the given time window (e.g. "24h", "7d", "2w").

Duration parsing supports Go's time.ParseDuration forms (h, m, s) plus a
convenience extension:
  - Ns         e.g. 90s
  - Nm         e.g. 30m
  - Nh         e.g. 24h
  - Nd         e.g. 7d   (days)
  - Nw         e.g. 2w   (weeks)`,
		Example: `  contact-goat-pp-cli since 24h
  contact-goat-pp-cli since 7d --json
  contact-goat-pp-cli since 2w --sources hp --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			dur, err := parseExtDuration(args[0])
			if err != nil {
				// Dry-run with a bad duration still reports intent (useful for agents previewing the call).
				if flags.dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "dry-run: since %s (invalid duration — use e.g. 24h, 7d, 2w)\n", args[0])
					return nil
				}
				// Non-dry-run with an unparseable duration: emit an empty result with a
				// human-readable note rather than a hard error. Exit code stays 0 so
				// scripted callers can still branch on the counts.
				empty := SinceResult{
					Window:           args[0],
					Sources:          []string{},
					Counts:           map[string]int{},
					HappenstanceNote: fmt.Sprintf("invalid duration %q — use e.g. 24h, 7d, 2w", args[0]),
					LinkedInNote:     fmt.Sprintf("invalid duration %q — use e.g. 24h, 7d, 2w", args[0]),
				}
				return emitSinceResult(cmd, flags, &empty)
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: since %s (cutoff %s UTC)\n", args[0], time.Now().Add(-dur).UTC().Format(time.RFC3339))
				return nil
			}
			sources := parseSourcesCSV(sourcesCSV)
			if len(sources) == 0 {
				return usageErr(fmt.Errorf("--sources must include at least one of: li, hp"))
			}

			srcList := make([]string, 0, len(sources))
			for k := range sources {
				srcList = append(srcList, k)
			}
			res := SinceResult{
				Window:      args[0],
				WindowSince: time.Now().Add(-dur).UTC(),
				Sources:     srcList,
				Counts:      map[string]int{},
			}

			if sources["hp"] {
				// Happenstance requires cookie auth. Failing here is not a
				// hard error because the caller may only be interested in LI.
				c, err := flags.newClientRequireCookies("happenstance")
				if err != nil {
					res.HappenstanceNote = "happenstance skipped: " + err.Error()
				} else {
					fillHappenstanceSince(c, flags, &res, dur, perSourceLimit)
				}
			}

			if sources["li"] {
				fillLinkedInSince(&res, dur, perSourceLimit)
			}

			res.Counts["linkedin_new_connections"] = len(res.LinkedInNewConnections)
			res.Counts["happenstance_new_feed"] = len(res.HappenstanceNewFeed)
			res.Counts["happenstance_new_research"] = len(res.HappenstanceNewResearch)
			res.Counts["happenstance_new_searches"] = len(res.HappenstanceNewSearches)

			return emitSinceResult(cmd, flags, &res)
		},
	}

	cmd.Flags().StringVar(&sourcesCSV, "sources", "li,hp", "Comma-separated sources to poll: li, hp")
	cmd.Flags().IntVar(&perSourceLimit, "limit", 20, "Max items to keep per source group")
	return cmd
}

func parseSourcesCSV(s string) map[string]bool {
	out := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		switch p {
		case "li", "linkedin":
			out["li"] = true
		case "hp", "happenstance":
			out["hp"] = true
		default:
			// ignore unknown tokens rather than fail — be permissive
		}
	}
	return out
}

// parseExtDuration extends time.ParseDuration with "d" (days) and "w" (weeks).
func parseExtDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	last := s[len(s)-1]
	// Try Go's ParseDuration first (handles "h", "m", "s", and combos).
	if last != 'd' && last != 'w' {
		d, err := time.ParseDuration(s)
		if err == nil {
			return d, nil
		}
	}
	nStr := s[:len(s)-1]
	n, err := strconv.Atoi(nStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use e.g. 24h, 7d, 2w)", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	switch last {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return 0, fmt.Errorf("invalid duration %q", s)
}

func fillHappenstanceSince(c *client.Client, _ *rootFlags, res *SinceResult, dur time.Duration, limit int) {
	// 1. Feed: unseen items surface naturally as "new". We request up to 50
	// and filter client-side by window just to be safe.
	if feed, err := c.Get("/api/feed", map[string]string{"unseen": "true", "limit": "50"}); err == nil {
		items := filterByTimestamp(extractArray(feed), res.WindowSince, limit)
		res.HappenstanceNewFeed = items
	} else {
		fmt.Fprintf(os.Stderr, "warning: happenstance feed fetch failed: %v\n", err)
	}

	// 2. Research: list recent and filter.
	if r, err := c.Get("/api/research/recent", nil); err == nil {
		res.HappenstanceNewResearch = filterByTimestamp(extractArray(r), res.WindowSince, limit)
	}

	// 3. Searches: recent dynamo searches.
	if r, err := c.Get("/api/dynamo/recent_searches", nil); err == nil {
		res.HappenstanceNewSearches = filterByTimestamp(extractArray(r), res.WindowSince, limit)
	} else if r2, err2 := c.Get("/api/search/recent", nil); err2 == nil {
		res.HappenstanceNewSearches = filterByTimestamp(extractArray(r2), res.WindowSince, limit)
	}

	_ = dur
}

func fillLinkedInSince(res *SinceResult, dur time.Duration, limit int) {
	// The LinkedIn MCP has no "new connections since T" endpoint, so we diff
	// against the local synced cache in the resources table. If no local
	// data exists, stamp a note and move on.
	s, err := openP2Store()
	if err != nil || s == nil {
		res.LinkedInNote = "no local linkedin cache — run `contact-goat-pp-cli linkedin ...` first, or sync"
		return
	}
	defer s.Close()

	items, err := s.ListRecentResources("linkedin_connections", res.WindowSince, limit)
	if err != nil || len(items) == 0 {
		// Fall back to any linkedin_person cache (get-person calls).
		items2, _ := s.ListRecentResources("linkedin_person", res.WindowSince, limit)
		if len(items2) == 0 {
			res.LinkedInNote = "LinkedIn MCP has no new-connections feed — nothing to diff locally either."
			_ = dur
			return
		}
		res.LinkedInNewConnections = items2
		return
	}
	res.LinkedInNewConnections = items
}

// extractArray tries hard to pull a []json.RawMessage out of an arbitrary
// response: a bare array, or a common wrapper envelope.
func extractArray(data json.RawMessage) []json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	for _, k := range []string{"data", "items", "results", "feed", "entries"} {
		if raw, ok := obj[k]; ok {
			if err := json.Unmarshal(raw, &arr); err == nil {
				return arr
			}
		}
	}
	return nil
}

// filterByTimestamp keeps items whose best-guess timestamp is >= since.
// It also applies the limit. Items with no timestamp are dropped when a
// window is active — we would rather under-report than lie about freshness.
func filterByTimestamp(items []json.RawMessage, since time.Time, limit int) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		ts, ok := extractTimestamp(m)
		if !ok {
			continue
		}
		if ts.Before(since) {
			continue
		}
		out = append(out, raw)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// extractTimestamp returns the first timestamp-looking field on an object.
func extractTimestamp(m map[string]any) (time.Time, bool) {
	for _, k := range []string{"created_at", "createdAt", "updated_at", "updatedAt", "timestamp", "time", "date", "inserted_at", "posted_at"} {
		if v, ok := m[k]; ok {
			if t, ok := coerceTime(v); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

func coerceTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t, true
			}
		}
	case float64:
		// Heuristic: unix seconds if < 1e12, else millis.
		if x > 1e12 {
			return time.Unix(int64(x)/1000, 0), true
		}
		if x > 0 {
			return time.Unix(int64(x), 0), true
		}
	case int64:
		if x > 1e12 {
			return time.Unix(x/1000, 0), true
		}
		return time.Unix(x, 0), true
	}
	return time.Time{}, false
}

func emitSinceResult(cmd *cobra.Command, flags *rootFlags, res *SinceResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "since %s (cutoff %s UTC)\n", res.Window, res.WindowSince.Format(time.RFC3339))
	fmt.Fprintf(w, "  linkedin_new_connections   %d\n", res.Counts["linkedin_new_connections"])
	fmt.Fprintf(w, "  happenstance_new_feed      %d\n", res.Counts["happenstance_new_feed"])
	fmt.Fprintf(w, "  happenstance_new_research  %d\n", res.Counts["happenstance_new_research"])
	fmt.Fprintf(w, "  happenstance_new_searches  %d\n", res.Counts["happenstance_new_searches"])
	if res.LinkedInNote != "" {
		fmt.Fprintf(w, "  [li note] %s\n", res.LinkedInNote)
	}
	if res.HappenstanceNote != "" {
		fmt.Fprintf(w, "  [hp note] %s\n", res.HappenstanceNote)
	}
	return nil
}
