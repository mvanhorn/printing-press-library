package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

type competitorRow struct {
	ChannelID         string   `json:"channel_id"`
	ChannelTitle      string   `json:"channel_title"`
	Videos            int      `json:"videos_cached"`
	UploadsPer30d     float64  `json:"uploads_per_30d"`
	AvgViews          float64  `json:"avg_views"`
	MedianDurationSec int64    `json:"median_duration_sec"`
	TopTags           []string `json:"top_tags"`
}

func newCompetitorDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "competitor-diff",
		Short:       "Compare uploads/cadence/avg views across cached channels (your channel + competitors)",
		Example:     "  youtube-creator-analytics-pp-cli competitor-diff --json",
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
			vids, err := loadVideos(db, "", 0)
			if err != nil {
				return err
			}
			if len(vids) == 0 {
				return writeNoop(flags, "no_videos", "no cached videos; sync your channel and competitors first")
			}
			byCh := map[string]*competitorRow{}
			tagCount := map[string]map[string]int{}
			now := time.Now()
			for _, v := range vids {
				row, ok := byCh[v.ChannelID]
				if !ok {
					row = &competitorRow{ChannelID: v.ChannelID, ChannelTitle: v.ChannelTitle}
					byCh[v.ChannelID] = row
					tagCount[v.ChannelID] = map[string]int{}
				}
				row.Videos++
				row.AvgViews += float64(v.ViewCount)
				row.MedianDurationSec += v.DurationSec
				if !v.PublishedAt.IsZero() && now.Sub(v.PublishedAt).Hours()/24 <= 30 {
					row.UploadsPer30d++
				}
				for _, t := range v.Tags {
					tagCount[v.ChannelID][strings.ToLower(t)]++
				}
			}
			out := make([]competitorRow, 0, len(byCh))
			for id, r := range byCh {
				if r.Videos > 0 {
					r.AvgViews /= float64(r.Videos)
					r.MedianDurationSec /= int64(r.Videos)
				}
				r.TopTags = topNTags(tagCount[id], 5)
				out = append(out, *r)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].AvgViews > out[j].AvgViews })
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func topNTags(m map[string]int, n int) []string {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.k)
	}
	return out
}
