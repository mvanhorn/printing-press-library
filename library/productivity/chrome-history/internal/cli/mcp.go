package cli

import (
	"strings"

	imcp "github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "mcp",
		Short:   "Start MCP stdio server",
		Example: strings.Trim("chrome-history-pp-cli mcp", "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return imcp.ServeStdio()
		},
	}
}
