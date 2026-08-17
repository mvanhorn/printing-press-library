// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: populate the cross-zone record mirror. The framework `sync`
// cannot mirror records (hierarchical endpoint), so this command lists every
// domain and fetches its records live, tagging each with its zone, then writes
// the zone_records mirror and appends a drift snapshot.
// pp:data-source live

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dnsmadeeasy/internal/store"

	"github.com/spf13/cobra"
)

type syncRecordsView struct {
	Zones         int    `json:"zones"`
	Records       int    `json:"records"`
	FailedZones   int    `json:"failed_zones"`
	Batch         string `json:"snapshot_batch"`
	FetchFailures []struct {
		Zone  string `json:"zone"`
		Error string `json:"error"`
	} `json:"fetch_failures,omitempty"`
}

func newNovelSyncRecordsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync-records",
		Short: "Mirror every zone's records into the local store for cross-zone commands",
		Long: `Fetch all managed domains and their records live, tag each record with its
zone, and write the local cross-zone mirror used by where-used, drift, health,
and export. Each run also appends a snapshot so drift can compare over time.

DNS Made Easy is rate limited (~150 requests / 5 minutes); this makes one
request per zone, so large accounts may take a few minutes.`,
		Example:     "  dnsmadeeasy-pp-cli sync-records",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch all zones + records and write the local mirror")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			recs, domains, failures, err := fetchAllZoneRecords(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()

			batch, err := writeZoneMirror(ctx, s, recs)
			if err != nil {
				return fmt.Errorf("writing zone mirror: %w", err)
			}

			view := syncRecordsView{
				Zones:       len(domains),
				Records:     len(recs),
				FailedZones: len(failures),
				Batch:       batch,
			}
			for zone, msg := range failures {
				view.FetchFailures = append(view.FetchFailures, struct {
					Zone  string `json:"zone"`
					Error string `json:"error"`
				}{Zone: zone, Error: msg})
			}

			if len(failures) > 0 {
				fmt.Fprintf(os.Stderr, "warning: %d of %d zones could not be read; their records are absent from the mirror\n", len(failures), len(domains))
			}

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Mirrored %d records across %d zones (snapshot %s).\n", view.Records, view.Zones, view.Batch)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database")
	return cmd
}
