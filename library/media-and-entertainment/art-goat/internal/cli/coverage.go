// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var topSources int

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report your practice's coverage breadth across sources, mediums, regions, and periods",
		Long: `Coverage is the inverse of repetition. Where 'today' protects you from
sitting with the same piece twice, coverage zooms out and surfaces *what
you haven't sat with yet* relative to the corpus.

For each dimension art-goat tracks — source, culture region, medium —
coverage reports the number of distinct values you've actually sat with
and the number available in the local corpus, then computes a percentage.
Low coverage in a dimension means there's still a lot of unexplored
territory there; high coverage means the practice has wandered widely on
that axis.

This is a corpus-relative measurement; it grows when you sync more works
and when you sit with new ones. Use it alongside 'gaps' (which points to
specific overlooked regions and mediums) to steer your next sit.`,
		Example: `  art-goat-pp-cli coverage
  art-goat-pp-cli coverage --json
  art-goat-pp-cli coverage --json --select sources_sat,sources_total`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitCoverageVerifyEnvelope(cmd)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			report, err := collectCoverage(cmd, db)
			if err != nil {
				return err
			}
			if topSources > 0 && len(report.WorksPerSource) > topSources {
				// Cap to the top N sources by work count, descending.
				type kv struct {
					name  string
					count int
				}
				pairs := make([]kv, 0, len(report.WorksPerSource))
				for k, v := range report.WorksPerSource {
					pairs = append(pairs, kv{k, v})
				}
				sort.Slice(pairs, func(i, j int) bool {
					if pairs[i].count != pairs[j].count {
						return pairs[i].count > pairs[j].count
					}
					return pairs[i].name < pairs[j].name
				})
				capped := make(map[string]int, topSources)
				for i := 0; i < topSources && i < len(pairs); i++ {
					capped[pairs[i].name] = pairs[i].count
				}
				report.WorksPerSource = capped
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			renderCoverage(cmd, flags, report)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().IntVar(&topSources, "top", 0, "Cap the per-source breakdown to the top N sources by work count (0 = no cap)")
	if cliutil.IsStrictFlagsEnv() {
		// Audit mode forces an explicit cap so a downstream consumer can
		// reason about response size before parsing.
		_ = cmd.MarkFlagRequired("top")
	}
	return cmd
}

// coverageReport is the JSON envelope returned by `coverage`. Keys are
// agent-friendly snake_case; per-source counts are included so callers
// can rank or compare without re-running SQL.
type coverageReport struct {
	SourcesSat     int            `json:"sources_sat"`
	SourcesTotal   int            `json:"sources_total"`
	SourcesPercent float64        `json:"sources_percent"`
	RegionsSat     int            `json:"regions_sat"`
	RegionsTotal   int            `json:"regions_total"`
	RegionsPercent float64        `json:"regions_percent"`
	MediumsSat     int            `json:"mediums_sat"`
	MediumsTotal   int            `json:"mediums_total"`
	MediumsPercent float64        `json:"mediums_percent"`
	WorksPerSource map[string]int `json:"works_per_source"`
	SitsTotal      int            `json:"sits_total"`
}

