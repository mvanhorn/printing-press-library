package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/output"
	"github.com/spf13/cobra"
)

func newDevicesCmd(opts *RootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "devices",
		Short:       "List the synced-device origins behind your history, each with visit counts, first/last seen, and top domains",
		Example:     strings.Trim("chrome-history-pp-cli devices --json", "\n"),
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
