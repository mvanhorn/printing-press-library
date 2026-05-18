// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/store"
)

func newThemesCmd(flags *rootFlags) *cobra.Command {
	var flagLast, flagAgainst string
	var flagLimit, flagMinCount int

	cmd := &cobra.Command{
		Use:   "themes",
		Short: "N-gram frequency deltas between two time windows (emerging vs decaying)",
		Long: "Tokenizes transcript content into 1-3 gram shingles, filters stop\n" +
			"words, and computes per-window frequencies for the last and prior\n" +
			"windows. Output sorts by abs(delta) — what's growing or fading.\n" +
			"Mechanical extraction; no LLM call.",
		Example: `  plaud-pp-cli themes --last 30d --against 30d-prior
  plaud-pp-cli themes --last 7d --against 7d-prior --agent
  plaud-pp-cli themes --last 90d --json --select ngram,delta`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			now := time.Now().Unix()
			lastDur, err := parseDurationFlag(flagLast)
			if err != nil {
				return usageErr(err)
			}
			lastStart := now - lastDur
			var priorStart, priorEnd int64
			if flagAgainst == "" || strings.HasSuffix(flagAgainst, "-prior") {
				priorDur := lastDur
				if flagAgainst != "" && flagAgainst != "-prior" {
					if d, err := parseDurationFlag(strings.TrimSuffix(flagAgainst, "-prior")); err == nil {
						priorDur = d
					}
				}
				priorEnd = lastStart
				priorStart = priorEnd - priorDur
			} else {
				against, err := parseDurationFlag(flagAgainst)
				if err != nil {
					return usageErr(err)
				}
				priorEnd = lastStart
				priorStart = priorEnd - against
			}

			s, err := openPlaudStore(cmd.Context())
			var _ *store.Store = s
			if err != nil {
				return err
			}
			defer s.Close()

			lastTexts, err := fetchTranscriptsInWindow(cmd.Context(), s.DB(), lastStart, now)
			if err != nil {
				return apiErr(fmt.Errorf("loading last window: %w", err))
			}
			priorTexts, err := fetchTranscriptsInWindow(cmd.Context(), s.DB(), priorStart, priorEnd)
			if err != nil {
				return apiErr(fmt.Errorf("loading prior window: %w", err))
			}

			lastCounts := tokenizeAndCount(lastTexts)
			priorCounts := tokenizeAndCount(priorTexts)

			allGrams := map[string]bool{}
			for g := range lastCounts {
				allGrams[g] = true
			}
			for g := range priorCounts {
				allGrams[g] = true
			}
			type themeRow struct {
				NGram      string `json:"ngram"`
				LastCount  int    `json:"last_count"`
				PriorCount int    `json:"prior_count"`
				Delta      int    `json:"delta"`
			}
			rows := make([]themeRow, 0, len(allGrams))
			for g := range allGrams {
				lc := lastCounts[g]
				pc := priorCounts[g]
				if lc < flagMinCount && pc < flagMinCount {
					continue
				}
				rows = append(rows, themeRow{NGram: g, LastCount: lc, PriorCount: pc, Delta: lc - pc})
			}
			sort.Slice(rows, func(i, j int) bool {
				if absInt(rows[i].Delta) != absInt(rows[j].Delta) {
					return absInt(rows[i].Delta) > absInt(rows[j].Delta)
				}
				return rows[i].LastCount > rows[j].LastCount
			})
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			out := map[string]any{
				"last_window":  map[string]int64{"start": lastStart, "end": now},
				"prior_window": map[string]int64{"start": priorStart, "end": priorEnd},
				"themes":       rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagLast, "last", "30d", "Recent window duration (e.g. 30d, 12h)")
	cmd.Flags().StringVar(&flagAgainst, "against", "30d-prior", "Comparison window: 'Nd-prior' for prior equivalent, or a relative duration")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max themes to return")
	cmd.Flags().IntVar(&flagMinCount, "min-count", 2, "Minimum mention count to consider an ngram")
	return cmd
}

// parseDurationFlag accepts "30d", "12h", "10m" and returns seconds.
func parseDurationFlag(s string) (int64, error) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "-prior"))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	unit := s[len(s)-1]
	var n int64
	fmt.Sscanf(s[:len(s)-1], "%d", &n)
	if n <= 0 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	switch unit {
	case 'd':
		return n * 86400, nil
	case 'h':
		return n * 3600, nil
	case 'm':
		return n * 60, nil
	}
	return 0, fmt.Errorf("invalid duration unit in %q (expected d/h/m)", s)
}

// fetchTranscriptsInWindow returns every transcript content string from
// recordings with start_time in [start, end].
func fetchTranscriptsInWindow(ctx context.Context, db *sql.DB, start, end int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.content
		FROM transcripts t
		JOIN recordings_typed r ON r.id = t.recording_id
		WHERE r.start_time >= ? AND r.start_time <= ? AND r.is_trash = 0
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c sql.NullString
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		if c.Valid {
			out = append(out, c.String)
		}
	}
	return out, rows.Err()
}

func tokenizeAndCount(texts []string) map[string]int {
	out := map[string]int{}
	stop := stopWords()
	for _, txt := range texts {
		txt = strings.ToLower(txt)
		words := strings.FieldsFunc(txt, func(r rune) bool {
			return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '\''
		})
		filtered := make([]string, 0, len(words))
		for _, w := range words {
			if len(w) < 3 || stop[w] {
				continue
			}
			filtered = append(filtered, w)
		}
		for _, w := range filtered {
			out[w]++
		}
		for i := 0; i < len(filtered)-1; i++ {
			out[filtered[i]+" "+filtered[i+1]]++
		}
		for i := 0; i < len(filtered)-2; i++ {
			out[filtered[i]+" "+filtered[i+1]+" "+filtered[i+2]]++
		}
	}
	return out
}

func stopWords() map[string]bool {
	return map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "from": true,
		"that": true, "this": true, "have": true, "has": true, "had": true,
		"will": true, "would": true, "could": true, "should": true,
		"about": true, "into": true, "than": true, "then": true, "them": true,
		"they": true, "their": true, "there": true, "these": true, "those": true,
		"are": true, "was": true, "were": true, "been": true, "being": true,
		"you": true, "your": true, "yours": true, "our": true, "ours": true,
		"all": true, "any": true, "but": true, "out": true, "yes": true, "no": true,
		"can": true, "let": true, "get": true, "got": true, "going": true,
		"like": true, "just": true, "know": true, "think": true, "thing": true,
		"things": true, "really": true, "kind": true, "right": true, "okay": true,
		"yeah": true, "well": true, "actually": true,
	}
}

func absInt(i int) int {
	if i < 0 {
		return -i
	}
	return i
}
