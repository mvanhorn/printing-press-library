// Users and teams commands for monday.com.

package cli

import (
	"github.com/spf13/cobra"
)

const queryUsersList = `query($limit: Int!, $page: Int!, $kind: UserKind) {
  users(limit: $limit, page: $page, kind: $kind) {
    id
    name
    email
    title
    enabled
    is_admin
    is_guest
    is_view_only
    teams { id name }
    created_at
  }
}`

const queryUserGet = `query($ids: [ID!]) {
  users(ids: $ids) {
    id
    name
    email
    title
    enabled
    is_admin
    is_guest
    is_view_only
    teams { id name }
    created_at
  }
}`

const queryTeamsList = `query {
  teams { id name picture_url users { id name email } }
}`

func newUsersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "List and get users in the monday.com account.",
	}
	cmd.AddCommand(newUsersListCmd(flags))
	cmd.AddCommand(newUsersGetCmd(flags))
	return cmd
}

func newUsersListCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var kind string
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List users in the account.",
		Example: "  monday-pp-cli users list --kind non_guests --json --select id,name,email",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"limit": limit, "page": page}
			if kind != "" {
				vars["kind"] = kind
			}
			data, err := c.GraphQL(queryUsersList, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "users")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Page size.")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-based).")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter: all, non_guests, guests, non_pending.")
	return cmd
}

func newUsersGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Fetch one user by ID.",
		Example: "  monday-pp-cli users get 12345 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("user id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryUserGet, map[string]any{"ids": []string{args[0]}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckFirstFromArrayField(data, "users")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}

func newTeamsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "teams",
		Short: "List teams in the monday.com account.",
	}
	cmd.AddCommand(newTeamsListCmd(flags))
	return cmd
}

func newTeamsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List teams in the account.",
		Example: "  monday-pp-cli teams list --json --select id,name,users",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryTeamsList, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "teams")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
