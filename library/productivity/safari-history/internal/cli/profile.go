package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func newProfileCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "profile",
		Short:       "Summarize your browsing self over --since: peak hours, busiest weekday, top domains, and productivity split",
		Example:     strings.Trim("safari-history-pp-cli profile --since 30d --json", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, _, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			st, err := openSnapshotStore()
			if err != nil {
				return err
			}
			defer st.Close()
			pd, err := opts.Source.ProfileAggregates(st.DB(), source.VisitFilter{Since: start, Limit: 5, Device: opts.Device})
			if err != nil {
				return err
			}
			hours, weekdays, domains, terms := pd.Hourly, pd.Weekday, pd.TopDomains, pd.TopSearchTerms
			pages, visits := pd.Pages, pd.Visits
			prod, err := domainProductivitySplit(opts.Source, st.DB(), start, opts.Device)
			if err != nil {
				return err
			}
			out := map[string]any{"since": sinceOrDefault(since, "30d"), "time_zone": time.Local.String(), "peak_hours": hours, "busiest_weekday": weekdays, "top_domains": domains, "top_search_terms": terms, "productivity_split": prod, "totals": map[string]any{"pages": pages, "visits": visits}}
			maybePrintEmptyWindowHint(st.DB(), since, visits == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}
