package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// obsoleteVersionThreshold is Dogecoin Core 1.14.0 numeric version.
const obsoleteVersionThreshold = 1140000

func newNodeHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Node health check: version, sync, peers — exit 0=healthy, exit 1=unhealthy",
		Long:  "Checks node version (warns if < 1.14.0), blockchain sync progress, and peer count. Exit 0 when all checks pass; exit 1 if any critical check fails.",
		Example: `  dogecoin-pp-cli node health
  dogecoin-pp-cli node health --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		// pp:client-call — calls getnetworkinfo + getblockchaininfo + uptime via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"calls":["getnetworkinfo","getblockchaininfo","uptime"]}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()

			netRaw, err := c.Call(ctx, "getnetworkinfo", nil)
			if err != nil {
				return fmt.Errorf("getnetworkinfo: %w", err)
			}
			var netInfo struct {
				Version     int64  `json:"version"`
				SubVersion  string `json:"subversion"`
				Connections int64  `json:"connections"`
			}
			if err := json.Unmarshal(netRaw, &netInfo); err != nil {
				return fmt.Errorf("parsing network info: %w", err)
			}

			chainRaw, err := c.Call(ctx, "getblockchaininfo", nil)
			if err != nil {
				return fmt.Errorf("getblockchaininfo: %w", err)
			}
			var chainInfo struct {
				Blocks               int64   `json:"blocks"`
				VerificationProgress float64 `json:"verificationprogress"`
			}
			if err := json.Unmarshal(chainRaw, &chainInfo); err != nil {
				return fmt.Errorf("parsing chain info: %w", err)
			}

			uptimeRaw, _ := c.Call(ctx, "uptime", nil)
			var uptime int64
			_ = json.Unmarshal(uptimeRaw, &uptime)

			versionObs := netInfo.Version < obsoleteVersionThreshold
			synced := chainInfo.VerificationProgress > 0.9999
			healthy := synced && netInfo.Connections > 0

			result := map[string]any{
				"healthy":          healthy,
				"version":          netInfo.Version,
				"subversion":       netInfo.SubVersion,
				"version_obsolete": versionObs,
				"connections":      netInfo.Connections,
				"block_height":     chainInfo.Blocks,
				"sync_progress":    chainInfo.VerificationProgress,
				"synced":           synced,
				"uptime_seconds":   uptime,
			}
			if versionObs {
				result["version_warning"] = fmt.Sprintf("Node running version %d — upgrade to Dogecoin Core 1.14.x recommended (current: 1.14.x series)", netInfo.Version)
			}

			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			if versionObs {
				fmt.Fprintf(os.Stderr, "WARNING: node version %d is obsolete; upgrade to Dogecoin Core 1.14.x\n", netInfo.Version)
			}
			if !healthy {
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}
