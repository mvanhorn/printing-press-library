package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/mcp"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP stdio JSON-RPC server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcp.Run(os.Stdin, os.Stdout, os.Stderr)
		},
	}
}
