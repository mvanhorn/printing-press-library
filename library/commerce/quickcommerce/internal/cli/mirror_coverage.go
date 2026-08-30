// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelMirrorCoverageCmd(flags *rootFlags) *cobra.Command {
	var location, dbPath string
	cmd := &cobra.Command{
		Use: "coverage", Short: "Report observed, missing, and stale platform coverage for a location.",
		Example:     "  quickcommerce-pp-cli mirror coverage --location 12.9021,77.6639 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror coverage")
			}
			if location == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--location is required as latitude,longitude"))
			}
			_, _, canonical, err := parseQCLocation(location)
			if err != nil {
				return usageErr(err)
			}
			path := qcDBPath(dbPath)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				return qcMissingMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, "products")
			}
			if err != nil {
				return apiErr(err)
			}
			db, err := openQCCreatable(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT resource,platform,COUNT(*) FROM quickcommerce_observations WHERE location=? GROUP BY resource,platform`, canonical)
			if err != nil {
				return apiErr(fmt.Errorf("querying coverage: %w", err))
			}
			defer rows.Close()
			observed := map[string]int{}
			for rows.Next() {
				var resource, platform string
				var count int
				if err := rows.Scan(&resource, &platform, &count); err != nil {
					return apiErr(err)
				}
				observed[resource+"\x00"+platform] = count
			}
			if err := rows.Err(); err != nil {
				return apiErr(err)
			}
			platforms := []string{"BlinkIt", "Zepto", "Swiggy", "BigBasket", "DMart", "JioMart", "Minutes", "Amazon", "Nykaa", "Myntra", "Flipkart"}
			etaPlatforms := map[string]bool{"BlinkIt": true, "Zepto": true, "Swiggy": true, "BigBasket": true, "DMart": true, "JioMart": true, "Minutes": true}
			out := make([]map[string]any, 0, len(platforms)*2)
			for _, platform := range platforms {
				for _, resource := range []string{"products", "delivery"} {
					if resource == "delivery" && !etaPlatforms[platform] {
						continue
					}
					count := observed[resource+"\x00"+platform]
					status := "missing"
					if count > 0 {
						status = "observed"
					}
					out = append(out, map[string]any{"platform": platform, "resource": resource, "status": status, "observations": count})
				}
			}
			sort.Slice(out, func(i, j int) bool {
				return fmt.Sprint(out[i]["platform"], out[i]["resource"]) < fmt.Sprint(out[j]["platform"], out[j]["resource"])
			})
			view := map[string]any{"location": canonical, "coverage": out, "observed_count": countCoverage(out)}
			return qcPrint(cmd.OutOrStdout(), flags, view, out)
		},
	}
	cmd.Flags().StringVar(&location, "location", "", "Coordinates as latitude,longitude")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path")
	return cmd
}
func countCoverage(rows []map[string]any) int {
	n := 0
	for _, r := range rows {
		if r["status"] == "observed" {
			n++
		}
	}
	return n
}
