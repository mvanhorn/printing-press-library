// Hand-rewritten: monday.com is GraphQL — replaces the generator's REST scaffold.

package cli

import (
	"github.com/spf13/cobra"
)

const queryAccountInfo = `query {
  account { id name slug tier country_code logo show_timeline_weekends sign_up_product_kind }
}`

func newAccountInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Return account-level metadata for the authenticated monday.com workspace.",
		Long:    "Show account id, name, slug, plan tier, country code, and sign-up product (work-management, dev, sales-crm, etc.).",
		Example: "  monday-pp-cli account get --json",
		Annotations: map[string]string{
			"pp:endpoint":   "account.get",
			"pp:method":     "GET",
			"pp:path":       "/graphql",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryAccountInfo, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "account")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
