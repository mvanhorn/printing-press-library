package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/acquisition/storefront"
	"github.com/spf13/cobra"
)

func newSuggestPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagQ string

	cmd := &cobra.Command{
		Use:         "suggest",
		Aliases:     []string{"sugestoes", "autocomplete"},
		Short:       "Get structured search suggestions",
		Long:        "Get structured product suggestions parsed from the continente.pt suggestion endpoint.",
		Example:     "  continente-pp-cli suggest --q leite",
		Annotations: map[string]string{"pp:endpoint": "on.get-suggestions", "pp:method": "GET", "pp:path": "/on/demandware.store/Sites-continente-Site/default/SearchServices-GetSuggestions", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("q") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "q")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := storefront.FetchSuggestions(cmd.Context(), c, fmt.Sprintf("%v", flagQ))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			payload, err := parseSuggestionsHTML(flagQ, data)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(payload.Items))
			for _, item := range payload.Items {
				rows = append(rows, storefrontItemHumanRow(item))
			}
			return emitStructuredOutput(cmd, flags, payload, DataProvenance{Source: "live", ResourceType: "suggestions"}, len(payload.Items), rows)
		},
	}
	cmd.Flags().StringVar(&flagQ, "q", "", "Search query")
	return cmd
}
