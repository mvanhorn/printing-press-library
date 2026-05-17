package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newMempoolStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Compound mempool snapshot: size, bytes, and fee estimates",
		Long:  "Calls getmempoolinfo + estimatefee in one command. Returns pool depth and fee estimates for 1-block and 6-block confirmation targets.",
		Example: `  dogecoin-pp-cli mempool status --json
  dogecoin-pp-cli mempool status --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		// pp:client-call — calls getmempoolinfo + estimatefee via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"calls":["getmempoolinfo","estimatefee"]}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			poolRaw, err := c.Call(ctx, "getmempoolinfo", nil)
			if err != nil {
				return fmt.Errorf("getmempoolinfo: %w", err)
			}
			var pool struct {
				Size  int64 `json:"size"`
				Bytes int64 `json:"bytes"`
			}
			if err := json.Unmarshal(poolRaw, &pool); err != nil {
				return fmt.Errorf("parsing mempool: %w", err)
			}

			fee1 := estimateFeeOrNA(ctx, c, 1)
			fee6 := estimateFeeOrNA(ctx, c, 6)

			result := map[string]any{
				"tx_count":               pool.Size,
				"pool_bytes":             pool.Bytes,
				"fee_1block_doge_per_kb": fee1,
				"fee_6block_doge_per_kb": fee6,
			}

			// Add fee tier labels
			if fee1 >= 0 {
				switch {
				case fee1 < 0.001:
					result["fee_tier"] = "low"
				case fee1 < 0.01:
					result["fee_tier"] = "medium"
				default:
					result["fee_tier"] = "high"
				}
			} else {
				result["fee_note"] = "fee estimation unavailable — insufficient blockchain data"
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func estimateFeeOrNA(ctx context.Context, c interface {
	Call(context.Context, string, any) (json.RawMessage, error)
}, blocks int) float64 {
	raw, err := c.Call(ctx, "estimatefee", []any{blocks})
	if err != nil {
		return -1
	}
	var fee float64
	if err := json.Unmarshal(raw, &fee); err != nil {
		return -1
	}
	return fee
}
