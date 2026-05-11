// Copyright 2026 rai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// teamMatch is a minimal projection of CricAPI match objects for the team filter.
type teamMatch struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MatchType   string   `json:"matchType"`
	Status      string   `json:"status"`
	Venue       string   `json:"venue"`
	Date        string   `json:"date"`
	DateTimeGMT string   `json:"dateTimeGMT"`
	Teams       []string `json:"teams"`
	Started     bool     `json:"matchStarted"`
	Ended       bool     `json:"matchEnded"`
}

func newTeamCmd(flags *rootFlags) *cobra.Command {
	var scope string
	var limit int

	cmd := &cobra.Command{
		Use:   "team [name]",
		Short: "Find matches involving a team (upcoming, recent, or all)",
		Long: `Resolve a team name to all matches involving that team across CricAPI's
match list. Answers the natural-language question: "when does Pakistan play next?"`,
		Example: "  cricapi-pp-cli team pakistan\n  cricapi-pp-cli team india --scope upcoming --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.ToLower(strings.TrimSpace(args[0]))
			if needle == "" {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Pull /matches with offset=0; CricAPI returns the broad list there.
			path := "/matches"
			params := map[string]string{"offset": "0"}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "matches", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var items []teamMatch
			if err := json.Unmarshal(data, &items); err != nil {
				// Fall back: API may wrap in envelope; try .data
				var env struct {
					Data []teamMatch `json:"data"`
				}
				if jerr := json.Unmarshal(data, &env); jerr == nil {
					items = env.Data
				}
			}

			out := make([]teamMatch, 0, len(items))
			for _, m := range items {
				if !teamMatches(m, needle) {
					continue
				}
				switch scope {
				case "upcoming":
					if m.Ended {
						continue
					}
				case "recent":
					if !m.Ended {
						continue
					}
				}
				out = append(out, m)
				if limit > 0 && len(out) >= limit {
					break
				}
			}

			printProvenance(cmd, len(out), prov)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Human-friendly table
			rows := make([]map[string]any, 0, len(out))
			for _, m := range out {
				rows = append(rows, map[string]any{
					"date":   firstNonEmpty(m.DateTimeGMT, m.Date),
					"match":  m.Name,
					"format": m.MatchType,
					"status": m.Status,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matches found involving %q.\n", args[0])
				return nil
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "all", "Filter scope: upcoming, recent, all")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max matches to return (0 = no limit)")
	return cmd
}

func teamMatches(m teamMatch, needle string) bool {
	for _, t := range m.Teams {
		if strings.Contains(strings.ToLower(t), needle) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(m.Name), needle)
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
