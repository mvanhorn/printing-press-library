package cli

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/categorize"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	"github.com/spf13/cobra"
)

func newDwellCmd(opts *RootOptions) *cobra.Command {
	var since, gap string
	cmd := &cobra.Command{
		Use:         "dwell",
		Short:       "Estimate time-on-site per domain over --since, capping per-visit dwell at --gap (defaults to 30m sessions)",
		Example:     strings.Trim("chrome-history-pp-cli dwell --since 14d --gap 30m", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			gd, err := time.ParseDuration(gap)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			st, err := openSnapshotStore()
			if err != nil {
				return err
			}
			defer st.Close()
			events, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: end, Limit: 20000, Device: opts.Device})
			if err != nil {
				return err
			}
			sessions := splitSessions(events, gd)
			tot := map[string]int64{}
			for _, s := range sessions {
				for i, e := range s.Items {
					dwell := e.VisitDuration
					if dwell <= 0 {
						if i < len(s.Items)-1 {
							dwell = s.Items[i+1].VisitTime.Sub(e.VisitTime)
						} else {
							dwell = 0
						}
					}
					if dwell > gd {
						dwell = gd
					}
					tot[source.DomainFromURL(e.URL)] += dwell.Microseconds()
				}
			}
			type rec struct {
				d  string
				us int64
			}
			arr := []rec{}
			for d, us := range tot {
				arr = append(arr, rec{d, us})
			}
			sort.Slice(arr, func(i, j int) bool { return arr[i].us > arr[j].us })
			if len(arr) > opts.Output.Limit {
				arr = arr[:opts.Output.Limit]
			}
			out := []map[string]any{}
			for _, a := range arr {
				b, p := categorize.Classify(a.d)
				out = append(out, map[string]any{"domain": a.d, "category": b, "productivity": p, "estimated_dwell_seconds": float64(a.us) / 1_000_000.0})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			result := map[string]any{"note": "estimate based on visit duration or next-visit delta", "rows": out}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, result)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	cmd.Flags().StringVar(&gap, "gap", "30m", "dwell cap/session gap")
	return cmd
}
