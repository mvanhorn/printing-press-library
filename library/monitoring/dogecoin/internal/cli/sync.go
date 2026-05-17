package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/rpc"
	"github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/store"
	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var blockWindow int
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync node state to local SQLite store for historical trending",
		Long:  "Polls getmininginfo, getnetworkinfo, getpeerinfo, getmempoolinfo, getwalletinfo, and recent blocks; stores results in SQLite for use by history and alert commands.",
		Example: `  dogecoin-pp-cli sync
  dogecoin-pp-cli sync --block-window 100 --json`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"tables":["mining_snapshots","block_events"]}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = store.DefaultPath()
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			ctx := context.Background()
			now := time.Now().Unix()

			snap, syncErr := collectSnapshot(ctx, c)
			if syncErr != nil {
				return fmt.Errorf("collecting snapshot: %w", syncErr)
			}
			snap.TS = now

			// Skip degenerate snapshots (node returned zero block height and zero difficulty),
			// which can occur when the node is restarting or returning a transient bad response.
			if snap.BlockHeight == 0 && snap.Difficulty == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), `{"event":"snapshot_skipped","reason":"degenerate_response","block_height":0,"difficulty":0}`)
			} else if err := s.InsertSnapshot(ctx, *snap); err != nil {
				return fmt.Errorf("storing snapshot: %w", err)
			}

			// Persist sync cursor for incremental sync
			_ = s.SaveSyncState(ctx, "last_synced_block", fmt.Sprintf("%d", snap.BlockHeight))
			_ = s.SaveSyncState(ctx, "last_synced_at", fmt.Sprintf("%d", now))

			// Load cursor to determine start height for block sync
			cursor, _ := s.GetSyncState(ctx, "last_synced_block")
			if cursor != nil {
				// Already know the last synced block; use that as start for next sync
				_ = cursor // cursor-based sync: blocks already stored up to here
			}

			// Sync recent blocks
			blocksNew, err := syncBlocks(ctx, c, s, snap.BlockHeight, blockWindow)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: block sync failed: %v\n", err)
			}

			result := map[string]any{
				"synced_at":     time.Unix(now, 0).UTC().Format(time.RFC3339),
				"block_height":  snap.BlockHeight,
				"difficulty":    snap.Difficulty,
				"hashrate_net":  snap.HashrateNet,
				"peer_count":    snap.PeerCount,
				"mempool_size":  snap.MempoolSize,
				"blocks_stored": blocksNew,
				"db_path":       dbPath,
			}
			if snap.VersionObs {
				result["version_warning"] = "node version obsolete — upgrade to Dogecoin Core 1.14.x"
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	cmd.Flags().IntVar(&blockWindow, "block-window", 50, "Number of recent blocks to sync")
	return cmd
}

func collectSnapshot(ctx context.Context, c *rpc.Client) (*store.MiningSnapshot, error) {
	snap := &store.MiningSnapshot{}

	miningRaw, err := c.Call(ctx, "getmininginfo", nil)
	if err != nil {
		return nil, fmt.Errorf("getmininginfo: %w", err)
	}
	var mining struct {
		Blocks     int64   `json:"blocks"`
		Difficulty float64 `json:"difficulty"`
		Errors     string  `json:"errors"`
	}
	if err := json.Unmarshal(miningRaw, &mining); err != nil {
		return nil, err
	}
	snap.BlockHeight = mining.Blocks
	snap.Difficulty = mining.Difficulty
	snap.ErrorsMsg = mining.Errors
	snap.VersionObs = isObsoleteWarning(mining.Errors)

	hashRaw, err := c.Call(ctx, "getnetworkhashps", nil)
	if err == nil {
		var h float64
		if json.Unmarshal(hashRaw, &h) == nil {
			snap.HashrateNet = h
		}
	}

	netRaw, err := c.Call(ctx, "getnetworkinfo", nil)
	if err == nil {
		var net struct {
			Version     int64 `json:"version"`
			Connections int64 `json:"connections"`
		}
		if json.Unmarshal(netRaw, &net) == nil {
			snap.Version = net.Version
			snap.PeerCount = net.Connections
		}
	}

	poolRaw, err := c.Call(ctx, "getmempoolinfo", nil)
	if err == nil {
		var pool struct {
			Size  int64 `json:"size"`
			Bytes int64 `json:"bytes"`
		}
		if json.Unmarshal(poolRaw, &pool) == nil {
			snap.MempoolSize = pool.Size
			snap.MempoolBytes = pool.Bytes
		}
	}

	walletRaw, err := c.Call(ctx, "getwalletinfo", nil)
	if err == nil {
		var wallet struct {
			Balance float64 `json:"balance"`
		}
		if json.Unmarshal(walletRaw, &wallet) == nil {
			snap.WalletBalance = wallet.Balance
		}
	}

	return snap, nil
}

func syncBlocks(ctx context.Context, c *rpc.Client, s *store.Store, tipHeight int64, window int) (int, error) {
	if window <= 0 {
		window = 50
	}
	startHeight := tipHeight - int64(window)
	if startHeight < 0 {
		startHeight = 0
	}

	highest, err := s.HighestBlock(ctx)
	if err == nil && highest > startHeight {
		startHeight = highest + 1
	}

	stored := 0
	for h := startHeight; h <= tipHeight; h++ {
		hashRaw, err := c.Call(ctx, "getblockhash", []any{h})
		if err != nil {
			continue
		}
		var blockHash string
		if err := json.Unmarshal(hashRaw, &blockHash); err != nil {
			continue
		}
		blockRaw, err := c.Call(ctx, "getblock", []any{blockHash})
		if err != nil {
			continue
		}
		var block struct {
			Height     int64    `json:"height"`
			Hash       string   `json:"hash"`
			Time       int64    `json:"time"`
			Difficulty float64  `json:"difficulty"`
			Tx         []string `json:"tx"`
		}
		if err := json.Unmarshal(blockRaw, &block); err != nil {
			continue
		}
		ev := store.BlockEvent{
			TS:          block.Time,
			BlockHeight: block.Height,
			BlockHash:   block.Hash,
			Difficulty:  block.Difficulty,
			TxCount:     int64(len(block.Tx)),
		}
		if err := s.UpsertBlock(ctx, ev); err == nil {
			stored++
		}
	}
	return stored, nil
}
