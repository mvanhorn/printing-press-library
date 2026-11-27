// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newMediaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "Download ad creative images and videos from locally stored Meta ads",
	}
	cmd.AddCommand(newMediaDownloadCmd(flags))
	return cmd
}

func newMediaDownloadCmd(flags *rootFlags) *cobra.Command {
	var adIDs []string
	var pageFilter string
	var outDir string
	var imagesOnly bool
	var videosOnly bool
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "download [ad-archive-id...]",
		Short: "Download creative images and videos for synced ads to a local folder",
		Example: strings.Trim(`
  meta-ads-library-pp-cli media download --page-id 15087023444 --output-dir ./creatives --limit 5 --dry-run
  meta-ads-library-pp-cli media download 1550534860016399 --output-dir ./creatives --json
`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"pp:happy-args":  "--page-id=15087023444;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, a := range args {
				if trimmed := strings.TrimSpace(a); trimmed != "" {
					adIDs = append(adIDs, trimmed)
				}
			}
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "media download")
			}
			if len(adIDs) == 0 && strings.TrimSpace(pageFilter) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least one ad archive ID or --page-id"))
			}
			if imagesOnly && videosOnly {
				return usageErr(fmt.Errorf("--images-only and --videos-only cannot both be set"))
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads search --page-id <page_id> --country US\n", resolvedDB)
				empty := adlibrary.MediaDownloadReport{OutputDir: outDir, Files: make([]adlibrary.MediaFile, 0)}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
				}
				return nil
			}

			ctx := cmd.Context()
			cancel := func() {}
			if flags != nil && flags.timeoutExplicit {
				ctx, cancel = boundCtx(cmd.Context(), flags)
			}
			defer cancel()

			db, err := store.OpenReadOnlyContext(ctx, resolvedDB)
			if err != nil {
				return err
			}
			defer db.Close()

			allAds, err := adlibrary.LoadAds(ctx, db, pageFilter, false)
			if err != nil {
				return err
			}

			var selected []adlibrary.Ad
			if len(adIDs) > 0 {
				want := make(map[string]bool, len(adIDs))
				for _, id := range adIDs {
					want[id] = true
				}
				for _, ad := range allAds {
					if want[ad.AdArchiveID] {
						selected = append(selected, ad)
					}
				}
			} else {
				selected = allAds
			}

			if limit > 0 && len(selected) > limit {
				selected = selected[:limit]
			}
			if cliutil.IsDogfoodEnv() && len(selected) > 1 {
				selected = selected[:1]
			}
			if cliutil.IsVerifyEnv() {
				empty := adlibrary.MediaDownloadReport{OutputDir: outDir, Files: make([]adlibrary.MediaFile, 0)}
				return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
			}

			client := newAdLibraryClient(flags)
			report, err := client.DownloadMedia(ctx, selected, outDir, imagesOnly, videosOnly)
			if err != nil {
				return classifyAdLibraryErr(cmd, flags, err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if report.Total == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No media assets found for the selected ads.")
				return nil
			}
			rows := make([][]string, 0, len(report.Files))
			for _, f := range report.Files {
				rows = append(rows, []string{
					f.AdArchiveID,
					f.Kind,
					f.Status,
					fmt.Sprintf("%d", f.Bytes),
					f.LocalPath,
				})
			}
			return flags.printTable(cmd, []string{"AD ID", "KIND", "STATUS", "BYTES", "PATH"}, rows)
		},
	}

	cmd.Flags().StringSliceVar(&adIDs, "ad-id", nil, "Numeric Meta Ad Library archive IDs to download media for")
	cmd.Flags().StringVar(&pageFilter, "page-id", "", "Download media for all locally synced ads belonging to this Facebook Page ID or name")
	cmd.Flags().StringVarP(&outDir, "output-dir", "o", "./meta-ads-media", "Directory where downloaded creative images and videos are written")
	cmd.Flags().BoolVar(&imagesOnly, "images-only", false, "Download only image creatives")
	cmd.Flags().BoolVar(&videosOnly, "videos-only", false, "Download only video creatives")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of ads to download media from")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
