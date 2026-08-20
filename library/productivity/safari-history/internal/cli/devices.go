package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
)

func newDevicesCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "devices",
		Short:       "List visit-origin buckets with visit counts, first/last seen, and top domains (Safari reports a single local origin, no per-device sync split)",
		Example:     strings.Trim("safari-history-pp-cli devices --json", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openSnapshotStore()
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := opts.Source.Devices(st.DB())
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				out = append(out, map[string]any{
					"id":          r.ID,
					"kind":        r.Kind,
					"visits":      r.Visits,
					"first_seen":  r.FirstSeen,
					"last_seen":   r.LastSeen,
					"top_domains": r.TopDomains,
				})
			}
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	return cmd
}
