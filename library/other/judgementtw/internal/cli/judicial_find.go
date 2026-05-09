// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/source/fjud"
)

// newJudicialFindCmd builds the FJUD search command. It is registered by
// root.go in place of the generator's auto-emitted `find` shortcut so the
// command actually scrapes the FJUD website instead of GETing a JSON URL.
func newJudicialFindCmd(flags *rootFlags) *cobra.Command {
	var court, caseType, caseChar string
	var year, no, noEnd int
	var from, to, reason, verdict, keyword string
	var limit, page int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search Taiwan court judgments via judgment.judicial.gov.tw",
		Long: `Search the public Taiwan judgment database (FJUD) at judgment.judicial.gov.tw.
Combine any of: court, case-type, year, case-character, number range, date range,
case reason, verdict text, free-text keyword. All filters are optional but at
least one is recommended for narrow results.

Court codes are 3 letters (TPS, TPH, TPD, ...). Run 'judgementtw-pp-cli case-types courts'
to list all 41 courts. Case-type accepts long-form (criminal, civil, administrative,
disciplinary, constitutional) or short codes (M, V, A, P, C). Date flags accept
ROC (民國 115/4/30) or Gregorian (2026-04-30) form.`,
		Example: `  # Recent Supreme Court criminal narcotics rulings
  judgementtw-pp-cli find --court TPS --type criminal --keyword 毒品危害防制條例 --limit 10 --json

  # All 高等法院 drug appeals from 110-115
  judgementtw-pp-cli find --court TPH --case-char 毒抗 --from 110/1/1 --to 115/12/31 --json

  # Single case lookup by case number
  judgementtw-pp-cli find --court TPH --year 110 --case-char 毒抗 --no 1212 --no-end 1212 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			c := fjudClient(flags)
			res, err := c.Search(ctx, fjud.SearchParams{
				Courts:    parseCSVList(court),
				CaseTypes: resolveCaseTypes(caseType),
				Year:      year,
				CaseChar:  caseChar,
				NoStart:   no,
				NoEnd:     noEnd,
				From:      from,
				To:        to,
				Reason:    reason,
				Verdict:   verdict,
				Keyword:   keyword,
				Limit:     limit,
				Page:      page,
			})
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&court, "court", "", "Court code(s), comma-separated (e.g. TPS,TPH,TPD)")
	cmd.Flags().StringVar(&caseType, "type", "", "Case type(s), comma-separated: criminal,civil,administrative,disciplinary,constitutional")
	cmd.Flags().IntVar(&year, "year", 0, "ROC (民國) year, e.g. 115")
	cmd.Flags().StringVar(&caseChar, "case-char", "", "字別 (case character), e.g. 台抗, 毒抗, 訴")
	cmd.Flags().IntVar(&no, "no", 0, "Case number range start")
	cmd.Flags().IntVar(&noEnd, "no-end", 0, "Case number range end")
	cmd.Flags().StringVar(&from, "from", "", "Date range start (115/4/30 or 2026-04-30)")
	cmd.Flags().StringVar(&to, "to", "", "Date range end")
	cmd.Flags().StringVar(&reason, "reason", "", "裁判案由 substring")
	cmd.Flags().StringVar(&verdict, "verdict", "", "主文 substring")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Free-text keyword in body")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results to return")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (1-indexed)")
	return cmd
}
