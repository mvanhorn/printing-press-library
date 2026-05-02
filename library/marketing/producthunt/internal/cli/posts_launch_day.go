package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"

	"github.com/spf13/cobra"
)

func newPostsLaunchDayCmd(flags *rootFlags) *cobra.Command {
	var (
		topN int
	)
	cmd := &cobra.Command{
		Use:   "launch-day <my-slug>",
		Short: "Render YOUR launch's trajectory side-by-side with today's top 5 launches",
		Long: strings.Trim(`
Captures live snapshots of YOUR launch and today's top 5 launches (by RANKING),
persists them, then prints all six trajectories sorted by snapshot time.

Schedule this hourly (or run `+"`producthunt-pp-cli sync --resource posts`"+` on a cron)
on launch day to build a real comparative trajectory chart.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts launch-day my-launch-slug --json
  producthunt-pp-cli posts launch-day my-launch-slug --top 3
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

			// Fetch your launch
			var mine phgql.PostResponse
			if _, err := c.Query(cmd.Context(), phgql.PostQuery, map[string]any{"slug": args[0]}, &mine); err != nil {
				return fmt.Errorf("fetching your launch: %w", err)
			}
			if mine.Post.ID == "" {
				return fmt.Errorf("post not found: %s", args[0])
			}

			// Fetch today's top N
			topVars := map[string]any{"first": topN, "order": "RANKING", "postedAfter": midnightUTC()}
			var top phgql.PostsResponse
			if _, err := c.Query(cmd.Context(), phgql.PostsQuery, topVars, &top); err != nil {
				return fmt.Errorf("fetching top launches: %w", err)
			}

			// Persist snapshots for all
			now := time.Now().UTC()
			persist := func(p phgql.Post) {
				snap := snapshot{
					PostID: p.ID, Slug: p.Slug, Name: p.Name,
					VotesCount: p.VotesCount, CommentsCount: p.CommentsCount,
					Timestamp: now,
				}
				body, _ := json.Marshal(snap)
				_ = db.Upsert("post_snapshot", snapshotID(p.ID, now), body)
			}
			persist(mine.Post)
			for _, e := range top.Posts.Edges {
				persist(e.Node)
			}

			// Read all snapshots, group by slug
			items, err := db.List("post_snapshot", 10000)
			if err != nil {
				return err
			}
			grouped := map[string][]snapshot{}
			interesting := map[string]bool{mine.Post.Slug: true}
			for _, e := range top.Posts.Edges {
				interesting[e.Node.Slug] = true
			}
			for _, raw := range items {
				var s snapshot
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}
				if interesting[s.Slug] {
					grouped[s.Slug] = append(grouped[s.Slug], s)
				}
			}
			for slug := range grouped {
				sort.Slice(grouped[slug], func(i, j int) bool { return grouped[slug][i].Timestamp.Before(grouped[slug][j].Timestamp) })
			}

			out := launchDayOut{
				MySlug:       mine.Post.Slug,
				MyName:       mine.Post.Name,
				Top:          extractTop(top.Posts),
				Trajectories: grouped,
				Note:         "Schedule this command hourly on launch day to build trajectories with multiple data points; the first run captures only one snapshot per launch.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&topN, "top", 5, "How many of today's top launches to compare against")
	return cmd
}

type topLaunchSummary struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	VotesCount    int    `json:"votes_count"`
	CommentsCount int    `json:"comments_count"`
}

type launchDayOut struct {
	MySlug       string                `json:"my_slug"`
	MyName       string                `json:"my_name"`
	Top          []topLaunchSummary    `json:"top_launches_now"`
	Trajectories map[string][]snapshot `json:"trajectories"`
	Note         string                `json:"note"`
}

func extractTop(c phgql.PostConnection) []topLaunchSummary {
	out := make([]topLaunchSummary, 0, len(c.Edges))
	for _, e := range c.Edges {
		out = append(out, topLaunchSummary{
			Slug: e.Node.Slug, Name: e.Node.Name,
			VotesCount: e.Node.VotesCount, CommentsCount: e.Node.CommentsCount,
		})
	}
	return out
}
