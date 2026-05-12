// Copyright 2026 rai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var formatFilter string

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Live and about-to-start matches happening today",
		Long: `Show live + upcoming cricket matches happening now, with optional format filter
(t20i, odi, test, t20, t10, hundred).`,
		Example: "  cricapi-pp-cli today\n  cricapi-pp-cli today --format t20i,odi",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH: today is described as "Live and about-to-start matches",
			// so it must read live data. Without NoCache, any call within the
			// 5-min HTTP cache window would return a stale snapshot — score
			// changes and status updates would be invisible. Same fix as
			// watch.go, watchlist refresh, and sync.go.
			c.NoCache = true

			path := "/currentMatches"
			params := map[string]string{"offset": "0"}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "matches", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var items []teamMatch
			if err := json.Unmarshal(data, &items); err != nil {
				var env struct {
					Data []teamMatch `json:"data"`
				}
				if jerr := json.Unmarshal(data, &env); jerr == nil {
					items = env.Data
				}
			}

			formats := normalizeFormats(formatFilter)
			out := make([]teamMatch, 0, len(items))
			for _, m := range items {
				if len(formats) > 0 && !formatMatches(m.MatchType, formats) {
					continue
				}
				out = append(out, m)
			}

			printProvenance(cmd, len(out), prov)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			rows := make([]map[string]any, 0, len(out))
			for _, m := range out {
				rows = append(rows, map[string]any{
					"time":   firstNonEmpty(m.DateTimeGMT, m.Date),
					"match":  m.Name,
					"format": m.MatchType,
					"status": m.Status,
				})
			}
			if len(rows) == 0 {
				if formatFilter != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No matches today in formats: %s\n", formatFilter)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No live or upcoming matches right now.")
				}
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&formatFilter, "format", "", "Comma-separated formats (case-insensitive, exact match against CricAPI matchType tokens: t20, odi, test, t10, hundred)")
	return cmd
}

func normalizeFormats(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// PATCH: exact equality, not substring. Previously strings.Contains made
// --format t20 silently include T20I (because "t20i" contains "t20"), and
// --format t would match every format. CricAPI matchType tokens are short
// and non-overlapping (t20, odi, test, t10, hundred); compare them exactly.
func formatMatches(matchType string, formats []string) bool {
	lt := strings.ToLower(matchType)
	for _, f := range formats {
		if lt == f {
			return true
		}
	}
	return false
}
