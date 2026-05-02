package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newTodayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "today",
		Short: "Today's top launches (alias for `posts list --order=RANKING --posted-after=midnight`)",
		Long: strings.Trim(`
Returns today's top launches by Product Hunt's RANKING score, posted after
midnight UTC. Convenience wrapper for the morning skim — for fuller filtering
see `+"`posts list`"+`.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli today
  producthunt-pp-cli today --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"first": 20, "order": "RANKING", "postedAfter": midnightUTC()}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Posts, flags)
		},
	}
	return cmd
}

func newRecentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Most recent launches (alias for `posts list --order=NEWEST`)",
		Example: strings.Trim(`
  producthunt-pp-cli recent
  producthunt-pp-cli recent --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"first": 20, "order": "NEWEST"}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Posts, flags)
		},
	}
	return cmd
}
