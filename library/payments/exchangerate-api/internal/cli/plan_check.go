// Novel command: plan-check. Probes each tier-gated endpoint with a single
// low-cost request and reports which tier the user's key supports. Saves the
// user from reading the pricing page to know whether 'enriched' or 'history'
// will return data.
package cli

// PATCH exchangerate-novel-plan-check: probe tier-gated endpoints; report supported tier; key masked in output.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newPlanCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan-check",
		Short: "Probe each tier-gated endpoint to see which tier your API key supports",
		Long: `Sends one low-cost request to each tier-gated endpoint (/codes, /pair,
/quota, /enriched, /history) and reports which succeeded and which returned
plan-upgrade-required. Burns ~5 quota ticks.`,
		Example:     "  exchangerate-api-pp-cli plan-check\n  exchangerate-api-pp-cli plan-check --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if c.Config == nil || c.Config.ExchangerateApiKey == "" {
				return usageErr(fmt.Errorf("EXCHANGERATE_API_KEY is required; export it or run 'auth set-token <key>'"))
			}
			k := c.Config.ExchangerateApiKey
			type probe struct {
				Name string `json:"name"`
				Tier string `json:"tier_required"`
				// Path is the public path shape (key masked); the real
				// request URL is built per-probe below.
				Path string `json:"path"`
				OK   bool   `json:"ok"`
				Note string `json:"note,omitempty"`
			}
			// liveURL builds the actual request URL with the key; pathPub is
			// the redacted version safe to print and serialize.
			liveURL := func(template string) string {
				return strings.Replace(template, "{KEY}", k, 1)
			}
			pathPub := func(template string) string {
				return strings.Replace(template, "{KEY}", "****"+lastN(k, 4), 1)
			}
			probesTpl := []probe{
				{Name: "codes", Tier: "Free", Path: "/v6/{KEY}/codes"},
				{Name: "pair", Tier: "Free", Path: "/v6/{KEY}/pair/USD/EUR"},
				{Name: "latest", Tier: "Free", Path: "/v6/{KEY}/latest/USD"},
				{Name: "quota", Tier: "Free", Path: "/v6/{KEY}/quota"},
				{Name: "enriched", Tier: "Business", Path: "/v6/{KEY}/enriched/USD/EUR"},
				{Name: "history", Tier: "Pro", Path: "/v6/{KEY}/history/USD/2024/1/1"},
			}
			probes := make([]probe, len(probesTpl))
			for i, p := range probesTpl {
				probes[i] = probe{Name: p.Name, Tier: p.Tier, Path: pathPub(p.Path)}
			}
			topTier := ""
			results := make([]probe, 0, len(probes))
			for i, p := range probes {
				body, err := c.Get(liveURL(probesTpl[i].Path), nil)
				if err == nil {
					var resp struct {
						Result    string `json:"result"`
						ErrorType string `json:"error-type"`
					}
					if json.Unmarshal(body, &resp) == nil && resp.Result == "success" {
						p.OK = true
					} else {
						p.OK = false
						p.Note = resp.ErrorType
					}
				} else {
					p.OK = false
					p.Note = err.Error()
					if strings.Contains(err.Error(), "plan-upgrade-required") {
						p.Note = "plan-upgrade-required"
					}
				}
				if p.OK {
					switch p.Tier {
					case "Pro":
						if tierRank(topTier) < tierRank("Pro") {
							topTier = "Pro"
						}
					case "Business":
						if tierRank(topTier) < tierRank("Business") {
							topTier = "Business"
						}
					default:
						if tierRank(topTier) < tierRank("Free") {
							topTier = "Free"
						}
					}
				}
				results = append(results, p)
			}
			if topTier == "" {
				topTier = "Unknown (auth failed or all probes errored)"
			}
			payload := map[string]any{
				"detected_tier": topTier,
				"probes":        results,
				"api_calls":     len(results),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Detected tier: %s\n\n", topTier)
			fmt.Fprintf(out, "%-12s %-12s %-6s %s\n", "ENDPOINT", "TIER", "OK", "NOTE")
			for _, p := range results {
				okStr := "no"
				if p.OK {
					okStr = "yes"
				}
				fmt.Fprintf(out, "%-12s %-12s %-6s %s\n", p.Name, p.Tier, okStr, p.Note)
			}
			return nil
		},
	}
	return cmd
}

func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func tierRank(t string) int {
	switch t {
	case "Free":
		return 1
	case "Pro":
		return 2
	case "Business":
		return 3
	case "Volume":
		return 4
	}
	return 0
}
