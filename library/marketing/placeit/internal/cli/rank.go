// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// rank shows a template's purchase percentile within its category and tag
// cohort — a local statistic no single Placeit call exposes.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rank <id>",
		Short: "Show a template's purchase percentile within its category and tag cohort.",
		Long: strings.Trim(`
Given a template id, compute how its purchase count ranks against the rest of
its category and its primary device-tag cohort in your local mirror — context
the Placeit UI never shows. Run 'sync' first to populate the mirror.

Use to judge one template's popularity in context. To rank a whole result set,
use 'top'.`, "\n"),
		Example:     "  placeit-pp-cli rank 41935 --agent",
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
				return usageErr(fmt.Errorf("a template id is required"))
			}
			id := args[0]
			db, ok, err := openCatalogStore(cmd, flags)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, templateResource) {
				hintIfStale(cmd, db, templateResource, flags.maxAge)
			}
			all, err := loadCatalog(db)
			if err != nil {
				return apiErr(err)
			}
			var target map[string]any
			for _, m := range all {
				if fmt.Sprint(m["id"]) == id || stageID(m) == id {
					target = m
					break
				}
			}
			if target == nil {
				return notFoundErr(fmt.Errorf("template %q not in local mirror; run 'sync' covering it first", id))
			}
			tp := asFloat(target["purchases"])
			cat := fmt.Sprint(target["category_name"])
			devices := tagValues(target, "device_tags")
			var primaryDevice string
			if len(devices) > 0 {
				primaryDevice = devices[0]
			}

			catPct, catN := percentile(all, tp, func(m map[string]any) bool {
				return fmt.Sprint(m["category_name"]) == cat
			})
			var devicePct float64
			var deviceN int
			if primaryDevice != "" {
				devicePct, deviceN = percentile(all, tp, func(m map[string]any) bool {
					return containsTag(m, "device_tags", primaryDevice)
				})
			}
			result := map[string]any{
				"id":                   target["id"],
				"name":                 target["name"],
				"category_name":        cat,
				"purchases":            tp,
				"category_percentile":  round1(catPct),
				"category_cohort_size": catN,
				"primary_device_tag":   primaryDevice,
				"device_percentile":    round1(devicePct),
				"device_cohort_size":   deviceN,
				"deep_link":            stageDeepLink(target),
			}
			return flags.printJSON(cmd, result)
		},
	}
	return cmd
}

// percentile returns the percentile rank (0-100) of value among cohort members
// (those matching pred), plus the cohort size. The denominator excludes the
// target itself (which has purchases == value, so it is never counted in
// `below`), so a sole most-purchased template reads 100 and a template tied at
// the top reads below 100. 0 means least-purchased.
func percentile(all []map[string]any, value float64, pred func(map[string]any) bool) (float64, int) {
	n := 0
	below := 0
	for _, m := range all {
		if !pred(m) {
			continue
		}
		n++
		if asFloat(m["purchases"]) < value {
			below++
		}
	}
	if n == 0 {
		return 0, 0
	}
	denom := n - 1
	if denom < 1 {
		denom = 1
	}
	return float64(below) / float64(denom) * 100, n
}

func round1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
