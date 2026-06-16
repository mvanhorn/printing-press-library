// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// kdpStopwords are common English tokens dropped from title keyword counts.
var kdpStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "your": true, "with": true,
	"you": true, "how": true, "that": true, "this": true, "after": true,
	"without": true, "step": true, "from": true, "are": true, "was": true,
	"has": true, "have": true, "will": true, "can": true, "all": true,
	"out": true, "off": true, "into": true, "over": true, "more": true,
	"book": true, "books": true, "guide": true, "edition": true,
}

func newNovelKeywordsCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagMinCount int
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:         "keywords",
		Short:       "Tokenize synced book titles and count keyword frequency to surface hot terms for a niche.",
		Example:     "  kdpnichefinder-pp-cli keywords --type evergreen --min-count 3",
		Long:        "Use to surface frequent title keywords across synced niches; optionally scope to one bucket with --type.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateBucket(flagType); err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, _, missing, err := openKDPLocal(ctx, flags, flagDB, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if missing {
				return nil
			}
			defer db.Close()

			niches, err := loadNiches(ctx, db, flagType)
			if err != nil {
				return err
			}

			counts := map[string]int{}
			for _, n := range niches {
				for _, tok := range tokenizeTitle(n.Title) {
					counts[tok]++
				}
			}

			type kwRow struct {
				Keyword string `json:"keyword"`
				Count   int    `json:"count"`
			}
			out := make([]kwRow, 0, len(counts))
			for kw, c := range counts {
				if c < flagMinCount {
					continue
				}
				out = append(out, kwRow{Keyword: kw, Count: c})
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].Count != out[j].Count {
					return out[i].Count > out[j].Count
				}
				return out[i].Keyword < out[j].Keyword
			})
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Limit to a single bucket (evergreen, fresh_money, hidden_gems, high_ticket)")
	cmd.Flags().IntVar(&flagMinCount, "min-count", 2, "Only return keywords with at least this frequency")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of keywords to return")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror database (defaults to the standard location)")
	return cmd
}

// tokenizeTitle lowercases a title, splits on non-alphanumeric runs, and drops
// stopwords and tokens shorter than 3 characters.
func tokenizeTitle(title string) []string {
	lower := strings.ToLower(title)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 3 || kdpStopwords[f] {
			continue
		}
		out = append(out, f)
	}
	return out
}
