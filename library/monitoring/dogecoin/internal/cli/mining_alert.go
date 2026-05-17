package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/store"
	"github.com/spf13/cobra"
)

const exitCodeHashrateDrop = 3

func newMiningAlertCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var dbPath string
	cmd := &cobra.Command{
		Use:   "alert",
		Short: "Alert when network hashrate drops by threshold percent",
		Long:  "Compares current network hashrate to the last stored snapshot. Exits 3 if the drop exceeds --threshold percent. Stores a new snapshot on every call.",
		Example: `  dogecoin-pp-cli mining alert
  dogecoin-pp-cli mining alert --threshold 20 --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		// pp:client-call — calls Dogecoin Core JSON-RPC via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getnetworkhashps","exit_3":"drop_exceeds_threshold"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			last, err := s.LatestSnapshot(ctx)
			if err != nil {
				return fmt.Errorf("reading store: %w", err)
			}

			// Collect full snapshot (hashrate + difficulty + peers + mempool)
			snap, err := collectSnapshot(ctx, c)
			if err != nil {
				return fmt.Errorf("collecting snapshot: %w", err)
			}
			snap.TS = time.Now().Unix()
			_ = s.InsertSnapshot(ctx, *snap)
			currentHashrate := snap.HashrateNet

			result := map[string]any{
				"hashrate_net_hps": currentHashrate,
				"hashrate_net_ths": currentHashrate / 1e12,
				"alert":            false,
			}

			if last != nil && last.HashrateNet > 0 {
				dropPct := (last.HashrateNet - currentHashrate) / last.HashrateNet * 100
				result["previous_hashrate_hps"] = last.HashrateNet
				result["drop_pct"] = dropPct
				result["threshold_pct"] = threshold

				if dropPct >= threshold {
					result["alert"] = true
					result["message"] = fmt.Sprintf("hashrate dropped %.1f%% (threshold: %.0f%%)", dropPct, threshold)
					if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
						return err
					}
					fmt.Fprintf(os.Stderr, "hashrate alert: %.1f%% drop detected\n", dropPct)
					os.Exit(exitCodeHashrateDrop)
				}
			} else {
				result["message"] = "no previous snapshot — baseline established"
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 20.0, "Percent drop that triggers exit 3")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: ~/.local/share/dogecoin-pp-cli/db.sqlite)")
	return cmd
}
