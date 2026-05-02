package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newUsersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Get a Product Hunt user profile and their launch/voted-post history",
		Long: strings.Trim(`
WARNING: Product Hunt redacts non-self user lookups. `+"`users get <username>`"+`
returns id "0" with username/name "[REDACTED]" for anyone other than yourself.
Use `+"`whoami`"+` to see your own (unredacted) profile. The `+"`users posts`"+`
and `+"`users voted-posts`"+` subcommands return real post data even though the
user identity itself is redacted.
`, "\n"),
	}
	cmd.AddCommand(newUsersGetCmd(flags))
	cmd.AddCommand(newUsersPostsCmd(flags))
	cmd.AddCommand(newUsersVotedPostsCmd(flags))
	return cmd
}

func newUsersGetCmd(flags *rootFlags) *cobra.Command {
	var asID bool
	cmd := &cobra.Command{
		Use:   "get [id-or-username]",
		Short: "Fetch a user profile (REDACTED for non-self lookups; use `whoami` for your own)",
		Example: strings.Trim(`
  producthunt-pp-cli users get benln --json
  producthunt-pp-cli users get 1880 --id --json
  producthunt-pp-cli whoami --json    # use this instead for your own data
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{}
			if asID {
				vars["id"] = args[0]
			} else {
				vars["username"] = args[0]
			}
			var resp phgql.UserResponse
			if _, err := c.Query(cmd.Context(), phgql.UserQuery, vars, &resp); err != nil {
				return err
			}
			if resp.User.ID == "" {
				return fmt.Errorf("user not found: %s", args[0])
			}
			out := userGetOut{
				User:     resp.User,
				Redacted: resp.User.Redacted(),
			}
			if out.Redacted {
				out.Note = "Product Hunt redacts non-self user lookups (id `0`, name/username `[REDACTED]`). Use `whoami` to view your own profile."
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&asID, "id", false, "Treat the positional argument as a numeric id")
	return cmd
}

type userGetOut struct {
	User     phgql.User `json:"user"`
	Redacted bool       `json:"redacted_by_product_hunt"`
	Note     string     `json:"note,omitempty"`
}

func newUsersPostsCmd(flags *rootFlags) *cobra.Command {
	var (
		count int
		after string
	)
	cmd := &cobra.Command{
		Use:   "posts <username>",
		Short: "List posts a user has made (post data is intact; user identity may be redacted)",
		Example: strings.Trim(`
  producthunt-pp-cli users posts benln --count 10 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"username": args[0], "first": count}
			if after != "" {
				vars["after"] = after
			}
			var resp phgql.UserPostsResponse
			if _, err := c.Query(cmd.Context(), phgql.UserPostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.User, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "Number of posts to return")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from a prior pageInfo.endCursor")
	return cmd
}

func newUsersVotedPostsCmd(flags *rootFlags) *cobra.Command {
	var (
		count int
		after string
	)
	cmd := &cobra.Command{
		Use:   "voted-posts <username>",
		Short: "List posts a user has voted for (post data intact)",
		Example: strings.Trim(`
  producthunt-pp-cli users voted-posts benln --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"username": args[0], "first": count}
			if after != "" {
				vars["after"] = after
			}
			var resp phgql.UserVotedPostsResponse
			if _, err := c.Query(cmd.Context(), phgql.UserVotedPostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.User, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "Number of posts to return")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from a prior pageInfo.endCursor")
	return cmd
}
