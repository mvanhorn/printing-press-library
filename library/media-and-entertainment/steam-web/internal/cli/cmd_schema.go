package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema <appid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show a game's stat and achievement schema",
		Long:  `Display the full schema of stats and achievements defined for a game.`,
		Example: `  # Show schema for CS2
  steam-web-pp-cli schema 730

  # Output as JSON
  steam-web-pp-cli schema 730 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUserStats/GetSchemaForGame/v2"
			params := map[string]string{
				"appid": args[0],
			}
			data, prov, err := resolveRead(c, flags, "isteam-user-stats", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract game.availableGameStats from the response
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if game, ok := wrapper["game"]; ok {
					var inner map[string]json.RawMessage
					if err := json.Unmarshal(game, &inner); err == nil {
						if gs, ok := inner["availableGameStats"]; ok {
							data = gs
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

			// Try to display as table — the schema usually has stats[] and achievements[]
			var schema map[string]json.RawMessage
			if json.Unmarshal(data, &schema) == nil {
				// Try stats array first
				for _, key := range []string{"achievements", "stats"} {
					if arr, ok := schema[key]; ok {
						var items []map[string]any
						if json.Unmarshal(arr, &items) == nil && len(items) > 0 {
							fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", key, len(items))
							if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
								return err
							}
							if len(items) >= 25 {
								fmt.Fprintf(os.Stderr, "\nShowing %d %s. To narrow: use --json --select.\n", len(items), key)
							}
							fmt.Fprintln(cmd.OutOrStdout())
						}
					}
				}
				return nil
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
