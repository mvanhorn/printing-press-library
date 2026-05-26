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

func newReportCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Summarize browsing activity over --since: per-day and per-hour counts, top domains, and productive/neutral/distracting split",
		Example:     strings.Trim("safari-history-pp-cli report --since 7d", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, _, err := sourceTimeWindow(since, "", 7*24*time.Hour)
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
			pd, err := opts.Source.ProfileAggregates(st.DB(), source.VisitFilter{Since: start, Limit: opts.Output.Limit, Device: opts.Device})
			if err != nil {
				return err
			}
			daily, hours, top := pd.Daily, pd.Hourly, pd.TopDomains
			prod, err := domainProductivitySplit(opts.Source, st.DB(), start, opts.Device)
			if err != nil {
				return err
			}
			out := map[string]any{"since": sinceOrDefault(since, "7d"), "time_zone": time.Local.String(), "per_day": daily, "hour_of_day": hours, "top_domains": top, "productivity_split": prod}
			maybePrintEmptyWindowHint(st.DB(), since, len(daily) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}

func sinceOrDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
