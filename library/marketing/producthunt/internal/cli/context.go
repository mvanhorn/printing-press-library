package cli

import (
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

func newContextCmd(flags *rootFlags) *cobra.Command {
	var (
		topic string
		since string
	)
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Single-call agent snapshot: top posts + topic state + viewer in one JSON blob",
		Long: strings.Trim(`
Folds three GraphQL calls into one structured JSON blob designed for an agent's
first read of the Product Hunt state. Returns:
  - topic: id, name, description, follower/post counts (when --topic supplied)
  - top_posts: top 10 posts in the window by votes
  - viewer: your authenticated user (or null when running under OAuth client_credentials)
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli context --topic artificial-intelligence --since 24h --json
  producthunt-pp-cli context --json
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

			out := contextOut{
				FetchedAt: time.Now().UTC().Format(time.RFC3339),
			}

			if topic != "" {
				var t phgql.TopicResponse
				if _, err := c.Query(cmd.Context(), phgql.TopicQuery, map[string]any{"slug": topic}, &t); err != nil {
					return err
				}
				out.Topic = &t.Topic
			}

			vars := map[string]any{"first": 10, "order": "VOTES"}
			if topic != "" {
				vars["topic"] = topic
			}
			if since != "" {
				postedAfter, err := parseSinceDurationISO(since)
				if err != nil {
					return err
				}
				vars["postedAfter"] = postedAfter
				out.PostedAfter = postedAfter
			}
			var p phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, vars, &p); err != nil {
				return err
			}
			out.TopPosts = make([]postBrief, 0, len(p.Posts.Edges))
			for _, e := range p.Posts.Edges {
				out.TopPosts = append(out.TopPosts, postBrief{
					Slug: e.Node.Slug, Name: e.Node.Name,
					VotesCount: e.Node.VotesCount, CommentsCount: e.Node.CommentsCount,
					Tagline: e.Node.Tagline, PosterHandle: e.Node.User.Username,
				})
			}

			var v phgql.ViewerResponse
			if _, err := c.Query(cmd.Context(), phgql.ViewerQuery, nil, &v); err == nil {
				out.Viewer = v.Viewer.User
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&topic, "topic", "", "Optional topic slug to scope the snapshot")
	cmd.Flags().StringVar(&since, "since", "24h", "Window for top posts (e.g. 6h, 24h, 7d)")
	return cmd
}

type contextOut struct {
	FetchedAt   string            `json:"fetched_at"`
	Topic       *phgql.Topic      `json:"topic,omitempty"`
	TopPosts    []postBrief       `json:"top_posts"`
	PostedAfter string            `json:"posted_after,omitempty"`
	Viewer      *phgql.ViewerUser `json:"viewer,omitempty"`
}
