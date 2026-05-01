package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newBadgesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "badges <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a player's badges",
		Long:  `List all badges a player has earned, with XP and level info.`,
		Example: `  # Show badges
  steam-web-pp-cli badges 76561198000000000

  # Output as JSON
  steam-web-pp-cli badges 76561198000000000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/IPlayerService/GetBadges/v1"
			params := map[string]string{
				"steamid": args[0],
			}
			data, prov, err := resolveRead(c, flags, "iplayer-service", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract response.badges array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if inner, ok := wrapper["response"]; ok {
					var resp map[string]json.RawMessage
					if err := json.Unmarshal(inner, &resp); err == nil {
						if badges, ok := resp["badges"]; ok {
							data = badges
						}
					}
				}
			}

			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.compact {
					filtered = compactFields(filtered)
				}
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
