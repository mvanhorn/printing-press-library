package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/store"
	"github.com/spf13/cobra"
)

func newListCmd(opts *RootOptions) *cobra.Command {
	var since, until, domain, transition string
	var minVisits int64
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recent visits from the synced snapshot, filtered by --since/--until, --domain, --transition, and --min-visits",
		Example:     strings.Trim("chrome-history-pp-cli list --since 7d --limit 20\n  chrome-history-pp-cli list --domain github.com --transition typed", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			start, end, err := sourceTimeWindow(since, until, 30*24*time.Hour)
			if err != nil {
				return errors.Join(ErrUsage, err)
			}
			var st *store.Store
			if strings.TrimSpace(transition) == "" {
				var err error
				st, _, err = openCoreHistoryStore(opts.Device)
				if err != nil {
					return err
				}
			} else {
				var err error
				st, err = openSnapshotStore()
				if err != nil {
					return err
				}
			}
			defer st.Close()
			rows, err := opts.Source.RecentVisits(st.DB(), source.VisitFilter{Since: start, Until: end, Limit: opts.Output.Limit, MinVisits: minVisits, Domain: domain, Device: opts.Device})
			if err != nil {
				return err
			}
			if !opts.Source.Capabilities().Transitions && transition != "" {
				msg := []map[string]any{{"message": "transition filtering not available for " + opts.Source.Name()}}
				output.DefaultToJSONIfNotTTY(&opts.Output)
				return output.Render(opts.Output, msg)
			}
			out := []map[string]any{}
			for _, r := range rows {
				d := source.DomainFromURL(r.URL)
				core := r.Transition
				if domain != "" && source.NormalizeTargetDomain(domain) != d {
					continue
				}
				if transition != "" && !strings.EqualFold(transition, core) {
					continue
				}
				out = append(out, map[string]any{"last_visit": r.VisitTime.Format(time.RFC3339), "visits": r.VisitCount, "typed": r.TypedCount > 0, "transition": core, "title": r.Title, "url": r.URL, "origin": r.Origin})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time (e.g. 7d or 2026-05-01)")
	cmd.Flags().StringVar(&until, "until", "", "end time")
	cmd.Flags().StringVar(&domain, "domain", "", "registrable domain")
	cmd.Flags().StringVar(&transition, "transition", "", "transition core type")
	cmd.Flags().Int64Var(&minVisits, "min-visits", 1, "minimum visit_count")
	return cmd
}
