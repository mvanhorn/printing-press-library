// Novel command: open. Hits the keyless open-access endpoint
// (https://open.er-api.com/v6/latest/<BASE>) so the CLI is useful even
// without an API key. Attribution: rates by ExchangeRate-API.
package cli

// PATCH exchangerate-novel-open: keyless https://open.er-api.com/v6/latest endpoint with attribution.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const openAccessBase = "https://open.er-api.com/v6/latest"

func newOpenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <base_code>",
		Short: "Latest rates from the keyless open-access endpoint (24h refresh, attribution required)",
		Long: `Hits https://open.er-api.com/v6/latest/<BASE> — the no-key open-access
endpoint operated by ExchangeRate-API. Data refreshes every 24 hours and
the response includes 'time_eol_unix' marking when the cached snapshot
expires.

Per their terms, if you redistribute this data you MUST display
"Rates By Exchange Rate API" linking to https://www.exchangerate-api.com.`,
		Example:     "  exchangerate-api-pp-cli open USD\n  exchangerate-api-pp-cli open EUR --json --select rates.JPY",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			base := strings.ToUpper(strings.TrimSpace(args[0]))
			if base == "" {
				return usageErr(fmt.Errorf("base currency is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			url := openAccessBase + "/" + base
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return fmt.Errorf("building request: %w", err)
			}
			req.Header.Set("User-Agent", "exchangerate-api-pp-cli/"+version)
			httpClient := &http.Client{Timeout: 15 * time.Second}
			resp, err := httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", url, err)
			}
			defer resp.Body.Close()
			// Cap at 5 MiB: the keyless endpoint's full /latest payload is
			// ~3 KB for 161 currencies. A misconfigured or malicious upstream
			// returning an unbounded stream would OOM the process without
			// the limit.
			body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}
			if resp.StatusCode == http.StatusTooManyRequests {
				return fmt.Errorf("rate-limited by open-access endpoint (HTTP 429). Wait ~20 minutes or switch to keyed 'rates latest %s'", base)
			}
			if resp.StatusCode >= 400 {
				return fmt.Errorf("open-access endpoint returned HTTP %d: %s", resp.StatusCode, truncOpen(string(body)))
			}
			// Pass through the raw payload (it's a flat JSON object already).
			payload := map[string]any{
				"source":      "open.er-api.com",
				"attribution": "Rates By Exchange Rate API (https://www.exchangerate-api.com)",
			}
			var raw map[string]any
			if err := json.Unmarshal(body, &raw); err != nil {
				return fmt.Errorf("parsing open-access response: %w", err)
			}
			for k, v := range raw {
				payload[k] = v
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Base: %s (no-key endpoint, refreshes daily)\n", base)
			if r, ok := raw["rates"].(map[string]any); ok {
				fmt.Fprintf(out, "Rates loaded: %d currencies\n", len(r))
				for _, code := range []string{"EUR", "GBP", "JPY", "CAD", "AUD", "CHF"} {
					if v, ok := r[code]; ok {
						fmt.Fprintf(out, "  %s = %v\n", code, v)
					}
				}
			}
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "Attribution: Rates By Exchange Rate API (https://www.exchangerate-api.com)")
			return nil
		},
	}
	return cmd
}

func truncOpen(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
