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

func newTimelineCmd(opts *RootOptions) *cobra.Command {
	var gap, since, until string
	cmd := &cobra.Command{
		Use:         "timeline <date|range>",
		Short:       "Reconstruct ordered browsing sessions for a date or --since/--until window, splitting on a --gap idle threshold",
		Args:        cobra.MaximumNArgs(1),
		Example:     strings.Trim("chrome-history-pp-cli timeline 2026-05-01\n  chrome-history-pp-cli timeline --since 7d --gap 45m", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && strings.TrimSpace(since) == "" {
				since = args[0]
			}
			start, end, err := timelineWindow(since, until)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			gd, err := time.ParseDuration(gap)
			if err != nil {
				return errors.Join(ErrUsage, fmt.Errorf("invalid --gap: %w", err))
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
			events, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: end, Limit: 5000, Device: opts.Device})
			if err != nil {
				return err
			}
			sessions := splitSessions(events, gd)
			out := []map[string]any{}
			for i, s := range sessions {
				items := []map[string]any{}
				for _, e := range s.Items {
					items = append(items, map[string]any{"visit_id": e.VisitID, "when": e.VisitTime.Format(time.RFC3339), "from_visit": e.FromVisit, "transition": e.Transition, "title": e.Title, "url": e.URL})
				}
				out = append(out, map[string]any{"session": i + 1, "start": s.Start.Format(time.RFC3339), "end": s.End.Format(time.RFC3339), "count": len(items), "visits": items})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	cmd.Flags().StringVar(&until, "until", "", "end time")
	cmd.Flags().StringVar(&gap, "gap", "30m", "session gap")
	return cmd
}
