// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the Kakao Transparency CLI. Carried across
// regen via the novel-command merge path; the generated stub was replaced.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// seriesRow is one service-corporation x request-category observation for one
// half-year. Counts stay strings for source fidelity: the API returns them as
// strings and uses "-1" to mean N/A (rendered as such on the site).
type seriesRow struct {
	Year              string `json:"year"`
	HalfYear          string `json:"halfYear"`
	Service           string `json:"service"`
	Category          string `json:"category"`
	CategoryKo        string `json:"categoryKo"`
	NumberOfRequests  string `json:"numberOfRequests"`
	NumberOfProcesses string `json:"numberOfProcesses"`
	NumberOfAccounts  string `json:"numberOfAccounts"`
}

// pp:data-source live
func newNovelSeriesCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagService string
	var flagSince int

	cmd := &cobra.Command{
		Use:   "series",
		Short: "Whole-archive statistics series: one tidy row per half-year, service, and request category since 2012",
		Long: "Walks every published half-year report (1H 2012 through the latest) and reshapes the " +
			"eight per-period statistics rows into one long-form table — one row per half-year, service " +
			"corporation (Kakao/Daum), and government request category. This is the longitudinal series the " +
			"one-period-at-a-time web page cannot show. Counts are source-fidelity strings; \"-1\" means N/A.",
		Example: "  github.com/mvanhorn/printing-press-library/library/other/kakao-transparency series --category warrant --service kakao\n" +
			"  github.com/mvanhorn/printing-press-library/library/other/kakao-transparency series --since 2020 --agent --select year,halfYear,numberOfRequests,numberOfProcesses\n" +
			"  github.com/mvanhorn/printing-press-library/library/other/kakao-transparency series --csv",
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
			wantCategory := strings.ToLower(strings.TrimSpace(flagCategory))
			wantService := strings.ToLower(strings.TrimSpace(flagService))

			rows := make([]seriesRow, 0, 64)
			fetched := 0
			err = walkKakaoArchive(ctx, c, first, 1, func(data kakaoReportData) bool {
				fetched++
				for _, r := range data.Reports {
					service := kakaoServiceSlug(r.ServiceCorp)
					if wantService != "" && service != wantService {
						continue
					}
					if wantCategory != "" && !strings.Contains(strings.ToLower(r.EnCategory), wantCategory) {
						continue
					}
					rows = append(rows, seriesRow{
						Year:              data.Year,
						HalfYear:          halfLabel(data.HalfYearID),
						Service:           service,
						Category:          r.EnCategory,
						CategoryKo:        r.Category,
						NumberOfRequests:  string(r.NumberOfRequests),
						NumberOfProcesses: string(r.NumberOfProcesses),
						NumberOfAccounts:  string(r.NumberOfAccounts),
					})
				}
				return true
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if fetched == 0 {
				return apiErr(fmt.Errorf("no published reports found from %d onward", first))
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter to one request category by English-label substring (e.g. warrant, restriction, confirmation, user information)")
	cmd.Flags().StringVar(&flagService, "service", "", "Filter to one service corporation: kakao or daum")
	cmd.Flags().IntVar(&flagSince, "since", kakaoArchiveStartYear, "First year to include (archive starts 2012)")
	return cmd
}
