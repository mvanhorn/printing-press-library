// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"

	"github.com/spf13/cobra"
)

// versionsPageMax is how many versions the product page's version history
// lists (the cap on every public surface).
const versionsPageMax = 25

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newVersionsCmd(flags))
	})
}

func newVersionsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "versions <app>",
		Short: "Fetch an app's version history live from its product page, newest first (up to 25)",
		Long: strings.Trim(`
List an app's version history from its product page, newest first: version,
release time (RFC3339 UTC) and release notes. Apple shows the 25 newest
versions on every public surface; each fresh page is recorded append-only in
the local store, so the stored history keeps growing past 25 for watch-listing,
changes and impact.

Use this for the release timeline; 'app' shows only the current version.
An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli versions 324684580 --limit 25 --json
  app-store-pp-cli versions com.spotify.client --limit 5`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "versions")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			if limit < 0 || limit > versionsPageMax {
				return usageErr(fmt.Errorf("--limit must be between 1 and %d (the product page lists the %d newest versions); got %d", versionsPageMax, versionsPageMax, limit))
			}
			if limit == 0 {
				limit = versionsPageMax
			}
			cc, err := refCountry(cmd, flags, ref)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			lk, err := lookupAppRef(ctx, c, ref, cc)
			if err != nil {
				return err
			}
			page, err := s2aProductPage(ctx, c, lk, cc, "")
			if err != nil {
				return err
			}
			s2aWarnStore(cmd, "the version history", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))

			rows := make([]appstore.VersionEntry, 0, limit)
			for _, v := range page.Versions {
				if len(rows) == limit {
					break
				}
				rows = append(rows, v)
			}
			if len(rows) == 0 {
				s2aNote(cmd, flags, fmt.Sprintf("the %s product page lists no version history for %s", cc, lk.AppID()))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, v := range rows {
				table = append(table, []string{v.Version, v.ReleasedAt, s2aOneLine(v.Notes, 90)})
			}
			return s2aTable(cmd.OutOrStdout(), []string{"version", "released", "notes"}, table)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", versionsPageMax, "Versions to return, newest first (1-25; the page lists the 25 newest)")
	s2bDBFlag(cmd)
	return cmd
}
