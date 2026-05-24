package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/output"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/store"
)

func newDownloadsCmd(opts *RootOptions) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "downloads",
		Short:       "List downloaded files over --since with filename, size, MIME, and state (not available for Safari, which omits downloads from History.db)",
		Example:     strings.Trim("safari-history-pp-cli downloads --since 30d --limit 20", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.Source.Capabilities().Downloads {
				return renderNotAvailable(opts, "downloads", "Safari does not store downloads in History.db")
			}
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
			rows, err := opts.Source.Downloads(st.DB(), source.VisitFilter{Since: start, Limit: opts.Output.Limit, Device: opts.Device})
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for _, r := range rows {
				out = append(out, map[string]any{"filename": filepath.Base(r.TargetPath), "size": r.Bytes, "mime": r.MIME, "source": r.Source, "when": r.When.Format(time.RFC3339), "state": r.State})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	return cmd
}
