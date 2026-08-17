// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: compare two publications' cadence and free/paid mix from the
// local corpus. Hand-implemented; a local join across two archived publications
// that no single API call provides. generate --force preserves implemented bodies.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack-reader/internal/substack"
)

type authorStats struct {
	Host     string
	Total    int
	Free     int
	Paid     int
	Dated    int
	Earliest time.Time
	Latest   time.Time
	PerWeek  float64
}

// pp:data-source local
func newNovelAuthorCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "author-compare <pub-a> <pub-b>",
		Short: "Compare two publications' cadence and free/paid mix from the local corpus",
		Long: `Compare two archived publications side by side: how many posts you've
mirrored, their date range, publishing cadence (posts/week), and free/paid mix.

Reads only the local corpus — archive both publications first
('substack-reader-pp-cli archive <pub>'). Each publication is matched by the host it
was archived under, so pass the same handle/host you archived with.`,
		Example:     "  substack-reader-pp-cli author-compare astralcodexten blog.bytebytego.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare two publications from the local corpus")
				return nil
			}
			if len(args) != 2 {
				return usageErr(fmt.Errorf("author-compare needs exactly two publications; got %d", len(args)))
			}
			// Lowercase the read keys to match the lowercased join key stored by
			// parseCorpusPost (Substack hosts are case-insensitive).
			hostA := strings.ToLower(hostFromURL(substack.ResolveHost(args[0])))
			hostB := strings.ToLower(hostFromURL(substack.ResolveHost(args[1])))
			if hostA == "" || hostB == "" {
				return usageErr(fmt.Errorf("could not resolve both publications from %q and %q", args[0], args[1]))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			posts, err := loadCorpusPosts(ctx)
			if err != nil {
				return err
			}
			statsA := computeAuthorStats(posts, hostA)
			statsB := computeAuthorStats(posts, hostB)

			// Machine output.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				// All-zero stats for a live newsletter mean "not archived", not
				// "publishes nothing"; say so on stderr so JSON stays clean.
				for _, s := range []authorStats{statsA, statsB} {
					if s.Total == 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "note: no archived posts for %s — run 'substack-reader-pp-cli archive %s' first\n", s.Host, s.Host)
					}
				}
				envelope := map[string]any{
					"a": authorStatsJSON(statsA),
					"b": authorStatsJSON(statsB),
				}
				data, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			// Human output.
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "author-compare: %s  vs  %s\n\n", hostA, hostB)
			for _, s := range []authorStats{statsA, statsB} {
				if s.Total == 0 {
					fmt.Fprintf(w, "note: no archived posts for %s — run 'substack-reader-pp-cli archive %s' first\n", s.Host, s.Host)
				}
			}
			tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "\tA: %s\tB: %s\n", hostA, hostB)
			fmt.Fprintf(tw, "posts (archived)\t%d\t%d\n", statsA.Total, statsB.Total)
			fmt.Fprintf(tw, "free / paid\t%d / %d\t%d / %d\n", statsA.Free, statsA.Paid, statsB.Free, statsB.Paid)
			fmt.Fprintf(tw, "date range\t%s\t%s\n", dateRange(statsA), dateRange(statsB))
			fmt.Fprintf(tw, "cadence (posts/week)\t%s\t%s\n", cadence(statsA), cadence(statsB))
			return tw.Flush()
		},
	}
	return cmd
}

func computeAuthorStats(posts []corpusPost, host string) authorStats {
	s := authorStats{Host: host}
	for _, p := range posts {
		if p.Host != host {
			continue
		}
		s.Total++
		if p.IsPaid() {
			s.Paid++
		} else {
			s.Free++
		}
		if p.HasDate {
			s.Dated++
			if s.Earliest.IsZero() || p.Parsed.Before(s.Earliest) {
				s.Earliest = p.Parsed
			}
			if p.Parsed.After(s.Latest) {
				s.Latest = p.Parsed
			}
		}
	}
	if s.Dated > 1 && s.Latest.After(s.Earliest) {
		weeks := s.Latest.Sub(s.Earliest).Hours() / (24 * 7)
		if weeks >= 1 {
			s.PerWeek = float64(s.Dated) / weeks
		} else {
			s.PerWeek = float64(s.Dated) // all within a week
		}
	}
	return s
}

func authorStatsJSON(s authorStats) map[string]any {
	m := map[string]any{
		"host":           s.Host,
		"total":          s.Total,
		"free":           s.Free,
		"paid":           s.Paid,
		"posts_per_week": round1(s.PerWeek),
	}
	if s.Dated > 0 {
		m["earliest"] = s.Earliest.Format("2006-01-02")
		m["latest"] = s.Latest.Format("2006-01-02")
	}
	return m
}

func dateRange(s authorStats) string {
	if s.Dated == 0 {
		return "—"
	}
	return s.Earliest.Format("2006-01-02") + " → " + s.Latest.Format("2006-01-02")
}

func cadence(s authorStats) string {
	if s.PerWeek == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", s.PerWeek)
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
