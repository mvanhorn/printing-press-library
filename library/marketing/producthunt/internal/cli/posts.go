package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

// postsRoot is the parent for `producthunt-pp-cli posts ...`.
func newPostsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "posts",
		Short: "Get, list, and analyze Product Hunt launches via GraphQL",
		Long: strings.Trim(`
Posts are Product Hunt launches. Subcommands cover the full read-side surface
(get a single post by id or slug, list with filters, fetch comments) plus
local-store-backed transcendence commands (trajectory, launch-day cockpit,
benchmark percentiles, side-by-side compare, comment question-triage,
brand-mention grep, lookalike competitive set, time-window since query).
`, "\n"),
	}
	cmd.AddCommand(newPostsGetCmd(flags))
	cmd.AddCommand(newPostsListCmd(flags))
	cmd.AddCommand(newPostsCommentsCmd(flags))
	cmd.AddCommand(newPostsTrajectoryCmd(flags))
	cmd.AddCommand(newPostsLaunchDayCmd(flags))
	cmd.AddCommand(newPostsBenchmarkCmd(flags))
	cmd.AddCommand(newPostsCompareCmd(flags))
	cmd.AddCommand(newPostsQuestionsCmd(flags))
	cmd.AddCommand(newPostsGrepCmd(flags))
	cmd.AddCommand(newPostsLookalikeCmd(flags))
	cmd.AddCommand(newPostsSinceCmd(flags))
	return cmd
}

func newPostsGetCmd(flags *rootFlags) *cobra.Command {
	var asID bool
	cmd := &cobra.Command{
		Use:   "get [id-or-slug]",
		Short: "Fetch a single launch by slug (default) or numeric id (--id)",
		Long: strings.Trim(`
Returns full detail for a single launch: votes, comments count, topics, makers
(redacted by Product Hunt), media, poster.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts get notion --json
  producthunt-pp-cli posts get notion --json --select id,name,votesCount,topics.edges.node.slug
  producthunt-pp-cli posts get 1132754 --id --json
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
				vars["slug"] = args[0]
			}
			var resp phgql.PostResponse
			if _, err := c.Query(cmd.Context(), phgql.PostQuery, vars, &resp); err != nil {
				return err
			}
			if resp.Post.ID == "" {
				return fmt.Errorf("post not found: %s", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Post, flags)
		},
	}
	cmd.Flags().BoolVar(&asID, "id", false, "Treat the positional argument as a numeric Product Hunt id rather than a slug")
	return cmd
}

func newPostsListCmd(flags *rootFlags) *cobra.Command {
	var (
		topic        string
		order        string
		count        int
		featured     bool
		featuredSet  bool
		postedBefore string
		postedAfter  string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List launches filtered by topic, order, featured, or posted-after window",
		Long: strings.Trim(`
Returns the GraphQL `+"`posts`"+` connection with edges/node shape. Use
--posted-after / --posted-before to bound the window; --order accepts RANKING,
NEWEST, VOTES, FEATURED_AT.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts list --topic artificial-intelligence --count 10
  producthunt-pp-cli posts list --order=NEWEST --count 5 --json
  producthunt-pp-cli posts list --posted-after 2026-04-01 --order=VOTES --json --select edges.node.name,edges.node.votesCount
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
			vars := map[string]any{"first": count}
			if topic != "" {
				vars["topic"] = topic
			}
			if order != "" {
				vars["order"] = order
			}
			if featuredSet {
				vars["featured"] = featured
			}
			if postedAfter != "" {
				if iso, err := normalizeDate(postedAfter); err != nil {
					return fmt.Errorf("--posted-after: %w", err)
				} else {
					vars["postedAfter"] = iso
				}
			}
			if postedBefore != "" {
				if iso, err := normalizeDate(postedBefore); err != nil {
					return fmt.Errorf("--posted-before: %w", err)
				} else {
					vars["postedBefore"] = iso
				}
			}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Posts, flags)
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Filter by topic slug (e.g. artificial-intelligence)")
	cmd.Flags().StringVar(&order, "order", "RANKING", "Order: RANKING | NEWEST | VOTES | FEATURED_AT")
	cmd.Flags().IntVar(&count, "count", 20, "Number of posts to return (max 20 per page)")
	cmd.Flags().BoolVar(&featured, "featured", false, "Only featured posts")
	cmd.Flags().StringVar(&postedAfter, "posted-after", "", "Only posts created after this date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&postedBefore, "posted-before", "", "Only posts created before this date (YYYY-MM-DD or RFC3339)")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		featuredSet = cmd.Flags().Changed("featured")
	}
	return cmd
}

func newPostsCommentsCmd(flags *rootFlags) *cobra.Command {
	var (
		asID  bool
		count int
		after string
		order string
	)
	cmd := &cobra.Command{
		Use:   "comments [id-or-slug]",
		Short: "List comments on a launch (commenter identities are redacted by Product Hunt)",
		Long: strings.Trim(`
Returns the GraphQL post.comments connection. Each comment's user fields are
redacted by Product Hunt's policy (id "0", username/name "[REDACTED]"); the
body is intact. Use this to triage launch-day comments — for question filtering
specifically see `+"`posts questions`"+`.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts comments notion --count 20 --json
  producthunt-pp-cli posts comments 1132754 --id --order=VOTES --json
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
			vars := map[string]any{"first": count}
			if asID {
				vars["id"] = args[0]
			} else {
				vars["slug"] = args[0]
			}
			if order != "" {
				vars["order"] = order
			}
			if after != "" {
				vars["after"] = after
			}
			var resp phgql.PostCommentsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostCommentsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Post, flags)
		},
	}
	cmd.Flags().BoolVar(&asID, "id", false, "Treat the positional argument as a numeric id rather than a slug")
	cmd.Flags().IntVar(&count, "count", 20, "Number of comments to return per page (max 20)")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from a prior pageInfo.endCursor")
	cmd.Flags().StringVar(&order, "order", "VOTES_COUNT", "Order: VOTES_COUNT | NEWEST")
	return cmd
}

// normalizeDate accepts YYYY-MM-DD or RFC3339 and returns a Product Hunt
// DateTime-compatible RFC3339 string at midnight UTC.
func normalizeDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty date")
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("date %q must be YYYY-MM-DD or RFC3339", s)
}
