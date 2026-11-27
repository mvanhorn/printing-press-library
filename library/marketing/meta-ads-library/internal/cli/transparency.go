// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelTransparencyCmd(flags *rootFlags) *cobra.Command {
	var byline string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "transparency [page-or-id]",
		Short: "Roll up EU/UK/BR reach, age-gender breakdowns, beneficiary/payer pairs, spend brackets, and violation flags per advertiser or byline.",
		Long: `Use this command for an advertiser-level or byline-level rollup of EU/UK/BR reach, age/gender demographics, beneficiary/payer disclosures, spend brackets, and violation flags across synced ads.
Do NOT use this command to inspect a single ad's raw transparency payload (use 'ads details') or query raw official Graph API rows directly (use 'archive').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli transparency 15087023444 --agent
  meta-ads-library-pp-cli transparency --byline "Paid for by" --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "page=15087023444",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "transparency")
			}

			pageFilter := ""
			if len(args) > 0 {
				pageFilter = strings.TrimSpace(args[0])
			}
			if pageFilter == "" && strings.TrimSpace(byline) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide a page ID/name or --byline"))
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads details <ad_id> --page-id <page_id> --country DE\n", resolvedDB)
				empty := adlibrary.TransparencyReport{
					PageID:            pageFilter,
					BylineFilter:      strings.TrimSpace(byline),
					Currencies:        make([]string, 0),
					Bylines:           make([]string, 0),
					BeneficiaryPayers: make([]adlibrary.BeneficiaryPayer, 0),
					GenderReach:       make(map[string]int64),
					AgeReach:          make(map[string]int64),
					ViolationTypes:    make([]string, 0),
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(cmd.Context(), resolvedDB)
			if err != nil {
				return err
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "ads") {
				hintIfStale(cmd, db, "ads", flags.maxAge)
			}

			report, err := adlibrary.ComputeTransparency(cmd.Context(), db, pageFilter, byline)
			if err != nil {
				return err
			}
			if pageFilter != "" && report.AdsAnalyzed == 0 && report.DetailsAnalyzed == 0 && !isAllDigits(pageFilter) {
				nf := notFoundErr(fmt.Errorf("advertiser %q not found in local SQLite; pass a numeric Facebook Page ID or run 'meta-ads-library-pp-cli ads search' first", pageFilter))
				writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, nf, ExitCode(nf))
				return nf
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			rows := [][]string{
				{"Page", fmt.Sprintf("%s (%s)", report.PageName, report.PageID)},
				{"Ads Analyzed", fmt.Sprintf("%d", report.AdsAnalyzed)},
				{"Ad Details Analyzed", fmt.Sprintf("%d", report.DetailsAnalyzed)},
				{"EU Total Reach", fmt.Sprintf("%d", report.EUTotalReachSum)},
				{"UK Total Reach", fmt.Sprintf("%d", report.UKTotalReachSum)},
				{"BR Total Reach", fmt.Sprintf("%d", report.BRTotalReachSum)},
				{"Spend Bracket Sum", fmt.Sprintf("%d - %d (%s)", report.SpendLowerSum, report.SpendUpperSum, strings.Join(report.Currencies, ","))},
				{"Beneficiary/Payer Pairs", fmt.Sprintf("%d", len(report.BeneficiaryPayers))},
				{"Violating EU SIEP Ads", fmt.Sprintf("%d", report.ViolatingAds)},
				{"AI Disclosure Eligible Ads", fmt.Sprintf("%d", report.AIDisclosureAds)},
			}
			return flags.printTable(cmd, []string{"METRIC", "VALUE"}, rows)
		},
	}

	cmd.Flags().StringVar(&byline, "byline", "", "Filter transparency rollup by 'paid for by' disclaimer byline substring")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
