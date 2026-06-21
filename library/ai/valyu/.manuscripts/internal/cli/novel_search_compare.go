// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newSearchCompareCmd(flags *rootFlags) *cobra.Command {
	var query string
	var configA string
	var configB string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare two search configurations side-by-side for the same query.",
		Long: `Run the same query against two different source/type configurations and display
a side-by-side comparison of result URLs, unique hits, overlap, and cost delta.

Use --config-a and --config-b as comma-separated key=value pairs mapping to
semantic-search flags (e.g. "type=web,max=10").`,
		Example: `  valyu-pp-cli semantic-search compare --query "CRISPR gene editing" --config-a "type=paper" --config-b "type=web"
  valyu-pp-cli semantic-search compare --query "Q2 earnings beat" --config-a "type=finance" --config-b "type=all,max=20" --json`,
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				return fmt.Errorf("required flag \"query\" not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			bodyA := parseConfigKV(configA)
			bodyA["query"] = query
			bodyB := parseConfigKV(configB)
			bodyB["query"] = query

			dataA, _, errA := c.PostQueryWithParams(cmd.Context(), "/v1/search", map[string]string{}, bodyA)
			if errA != nil {
				return fmt.Errorf("config-a search failed: %w", classifyAPIError(errA, flags))
			}
			dataB, _, errB := c.PostQueryWithParams(cmd.Context(), "/v1/search", map[string]string{}, bodyB)
			if errB != nil {
				return fmt.Errorf("config-b search failed: %w", classifyAPIError(errB, flags))
			}

			costA := extractCostFromResponse(dataA)
			costB := extractCostFromResponse(dataB)

			db, dbErr := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if dbErr == nil {
				defer db.Close()
				recordCost(db, "semantic-search compare", costA+costB)
			}

			urlsA := extractURLsFromSearchResponse(dataA)
			urlsB := extractURLsFromSearchResponse(dataB)

			setA := make(map[string]bool, len(urlsA))
			for _, u := range urlsA {
				setA[u] = true
			}
			setB := make(map[string]bool, len(urlsB))
			for _, u := range urlsB {
				setB[u] = true
			}

			intersection := 0
			for u := range setA {
				if setB[u] {
					intersection++
				}
			}
			union := len(setA) + len(setB) - intersection
			jaccard := 0.0
			if union > 0 {
				jaccard = float64(intersection) / float64(union)
			}

			var onlyA, onlyB, both []string
			for u := range setA {
				if setB[u] {
					both = append(both, u)
				} else {
					onlyA = append(onlyA, u)
				}
			}
			for u := range setB {
				if !setA[u] {
					onlyB = append(onlyB, u)
				}
			}

			if flags.asJSON {
				out := map[string]any{
					"query":      query,
					"config_a":   configA,
					"config_b":   configB,
					"cost_a":     costA,
					"cost_b":     costB,
					"cost_delta": costB - costA,
					"urls_a":     urlsA,
					"urls_b":     urlsB,
					"only_a":     onlyA,
					"only_b":     onlyB,
					"overlap":    both,
					"jaccard":    jaccard,
				}
				raw, _ := json.Marshal(out)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(raw), true)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "Query:\t%s\n", query)
			fmt.Fprintf(tw, "Config A:\t%s\n", configA)
			fmt.Fprintf(tw, "Config B:\t%s\n", configB)
			fmt.Fprintf(tw, "\n")
			fmt.Fprintf(tw, "Results A:\t%d\n", len(urlsA))
			fmt.Fprintf(tw, "Results B:\t%d\n", len(urlsB))
			fmt.Fprintf(tw, "Overlap:\t%d\n", len(both))
			fmt.Fprintf(tw, "Only in A:\t%d\n", len(onlyA))
			fmt.Fprintf(tw, "Only in B:\t%d\n", len(onlyB))
			fmt.Fprintf(tw, "Jaccard similarity:\t%.2f\n", jaccard)
			fmt.Fprintf(tw, "Cost A:\t$%.4f\n", costA)
			fmt.Fprintf(tw, "Cost B:\t$%.4f\n", costB)
			fmt.Fprintf(tw, "Cost delta (B-A):\t$%.4f\n", costB-costA)
			if len(onlyA) > 0 {
				fmt.Fprintf(tw, "\nOnly in A:\n")
				for _, u := range onlyA {
					fmt.Fprintf(tw, "  %s\n", u)
				}
			}
			if len(onlyB) > 0 {
				fmt.Fprintf(tw, "\nOnly in B:\n")
				for _, u := range onlyB {
					fmt.Fprintf(tw, "  %s\n", u)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "The search query to run against both configurations")
	cmd.Flags().StringVar(&configA, "config-a", "type=all", "Config A as comma-separated key=value pairs (e.g. 'type=web,max=10')")
	cmd.Flags().StringVar(&configB, "config-b", "type=web", "Config B as comma-separated key=value pairs (e.g. 'type=paper,max=10')")
	return cmd
}

// parseConfigKV parses "key=val,key2=val2" into a search body map.
func parseConfigKV(s string) map[string]any {
	body := map[string]any{}
	if s == "" {
		return body
	}
	for _, pair := range strings.Split(s, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		switch k {
		case "max", "max_num_results":
			var n int
			fmt.Sscanf(v, "%d", &n)
			body["max_num_results"] = n
		case "type", "search_type":
			body["search_type"] = v
		case "sources", "included_sources":
			body["included_sources"] = strings.Split(v, "|")
		default:
			body[k] = v
		}
	}
	return body
}

// extractURLsFromSearchResponse extracts URL strings from a Valyu search response JSON.
func extractURLsFromSearchResponse(data json.RawMessage) []string {
	var resp struct {
		Results []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	var urls []string
	if err := json.Unmarshal(data, &resp); err == nil && len(resp.Results) > 0 {
		for _, r := range resp.Results {
			if r.URL != "" {
				urls = append(urls, r.URL)
			}
		}
		return urls
	}
	// Try as array directly
	var arr []struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(data, &arr) == nil {
		for _, r := range arr {
			if r.URL != "" {
				urls = append(urls, r.URL)
			}
		}
	}
	return urls
}

// extractCostFromResponse pulls the numeric cost field from a Valyu API response.
func extractCostFromResponse(data json.RawMessage) float64 {
	var resp struct {
		Cost float64 `json:"cost"`
	}
	json.Unmarshal(data, &resp)
	return resp.Cost
}
