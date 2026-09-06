// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// fleet: cross-container matrix over every mirrored container.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelFleetCmd(flags *rootFlags) *cobra.Command {
	var dbPath, metric string

	cmd := &cobra.Command{
		Use:         "fleet",
		Short:       "Cross-container matrix over every mirrored container",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Summarize every container in the local mirror in one table: GA4/measurement-id
inventory, Consent Mode coverage, Custom HTML count, and entity counts. Uses the
most recent snapshot per container. Pull each container first.

--metric focuses the view: ga4 | consent | custom-html | counts (default: all).`,
		Example: `  # Full fleet matrix
  google-tag-manager-pp-cli fleet --json

  # Just the GA4 measurement-id inventory
  google-tag-manager-pp-cli fleet --metric ga4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would summarize every mirrored container")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, snaps, ok, err := gtmReadSnapshots(ctx, cmd, flags, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			rows := make([]fleetRow, 0)
			for _, snap := range latestPerContainer(snaps) {
				entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "")
				if err != nil {
					return err
				}
				rows = append(rows, fleetRowFor(snap, entities))
			}
			return gtmEmit(cmd, flags, rows, func(w io.Writer) {
				tw := newTabWriter(w)
				switch metric {
				case "ga4":
					fmt.Fprintln(tw, "CONTAINER\tMEASUREMENT IDS")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%s\n", r.Container, strings.Join(r.MeasurementIDs, " "))
					}
				case "consent":
					fmt.Fprintln(tw, "CONTAINER\tCONSENT COVERAGE")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%d%%\n", r.Container, r.ConsentPct)
					}
				case "custom-html":
					fmt.Fprintln(tw, "CONTAINER\tCUSTOM HTML")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%d\n", r.Container, r.CustomHTML)
					}
				default:
					fmt.Fprintln(tw, "CONTAINER\tTAGS\tTRIGGERS\tVARS\tCUSTOM-HTML\tCONSENT\tGA4 IDS")
					for _, r := range rows {
						fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d%%\t%d\n",
							r.Container, r.Tags, r.Triggers, r.Variables, r.CustomHTML, r.ConsentPct, len(r.MeasurementIDs))
					}
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metric, "metric", "", "Focus a single metric: ga4 | consent | custom-html | counts")
	return cmd
}
