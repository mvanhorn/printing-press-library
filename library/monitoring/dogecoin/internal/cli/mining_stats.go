package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/rpc"
	"github.com/spf13/cobra"
)

type miningStatsResult struct {
	BlockHeight    int64   `json:"block_height"`
	Difficulty     float64 `json:"difficulty"`
	HashrateNet    float64 `json:"hashrate_net_hps"`
	HashrateNetTH  float64 `json:"hashrate_net_ths"`
	Generate       bool    `json:"generate"`
	Synced         bool    `json:"synced"`
	SyncProgress   float64 `json:"sync_progress,omitempty"`
	NodeVersion    string  `json:"node_version,omitempty"`
	VersionWarning string  `json:"version_warning,omitempty"`
	ErrorsMsg      string  `json:"errors_msg,omitempty"`
}

func newMiningStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Compound mining snapshot: hashrate, difficulty, block height, network hashrate",
		Long:  "Calls getmininginfo + getblockchaininfo + getnetworkhashps in one command. Designed for n8n shell nodes and XEMD dashboard widgets.",
		Example: strings.TrimLeft(`
  dogecoin-pp-cli mining stats --json
  dogecoin-pp-cli mining stats --compact
  dogecoin-pp-cli mining stats --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		// pp:client-call — calls Dogecoin Core JSON-RPC via rpc.Client
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"calls":["getmininginfo","getblockchaininfo","getnetworkhashps"]}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			result, err := buildMiningStats(ctx, c)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func buildMiningStats(ctx context.Context, c *rpc.Client) (*miningStatsResult, error) {
	miningRaw, err := c.Call(ctx, "getmininginfo", nil)
	if err != nil {
		return nil, fmt.Errorf("getmininginfo: %w", err)
	}
	chainRaw, err := c.Call(ctx, "getblockchaininfo", nil)
	if err != nil {
		return nil, fmt.Errorf("getblockchaininfo: %w", err)
	}
	hashRaw, err := c.Call(ctx, "getnetworkhashps", nil)
	if err != nil {
		return nil, fmt.Errorf("getnetworkhashps: %w", err)
	}

	var mining struct {
		Blocks        int64   `json:"blocks"`
		Difficulty    float64 `json:"difficulty"`
		NetworkHashPS float64 `json:"networkhashps"`
		Generate      bool    `json:"generate"`
		Errors        string  `json:"errors"`
	}
	if err := json.Unmarshal(miningRaw, &mining); err != nil {
		return nil, fmt.Errorf("parsing getmininginfo: %w", err)
	}

	var chain struct {
		VerificationProgress float64 `json:"verificationprogress"`
	}
	if err := json.Unmarshal(chainRaw, &chain); err != nil {
		return nil, fmt.Errorf("parsing getblockchaininfo: %w", err)
	}

	var networkHashPS float64
	if err := json.Unmarshal(hashRaw, &networkHashPS); err != nil {
		// Sometimes getnetworkhashps is not available; use what getmininginfo gave us
		networkHashPS = mining.NetworkHashPS
	}

	synced := chain.VerificationProgress > 0.9999
	result := &miningStatsResult{
		BlockHeight:   mining.Blocks,
		Difficulty:    mining.Difficulty,
		HashrateNet:   networkHashPS,
		HashrateNetTH: networkHashPS / 1e12,
		Generate:      mining.Generate,
		Synced:        synced,
	}
	if !synced {
		result.SyncProgress = chain.VerificationProgress
	}
	if mining.Errors != "" {
		result.ErrorsMsg = mining.Errors
		if isObsoleteWarning(mining.Errors) {
			result.VersionWarning = "Node version is obsolete — upgrade to Dogecoin Core 1.14.x recommended"
		}
	}
	return result, nil
}

func isObsoleteWarning(msg string) bool {
	return len(msg) > 0 && (contains(msg, "obsolete") || contains(msg, "upgrade required"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
