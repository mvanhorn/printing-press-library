package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func newJourneysCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "journeys",
		Short:       "List topic clusters over --since with labels and top pages (not available for Safari, which has no journeys tables; reports unavailable)",
		Example:     strings.Trim("safari-history-pp-cli journeys --limit 20\n  safari-history-pp-cli journeys --since 90d --limit 20", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.Source.Capabilities().Journeys {
				return renderNotAvailable(opts, "journeys", "Safari History.db has no clusters/journeys tables")
			}
			effectiveLimit := opts.Output.Limit
			if !cmd.Flags().Changed("limit") {
				effectiveLimit = 500
			}
			var start time.Time
			if strings.TrimSpace(since) != "" {
				var err error
				start, _, err = sourceTimeWindow(since, "", 30*24*time.Hour)
				if err != nil {
					return errors.Join(ErrUsage, err)
				}
			}
			snapshot, err := snapshotPath()
			if err != nil {
				return err
			}
			st, err := store.OpenExisting(snapshot)
			if err != nil {
				if errors.Is(err, store.ErrNoSnapshot) {
					return ErrNoSnapshot
				}
				return err
			}
			defer st.Close()
			clusters, note, err := opts.Source.Clusters(st.DB(), source.ClusterFilter{Since: start, Limit: effectiveLimit})
			if err != nil {
				return err
			}
			if note != "" {
				out := map[string]any{"note": note, "journeys": []map[string]any{}}
				output.DefaultToJSONIfNotTTY(&opts.Output)
				return output.Render(opts.Output, out)
			}
			out := make([]map[string]any, 0, len(clusters))
			for _, c := range clusters {
				top := make([]map[string]any, 0, len(c.TopPages))
				for _, p := range c.TopPages {
					top = append(top, map[string]any{"url": p.URL, "count": p.Count})
				}
				out = append(out, map[string]any{"cluster_id": c.ClusterID, "label": c.Label, "page_count": c.PageCount, "last_visit": c.LastVisit.Format(time.RFC3339), "top_pages": top})
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}
