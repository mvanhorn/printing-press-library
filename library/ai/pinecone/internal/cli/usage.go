// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// usage: compute vector-count growth and per-namespace distribution shifts
// from snapshot history, with a projection to a future horizon.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type usageRow struct {
	CapturedAt   string `json:"captured_at"`
	TotalVectors int64  `json:"total_vectors"`
	Note         string `json:"note"`
}

type usageEnvelope struct {
	Index           string           `json:"index"`
	Since           string           `json:"since"`
	Snapshots       []usageRow       `json:"snapshots"`
	GrowthPerDay    float64          `json:"growth_per_day"`
	Projected30d    int64            `json:"projected_vectors_30d"`
	NamespaceShifts map[string]shift `json:"namespace_shifts,omitempty"`
}

type shift struct {
	First int64 `json:"first"`
	Last  int64 `json:"last"`
	Delta int64 `json:"delta"`
}

func newNovelUsageCmd(flags *rootFlags) *cobra.Command {
	var indexName string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Compute vector-count growth and namespace shifts from snapshot history",
		Long: `Compute growth and usage deltas from snapshot history.

Use this command for growth and usage deltas computed from snapshot history.
Do NOT use this command to capture a new point-in-time snapshot; use 'snapshot'.`,
		Example:     `  pinecone-pp-cli usage --index travel-chat-embeddings --since 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "usage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if indexName == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--index is required"))
			}
			resolvedDB, err := defaultNovelDB(dbPath)
			if err != nil {
				return err
			}
			if missingMirrorHint(cmd.ErrOrStderr(), resolvedDB) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), usageEnvelope{Index: indexName, Snapshots: []usageRow{}}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No local snapshot data; run 'pinecone-pp-cli snapshot <index>' first.")
				return nil
			}
			s, db, err := openNovelDB(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			sinceTime := time.Now().Add(-30 * 24 * time.Hour)
			if since != "" {
				if d, err := time.ParseDuration(since); err == nil {
					sinceTime = time.Now().Add(-d)
				}
			}
			rows, err := db.QueryContext(ctx,
				`SELECT captured_at, total_vectors, note, data FROM pp_snapshots
				 WHERE index_name = ? AND captured_at >= ? ORDER BY captured_at ASC`,
				indexName, sinceTime.UTC().Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying snapshots: %w", err)
			}
			var snapshots []usageRow = make([]usageRow, 0)
			var nsData []map[string]int64 = make([]map[string]int64, 0)
			for rows.Next() {
				var captured, note, data string
				var total int64
				if err := rows.Scan(&captured, &total, &note, &data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning snapshot: %w", err)
				}
				snapshots = append(snapshots, usageRow{CapturedAt: captured, TotalVectors: total, Note: note})
				var env struct {
					Namespaces map[string]int64 `json:"namespaces"`
				}
				_ = json.Unmarshal([]byte(data), &env)
				nsData = append(nsData, env.Namespaces)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating snapshots: %w", err)
			}
			_ = rows.Close()

			var growthPerDay float64
			if len(snapshots) >= 2 {
				first, _ := time.Parse(time.RFC3339, snapshots[0].CapturedAt)
				last, _ := time.Parse(time.RFC3339, snapshots[len(snapshots)-1].CapturedAt)
				days := last.Sub(first).Hours() / 24
				if days > 0 {
					growthPerDay = float64(snapshots[len(snapshots)-1].TotalVectors-snapshots[0].TotalVectors) / days
				}
			}
			var projected int64
			if len(snapshots) > 0 {
				projected = snapshots[len(snapshots)-1].TotalVectors + int64(growthPerDay*30)
			}
			shifts := map[string]shift{}
			if len(nsData) >= 2 {
				for name, count := range nsData[len(nsData)-1] {
					first := nsData[0][name]
					shifts[name] = shift{First: first, Last: count, Delta: count - first}
				}
				for name, count := range nsData[0] {
					if _, ok := nsData[len(nsData)-1][name]; !ok {
						shifts[name] = shift{First: count, Last: 0, Delta: -count}
					}
				}
			}
			env := usageEnvelope{
				Index:           indexName,
				Since:           sinceTime.UTC().Format(time.RFC3339),
				Snapshots:       snapshots,
				GrowthPerDay:    growthPerDay,
				Projected30d:    projected,
				NamespaceShifts: shifts,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}
			if len(snapshots) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No snapshots for %s since %s. Run: pinecone-pp-cli snapshot %s\n", indexName, env.Since, indexName)
				return nil
			}
			last := snapshots[len(snapshots)-1]
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %d vectors (last snapshot %s)\n", indexName, last.TotalVectors, last.CapturedAt)
			fmt.Fprintf(cmd.OutOrStdout(), "growth: %.1f vectors/day; projected 30d: %d\n", growthPerDay, projected)
			for name, sh := range shifts {
				fmt.Fprintf(cmd.OutOrStdout(), "  namespace %-24s %d -> %d (delta %d)\n", name, sh.First, sh.Last, sh.Delta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&indexName, "index", "", "Index name to analyze")
	cmd.Flags().StringVar(&since, "since", "30d", "Window to analyze (Go duration: 7d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: platform data dir)")
	return cmd
}
