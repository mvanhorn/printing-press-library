package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a Steam player's profile summary",
		Long:  `Display player name, profile URL, avatar, online status, and other profile details.`,
		Example: `  # Show a player's profile
  steam-web-pp-cli profile 76561198000000000

  # Show profile as JSON
  steam-web-pp-cli profile 76561198000000000 --json

  # Show only selected fields
  steam-web-pp-cli profile 76561198000000000 --json --select personaname,profileurl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUser/GetPlayerSummaries/v2"
			params := map[string]string{
				"steamids": args[0],
			}
			data, prov, err := resolveRead(c, flags, "isteam-user", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract response.players array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if inner, ok := wrapper["response"]; ok {
					var resp map[string]json.RawMessage
					if err := json.Unmarshal(inner, &resp); err == nil {
						if players, ok := resp["players"]; ok {
							data = players
						}
					}
				}
			}

			// If it's an array with one element, unwrap it for single-player display
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
			var player map[string]any
			if err := json.Unmarshal(data, &player); err == nil {
				tw := newTabWriter(cmd.OutOrStdout())
				printField := func(label string, key string) {
					if v, ok := player[key]; ok && v != nil {
						fmt.Fprintf(tw, "%s\t%v\n", label, v)
					}
				}
				printField("Name", "personaname")
				printField("Steam ID", "steamid")
				printField("Profile URL", "profileurl")
				printField("Avatar", "avatarfull")
				// Map persona state to human-readable status
				if state, ok := player["personastate"]; ok {
					stateMap := map[float64]string{
						0: "Offline", 1: "Online", 2: "Busy",
						3: "Away", 4: "Snooze", 5: "Looking to trade", 6: "Looking to play",
					}
					if s, ok := state.(float64); ok {
						if label, found := stateMap[s]; found {
							fmt.Fprintf(tw, "Status\t%s\n", label)
						} else {
							fmt.Fprintf(tw, "Status\t%v\n", state)
						}
					}
				}
				printField("Real Name", "realname")
				printField("Country", "loccountrycode")
				printField("Created", "timecreated")
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
