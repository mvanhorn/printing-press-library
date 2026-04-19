// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written promoted command. Spec-driven shape declared in spec.yaml.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newLeadersPromotedCmd(flags *rootFlags) *cobra.Command {
	var category string

	cmd := &cobra.Command{
		Use:   "leaders <sport> <league>",
		Short: "Statistical leaders across categories",
		Example: `  espn-pp-cli leaders football nfl
  espn-pp-cli leaders basketball nba --category points
  espn-pp-cli leaders baseball mlb --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: leaders <sport> <league> [--category <name>]"))
			}
			sport, league := args[0], args[1]

			// Leaders / statistical-by-athlete endpoint lives under site.web.api.espn.com common/v3.
			// The "leaders" path 404s; "statistics/byathlete" is what ESPN's web client uses.
			url := fmt.Sprintf("https://site.web.api.espn.com/apis/common/v3/sports/%s/%s/statistics/byathlete?limit=50", sport, league)

			body, err := espnHTTPGet(flags.timeout, url)
			if err != nil {
				return err
			}

			if category != "" {
				body = filterLeaderCategory(body, category)
				if body == nil {
					return usageErr(fmt.Errorf("category %q not found in response. Run without --category to list available categories.", category))
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				var raw json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					return err
				}
				return enc.Encode(raw)
			}

			return renderLeaders(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "Optional stat category filter (e.g. passing, rushing, points)")
	return cmd
}

// filterLeaderCategory restricts the byathlete payload to a single category.
// Each athlete entry has a "categories" array; we keep only entries that have
// the named category and rewrite each athlete's categories to just that one.
// Returns nil if the category does not exist anywhere.
func filterLeaderCategory(data []byte, category string) []byte {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(data, &resp); err != nil {
		return data
	}
	athletesRaw, ok := resp["athletes"]
	if !ok {
		return data
	}
	var athletes []map[string]any
	if err := json.Unmarshal(athletesRaw, &athletes); err != nil {
		return data
	}
	matched := false
	out := make([]map[string]any, 0, len(athletes))
	for _, a := range athletes {
		cats, _ := a["categories"].([]any)
		var keep []any
		for _, c := range cats {
			cm, _ := c.(map[string]any)
			if cm == nil {
				continue
			}
			name, _ := cm["name"].(string)
			display, _ := cm["displayName"].(string)
			if strings.EqualFold(name, category) || strings.EqualFold(display, category) {
				keep = append(keep, cm)
				matched = true
			}
		}
		if len(keep) > 0 {
			a["categories"] = keep
			out = append(out, a)
		}
	}
	if !matched {
		return nil
	}
	resp["athletes"], _ = json.Marshal(out)
	res, _ := json.Marshal(resp)
	return res
}

func renderLeaders(w io.Writer, data []byte) error {
	var resp struct {
		Athletes []struct {
			Athlete struct {
				DisplayName string `json:"displayName"`
				Team        struct {
					Abbreviation string `json:"abbreviation"`
				} `json:"team"`
				Position struct {
					Abbreviation string `json:"abbreviation"`
				} `json:"position"`
			} `json:"athlete"`
			Categories []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				Values      []struct {
					Name         string `json:"name"`
					DisplayName  string `json:"displayName"`
					DisplayValue string `json:"displayValue"`
				} `json:"values"`
			} `json:"categories"`
		} `json:"athletes"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing leaders: %w", err)
	}
	if len(resp.Athletes) == 0 {
		fmt.Fprintln(w, "No leaders found.")
		return nil
	}

	tw := newTabWriter(w)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
		bold("RANK"), bold("PLAYER"), bold("TEAM"), bold("POS"), bold("KEY STATS"))
	for i, a := range resp.Athletes {
		if i >= 25 {
			break
		}
		// Pull a few headline values from the first category.
		stats := ""
		if len(a.Categories) > 0 {
			cat := a.Categories[0]
			parts := []string{}
			for j, v := range cat.Values {
				if j >= 3 {
					break
				}
				parts = append(parts, fmt.Sprintf("%s %s", v.DisplayName, v.DisplayValue))
			}
			stats = strings.Join(parts, ", ")
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1,
			truncate(a.Athlete.DisplayName, 22),
			a.Athlete.Team.Abbreviation,
			a.Athlete.Position.Abbreviation,
			truncate(stats, 60))
	}
	return tw.Flush()
}
