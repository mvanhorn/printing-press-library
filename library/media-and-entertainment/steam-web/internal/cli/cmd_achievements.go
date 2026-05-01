package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newAchievementsCmd(flags *rootFlags) *cobra.Command {
	var appID string

	cmd := &cobra.Command{
		Use:   "achievements <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a player's achievements for a game",
		Long:  `List all achievements a player has earned (or not) for a specific game.`,
		Example: `  # Show achievements for CS2 (appid 730)
  steam-web-pp-cli achievements 76561198000000000 --app 730

  # Output as JSON
  steam-web-pp-cli achievements 76561198000000000 --app 730 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUserStats/GetPlayerAchievements/v1"
			params := map[string]string{
				"steamid": args[0],
				"appid":   appID,
			}
			data, prov, err := resolveRead(c, flags, "isteam-user-stats", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract playerstats.achievements array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if ps, ok := wrapper["playerstats"]; ok {
					var inner map[string]json.RawMessage
					if err := json.Unmarshal(ps, &inner); err == nil {
						if achievements, ok := inner["achievements"]; ok {
							data = achievements
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

	cmd.Flags().StringVar(&appID, "app", "", "App ID of the game (required)")

	return cmd
}
