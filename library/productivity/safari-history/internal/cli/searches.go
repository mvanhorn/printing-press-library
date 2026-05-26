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

func newSearchesCmd(opts *RootOptions) *cobra.Command {
	var since, domain string
	cmd := &cobra.Command{
		Use:         "searches",
		Short:       "List keyword search-engine queries over --since, optionally by --domain (not available for Safari, which omits search terms from History.db)",
		Example:     strings.Trim("safari-history-pp-cli searches --since 14d --limit 20", "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.Source.Capabilities().SearchTerms {
				return renderNotAvailable(opts, "searches", "Safari does not store search terms in History.db")
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
			rows, err := opts.Source.SearchTerms(st.DB(), source.VisitFilter{Since: start, Limit: opts.Output.Limit, Domain: domain, Device: opts.Device})
			if err != nil {
				return err
			}
			df := source.NormalizeTargetDomain(domain)
			out := []map[string]any{}
			for _, r := range rows {
				d := source.DomainFromURL(r.URL)
				if df != "" && d != df {
					continue
				}
				out = append(out, map[string]any{"term": r.Term, "when": r.When.Format(time.RFC3339), "url": r.URL, "title": r.Title, "domain": d})
			}
			maybePrintEmptyWindowHint(st.DB(), since, len(out) == 0)
			output.DefaultToJSONIfNotTTY(&opts.Output)
			return output.Render(opts.Output, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "start time")
	cmd.Flags().StringVar(&domain, "domain", "", "registrable domain")
	return cmd
}
