// Browse one level of the public website area hierarchy.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
	"github.com/spf13/cobra"
)

func newPublicAreasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "areas", Short: "Browse Japan area IDs from the source website", Example: "  rakuten-travel-pp-cli areas list --parent tokyo --limit 5", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}, RunE: publicTravelGroup}
	cmd.AddCommand(newPublicAreasListCmd(flags))
	return cmd
}

func newPublicAreasListCmd(flags *rootFlags) *cobra.Command {
	var options publicTravelOptions
	var parent string
	var limit, offset int
	cmd := &cobra.Command{
		Use: "list", Short: "List one level of areas, preserving website path IDs",
		Example:     "  rakuten-travel-pp-cli areas list --parent tokyo --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--parent=tokyo;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return publicTravelDryRun(cmd, flags)
			}
			if err := options.validate(cmd, flags, args); err != nil {
				return err
			}
			if err := travel.ValidateAreaParent(parent); err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
				return usageErr(fmt.Errorf("--limit must be 1–100 and --offset 0–10000"))
			}
			api, err := options.client(cmd, flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			result, err := api.Areas(ctx, parent)
			if err != nil {
				return publicTravelError(err)
			}
			total := len(result.Areas)
			start := min(offset, total)
			end := min(start+limit, total)
			areas := append([]travel.Area{}, result.Areas[start:end]...)
			page := result.Page
			page.SourcePage, page.SourceUnit = 1, "area_links"
			page.Offset, page.Limit = offset, limit
			page.SourceItemsSeen, page.RowsSeen, page.RowsScanned, page.Emitted = total, total, end, len(areas)
			page.SourceTotal = &total
			page.HasMore, page.Coverage = end < total, "one_hierarchy_level"
			page.NextPage, page.NextOffset = nil, nil
			if page.HasMore {
				nextPage, nextOffset := 1, end
				page.NextPage, page.NextOffset = &nextPage, &nextOffset
			}
			meta := publicTravelMeta(result.Status, result.Source, &page, map[string]any{"parent": parent, "limit": limit, "offset": offset}, api)
			return options.output(cmd, flags, meta, areas)
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Website area parent, e.g. tokyo; empty lists prefectures")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum emitted area links (1–100)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Skip this many links within the source page")
	options.bind(cmd)
	return cmd
}
