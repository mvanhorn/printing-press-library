package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newPlayersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "players <appid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show the current number of players for a game",
		Long:  `Get the current number of players online for a game by its App ID.`,
		Example: `  # Show current players for CS2
  steam-web-pp-cli players 730

  # Output as JSON
  steam-web-pp-cli players 730 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUserStats/GetNumberOfCurrentPlayers/v1"
			params := map[string]string{
				"appid": args[0],
			}
			data, prov, err := resolveRead(c, flags, "isteam-user-stats", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract response.player_count
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if inner, ok := wrapper["response"]; ok {
					data = inner
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

			// Human-friendly: just print the count
			var result map[string]any
			if err := json.Unmarshal(data, &result); err == nil {
				if pc, ok := result["player_count"]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "%v players online\n", pc)
					return nil
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
