package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"

	"github.com/spf13/cobra"
)

func newTopicsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Get, list, and search Product Hunt topics (categories)",
	}
	cmd.AddCommand(newTopicsGetCmd(flags))
	cmd.AddCommand(newTopicsListCmd(flags))
	cmd.AddCommand(newTopicsSearchCmd(flags))
	cmd.AddCommand(newTopicsWatchCmd(flags))
	return cmd
}

func newTopicsGetCmd(flags *rootFlags) *cobra.Command {
	var asID bool
	cmd := &cobra.Command{
		Use:   "get [id-or-slug]",
		Short: "Fetch a Product Hunt topic by slug (default) or numeric id (--id); returns id, name, slug, description, followersCount, postsCount, and image URL",
		Example: strings.Trim(`
  producthunt-pp-cli topics get artificial-intelligence --json
  producthunt-pp-cli topics get 268 --id --json
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
			var resp phgql.TopicResponse
			if _, err := c.Query(cmd.Context(), phgql.TopicQuery, vars, &resp); err != nil {
				return err
			}
			if resp.Topic.ID == "" {
				return fmt.Errorf("topic not found: %s", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Topic, flags)
		},
	}
	cmd.Flags().BoolVar(&asID, "id", false, "Treat the positional argument as a numeric id")
	return cmd
}

func newTopicsListCmd(flags *rootFlags) *cobra.Command {
	var (
		count int
		order string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Product Hunt topics ordered by FOLLOWERS_COUNT (default) or NEWEST",
		Example: strings.Trim(`
  producthunt-pp-cli topics list --count 10 --json
  producthunt-pp-cli topics list --order NEWEST --json
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
			if order != "" {
				vars["order"] = order
			}
			var resp phgql.TopicsResponse
			if _, err := c.Query(cmd.Context(), phgql.TopicsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Topics, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "Number of topics to return per page")
	cmd.Flags().StringVar(&order, "order", "FOLLOWERS_COUNT", "Order: FOLLOWERS_COUNT | NEWEST")
	return cmd
}

func newTopicsSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		count int
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Product Hunt topics by free-text query against name and description; returns paginated topic ids, slugs, follower counts, and post counts",
		Example: strings.Trim(`
  producthunt-pp-cli topics search "ai" --count 5 --json
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
			vars := map[string]any{"first": count, "query": args[0]}
			var resp phgql.TopicsResponse
			if _, err := c.Query(cmd.Context(), phgql.TopicsQuery, vars, &resp); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), resp.Topics, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 20, "Number of topics to return")
	return cmd
}

func newTopicsWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		minVotes int
		count    int
	)
	cmd := &cobra.Command{
		Use:   "watch <topic-slug>",
		Short: "Detect new posts crossing a vote threshold in a topic since the last sync",
		Long: strings.Trim(`
Compares the current top-N posts in a topic against the IDs we recorded the
last time `+"`topics watch`"+` ran, and emits only the new posts crossing the
--min-votes threshold. Synthesizes an offline subscription against an API that
has none.

State persists per-topic in the local store; schedule this in cron to alert on
notable launches without hammering GraphQL.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli topics watch artificial-intelligence --min-votes 200
  producthunt-pp-cli topics watch developer-tools --min-votes 100 --json
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
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("producthunt-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			vars := map[string]any{"first": count, "topic": args[0], "order": "VOTES"}
			var resp phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &resp); err != nil {
				return err
			}

			watchID := "topic_watch:" + args[0]
			seen := map[string]bool{}
			if raw, err := db.Get("topic_watch_state", watchID); err == nil && len(raw) > 0 {
				var prev []string
				if err := json.Unmarshal(raw, &prev); err == nil {
					for _, id := range prev {
						seen[id] = true
					}
				}
			}

			added := []watchedPost{}
			seenNow := []string{}
			for _, e := range resp.Posts.Edges {
				seenNow = append(seenNow, e.Node.ID)
				if e.Node.VotesCount < minVotes {
					continue
				}
				if !seen[e.Node.ID] {
					added = append(added, watchedPost{
						ID: e.Node.ID, Slug: e.Node.Slug, Name: e.Node.Name,
						Tagline: e.Node.Tagline, VotesCount: e.Node.VotesCount,
					})
				}
			}
			sort.Slice(added, func(i, j int) bool { return added[i].VotesCount > added[j].VotesCount })

			body, _ := json.Marshal(seenNow)
			_ = db.Upsert("topic_watch_state", watchID, body)

			out := topicsWatchOut{
				Topic:           args[0],
				MinVotes:        minVotes,
				Scanned:         len(resp.Posts.Edges),
				NewSinceLastRun: added,
				Note:            fmt.Sprintf("This is the diff vs the previous `topics watch %s` run; first run reports everything as new.", args[0]),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&minVotes, "min-votes", 100, "Vote threshold for emitting a post")
	cmd.Flags().IntVar(&count, "count", 20, "Number of top-by-votes posts to scan in the topic")
	return cmd
}

type watchedPost struct {
	ID         string `json:"id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Tagline    string `json:"tagline"`
	VotesCount int    `json:"votes_count"`
}

type topicsWatchOut struct {
	Topic           string        `json:"topic"`
	MinVotes        int           `json:"min_votes"`
	Scanned         int           `json:"scanned"`
	NewSinceLastRun []watchedPost `json:"new_since_last_run"`
	Note            string        `json:"note"`
}