func collectCoverage(cmd *cobra.Command, db *store.Store) (*coverageReport, error) {
	ctx := cmd.Context()
	rep := &coverageReport{
		WorksPerSource: map[string]int{},
	}

	// SQL aggregation: works per source (corpus shape, COUNT + GROUP BY).
	rows, err := db.DB().QueryContext(ctx, `SELECT source, COUNT(*) FROM works GROUP BY source`)
	if err != nil {
		return nil, fmt.Errorf("coverage: works per source: %w", err)
	}
	sourcesAvail := map[string]bool{}
	for rows.Next() {
		var src string
		var n int
		if err := rows.Scan(&src, &n); err != nil {
			rows.Close()
			return nil, err
		}
		rep.WorksPerSource[src] = n
		sourcesAvail[src] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rep.SourcesTotal = len(sourcesAvail)

	// Sources actually sat with.
	rep.SourcesSat, err = countDistinct(ctx, db, `
SELECT COUNT(DISTINCT w.source)
FROM sits s JOIN works w ON w.id = s.work_id
WHERE w.source != ''`)
	if err != nil {
		return nil, err
	}

	// Available regions (non-empty).
	rep.RegionsTotal, err = countDistinct(ctx, db, `SELECT COUNT(DISTINCT culture_region) FROM works WHERE culture_region != ''`)
	if err != nil {
		return nil, err
	}
	rep.RegionsSat, err = countDistinct(ctx, db, `
SELECT COUNT(DISTINCT w.culture_region)
FROM sits s JOIN works w ON w.id = s.work_id
WHERE w.culture_region != ''`)
	if err != nil {
		return nil, err
	}

	// Available mediums (non-empty).
	rep.MediumsTotal, err = countDistinct(ctx, db, `SELECT COUNT(DISTINCT medium) FROM works WHERE medium != ''`)
	if err != nil {
		return nil, err
	}
	rep.MediumsSat, err = countDistinct(ctx, db, `
SELECT COUNT(DISTINCT w.medium)
FROM sits s JOIN works w ON w.id = s.work_id
WHERE w.medium != ''`)
	if err != nil {
		return nil, err
	}

	// Total sits, for context.
	rep.SitsTotal, err = countDistinct(ctx, db, `SELECT COUNT(*) FROM sits`)
	if err != nil {
		return nil, err
	}

	// Go-level aggregation: percentage per dimension.
	rep.SourcesPercent = pct(rep.SourcesSat, rep.SourcesTotal)
	rep.RegionsPercent = pct(rep.RegionsSat, rep.RegionsTotal)
	rep.MediumsPercent = pct(rep.MediumsSat, rep.MediumsTotal)

	return rep, nil
}

func countDistinct(ctx context.Context, db *store.Store, q string) (int, error) {
	var n int
	row := db.DB().QueryRowContext(ctx, q)
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

func pct(part, total int) float64 {
	if total <= 0 {
		return 0.0
	}
	return float64(part) / float64(total) * 100
}

func renderCoverage(cmd *cobra.Command, flags *rootFlags, rep *coverageReport) {
	headers := []string{"Dimension", "Visited", "Available", "Coverage %"}
	rows := [][]string{
		{"sources", fmt.Sprintf("%d", rep.SourcesSat), fmt.Sprintf("%d", rep.SourcesTotal), fmt.Sprintf("%.1f%%", rep.SourcesPercent)},
		{"regions", fmt.Sprintf("%d", rep.RegionsSat), fmt.Sprintf("%d", rep.RegionsTotal), fmt.Sprintf("%.1f%%", rep.RegionsPercent)},
		{"mediums", fmt.Sprintf("%d", rep.MediumsSat), fmt.Sprintf("%d", rep.MediumsTotal), fmt.Sprintf("%.1f%%", rep.MediumsPercent)},
	}
	_ = flags.printTable(cmd, headers, rows)

	// Per-source breakdown, sorted by count desc for a stable, readable list.
	if len(rep.WorksPerSource) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Works in corpus per source:")
		type kv struct {
			k string
			v int
		}
		var pairs []kv
		for k, v := range rep.WorksPerSource {
			pairs = append(pairs, kv{k, v})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].v != pairs[j].v {
				return pairs[i].v > pairs[j].v
			}
			return pairs[i].k < pairs[j].k
		})
		for _, p := range pairs {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %d\n", p.k, p.v)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal sits: %d\n", rep.SitsTotal)
}

func emitCoverageVerifyEnvelope(cmd *cobra.Command) error {
	envelope := map[string]any{
		"command":                 "coverage",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "coverage renders a table by default; PRINTING_PRESS_VERIFY=1 short-circuits the rendering. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
