package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newTrendingCmd hits ESPN's cross-league "now" headline feed and returns the
// ranked story list. ESPN's older popularity endpoint returns 404; the now
// feed is what powers the homepage and is the closest cross-league
// "what's trending" surface that is publicly reachable.
func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Cross-league trending headlines (top stories across leagues)",
		Example: `  espn-pp-cli trending
  espn-pp-cli trending --limit 20 --agent
  espn-pp-cli trending --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 25
			}
			url := fmt.Sprintf("https://now.core.api.espn.com/v1/sports/news?limit=%d", limit)
			body, err := espnHTTPGet(flags.timeout, url)
			if err != nil {
				return err
			}

			items := parseTrending(body)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}

			w := cmd.OutOrStdout()
			if len(items) == 0 {
				fmt.Fprintln(w, "No trending entries returned.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				bold("RANK"), bold("LEAGUE"), bold("HEADLINE"), bold("PUBLISHED"))
			for i, it := range items {
				published := it.Published
				if len(published) > 10 {
					published = published[:10]
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n",
					i+1, it.League, truncate(it.Name, 70), published)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Max entries to return")
	return cmd
}

type trendingItem struct {
	EntityType string `json:"entity_type"`
	Name       string `json:"name"`
	League     string `json:"league"`
	Published  string `json:"published"`
}

func parseTrending(data []byte) []trendingItem {
	// "now" feed shape: {"headlines":[{...}]}.
	var nowResp struct {
		Headlines []map[string]any `json:"headlines"`
	}
	if err := json.Unmarshal(data, &nowResp); err == nil && len(nowResp.Headlines) > 0 {
		return mapNowHeadlines(nowResp.Headlines)
	}
	// Defensive: older shapes (top-level array or object with items).
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil && len(arr) > 0 {
		return mapTrendingItems(arr)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil
	}
	for _, key := range []string{"items", "sports", "trending", "data"} {
		if raw, ok := obj[key]; ok {
			var inner []map[string]any
			if err := json.Unmarshal(raw, &inner); err == nil && len(inner) > 0 {
				return mapTrendingItems(inner)
			}
		}
	}
	return nil
}

func mapNowHeadlines(items []map[string]any) []trendingItem {
	var out []trendingItem
	for _, it := range items {
		out = append(out, trendingItem{
			EntityType: "story",
			Name:       jsonStrAny(it, "headline"),
			League:     strings.ToUpper(jsonStrAny(it, "section")),
			Published:  jsonStrAny(it, "published"),
		})
	}
	return out
}

func mapTrendingItems(items []map[string]any) []trendingItem {
	var out []trendingItem
	for _, it := range items {
		entityType := jsonStrAny(it, "entity_type")
		if entityType == "" {
			entityType = jsonStrAny(it, "type")
		}
		name := jsonStrAny(it, "name")
		if name == "" {
			name = jsonStrAny(it, "displayName")
		}
		league := jsonStrAny(it, "league")
		if league == "" {
			if l, ok := it["league"].(map[string]any); ok {
				league = jsonStrAny(l, "abbreviation")
			}
		}
		out = append(out, trendingItem{
			EntityType: entityType,
			Name:       name,
			League:     league,
		})
	}
	return out
}
