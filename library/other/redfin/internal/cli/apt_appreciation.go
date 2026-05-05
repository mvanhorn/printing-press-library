// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

// apprecRow is one ranked region × YoY appreciation record.
type apprecRow struct {
	RegionID   int64   `json:"region_id"`
	RegionType int     `json:"region_type"`
	Label      string  `json:"label,omitempty"`
	StartMonth string  `json:"start_month"`
	EndMonth   string  `json:"end_month"`
	StartValue float64 `json:"start_value"`
	EndValue   float64 `json:"end_value"`
	YoYPct     float64 `json:"yoy_pct"`
}

func newAppreciationCmd(flags *rootFlags) *cobra.Command {
	var parent string
	var period int
	var childIDs string

	cmd := &cobra.Command{
		Use:   "appreciation",
		Short: "Rank child neighborhoods under a metro by YoY median-sale change.",
		Long: `Resolves a parent metro slug, lists its child neighborhoods (from local
cache via past syncs, or from --child-ids fallback), calls aggregate-trends
per child, and emits the ranked YoY appreciation table.`,
		Example: `  redfin-pp-cli appreciation --parent "metro/12345" --period 12 --json
  redfin-pp-cli appreciation --child-ids "30772:6,29470:6" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if parent == "" && childIDs == "" {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/region/<type>/<id>/<months>/aggregate-trends (per child; parent or --child-ids required at runtime)")
					return printJSONFiltered(cmd.OutOrStdout(), []apprecRow{}, flags)
				}
				return usageErr(fmt.Errorf("either --parent or --child-ids required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/region/<type>/<id>/<months>/aggregate-trends (per child)")
				return nil
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			type child struct {
				id    int64
				typ   int
				label string
			}
			var children []child
			switch {
			case childIDs != "":
				for _, c := range strings.Split(childIDs, ",") {
					c = strings.TrimSpace(c)
					if c == "" {
						continue
					}
					id, typ, perr := parseRegionSlug(c)
					if perr != nil {
						return usageErr(perr)
					}
					children = append(children, child{id: id, typ: typ, label: strconv.FormatInt(id, 10)})
				}
			default:
				pid, _, perr := parseRegionSlug(parent)
				if perr != nil {
					return usageErr(perr)
				}
				rows, qerr := s.DB().QueryContext(cmd.Context(),
					`SELECT region_id, region_type, COALESCE(name,'') FROM regions WHERE parent_metro_id = ?`, pid)
				if qerr != nil {
					return qerr
				}
				defer rows.Close()
				for rows.Next() {
					var id int64
					var typ int
					var name string
					if err := rows.Scan(&id, &typ, &name); err != nil {
						return err
					}
					if name == "" {
						name = strconv.FormatInt(id, 10)
					}
					children = append(children, child{id: id, typ: typ, label: name})
				}
				if len(children) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: no cached children for parent; pass --child-ids or sync the metro first")
				}
			}

			out := []apprecRow{}
			for _, ch := range children {
				rows, err := fetchMarketTrends(flags, ch.id, ch.typ, period, ch.label)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: trends %s: %v\n", ch.label, err)
					continue
				}
				row := computeYoY(ch.id, ch.typ, ch.label, rows)
				if row != nil {
					out = append(out, *row)
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].YoYPct > out[j].YoYPct })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "Parent metro slug or id:type")
	cmd.Flags().IntVar(&period, "period", 12, "Window in months")
	cmd.Flags().StringVar(&childIDs, "child-ids", "", "Comma-separated child region ids (fallback when parent cache is empty)")
	return cmd
}

// computeYoY scans a long-format trend table for median_sale rows and returns
// an apprecRow comparing the earliest to the latest month.
func computeYoY(id int64, typ int, label string, rows []redfin.RegionTrendPoint) *apprecRow {
	var first, last redfin.RegionTrendPoint
	hasFirst, hasLast := false, false
	for _, r := range rows {
		if r.Metric != "median_sale" {
			continue
		}
		if !hasFirst || r.Month < first.Month {
			first = r
			hasFirst = true
		}
		if !hasLast || r.Month > last.Month {
			last = r
			hasLast = true
		}
	}
	if !hasFirst || !hasLast || first.Value == 0 {
		return nil
	}
	return &apprecRow{
		RegionID:   id,
		RegionType: typ,
		Label:      label,
		StartMonth: first.Month,
		EndMonth:   last.Month,
		StartValue: first.Value,
		EndValue:   last.Value,
		YoYPct:     (last.Value - first.Value) / first.Value * 100,
	}
}
