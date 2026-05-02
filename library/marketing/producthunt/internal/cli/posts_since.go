package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newPostsSinceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "since <duration>",
		Short: "List posts published in the last <duration> (e.g. 2h, 24h, 7d)",
		Long: strings.Trim(`
Time-window query: a thin wrapper around `+"`posts list --posted-after`"+` that
accepts shorthand durations (e.g. 2h, 24h, 7d). Convenient for agentic flows
asking "what's new on Product Hunt".

Currently runs against the live API on every call; a future revision will
fall through to the local store first when sync data is fresh enough to
cover the window.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts since 2h
  producthunt-pp-cli posts since 24h --json --select edges.node.name,edges.node.votesCount
  producthunt-pp-cli posts since 7d --topic developer-tools --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			postedAfter, err := parseSinceDurationISO(args[0])
			if err != nil {
				return err
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c := phgql.New(cfg)
			vars := map[string]any{"first": 20, "order": "NEWEST", "postedAfter": postedAfter}
			topic, _ := cmd.Flags().GetString("topic")
			if topic != "" {
				vars["topic"] = topic
			}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Posts, flags)
		},
	}
	cmd.Flags().String("topic", "", "Optional topic-slug filter")
	return cmd
}
