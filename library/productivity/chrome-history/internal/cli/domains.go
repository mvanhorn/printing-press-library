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

func newDomainsCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "domains",
		Short:       "Rank the most-visited registrable domains over --since, with page counts, visit sums, and productivity category",
		Example:     strings.Trim("chrome-history-pp-cli domains --since 30d --limit 20", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, _, err := sourceTimeWindow(since, "", 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			st, _, err := openCoreHistoryStore(opts.Device)
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := opts.Source.DomainStats(st.DB(), source.VisitFilter{Since: start, Limit: opts.Output.Limit, Device: opts.Device})
			if err != nil {
				return err
			}
			type aggRow struct {
				pc, vs int64
				lv     time.Time
			}
			agg := map[string]aggRow{}
			for _, r := range rows {
				d := source.DomainFromURL(r.Domain)
				x := agg[d]
				x.pc += r.PageCount
				x.vs += r.VisitSum
				if r.LastVisit.After(x.lv) {
					x.lv = r.LastVisit
				}
				agg[d] = x
			}
			out := []map[string]any{}
			keys := make([]string, 0, len(agg))
			for k := range agg {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool { return agg[keys[i]].vs > agg[keys[j]].vs })
			if len(keys) > opts.Output.Limit {
				keys = keys[:opts.Output.Limit]
			}
			for _, d := range keys {
				b, _ := categorize.Classify(d)
				out = append(out, map[string]any{"domain": d, "page_count": agg[d].pc, "visit_sum": agg[d].vs, "last_visit": agg[d].lv.Format(time.RFC3339), "category": b})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}
