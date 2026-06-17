// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/world-bank/internal/cliutil"

	"github.com/spf13/cobra"
)

type wbIndicatorRow struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Unit               string      `json:"unit"`
	Source             wbCodeValue `json:"source"`
	SourceNote         string      `json:"sourceNote"`
	SourceOrganization string      `json:"sourceOrganization"`
}

func newNovelIndicatorsFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var maxScanPages int

	cmd := &cobra.Command{
		Use:         "find <query>",
		Short:       "Search the full ~16,000-indicator catalog by keyword.",
		Long:        "Search the full ~16,000-indicator catalog by keyword across id, name, and source note.\nUse this command to discover indicator codes. Do NOT use it to fetch values; use 'data'.",
		Example:     "  world-bank-pp-cli indicators find \"co2 emissions\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("query is required"))
			}
			query := strings.ToLower(strings.Join(args, " "))
			if maxScanPages > 1 && cliutil.IsDogfoodEnv() {
				maxScanPages = 1
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			rows, err := wbGetAllRows(ctx, c, "/indicator", map[string]string{"per_page": "1000"}, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			matches := make([]wbIndicatorRow, 0, limit)
			for _, r := range rows {
				var ind wbIndicatorRow
				if json.Unmarshal(r, &ind) != nil {
					continue
				}
				hay := strings.ToLower(ind.ID + " " + ind.Name + " " + ind.SourceNote)
				if strings.Contains(hay, query) {
					matches = append(matches, ind)
				}
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}
			return flags.printJSON(cmd, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum matches to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 20, "maximum catalog pages to scan")
	return cmd
}
