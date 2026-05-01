package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve <vanityurl>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Resolve a Steam vanity URL to a Steam ID",
		Long:  `Look up a Steam user's 64-bit Steam ID from their custom profile URL name.`,
		Example: `  # Resolve a vanity URL to a Steam ID
  steam-web-pp-cli resolve trevin

  # Output as JSON
  steam-web-pp-cli resolve trevin --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamUser/ResolveVanityURL/v1"
			params := map[string]string{
				"vanityurl": args[0],
			}
			data, prov, err := resolveRead(c, flags, "isteam-user", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract the response wrapper
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

			// Human-friendly output: just print the Steam ID
			var result map[string]any
			if err := json.Unmarshal(data, &result); err == nil {
				if steamid, ok := result["steamid"]; ok {
					fmt.Fprintln(cmd.OutOrStdout(), steamid)
					return nil
				}
				if msg, ok := result["message"]; ok {
					return fmt.Errorf("resolve failed: %v", msg)
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
