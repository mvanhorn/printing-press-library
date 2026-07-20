// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// diet: difficulty-progression / readiness analysis. Joins the watched
// playlist to per-video 1-100 difficulty over a window to show whether the
// difficulty of what you actually watch is trending up. Hand-built novel
// feature. Works offline.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelDietCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string

	cmd := &cobra.Command{
		Use:   "diet",
		Short: "See whether the difficulty of what you actually watch is trending up over a window",
		Long: "Analyze the difficulty of the videos you've actually watched over a recent\n" +
			"window by joining your watched playlist to each video's 1-100 difficulty\n" +
			"rating. A rising trend is the real signal you're ready to level up. Works\n" +
			"offline against the synced store.",
		Example: strings.Trim(`
  dreaming-pp-cli diet --window 90d
  dreaming-pp-cli diet --window 12w --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			days, perr := parseWindowDays(flagWindow)
			if perr != nil {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(perr)
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openDreamingStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			cutoff := ""
			if days > 0 {
				cutoff = time.Now().AddDate(0, 0, -days).Format("2006-01-02")
			}
			q := `SELECT p.added_date, COALESCE(v.difficulty,0), COALESCE(v.level,'')
				FROM playlist p JOIN videos v ON v.id = p.video_id
				WHERE COALESCE(v.difficulty,0) > 0`
			var argv []any
			if cutoff != "" {
				q += ` AND substr(p.added_date,1,10) >= ?`
				argv = append(argv, cutoff)
			}
			q += ` ORDER BY p.added_date ASC`
			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return err
			}
			defer rows.Close()
			type pt struct {
				diff int
			}
			var pts []pt
			levelCounts := map[string]int{}
			for rows.Next() {
				var d string
				var diff int
				var lvl string
				if err := rows.Scan(&d, &diff, &lvl); err != nil {
					return err
				}
				pts = append(pts, pt{diff: diff})
				if lvl != "" {
					levelCounts[lvl]++
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			n := len(pts)
			if n == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"watched_in_window": 0,
						"trend":             "none",
						"hint":              "no watched videos with a difficulty rating in this window — run 'dreaming-pp-cli sync', watch some videos, then check back",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No watched videos with a difficulty rating in this window. Run 'sync' and watch some videos, then check back.")
				return nil
			}
			avg := func(s []pt) float64 {
				if len(s) == 0 {
					return 0
				}
				var sum int
				for _, p := range s {
					sum += p.diff
				}
				return float64(sum) / float64(len(s))
			}
			third := n / 3
			if third == 0 {
				third = 1
			}
			early := avg(pts[:third])
			late := avg(pts[n-third:])
			overall := avg(pts)
			trend := "flat"
			delta := late - early
			switch {
			case delta >= 5:
				trend = "rising"
			case delta <= -5:
				trend = "falling"
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"window_days":           days,
					"watched_in_window":     n,
					"avg_difficulty":        round1(overall),
					"early_avg_difficulty":  round1(early),
					"recent_avg_difficulty": round1(late),
					"delta":                 round1(delta),
					"trend":                 trend,
					"by_level":              levelCounts,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if days == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Watched %d rated videos across all time.\n", n)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Watched %d rated videos in the last %d days.\n", n, days)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Avg difficulty:   %.1f / 100\n", overall)
			fmt.Fprintf(cmd.OutOrStdout(), "  Early vs recent:  %.1f → %.1f  (Δ %+.1f)\n", early, late, delta)
			fmt.Fprintf(cmd.OutOrStdout(), "  Trend:            %s\n", trend)
			if trend == "rising" {
				fmt.Fprintln(cmd.OutOrStdout(), "  You're climbing into harder content — a good sign you're ready to level up.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "90d", "Lookback window, e.g. 30d, 12w, or 0 for all time")
	return cmd
}

// parseWindowDays parses "90d", "12w", "3m", or "0"/"all" into days (0 = all time).
func parseWindowDays(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "0" || s == "all" {
		return 0, nil
	}
	unit := s[len(s)-1]
	numStr := s
	mult := 1
	switch unit {
	case 'd':
		numStr = s[:len(s)-1]
	case 'w':
		numStr = s[:len(s)-1]
		mult = 7
	case 'm':
		numStr = s[:len(s)-1]
		mult = 30
	}
	n, err := strconv.Atoi(numStr)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid --window %q: use a form like 30d, 12w, 3m, or 0 for all time", s)
	}
	return n * mult, nil
}
