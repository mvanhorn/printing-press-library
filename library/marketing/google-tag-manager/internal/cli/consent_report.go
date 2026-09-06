// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// consent-report: Consent Mode v2 readiness for a mirrored container.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type consentReport struct {
	Snapshot string       `json:"snapshot"`
	Tags     int          `json:"tags"`
	Gated    int          `json:"gated"`
	Ungated  int          `json:"ungated"`
	Rows     []consentRow `json:"rows"`
}

func newNovelConsentReportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, container string

	cmd := &cobra.Command{
		Use:         "consent-report",
		Short:       "Consent Mode v2 readiness for a mirrored container",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Report every tag's Consent Mode (consentSettings) status, classified by
vendor, and flag tracking tags that fire without consent gating. Ungated tags
are listed first because they are the EEA compliance risk. Run 'pull' first.`,
		Example: `  # Consent coverage for the most recent snapshot
  google-tag-manager-pp-cli consent-report --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report Consent Mode coverage")
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

			snap, err := resolveSnapshot(snaps, container)
			if err != nil {
				return usageErr(err)
			}
			entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "tag")
			if err != nil {
				return err
			}
			rows := runConsentReport(entities)
			rep := consentReport{Snapshot: snap.Label, Tags: len(rows), Rows: rows}
			for _, r := range rows {
				if r.Gated {
					rep.Gated++
				} else {
					rep.Ungated++
				}
			}
			return gtmEmit(cmd, flags, rep, func(w io.Writer) {
				fmt.Fprintf(w, "Consent Mode coverage for %s — %d gated, %d ungated of %d tags\n",
					snap.Label, rep.Gated, rep.Ungated, rep.Tags)
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "  GATED\tVENDOR\tTAG\tCONSENT TYPES")
				for _, r := range rows {
					mark := "no"
					if r.Gated {
						mark = "yes"
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", mark, r.Vendor, r.Tag, strings.Join(r.ConsentTypes, ","))
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&container, "snapshot", "", "Snapshot ref: live, workspace:<id>, container:<id>, version:<id>, or a snapshot id (default: most recent)")
	return cmd
}
