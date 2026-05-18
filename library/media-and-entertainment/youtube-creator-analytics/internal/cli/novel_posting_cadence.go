package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/youtube-creator-analytics/internal/store"
)

type cadenceCell struct {
	Bucket   string  `json:"bucket"`
	Videos   int     `json:"videos"`
	AvgViews float64 `json:"avg_views"`
}

func newPostingCadenceCmd(flags *rootFlags) *cobra.Command {
	var channel, dbPath, mode string
	cmd := &cobra.Command{
		Use:         "posting-cadence",
		Short:       "Correlate publish day-of-week or hour-of-day with average views",
		Example:     "  youtube-creator-analytics-pp-cli posting-cadence --mode dow --json",
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
			ch, err := resolveChannel(db, channel)
			if err != nil {
				return err
			}
			videos, err := loadVideos(db, ch, 0)
			if err != nil {
				return err
			}
			if len(videos) == 0 {
				return writeNoop(flags, "no_videos", "no cached videos for this channel; run `youtube-creator-analytics-pp-cli sync` first")
			}
			buckets := map[string]*cadenceCell{}
			labels := []string{}
			if mode == "hour" {
				for h := 0; h < 24; h++ {
					k := fmt.Sprintf("%02d:00", h)
					buckets[k] = &cadenceCell{Bucket: k}
					labels = append(labels, k)
				}
			} else {
				for _, d := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
					buckets[d] = &cadenceCell{Bucket: d}
					labels = append(labels, d)
				}
			}
			for _, v := range videos {
				if v.PublishedAt.IsZero() {
					continue
				}
				var k string
				if mode == "hour" {
					k = fmt.Sprintf("%02d:00", v.PublishedAt.Hour())
				} else {
					k = v.PublishedAt.Weekday().String()[:3]
				}
				b, ok := buckets[k]
				if !ok {
					continue
				}
				b.Videos++
				b.AvgViews += float64(v.ViewCount)
			}
			out := make([]cadenceCell, 0, len(labels))
			for _, k := range labels {
				b := buckets[k]
				if b.Videos > 0 {
					b.AvgViews /= float64(b.Videos)
				}
				out = append(out, *b)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Channel ID (defaults to most recently synced)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&mode, "mode", "dow", "Bucket: dow (day-of-week) | hour")
	return cmd
}
