// Novel command: sync-rates. Pulls /latest for a base currency and writes
// every (base, target, rate) row to the local rates_snapshots append-only
// table. This is the data foundation for history-cache, drift, pair --as-of,
// and watch — local snapshots reconstruct history for free-tier users who
// can't access /history.
package cli

// PATCH exchangerate-novel-sync-rates: append rates_snapshots from /latest; foundation for snapshot-driven features.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/exchangerate-api/internal/store"
	"github.com/spf13/cobra"
)

func newSyncRatesCmd(flags *rootFlags) *cobra.Command {
	var (
		base   string
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "sync-rates",
		Short: "Fetch latest rates for a base currency and append to rates_snapshots",
		Long: `Issues one /latest call against --base (default USD) and appends every
(base, target, rate) pair to the local rates_snapshots table with a
captured_at timestamp. Run this daily (cron) to build your own free-tier
historical archive.`,
		Example:     "  exchangerate-api-pp-cli sync-rates --base USD\n  exchangerate-api-pp-cli sync-rates --base EUR --json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if base == "" {
				base = "USD"
			}
			base = strings.ToUpper(base)
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
			count, capturedBase, _, syncErr := syncRatesOnce(cmd.Context(), c, base, dbPath)
			if syncErr != nil {
				return syncErr
			}
			payload := map[string]any{
				"base":              capturedBase,
				"snapshots_written": count,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d rate snapshots for base %s\n", count, capturedBase)
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "USD", "Base currency to snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// syncRatesOnce is the reusable core used by sync-rates and watch check.
func syncRatesOnce(ctx context.Context, c *client.Client, base, dbPath string) (int, string, map[string]float64, error) {
	path := fmt.Sprintf("/v6/%s/latest/%s", c.Config.ExchangerateApiKey, base)
	body, err := c.Get(path, nil)
	if err != nil {
		return 0, "", nil, fmt.Errorf("fetching /latest/%s: %w", base, err)
	}
	var resp struct {
		Result          string             `json:"result"`
		ErrorType       string             `json:"error-type"`
		BaseCode        string             `json:"base_code"`
		ConversionRates map[string]float64 `json:"conversion_rates"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, "", nil, fmt.Errorf("parsing /latest: %w", err)
	}
	if resp.Result != "success" {
		return 0, "", nil, fmt.Errorf("API error: %s", resp.ErrorType)
	}
	if dbPath == "" {
		dbPath = defaultDBPath("exchangerate-api-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return 0, resp.BaseCode, resp.ConversionRates, fmt.Errorf("opening store: %w", err)
	}
	defer s.Close()
	n, err := s.InsertRateSnapshots(ctx, resp.BaseCode, "api:/latest", resp.ConversionRates)
	if err != nil {
		return n, resp.BaseCode, resp.ConversionRates, err
	}
	return n, resp.BaseCode, resp.ConversionRates, nil
}
