// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newAdvertisersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "advertisers",
		Short: "Look up advertiser Facebook Pages and Page IDs in Meta Ad Library (zero API key)",
	}
	cmd.AddCommand(newAdvertisersSearchCmd(flags))
	return cmd
}

func newAdvertisersSearchCmd(flags *rootFlags) *cobra.Command {
	var query string
	var country string
	var limit int
	var noSave bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search [brand-name]",
		Short: "Resolve a brand or advertiser name to its Facebook Page ID, likes, IG handle, and verification",
		Example: strings.Trim(`
  meta-ads-library-pp-cli advertisers search "Nike" --country US --agent
  meta-ads-library-pp-cli advertisers search --query "Gymshark" --country GB --limit 5 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "brand=Nike;--country=US;--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && strings.TrimSpace(query) == "" {
				query = strings.Join(args, " ")
			}
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "advertisers search")
			}
			if strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("brand or advertiser query is required (pass [brand-name] or --query)"))
			}

			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), make([]adlibrary.Advertiser, 0), flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := newAdLibraryClient(flags)
			advs, err := client.SearchAdvertisers(ctx, query, country)
			if err != nil {
				return classifyAdLibraryErr(cmd, flags, err)
			}
			if limit > 0 && len(advs) > limit {
				advs = advs[:limit]
			}

			if !noSave && !localDataDisabled() && len(advs) > 0 {
				resolvedDB := dbPath
				if resolvedDB == "" {
					resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
				}
				db, openErr := store.OpenWithContext(ctx, resolvedDB)
				if openErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to open local database %s: %v\n", resolvedDB, openErr)
				} else {
					if saveErr := adlibrary.SaveAdvertisers(ctx, db, advs); saveErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist advertisers locally: %v\n", saveErr)
					}
					_ = db.Close()
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), advs, flags)
			}
			if len(advs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching advertisers found.")
				return nil
			}
			rows := make([][]string, 0, len(advs))
			for _, a := range advs {
				rows = append(rows, []string{
					a.PageID,
					a.PageName,
					a.Category,
					fmt.Sprintf("%d", a.Likes),
					a.IGUsername,
					a.Verification,
				})
			}
			return flags.printTable(cmd, []string{"PAGE ID", "PAGE NAME", "CATEGORY", "LIKES", "IG HANDLE", "VERIFIED"}, rows)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Brand or Facebook Page name to look up")
	cmd.Flags().StringVarP(&country, "country", "c", "US", "ISO country code (e.g. US, MX, GB, DE) or ALL")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum advertiser pages to return")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Do not persist looked-up advertisers to local SQLite")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
