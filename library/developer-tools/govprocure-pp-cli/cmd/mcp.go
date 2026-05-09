package cmd

import (
	"fmt"
	"os"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the govprocure MCP server (stdio transport)",
	Long: `Start the govprocure Model Context Protocol server.

The server communicates over stdin/stdout using the MCP protocol.
Configure it in your Claude Desktop or other MCP client by pointing
to this binary:

  {
    "mcpServers": {
      "govprocure": {
        "command": "/path/to/govprocure-pp-cli",
        "args": ["mcp"]
      }
    }
  }

The server exposes these tools:
  search_grants       — Full-text search grants.gov
  search_sam          — Full-text search SAM.gov opportunities
  search_awards       — Full-text search USASpending.gov awards
  get_grant           — Fetch a single grant by ID
  get_sam_notice      — Fetch a single SAM notice by ID
  grants_closing_soon — List grants closing within N days
  sam_by_set_aside    — Filter SAM by SDVOSB/WOSB/8a/HUBZone
  agency_profile      — Full agency intelligence card
  sync_status         — Show last sync timestamps per source`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load config (%v), using defaults\n", err)
			cfg = &config.Config{
				DBPath:      "",
				GrantsURL:   config.DefaultGrantsURL,
				SAMURL:      config.DefaultSAMURL,
				USASpendURL: config.DefaultUSASpendURL,
				SyncDays:    config.DefaultSyncDays,
			}
			if dbPath, e := config.DefaultDBPath(); e == nil {
				cfg.DBPath = dbPath
			}
		}
		return mcp.Serve(cfg)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
