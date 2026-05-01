package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newBansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bans <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Check a player's VAC and game ban status",
		Long:  `Show whether a player has any VAC bans, game bans, or trade bans.`,
		Example: `  # Check ban status
  steam-web-pp-cli bans 76561198000000000

  # Output as JSON
  steam-web-pp-cli bans 76561198000000000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUser/GetPlayerBans/v1"
			params := map[string]string{
				"steamids": args[0],
			}
			data, prov, err := resolveRead(c, flags, "isteam-user", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract players array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if players, ok := wrapper["players"]; ok {
					data = players
				}
			}

			// If it's an array with one element, unwrap for single-player display
			var items []json.RawMessage
			if json.Unmarshal(data, &items) == nil && len(items) == 1 {
				data = items[0]
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

			// Human-friendly output
			var ban map[string]any
			if err := json.Unmarshal(data, &ban); err == nil {
				tw := newTabWriter(cmd.OutOrStdout())
				if v, ok := ban["SteamId"]; ok {
					fmt.Fprintf(tw, "Steam ID\t%v\n", v)
				}
				if v, ok := ban["CommunityBanned"]; ok {
					fmt.Fprintf(tw, "Community Banned\t%v\n", v)
				}
				if v, ok := ban["VACBanned"]; ok {
					fmt.Fprintf(tw, "VAC Banned\t%v\n", v)
				}
				if v, ok := ban["NumberOfVACBans"]; ok {
					fmt.Fprintf(tw, "VAC Bans\t%v\n", v)
				}
				if v, ok := ban["DaysSinceLastBan"]; ok {
					fmt.Fprintf(tw, "Days Since Last Ban\t%v\n", v)
				}
				if v, ok := ban["NumberOfGameBans"]; ok {
					fmt.Fprintf(tw, "Game Bans\t%v\n", v)
				}
				if v, ok := ban["EconomyBan"]; ok {
					fmt.Fprintf(tw, "Economy Ban\t%v\n", v)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				return nil
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var tableItems []map[string]any
				if json.Unmarshal(data, &tableItems) == nil && len(tableItems) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), tableItems); err != nil {
						return err
					}
					if len(tableItems) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(tableItems))
					}
					return nil
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
