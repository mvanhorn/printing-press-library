// Hand-rewritten: monday.com is GraphQL — replaces the generator's REST scaffold.

package cli

import (
	"github.com/spf13/cobra"
)

const queryAccountMe = `query {
  me {
    id
    name
    email
    title
    enabled
    is_admin
    is_guest
    is_view_only
    created_at
    teams { id name }
    account { id name slug tier country_code }
  }
}`

func newAccountMeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "me",
		Short:   "Return the authenticated monday.com user (the API token's owner).",
		Long:    "Show id, name, email, title, account, and teams of the user whose token is configured.",
		Example: "  monday-pp-cli account me --json --select id,name,email,account.name",
		Annotations: map[string]string{
			"pp:endpoint":   "account.me",
			"pp:method":     "GET",
			"pp:path":       "/graphql",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryAccountMe, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "me")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
