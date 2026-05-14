package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newTitlePatternsCmd(flags *rootFlags) *cobra.Command {
	var (
		winners bool
		losers  bool
		minN    int
		dbPath  string
		topK    int
	)
	cmd := &cobra.Command{
		Use:   "title-patterns",
		Short: "Token-level analysis of words correlating with above-median CTR (winners / losers)",
		Long: strings.TrimSpace(`
Computes the average CTR for videos whose titles contain each significant
token, compares against the channel-wide median CTR, and surfaces tokens
that consistently correlate with winners or losers.

Tokens are limited to characters [a-z0-9], minimum length 3, and stop-words
are filtered out. Pass --min-n to require a minimum number of videos per
token (default 3).`),
		Example:     "  yt-studio-pp-cli title-patterns --winners --losers --json --top 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !winners && !losers {
				winners = true
				losers = true
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			tokenCTR, median, err := computeTokenCTR(ctx, db)
			if err != nil {
				return err
			}
			if len(tokenCTR) == 0 {
				return notFoundErr(errors.New("no synced videos with CTR data; run `yt-studio-pp-cli sync` first"))
			}

			type entry struct {
				Token   string  `json:"token"`
				N       int     `json:"n"`
				MeanCTR float64 `json:"mean_ctr"`
				LiftPct float64 `json:"lift_pct"`
			}
			ws := []entry{}
			ls := []entry{}
			for tok, s := range tokenCTR {
				if s.n < minN {
					continue
				}
				mean := s.sum / float64(s.n)
				lift := 0.0
				if median > 0 {
					lift = (mean - median) / median * 100.0
				}
				e := entry{Token: tok, N: s.n, MeanCTR: mean, LiftPct: lift}
				if mean >= median {
					ws = append(ws, e)
				} else {
					ls = append(ls, e)
				}
			}
			sort.Slice(ws, func(i, j int) bool { return ws[i].LiftPct > ws[j].LiftPct })
			sort.Slice(ls, func(i, j int) bool { return ls[i].LiftPct < ls[j].LiftPct })
			if topK > 0 {
				if len(ws) > topK {
					ws = ws[:topK]
				}
				if len(ls) > topK {
					ls = ls[:topK]
				}
			}

			res := map[string]any{"median_ctr": median, "min_n": minN}
			if winners {
				res["winners"] = ws
			}
			if losers {
				res["losers"] = ls
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "title-patterns (median CTR: %.3f%%, min-n: %d)\n", median*100, minN)
			if winners {
				fmt.Fprintln(w, "Winners:")
				for _, e := range ws {
					fmt.Fprintf(w, "  %-20s  n=%-3d  CTR=%.3f%%  lift=%.1f%%\n", e.Token, e.N, e.MeanCTR*100, e.LiftPct)
				}
			}
			if losers {
				fmt.Fprintln(w, "Losers:")
				for _, e := range ls {
					fmt.Fprintf(w, "  %-20s  n=%-3d  CTR=%.3f%%  lift=%.1f%%\n", e.Token, e.N, e.MeanCTR*100, e.LiftPct)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&winners, "winners", false, "Include winning tokens (above-median CTR)")
	cmd.Flags().BoolVar(&losers, "losers", false, "Include losing tokens (below-median CTR)")
	cmd.Flags().IntVar(&minN, "min-n", 3, "Minimum number of videos per token")
	cmd.Flags().IntVar(&topK, "top", 20, "Top-K tokens per category (0 for all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func computeTokenCTR(ctx context.Context, db *sql.DB) (map[string]struct {
	sum float64
	n   int
}, float64, error) {
	stats, err := ytstore.OwnChannelTitleCTRs(ctx, db)
	if err != nil {
		return nil, 0, err
	}

	// Median
	ctrs := make([]float64, len(stats))
	for i, s := range stats {
		ctrs[i] = s.CTR
	}
	sort.Float64s(ctrs)
	var median float64
	if len(ctrs) > 0 {
		median = ctrs[len(ctrs)/2]
	}

	// Tokenize and aggregate
	out := map[string]struct {
		sum float64
		n   int
	}{}
	for _, s := range stats {
		for _, tok := range tokenizeTitle(s.Title) {
			cur := out[tok]
			cur.sum += s.CTR
			cur.n++
			out[tok] = cur
		}
	}
	return out, median, nil
}

func tokenizeTitle(s string) []string {
	lower := strings.ToLower(s)
	var tokens []string
	var b strings.Builder
	seen := map[string]bool{}
	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := b.String()
		b.Reset()
		if len(t) < 3 {
			return
		}
		// crude stop-words list
		switch t {
		case "the", "and", "for", "with", "that", "this", "are", "you", "your", "but", "from", "have", "has", "not", "all", "any", "can", "out", "how", "why", "what", "when", "who", "which", "into":
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true
		tokens = append(tokens, t)
	}
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}
