package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const exitCodeLowPeers = 3

func newPeersHealthCmd(flags *rootFlags) *cobra.Command {
	var minPeers int
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check peer count — exit 0=healthy, exit 3=below threshold",
		Long:  "Counts active peer connections. Exit 0 when peer count >= --min-peers; exit 3 when below. Use in n8n shell nodes to branch on peer health without parsing output.",
		Example: `  dogecoin-pp-cli peers health
  dogecoin-pp-cli peers health --min-peers 6 --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		// pp:client-call — calls Dogecoin Core JSON-RPC via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getpeerinfo","exit_0":"healthy","exit_3":"low_peers"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			raw, err := c.Call(ctx, "getpeerinfo", nil)
			if err != nil {
				return fmt.Errorf("getpeerinfo: %w", err)
			}
			var peers []struct {
				Addr    string `json:"addr"`
				Inbound bool   `json:"inbound"`
				Version int64  `json:"version"`
				SubVer  string `json:"subver"`
			}
			if err := json.Unmarshal(raw, &peers); err != nil {
				return fmt.Errorf("parsing peers: %w", err)
			}

			cfg, _ := flags.loadConfig()
			threshold := minPeers
			if threshold <= 0 && cfg != nil && cfg.MinPeers > 0 {
				threshold = cfg.MinPeers
			}
			if threshold <= 0 {
				threshold = 8
			}

			count := len(peers)
			healthy := count >= threshold
			result := map[string]any{
				"peer_count": count,
				"threshold":  threshold,
				"healthy":    healthy,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
				return err
			}
			if !healthy {
				fmt.Fprintf(os.Stderr, "low peer count: %d < %d\n", count, threshold)
				os.Exit(exitCodeLowPeers)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&minPeers, "min-peers", 0, "Minimum peer count for healthy status (default: config min_peers or 8)")
	return cmd
}

func newPeersBreakdownCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "breakdown",
		Short: "Peer version and connection type breakdown",
		Long:  "Groups peers by client version (subver) and inbound/outbound connection type. Useful for network health reporting.",
		Example: `  dogecoin-pp-cli peers breakdown --json
  dogecoin-pp-cli peers breakdown --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		// pp:client-call — calls Dogecoin Core JSON-RPC via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getpeerinfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			raw, err := c.Call(ctx, "getpeerinfo", nil)
			if err != nil {
				return fmt.Errorf("getpeerinfo: %w", err)
			}
			var peers []struct {
				Addr    string `json:"addr"`
				Inbound bool   `json:"inbound"`
				SubVer  string `json:"subver"`
			}
			if err := json.Unmarshal(raw, &peers); err != nil {
				return fmt.Errorf("parsing peers: %w", err)
			}

			byVersion := map[string]int{}
			inbound, outbound := 0, 0
			for _, p := range peers {
				sv := p.SubVer
				if sv == "" {
					sv = "unknown"
				}
				byVersion[sv]++
				if p.Inbound {
					inbound++
				} else {
					outbound++
				}
			}

			result := map[string]any{
				"total":     len(peers),
				"inbound":   inbound,
				"outbound":  outbound,
				"by_client": byVersion,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
