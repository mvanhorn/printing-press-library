// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// fetchMarketTrends shares aggregate-trends fetching across market, summary,
// trends, and appreciation. Returns the parsed RegionTrendPoint rows.
func fetchMarketTrends(flags *rootFlags, regionID int64, regionType, periodMonths int, label string) ([]redfin.RegionTrendPoint, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	if periodMonths <= 0 {
		periodMonths = 24
	}
	path := fmt.Sprintf("/stingray/api/region/%d/%d/%d/aggregate-trends", regionType, regionID, periodMonths)
	data, err := c.Get(path, nil)
	if err != nil {
		return nil, classifyAPIError(err)
	}
	return redfin.ParseTrendsResponse(data, label, regionID)
}

// marketDryRunPath returns the path that fetchMarketTrends would call, used
// by dry-run paths so users see exactly what region+period was resolved.
func marketDryRunPath(regionID int64, regionType, periodMonths int) string {
	if periodMonths <= 0 {
		periodMonths = 24
	}
	return fmt.Sprintf("/stingray/api/region/%d/%d/%d/aggregate-trends", regionType, regionID, periodMonths)
}

func newMarketCmd(flags *rootFlags) *cobra.Command {
	var period int

	cmd := &cobra.Command{
		Use:   "market [region-slug-or-id]",
		Short: "Fetch aggregate-trends JSON for one region and emit long-format trend rows.",
		Long: `Resolve a region (numeric ID, slug, or 'id:type' pair), call
/stingray/api/region/<type>/<id>/<months>/aggregate-trends, strip the {}&&
prefix, and emit a tidy long table of (month, metric, value) rows.

Region forms:
  - "30772"                    → city/30772
  - "30772:6"                  → region_id=30772, region_type=6
  - "city/30772/TX/Austin"     → parsed slug`,
		Example: `  redfin-pp-cli market 30772 --json
  redfin-pp-cli market "city/30772/TX/Austin" --period 12 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "would GET: "+marketDryRunPath(0, 6, period))
					return nil
				}
				return cmd.Help()
			}
			id, typ, err := parseRegionSlug(args[0])
			if err != nil {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.OutOrStdout(), "would GET: "+marketDryRunPath(0, 6, period))
					return nil
				}
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "would GET: "+marketDryRunPath(id, typ, period))
				return nil
			}
			label := strconv.FormatInt(id, 10)
			rows, err := fetchMarketTrends(flags, id, typ, period, label)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&period, "period", 24, "Window in months (typical: 12 or 24)")
	return cmd
}
