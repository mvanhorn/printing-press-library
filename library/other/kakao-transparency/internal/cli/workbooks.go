// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the Kakao Transparency CLI. Carried across
// regen via the novel-command merge path; the generated stub was replaced.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// workbookRow is one half-year's official XLSX workbook editions.
type workbookRow struct {
	Year       string `json:"year"`
	HalfYear   string `json:"halfYear"`
	Title      string `json:"title"`
	KoreanURL  string `json:"koreanUrl,omitempty"`
	EnglishURL string `json:"englishUrl,omitempty"`
}

// pp:data-source live
func newNovelWorkbooksCmd(flags *rootFlags) *cobra.Command {
	var flagSince int

	cmd := &cobra.Command{
		Use:   "workbooks",
		Short: "List the official XLSX workbook download URLs (Korean and English editions) per published half-year",
		Long: "Each half-year's workbook URLs are buried in that period's JSON payload. This command walks " +
			"the archive and collects them into one index, so a downstream job can mirror or cite the " +
			"primary-source files instead of the parsed numbers.",
		Example: "  kakao-transparency-pp-cli workbooks --since 2020 --agent\n" +
			"  kakao-transparency-pp-cli workbooks --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			first := kakaoSeriesStart(flagSince)
			rows := make([]workbookRow, 0, 32)
			err = walkKakaoArchiveLang(ctx, c, first, 1, "ko", func(data kakaoReportData) bool {
				rows = append(rows, workbookRow{
					Year:       data.Year,
					HalfYear:   halfLabel(data.HalfYearID),
					Title:      data.Title,
					KoreanURL:  data.FileURL,
					EnglishURL: data.EnFileURL,
				})
				return true
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(rows) == 0 {
				return apiErr(fmt.Errorf("no published reports found from %d onward", first))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&flagSince, "since", kakaoArchiveStartYear, "First year to include (archive starts 2012)")
	return cmd
}
