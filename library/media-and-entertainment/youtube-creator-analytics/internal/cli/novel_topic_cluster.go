package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

type topicCluster struct {
	Topic    string   `json:"topic"`
	Videos   int      `json:"videos"`
	AvgViews float64  `json:"avg_views"`
	Samples  []string `json:"sample_titles"`
}

// newTopicClusterCmd groups videos by dominant tag/keyword. Lightweight: not
// real embeddings — uses tag intersection + top keyword from title tokens.
func newTopicClusterCmd(flags *rootFlags) *cobra.Command {
	var dbPath, channel string
	var limit int
	cmd := &cobra.Command{
		Use:         "topic-cluster",
		Short:       "Group your videos into topic clusters by tag overlap and ranks them by avg views",
		Example:     "  youtube-creator-analytics-pp-cli topic-cluster --limit 12 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("youtube-creator-analytics-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			defer db.Close()
			vids, err := loadVideos(db, channel, 0)
			if err != nil {
				return err
			}
			if len(vids) == 0 {
				return writeNoop(flags, "no_videos", "no cached videos; run sync first")
			}
			byTopic := map[string]*topicCluster{}
			for _, v := range vids {
				key := pickTopicKey(v)
				c, ok := byTopic[key]
				if !ok {
					c = &topicCluster{Topic: key}
					byTopic[key] = c
				}
				c.Videos++
				c.AvgViews += float64(v.ViewCount)
				if len(c.Samples) < 3 {
					c.Samples = append(c.Samples, v.Title)
				}
			}
			out := make([]topicCluster, 0, len(byTopic))
			for _, c := range byTopic {
				if c.Videos > 0 {
					c.AvgViews /= float64(c.Videos)
				}
				out = append(out, *c)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].AvgViews > out[j].AvgViews })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&channel, "channel", "", "Channel ID")
	cmd.Flags().IntVar(&limit, "limit", 15, "Max clusters")
	return cmd
}

func pickTopicKey(v videoRow) string {
	if len(v.Tags) > 0 {
		return strings.ToLower(v.Tags[0])
	}
	for _, tok := range tokenize(v.Title) {
		if len(tok) > 3 {
			return tok
		}
	}
	return "uncategorized"
}
