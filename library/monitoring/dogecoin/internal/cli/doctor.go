package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/config"
	"github.com/spf13/cobra"
)

func looksLikeDoctorInterstitial(body []byte) string { return "" }

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health: auth, node connectivity, version, sync, peers",
		Example: `  dogecoin-pp-cli doctor
  dogecoin-pp-cli doctor --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{}
			ok := true

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				report["config"] = fmt.Sprintf("error: %s", err)
				ok = false
			} else {
				report["config"] = "ok"
				report["config_path"] = cfg.Path
				report["base_url"] = cfg.BaseURL
				report["rpc_user_set"] = cfg.RPCUser != ""
			}

			if cfg != nil {
				if cfg.RPCUser == "" {
					report["auth"] = "warning: DOGECOIN_RPC_USER not set — set env var or add rpc_user to config"
				} else {
					report["auth"] = fmt.Sprintf("rpc_user=%s (source: %s)", cfg.RPCUser, cfg.AuthSource)
				}

				c := newRPCClientFromConfig(cfg)
				ctx := context.Background()

				netRaw, netErr := c.Call(ctx, "getnetworkinfo", nil)
				if netErr != nil {
					report["api"] = fmt.Sprintf("error: %s", netErr)
					ok = false
				} else {
					report["api"] = "reachable"
					var netInfo struct {
						Version     int64  `json:"version"`
						SubVersion  string `json:"subversion"`
						Connections int64  `json:"connections"`
					}
					if json.Unmarshal(netRaw, &netInfo) == nil {
						report["node_version"] = netInfo.Version
						report["node_subversion"] = netInfo.SubVersion
						report["peer_count"] = netInfo.Connections
						if netInfo.Version < obsoleteVersionThreshold {
							report["version_warning"] = fmt.Sprintf(
								"node version %d is OBSOLETE — upgrade to Dogecoin Core 1.14.x (version >= 1140000)",
								netInfo.Version)
						}
					}
				}

				chainRaw, chainErr := c.Call(ctx, "getblockchaininfo", nil)
				if chainErr == nil {
					var chain struct {
						Blocks               int64   `json:"blocks"`
						VerificationProgress float64 `json:"verificationprogress"`
					}
					if json.Unmarshal(chainRaw, &chain) == nil {
						report["block_height"] = chain.Blocks
						report["sync_progress"] = chain.VerificationProgress
						report["synced"] = chain.VerificationProgress > 0.9999
					}
				}

				walletRaw, _ := c.Call(ctx, "getwalletinfo", nil)
				if walletRaw != nil {
					var wallet struct {
						Balance float64 `json:"balance"`
					}
					if json.Unmarshal(walletRaw, &wallet) == nil {
						report["wallet_balance"] = wallet.Balance
					}
				}
			}

			report["overall"] = "ok"
			if !ok {
				report["overall"] = "error"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			// Human output
			for k, v := range report {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %v\n", k+":", v)
			}
			if vw, ok := report["version_warning"].(string); ok && vw != "" {
				fmt.Fprintf(os.Stderr, "\nWARNING: %s\n", vw)
			}
			return nil
		},
	}
	return cmd
}
