package cli

import (
	"github.com/spf13/cobra"
)

// stats is a top-level shortcut for 'mining stats' — the most common command
// for n8n shell nodes and XEMD dashboard widgets.
func newStatsCmd(flags *rootFlags) *cobra.Command {
	inner := newMiningStatsCmd(flags)
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Compound mining snapshot: hashrate, difficulty, block height, network hashrate",
		Long:  "Shortcut for 'mining stats'. " + inner.Long,
		Example: `  dogecoin-pp-cli stats --json
  dogecoin-pp-cli stats --compact --agent`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: inner.RunE,
	}
	// Copy flags from the inner command
	cmd.Flags().AddFlagSet(inner.Flags())
	return cmd
}
