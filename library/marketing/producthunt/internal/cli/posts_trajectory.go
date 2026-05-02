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

// snapshot is the persisted shape under resource_type "post_snapshot".
type snapshot struct {
	PostID        string    `json:"post_id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	VotesCount    int       `json:"votes_count"`
	CommentsCount int       `json:"comments_count"`
	Timestamp     time.Time `json:"ts"`
}

func snapshotID(postID string, ts time.Time) string {
	return fmt.Sprintf("%s:%d", postID, ts.UTC().Unix())
}

func newPostsTrajectoryCmd(flags *rootFlags) *cobra.Command {
	var live bool
	cmd := &cobra.Command{
		Use:   "trajectory <slug>",
		Short: "Plot a launch's votes-over-time from local snapshots (live + persisted)",
		Long: strings.Trim(`
Reads stored snapshots for the launch and renders a sorted-by-time series of
(votes_count, comments_count). Each invocation also captures a fresh live
snapshot (unless --no-live is set) and persists it to the local store, so
repeated invocations build the trajectory over time.

To kick off a real trajectory, schedule this command (or `+"`producthunt-pp-cli sync`"+`)
to run hourly across launch day. Without periodic syncs, the trajectory will
be a single point.
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli posts trajectory notion --json
  producthunt-pp-cli posts trajectory my-launch-slug --no-live --json
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
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("producthunt-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			postID := ""
			if live {
				c := phgql.New(cfg)
				var resp phgql.PostResponse
				if _, err := c.Query(cmd.Context(), phgql.PostQuery, map[string]any{"slug": args[0]}, &resp); err != nil {
					return err
				}
				if resp.Post.ID == "" {
					return fmt.Errorf("post not found: %s", args[0])
				}
				postID = resp.Post.ID
				snap := snapshot{
					PostID: resp.Post.ID, Slug: resp.Post.Slug, Name: resp.Post.Name,
					VotesCount: resp.Post.VotesCount, CommentsCount: resp.Post.CommentsCount,
					Timestamp: time.Now().UTC(),
				}
				body, _ := json.Marshal(snap)
				_ = db.Upsert("post_snapshot", snapshotID(snap.PostID, snap.Timestamp), body)
			}

			// Pull all snapshots and filter for the slug
			items, err := db.List("post_snapshot", 5000)
			if err != nil {
				return fmt.Errorf("listing snapshots: %w", err)
			}
			snaps := make([]snapshot, 0, len(items))
			for _, raw := range items {
				var s snapshot
				if err := json.Unmarshal(raw, &s); err != nil {
					continue
				}
				if s.Slug == args[0] || (postID != "" && s.PostID == postID) {
					snaps = append(snaps, s)
				}
			}
			sort.Slice(snaps, func(i, j int) bool { return snaps[i].Timestamp.Before(snaps[j].Timestamp) })
			out := trajectoryOut{
				Slug:      args[0],
				Snapshots: snaps,
				Note:      "Schedule this command hourly (or use `producthunt-pp-cli sync`) on launch day to build a real trajectory; without periodic snapshots this returns a single point.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&live, "live", true, "Capture a fresh live snapshot before rendering (default true; set --live=false for store-only)")
	return cmd
}

type trajectoryOut struct {
	Slug      string     `json:"slug"`
	Snapshots []snapshot `json:"snapshots"`
	Note      string     `json:"note"`
}
