// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

type ownerOverloadItem struct {
	OwnerID       string         `json:"owner_id"`
	WeightedLoad  float64        `json:"weighted_load"`
	ContactCount  int            `json:"contact_count"`
	RatioToMedian float64        `json:"ratio_to_median"`
	ByStage       map[string]int `json:"by_stage"`
}

func newNovelOwnerOverloadCmd(flags *rootFlags) *cobra.Command {
	var weightByStage bool
	var top int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "owner-overload",
		Short: "Flag sales reps carrying more active contacts than team median, weighted by lifecycle stage.",
		Long: strings.Trim(`
Group contacts by hubspot_owner_id and compute a workload number per owner.
When --weight-by-stage is true (the default), each contact contributes a
stage-specific weight (lead=1, mql=2, sql=4, opportunity=8, customer=4,
other=1) so a rep with five opportunities counts heavier than a rep with
five leads. With --weight-by-stage=false, weighted_load collapses to a raw
contact count.

Returns the top --top owners whose weighted_load exceeds the team median,
ranked by load descending.
`, "\n"),
		Example: strings.Trim(`
  # Top 10 overloaded reps using stage-weighted load
  hubspot-pp-cli owner-overload --top 10 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if top <= 0 {
				return usageErr(fmt.Errorf("--top must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			type ownerBucket struct {
				weightedLoad float64
				contactCount int
				byStage      map[string]int
			}
			owners := map[string]*ownerBucket{}
			_, parseErrors, err := streamContacts(cmd.Context(), db, func(p contactProps) error {
				owner := strings.TrimSpace(p.OwnerID)
				if owner == "" {
					return nil
				}
				b := owners[owner]
				if b == nil {
					b = &ownerBucket{byStage: map[string]int{}}
					owners[owner] = b
				}
				b.contactCount++
				if weightByStage {
					b.weightedLoad += stageWeight(p.LifecycleStage)
				} else {
					b.weightedLoad++
				}
				b.byStage[shortLifecycleLabel(p.LifecycleStage)]++
				return nil
			})
			if err != nil {
				return err
			}

			if len(owners) == 0 {
				empty := map[string]any{
					"weight_by_stage": weightByStage,
					"median_load":     0,
					"items":           []ownerOverloadItem{},
					"owners_scanned":  0,
					"note":            "no contacts had hubspot_owner_id set",
				}
				if parseErrors > 0 {
					empty["parse_errors"] = parseErrors
				}
				return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
			}

			loads := make([]float64, 0, len(owners))
			for _, b := range owners {
				loads = append(loads, b.weightedLoad)
			}
			med := median(loads)

			items := make([]ownerOverloadItem, 0)
			for ownerID, b := range owners {
				if b.weightedLoad <= med {
					continue
				}
				ratio := 0.0
				if med > 0 {
					ratio = b.weightedLoad / med
				}
				items = append(items, ownerOverloadItem{
					OwnerID:       ownerID,
					WeightedLoad:  roundTo(b.weightedLoad, 2),
					ContactCount:  b.contactCount,
					RatioToMedian: roundTo(ratio, 2),
					ByStage:       b.byStage,
				})
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].WeightedLoad != items[j].WeightedLoad {
					return items[i].WeightedLoad > items[j].WeightedLoad
				}
				return items[i].OwnerID < items[j].OwnerID
			})
			items = limitInt(items, top)

			result := map[string]any{
				"weight_by_stage": weightByStage,
				"median_load":     roundTo(med, 2),
				"items":           items,
				"owners_scanned":  len(owners),
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().BoolVar(&weightByStage, "weight-by-stage", true, "Weight each contact by lifecycle stage (lead=1, mql=2, sql=4, opportunity=8, customer=4)")
	cmd.Flags().IntVar(&top, "top", 10, "Maximum number of owners to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}
