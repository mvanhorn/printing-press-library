// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

// newH2HCmd implements the promoted 'h2h' command: H2H history + live odds.
func newH2HCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "h2h <matchId>",
		Short: "Historical H2H results alongside live odds for a match.",
		Long: `Combines the H2H history feed with the live odds feed for a match.
Shows head-to-head results and fetches current 1X2 odds from the betting endpoint.`,
		Example:     "  diretta-pp-cli h2h vVn0EQM5\n  diretta-pp-cli h2h vVn0EQM5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("matchId is required\nUsage: %s <matchId>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch H2H
			path := replacePathParam("/x/feed/df_hh_1_{matchId}", "matchId", args[0])
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "match", false, path, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			h2h := parser.ParseH2H([]byte(raw))

			// Fetch odds (best-effort; failure does not abort)
			oddsData := map[string]any{}
			oddsRaw, _, oddsErr := c.Post("https://2.ds.lsapp.eu/pq_graphql", map[string]any{
				"projectId": "2",
				"geoIpCode": "IT",
				"eventId":   args[0],
			})
			if oddsErr == nil && len(oddsRaw) > 2 {
				_ = json.Unmarshal(oddsRaw, &oddsData)
			}

			// Compute basic H2H summary
			summary := buildH2HSummary(h2h)

			result := map[string]any{
				"match_id": args[0],
				"h2h":      h2h,
				"summary":  summary,
			}
			if len(oddsData) > 0 {
				result["odds"] = oddsData
			}

			data, _ := json.Marshal(result)
			jdata := json.RawMessage(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(h2h), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := jdata
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			// Human table: print summary then h2h list
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "H2H summary: %s\n", formatH2HSummaryLine(summary))
				fmt.Fprintln(w, "")
				var items []map[string]any
				if json.Unmarshal(jdata, &items) == nil {
					// Just print h2h rows
				}
				if len(h2h) > 0 {
					return printAutoTable(w, h2h)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	return cmd
}

func buildH2HSummary(h2h []map[string]any) map[string]any {
	if len(h2h) == 0 {
		return map[string]any{"played": 0}
	}
	homeTeam, _ := h2h[0]["home_team"].(string)
	awayTeam, _ := h2h[0]["away_team"].(string)
	hw, aw, draws := 0, 0, 0
	for _, m := range h2h {
		hs := parseInt(fmt.Sprintf("%v", m["home_score"]))
		as := parseInt(fmt.Sprintf("%v", m["away_score"]))
		ht, _ := m["home_team"].(string)
		// Count from the perspective of the first match's teams
		if ht == homeTeam {
			if hs > as {
				hw++
			} else if as > hs {
				aw++
			} else {
				draws++
			}
		} else {
			if as > hs {
				hw++
			} else if hs > as {
				aw++
			} else {
				draws++
			}
		}
	}
	return map[string]any{
		"played":    len(h2h),
		"home_team": homeTeam,
		"away_team": awayTeam,
		"home_wins": hw,
		"away_wins": aw,
		"draws":     draws,
	}
}

func formatH2HSummaryLine(s map[string]any) string {
	var parts []string
	if ht, ok := s["home_team"].(string); ok && ht != "" {
		parts = append(parts, fmt.Sprintf("%s W%v D%v L%v", ht, s["home_wins"], s["draws"], s["away_wins"]))
	}
	return strings.Join(parts, " | ")
}
