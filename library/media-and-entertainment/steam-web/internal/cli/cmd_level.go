package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newLevelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "level <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a player's Steam level",
		Long:  `Get the Steam level for a player by their Steam ID.`,
		Example: `  # Show Steam level
  steam-web-pp-cli level 76561198000000000

  # Output as JSON
  steam-web-pp-cli level 76561198000000000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/IPlayerService/GetSteamLevel/v1"
			params := map[string]string{
				"steamid": args[0],
			}
			data, prov, err := resolveRead(c, flags, "iplayer-service", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract response.player_level
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

			// Human-friendly: just print the level
			var result map[string]any
			if err := json.Unmarshal(data, &result); err == nil {
				if lvl, ok := result["player_level"]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "Level %v\n", lvl)
					return nil
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
