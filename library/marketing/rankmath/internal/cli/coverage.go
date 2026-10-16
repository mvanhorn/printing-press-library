// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type inspectionRow struct {
	Page          string `json:"page"`
	Verdict       string `json:"index_verdict"`
	CoverageState string `json:"coverage_state"`
	Robots        string `json:"robots_txt_state"`
}
type coverageGap struct {
	URL           string `json:"url"`
	PostID        int    `json:"post_id"`
	Verdict       string `json:"verdict"`
	CoverageState string `json:"coverage_state"`
	Robots        string `json:"robots"`
	Status        string `json:"status"`
}
type coverageResult struct {
	Published    int            `json:"published"`
	Inspected    int            `json:"inspected"`
	ScannedPages int            `json:"scanned_pages"`
	ScanComplete bool           `json:"scan_complete"`
	Note         string         `json:"note,omitempty"`
	Gaps         []coverageGap  `json:"gaps"`
	Summary      map[string]int `json:"summary"`
}

func decodeInspectionRows(raw json.RawMessage) ([]inspectionRow, error) {
	rows := make([]inspectionRow, 0)
	if isNoDataResponse(raw) {
		return rows, nil
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decoding inspection rows: %w", err)
	}
	return rows, nil
}

func classifyCoverage(inv []publishedInventoryItem, rows []inspectionRow, noindex map[string]bool, scanComplete bool) coverageResult {
	result := emptyCoverageResult()
	byPage := make(map[string]inspectionRow, len(rows))
	byPath := make(map[string]inspectionRow)
	for _, row := range rows {
		host, path, absolute := normalizePagePath(row.Page)
		if absolute {
			byPage[host+"|"+path] = row
		} else {
			byPath[path] = row
		}
	}
	noindexPages := make(map[string]bool, len(noindex))
	noindexPaths := make(map[string]bool)
	for page, enabled := range noindex {
		if !enabled {
			continue
		}
		host, path, absolute := normalizePagePath(page)
		if absolute {
			noindexPages[host+"|"+path] = true
		} else {
			noindexPaths[path] = true
		}
	}
	for _, item := range inv {
		result.Published++
		host, path, _ := normalizePagePath(item.URL)
		key := host + "|" + path
		r, ok := byPage[key]
		if !ok {
			r, ok = byPath[path]
		}
		status := "never_inspected"
		if ok {
			result.Inspected++
			status = "not_indexed"
			if isIndexed(r.Verdict, r.CoverageState) {
				status = "indexed"
			}
		} else if !scanComplete {
			status = "unknown"
		}
		if noindexPages[key] || noindexPaths[path] {
			status = "noindex_intentional"
		}
		result.Summary[status]++
		if status != "indexed" && status != "unknown" {
			result.Gaps = append(result.Gaps, coverageGap{URL: item.URL, PostID: item.PostID, Verdict: r.Verdict, CoverageState: r.CoverageState, Robots: r.Robots, Status: status})
		}
	}
	return result
}

func emptyCoverageResult() coverageResult {
	return coverageResult{Gaps: make([]coverageGap, 0), Summary: map[string]int{"indexed": 0, "not_indexed": 0, "never_inspected": 0, "noindex_intentional": 0, "unknown": 0}}
}

func isIndexed(verdict, state string) bool {
	state = strings.ToLower(state)
	return verdict == "PASS" || (verdict != "FAIL" && verdict != "NEUTRAL" && strings.Contains(state, "indexed") && !strings.Contains(state, "not indexed") && !strings.Contains(state, "not currently indexed"))
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var dbPath, status string
	var maxPages, limit int
	var includeNoindex bool
	cmd := &cobra.Command{
		Use:         "coverage",
		Short:       "Find published posts Google has not indexed",
		Long:        "Use this command to find published posts Google has not indexed. Do NOT use this command to push recently edited URLs; use 'reindex' instead.",
		Example:     "  rankmath-pp-cli coverage --status never_inspected --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			if maxPages < 1 || limit < 0 {
				return usageErr(fmt.Errorf("--max-scan-pages must be positive and --limit nonnegative"))
			}
			if status != "all" && status != "not_indexed" && status != "never_inspected" {
				return usageErr(fmt.Errorf("--status must be all, not_indexed, or never_inspected"))
			}
			result := emptyCoverageResult()
			emit := func() error {
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					if !result.ScanComplete {
						fmt.Fprintf(cmd.OutOrStdout(), "Partial scan: %d URL(s) unknown. %s\n", result.Summary["unknown"], result.Note)
					}
					if len(result.Gaps) == 0 && result.ScanComplete {
						fmt.Fprintln(cmd.OutOrStdout(), "No coverage gaps found.")
					} else if len(result.Gaps) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No coverage gaps found among inspected URLs.")
					}
					for _, g := range result.Gaps {
						fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", g.Status, g.URL)
					}
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			db, found, err := openMirrorOrHint(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !found {
				return emit()
			}
			defer db.Close()
			if err := db.EnsureSEOSnapshots(cmd.Context()); err != nil {
				return err
			}
			inventory, err := loadPublishedInventory(db)
			if err != nil {
				return err
			}
			inv := inventory
			snapshots, err := db.LatestSEOSnapshots(cmd.Context())
			if err != nil {
				return err
			}
			noindex := map[string]bool{}
			for _, s := range snapshots {
				noindex[s.URL] = s.Noindex
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			maxPages = scanCap(maxPages, 1)
			rows := make([]inspectionRow, 0)
			scanned := 0
			scanComplete := false
			for page := 1; page <= maxPages; page++ {
				raw, err := c.Get(ctx, "/rankmath/v1/an/inspectionResults", map[string]string{"page": strconv.Itoa(page)})
				if err != nil {
					return err
				}
				var response struct {
					Rows      json.RawMessage `json:"rows"`
					RowsFound int             `json:"rowsFound"`
				}
				if err = json.Unmarshal(raw, &response); err != nil {
					return err
				}
				batch, err := decodeInspectionRows(response.Rows)
				if err != nil {
					return err
				}
				scanned++
				rows = append(rows, batch...)
				if len(batch) < 25 || (response.RowsFound > 0 && len(rows) >= response.RowsFound) {
					scanComplete = true
					break
				}
			}
			result = classifyCoverage(inv, rows, noindex, scanComplete)
			result.ScannedPages = scanned
			result.ScanComplete = scanComplete
			if !scanComplete {
				result.Note = "Inspection scan reached --max-scan-pages; raise --max-scan-pages to classify URLs with no matching inspection row."
			}
			filtered := make([]coverageGap, 0)
			for _, g := range result.Gaps {
				if g.Status == "noindex_intentional" && !includeNoindex {
					continue
				}
				if status != "all" && status != g.Status {
					continue
				}
				if limit > 0 && len(filtered) >= limit {
					break
				}
				filtered = append(filtered, g)
			}
			result.Gaps = filtered
			return emit()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror database")
	cmd.Flags().StringVar(&status, "status", "all", "Gap status: all, not_indexed, never_inspected")
	cmd.Flags().BoolVar(&includeNoindex, "include-noindex", false, "Include intentional noindex URLs")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 20, "Maximum inspection pages")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum gaps (0 means all)")
	return cmd
}
