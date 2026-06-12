package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

func newVisitedCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "visited <url|domain>",
		Short:       "Check whether a URL or domain was visited, returning first/last seen, total and typed visit counts, and referrer examples",
		Args:        usageExactArgs(1),
		Example:     strings.Trim("safari-history-pp-cli visited github.com\n  safari-history-pp-cli visited https://docs.github.com", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openSnapshotStore()
			if err != nil {
				return err
			}
			defer st.Close()
			target := source.NormalizeTargetDomain(args[0])
			sum, err := opts.Source.VisitedSummary(st.DB(), target)
			if err != nil {
				return err
			}
			trans := map[string]any{}
			for k, v := range sum.TransitionBreakdown {
				trans[k] = v
			}
			out := []map[string]any{{"target": target, "found": sum.Found, "first_seen": sum.FirstSeen.Format(time.RFC3339), "last_seen": sum.LastSeen.Format(time.RFC3339), "total_visits": sum.TotalVisits, "typed_count": sum.TypedCount, "transition_breakdown": trans, "referrer_examples": sum.Referrers}}
			if !sum.Found {
				out[0]["message"] = "never visited"
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	return cmd
}
