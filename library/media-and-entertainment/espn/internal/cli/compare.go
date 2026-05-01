package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// newCompareCmd renders side-by-side season stats for two athletes.
// Resolves names via the ESPN search endpoint and pulls each athlete's
// season splits. Ambiguous matches print candidates and exit 2.
func newCompareCmd(flags *rootFlags) *cobra.Command {
	var sport, league string

	cmd := &cobra.Command{
		Use:   "compare <athlete1> <athlete2>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Side-by-side athlete stat comparison",
		Example: `  espn-pp-cli compare Mahomes Allen --sport football --league nfl
  espn-pp-cli compare LeBron Curry --sport basketball --league nba --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr(fmt.Errorf("two athlete names are required\nUsage: compare <athlete1> <athlete2> --sport <s> --league <l>"))
			}
			if sport == "" || league == "" {
				return usageErr(fmt.Errorf("--sport and --league are required"))
			}

			a1, err := resolveAthlete(flags, sport, league, args[0])
			if err != nil {
				return err
			}
			a2, err := resolveAthlete(flags, sport, league, args[1])
			if err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				out := map[string]any{
					"sport":    sport,
					"league":   league,
					"athlete1": a1,
					"athlete2": a2,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			return renderCompare(cmd, a1, a2)
		},
	}

	cmd.Flags().StringVar(&sport, "sport", "", "Sport slug (required)")
	cmd.Flags().StringVar(&league, "league", "", "League slug (required)")
	return cmd
}

type athleteResult struct {
	ID          string            `json:"id"`
	DisplayName string            `json:"display_name"`
	Position    string            `json:"position"`
	TeamAbbr    string            `json:"team"`
	Stats       map[string]string `json:"stats"`
}

func resolveAthlete(flags *rootFlags, sport, league, query string) (*athleteResult, error) {
	// ESPN's per-league /athletes?search= 400s; the cross-sport common/v3/search
	// endpoint with type=player works and we filter to the requested sport+league.
	searchURL := fmt.Sprintf(
		"https://site.web.api.espn.com/apis/common/v3/search?query=%s&type=player&limit=20",
		url.QueryEscape(query))
	body, err := espnHTTPGet(flags.timeout, searchURL)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Items []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Sport       string `json:"sport"`
			League      string `json:"league"`
			Position    struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"position"`
			Team struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"team"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing athlete search: %w", err)
	}

	// Filter to requested sport/league when set on the result.
	var matches []struct {
		ID          string
		DisplayName string
		Position    string
		TeamAbbr    string
	}
	for _, it := range resp.Items {
		if it.Sport != "" && !strings.EqualFold(it.Sport, sport) {
			continue
		}
		if it.League != "" && !strings.EqualFold(it.League, league) {
			continue
		}
		matches = append(matches, struct {
			ID          string
			DisplayName string
			Position    string
			TeamAbbr    string
		}{it.ID, it.DisplayName, it.Position.Abbreviation, it.Team.Abbreviation})
	}
	if len(matches) == 0 {
		return nil, notFoundErr(fmt.Errorf("no athlete matched %q in %s/%s", query, sport, league))
	}

	exact := -1
	for i, m := range matches {
		if strings.EqualFold(m.DisplayName, query) {
			exact = i
			break
		}
	}
	if exact == -1 && len(matches) > 1 {
		var lines []string
		for i, m := range matches {
			if i >= 5 {
				lines = append(lines, "  ...")
				break
			}
			lines = append(lines, fmt.Sprintf("  %s — %s (%s)", m.ID, m.DisplayName, m.TeamAbbr))
		}
		return nil, usageErr(fmt.Errorf("ambiguous athlete %q. Candidates:\n%s\nRefine the name or pass the athlete id directly.",
			query, strings.Join(lines, "\n")))
	}

	idx := 0
	if exact >= 0 {
		idx = exact
	}
	picked := matches[idx]
	a := &athleteResult{
		ID:          picked.ID,
		DisplayName: picked.DisplayName,
		Position:    picked.Position,
		TeamAbbr:    picked.TeamAbbr,
		Stats:       map[string]string{},
	}

	// Fetch per-athlete season stats. The "stats" subresource on the common v3
	// path returns a categories array similar to the leaders payload.
	statsURL := fmt.Sprintf(
		"https://site.web.api.espn.com/apis/common/v3/sports/%s/%s/athletes/%s/stats",
		sport, league, picked.ID)
	statsBody, err := espnHTTPGet(flags.timeout, statsURL)
	if err == nil {
		var stats struct {
			Categories []struct {
				Name   string   `json:"name"`
				Labels []string `json:"labels"`
				Names  []string `json:"names"`
				Totals []string `json:"totals"`
			} `json:"categories"`
		}
		if json.Unmarshal(statsBody, &stats) == nil {
			for _, cat := range stats.Categories {
				if len(cat.Totals) == 0 {
					continue
				}
				labels := cat.Labels
				if len(labels) == 0 {
					labels = cat.Names
				}
				for i, label := range labels {
					if i >= len(cat.Totals) {
						break
					}
					a.Stats[cat.Name+"."+label] = cat.Totals[i]
				}
			}
		}
	}

	return a, nil
}

func renderCompare(cmd *cobra.Command, a, b *athleteResult) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s (%s, %s)  vs  %s (%s, %s)\n\n",
		bold(a.DisplayName), a.Position, a.TeamAbbr,
		bold(b.DisplayName), b.Position, b.TeamAbbr)

	// Union of stat keys from both athletes.
	keys := map[string]bool{}
	for k := range a.Stats {
		keys[k] = true
	}
	for k := range b.Stats {
		keys[k] = true
	}
	if len(keys) == 0 {
		fmt.Fprintln(w, "No season stats available for either athlete.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\n", bold("STAT"), bold(a.DisplayName), bold(b.DisplayName))
	for k := range keys {
		va := a.Stats[k]
		vb := b.Stats[k]
		if va == "" {
			va = "-"
		}
		if vb == "" {
			vb = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\n", k, va, vb)
	}
	return tw.Flush()
}
