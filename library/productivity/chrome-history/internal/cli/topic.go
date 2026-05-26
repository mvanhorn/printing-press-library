package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
	"github.com/spf13/cobra"
)

func newTopicCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "topic <name>",
		Short:       "Gather everything you browsed about a topic over --since by merging matching Journeys clusters with full-text page matches",
		Args:        usageMinArgs(1),
		Example:     strings.Trim("chrome-history-pp-cli topic fountain pens --since 30d --limit 50", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, _, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
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
			name := strings.Join(args, " ")
			merged := []map[string]any{}
			if clusters, _, err := opts.Source.Clusters(st.DB(), source.ClusterFilter{Since: start, Limit: opts.Output.Limit * 20}); err == nil {
				for _, c := range clusters {
					if !strings.Contains(strings.ToLower(c.Label), strings.ToLower(name)) {
						continue
					}
					for _, p := range c.TopPages {
						merged = append(merged, map[string]any{"cluster": c.Label, "url": p.URL, "title": "", "when": c.LastVisit, "visits": p.Count})
					}
				}
			}
			fromFTS, err := opts.Source.FullTextSearch(st.DB(), name, source.VisitFilter{Since: start, Limit: opts.Output.Limit, Device: opts.Device})
			if err != nil {
				return err
			}
			for _, r := range fromFTS {
				merged = append(merged, map[string]any{"cluster": "", "url": r.URL, "title": r.Title, "when": r.LastVisit, "visits": r.VisitCount})
			}
			seen := map[string]bool{}
			out := []map[string]any{}
			for _, r := range merged {
				u, _ := r["url"].(string)
				w, _ := r["when"].(time.Time)
				k := fmt.Sprintf("%s|%s", u, w.Format(time.RFC3339Nano))
				if seen[k] {
					continue
				}
				seen[k] = true
				r["when"] = w.Format(time.RFC3339)
				out = append(out, r)
				if len(out) >= opts.Output.Limit {
					break
				}
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}
