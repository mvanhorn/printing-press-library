package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newLayersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "layers",
		Short: "Three-layer-memory protocol queries (KG / Events / Patterns).",
	}
	cmd.AddCommand(newLayersStatsCmd(flags))
	return cmd
}

func newLayersStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Per-layer counts (Knowledge Graph / Events / Patterns) with type breakdown.",
		Long: "Aggregate the synced notes against the three-layer-memory layer map. Shows\n" +
			"per-layer counts, per-type counts within each layer, average note age,\n" +
			"and notes added in the last 7 / 30 days.",
		Example:     "  obsidian-pp-cli layers stats\n  obsidian-pp-cli layers stats --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			rows, err := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(layer,'(unknown)'), COALESCE(type,'(none)'), COUNT(*),
				        AVG(?-mtime) as avg_age,
				        SUM(CASE WHEN mtime > ?-7*86400 THEN 1 ELSE 0 END),
				        SUM(CASE WHEN mtime > ?-30*86400 THEN 1 ELSE 0 END)
				 FROM notes GROUP BY layer, type ORDER BY layer, type`,
				time.Now().Unix(), time.Now().Unix(), time.Now().Unix())
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Layer      string  `json:"layer"`
				Type       string  `json:"type"`
				Count      int     `json:"count"`
				AvgAgeSecs float64 `json:"avg_age_seconds"`
				Last7Days  int     `json:"last_7_days"`
				Last30Days int     `json:"last_30_days"`
			}
			var out []entry
			layerTotal := map[string]int{}
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Layer, &e.Type, &e.Count, &e.AvgAgeSecs, &e.Last7Days, &e.Last30Days); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
				layerTotal[e.Layer] += e.Count
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"by_layer_and_type": out,
					"layer_totals":      layerTotal,
				})
			}
			currentLayer := ""
			for _, e := range out {
				if e.Layer != currentLayer {
					if currentLayer != "" {
						fmt.Fprintln(cmd.OutOrStdout())
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s (%d notes total)\n", e.Layer, layerTotal[e.Layer])
					currentLayer = e.Layer
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %5d notes  (last 7d: %d  last 30d: %d)\n",
					e.Type, e.Count, e.Last7Days, e.Last30Days)
			}
			return nil
		},
	}
	return cmd
}
