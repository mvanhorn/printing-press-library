package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newGamesCmd(flags *rootFlags) *cobra.Command {
	var includeFree bool
	var includeInfo bool

	cmd := &cobra.Command{
		Use:   "games <steamid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List a player's owned games with playtime",
		Long:  `Show all games owned by a Steam user, including playtime in minutes.`,
		Example: `  # List owned games
  steam-web-pp-cli games 76561198000000000

  # Include free-to-play games and app info
  steam-web-pp-cli games 76561198000000000 --include-free --include-info

  # Output as JSON
  steam-web-pp-cli games 76561198000000000 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/IPlayerService/GetOwnedGames/v1"
			params := map[string]string{
				"steamid": args[0],
			}
			if includeFree {
				params["include_played_free_games"] = "true"
			}
			if includeInfo {
				params["include_appinfo"] = "true"
			}
			data, prov, err := resolveRead(c, flags, "iplayer-service", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract response.games array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if inner, ok := wrapper["response"]; ok {
					var resp map[string]json.RawMessage
					if err := json.Unmarshal(inner, &resp); err == nil {
						if games, ok := resp["games"]; ok {
							data = games
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

	cmd.Flags().BoolVar(&includeFree, "include-free", false, "Include free-to-play games the user has played")
	cmd.Flags().BoolVar(&includeInfo, "include-info", false, "Include additional app info (name, icon)")

	return cmd
}
