// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelStationNoticesCmd(flags *rootFlags) *cobra.Command {
	var pages, detail, limit int
	cmd := &cobra.Command{Use: "station-notices [id]", Short: "Find notices explicitly linked to one station ID within page/detail scan limits", Long: "Use this command for recently published notices associated with one station ID. Do NOT use it to judge current opening or permission; use 'readiness' for listed evidence and unknowns. Exact canonical station-ID links are matched; title/name guesses are not authoritative. Every notice on the scanned index pages is retained, then --max-detail-records limits how many of those detail pages are opened. --max-scan-pages and --max-detail-records cap scanning independently of --limit. A station link after the detail prefix is not a match, and no match within opened details does not mean no advisory exists. Publication dates are Asia/Tokyo calendar dates, separate from dates discussed in text.", Example: "  michi-no-eki-pp-cli station-notices 19487 --max-scan-pages 1 --max-detail-records 10 --limit 5 --agent", Annotations: michiAnnotations("live", "id=19487;--max-scan-pages=1;--max-detail-records=2;--limit=5"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "station-notices")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) != 1 {
			return usageErr(fmt.Errorf("station-notices requires one numeric station ID, for example 19487"))
		}
		if e := michi.ValidateIDs(args[0]); e != nil || strings.Contains(args[0], ",") {
			if e == nil {
				e = fmt.Errorf("one station ID is required")
			}
			return usageErr(e)
		}
		if pages < 1 || pages > 5 || detail < 1 || detail > 50 || limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("require --max-scan-pages 1–5, --max-detail-records 1–50 and --limit 1–50"))
		}
		src, e := michiSource(flags)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		v, fetchErr := src.StationNotices(ctx, args[0], pages, detail, limit)
		if len(v.FetchFailures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d notice-detail fetches failed; matching uses %d successful details\n", len(v.FetchFailures), v.DetailRecords, v.DetailRecords-len(v.FetchFailures))
		}
		if fetchErr != nil && v.ScannedPages == 0 {
			return classifyAPIErrorOnly(fetchErr)
		}
		if e := printJSONFiltered(cmd.OutOrStdout(), v, flags); e != nil {
			return e
		}
		if fetchErr != nil {
			return classifyAPIErrorOnly(fetchErr)
		}
		return nil
	}}
	cmd.Flags().IntVar(&pages, "max-scan-pages", 1, "Maximum source notice index pages scanned,1–5; older pages require a wider scan")
	cmd.Flags().IntVar(&detail, "max-detail-records", 10, "Maximum notice detail pages fetched from the scanned index,1–50")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum exact linked notice matches returned,1–50; separate from scan bounds")
	return cmd
}
