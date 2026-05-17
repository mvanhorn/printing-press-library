package cli

import (
	"github.com/spf13/cobra"
)

// health is a top-level shortcut for 'node health' — quick node health check.
func newHealthCmd(flags *rootFlags) *cobra.Command {
	inner := newNodeHealthCmd(flags)
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Node health check: version, sync, peers — exit 0=healthy, exit 1=unhealthy",
		Long:  "Shortcut for 'node health'. " + inner.Long,
		Example: `  dogecoin-pp-cli health
  dogecoin-pp-cli health --json`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,1",
		},
		RunE: inner.RunE,
	}
	cmd.Flags().AddFlagSet(inner.Flags())
	return cmd
}
