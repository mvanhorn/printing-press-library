// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: keyword/objection pattern report over cached calls.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type patternMatch struct {
	ConversationID string `json:"conversationId"`
	Title          string `json:"title,omitempty"`
	StartTime      string `json:"startTime,omitempty"`
}

type patternGroup struct {
	DealName   string         `json:"dealName"`
	Stage      string         `json:"stage,omitempty"`
	MatchCount int            `json:"matchCount"`
	Calls      []patternMatch `json:"calls"`
}

func newNovelPatternsCmd(flags *rootFlags) *cobra.Command {
	var term string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "patterns",
		Short: "Count and locate transcript mentions of a term, grouped by deal and stage.",
		Long: `Scan cached conversations for a term and group the matches by the deal (and its
stage) the call is linked to. Use it to see where competitor mentions, pricing
objections, or "legal"/"contract" talk cluster across the pipeline.

--term accepts pipe-separated alternatives, matched case-insensitively
(e.g. "pricing|discount|competitor"). The scan covers each call's cached
content: titles always, plus summary and transcript text when conversation
detail has been synced. Run 'sync' first.`,
		Example: strings.Trim(`
  # Where pricing and competitor talk shows up
  sybill-pp-cli patterns --term "pricing|discount|competitor"

  # Legal/contract mentions in the last 30 days, as JSON
  sybill-pp-cli patterns --term "legal|contract|redline" --since 30d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			term = strings.TrimSpace(term)
			if term == "" {
				return fmt.Errorf("--term is required (e.g. --term \"pricing|competitor\")")
			}
			var needles []string
			for _, t := range strings.Split(term, "|") {
				if t = strings.ToLower(strings.TrimSpace(t)); t != "" {
					needles = append(needles, t)
				}
			}
			if len(needles) == 0 {
				return fmt.Errorf("--term contained no searchable text")
			}

			now := time.Now().UTC()
			var cutoff time.Time
			hasCutoff := false
			if strings.TrimSpace(since) != "" {
				c, err := parseSince(since, now)
				if err != nil {
					return err
				}
				cutoff, hasCutoff = c, true
			}

			out := cmd.OutOrStdout()
			if dbPath == "" {
				dbPath = defaultDBPath("sybill-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sybill-pp-cli sync' first.", err)
			}
			defer db.Close()

			deals, err := loadRecords(db, "deals")
			if err != nil {
				return err
			}
			convs, err := loadRecords(db, "conversations")
			if err != nil {
				return err
			}
			if len(convs) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No conversations in the local store. Run 'sybill-pp-cli sync' first.")
			}

			groups := map[string]*patternGroup{}
			total := 0
			for _, c := range convs {
				if hasCutoff {
					if t, ok := convStart(c); ok && t.Before(cutoff) {
						continue
					}
				}
				hay := searchableText(c)
				if !matchesAny(hay, needles) {
					continue
				}
				total++

				dealName := "(no linked deal)"
				stage := ""
				for _, d := range deals {
					if convMatchesDeal(c, d) {
						dealName = dealName2(d)
						stage = dealStage(d)
						break
					}
				}
				key := dealName + "\x00" + stage
				g := groups[key]
				if g == nil {
					g = &patternGroup{DealName: dealName, Stage: stage}
					groups[key] = g
				}
				g.MatchCount++
				start := ""
				if t, ok := convStart(c); ok {
					start = t.Format(time.RFC3339)
				}
				g.Calls = append(g.Calls, patternMatch{ConversationID: convID(c), Title: convTitle(c), StartTime: start})
			}

			results := make([]patternGroup, 0, len(groups))
			for _, g := range groups {
				results = append(results, *g)
			}
			sort.SliceStable(results, func(i, j int) bool { return results[i].MatchCount > results[j].MatchCount })

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, results, flags)
			}
			if total == 0 {
				fmt.Fprintf(out, "No calls mention %q.\n", term)
				return nil
			}
			fmt.Fprintf(out, "%-32s  %-16s  %s\n", "DEAL", "STAGE", "MATCHES")
			for _, g := range results {
				fmt.Fprintf(out, "%-32s  %-16s  %d\n", truncate(g.DealName, 32), truncate(g.Stage, 16), g.MatchCount)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d call(s) matched %q.\n", total, term)
			return nil
		},
	}
	cmd.Flags().StringVar(&term, "term", "", "Term(s) to match, pipe-separated for alternatives (e.g. \"pricing|competitor\")")
	cmd.Flags().StringVar(&since, "since", "", "Only calls newer than this window: 7d, 48h, or an RFC3339 timestamp")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// searchableText assembles the cached, lowercased text of a conversation:
// title plus the JSON of its summary and transcript when those were synced.
func searchableText(c map[string]any) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(convTitle(c)))
	if summary := nestedObj(c, "summary"); summary != nil {
		if raw, err := json.Marshal(summary); err == nil {
			b.WriteByte(' ')
			b.Write(lowerBytes(raw))
		}
	}
	if t, ok := c["transcript"].([]any); ok {
		for _, entry := range t {
			if m, ok := entry.(map[string]any); ok {
				b.WriteByte(' ')
				b.WriteString(strings.ToLower(firstStr(m, "text")))
			}
		}
	}
	return b.String()
}

func lowerBytes(b []byte) []byte {
	return []byte(strings.ToLower(string(b)))
}

func matchesAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// dealName2 returns the deal's display name, falling back to its id.
func dealName2(d map[string]any) string {
	if n := dealName(d); n != "" {
		return n
	}
	return dealID(d)
}
