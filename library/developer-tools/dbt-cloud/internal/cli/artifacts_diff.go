// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — diff run_results.json between two dbt Cloud runs.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/dbt-cloud/internal/config"

	"github.com/spf13/cobra"
)

// ModelResult holds the data for a single model in run_results.json.
type ModelResult struct {
	UniqueID      string  `json:"unique_id"`
	Status        string  `json:"status"`
	ExecutionTime float64 `json:"execution_time"`
}

// ArtifactsDiffResult is the structured output of the diff.
type ArtifactsDiffResult struct {
	RunA         string        `json:"run_a"`
	RunB         string        `json:"run_b"`
	NewlyFailed  []ModelChange `json:"newly_failed"`
	NewlyPassed  []ModelChange `json:"newly_passed"`
	TimingDeltas []TimingDelta `json:"timing_deltas"`
	Summary      DiffSummary   `json:"summary"`
}

// ModelChange describes a model whose status changed between the two runs.
type ModelChange struct {
	UniqueID string `json:"unique_id"`
	StatusA  string `json:"status_a"`
	StatusB  string `json:"status_b"`
}

// TimingDelta describes a model whose execution time changed significantly.
type TimingDelta struct {
	UniqueID  string  `json:"unique_id"`
	DurationA float64 `json:"duration_a_sec"`
	DurationB float64 `json:"duration_b_sec"`
	DeltaSec  float64 `json:"delta_sec"`
	DeltaPct  float64 `json:"delta_pct"`
}

// DiffSummary provides counts.
type DiffSummary struct {
	TotalModelsA     int `json:"total_models_a"`
	TotalModelsB     int `json:"total_models_b"`
	NewlyFailedCount int `json:"newly_failed_count"`
	NewlyPassedCount int `json:"newly_passed_count"`
	TimingDeltaCount int `json:"timing_delta_count"`
}

// diffIsFailStatus reports whether a dbt model status is a "failure".
func diffIsFailStatus(status string) bool {
	switch status {
	case "error", "fail":
		return true
	}
	return false
}

// diffIsPassStatus reports whether a dbt model status is a "pass/success".
func diffIsPassStatus(status string) bool {
	switch status {
	case "success", "pass":
		return true
	}
	return false
}

// computeArtifactsDiff diffs two maps of unique_id → ModelResult.
// timingThresholdSec: minimum absolute delta to report a timing change.
func computeArtifactsDiff(runA, runB string, modelsA, modelsB map[string]ModelResult, timingThresholdSec float64) ArtifactsDiffResult {
	result := ArtifactsDiffResult{RunA: runA, RunB: runB}
	result.Summary.TotalModelsA = len(modelsA)
	result.Summary.TotalModelsB = len(modelsB)

	// Find all unique IDs across both runs
	seen := map[string]bool{}
	for id := range modelsA {
		seen[id] = true
	}
	for id := range modelsB {
		seen[id] = true
	}

	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		ma, aOK := modelsA[id]
		mb, bOK := modelsB[id]

		// Status change
		if aOK && bOK {
			if diffIsPassStatus(ma.Status) && diffIsFailStatus(mb.Status) {
				result.NewlyFailed = append(result.NewlyFailed, ModelChange{
					UniqueID: id, StatusA: ma.Status, StatusB: mb.Status,
				})
			} else if diffIsFailStatus(ma.Status) && diffIsPassStatus(mb.Status) {
				result.NewlyPassed = append(result.NewlyPassed, ModelChange{
					UniqueID: id, StatusA: ma.Status, StatusB: mb.Status,
				})
			}

			// Timing delta
			if ma.ExecutionTime > 0 && mb.ExecutionTime > 0 {
				delta := mb.ExecutionTime - ma.ExecutionTime
				absDelta := math.Abs(delta)
				if absDelta >= timingThresholdSec {
					pct := 0.0
					if ma.ExecutionTime > 0 {
						pct = math.Round(delta/ma.ExecutionTime*10000) / 100
					}
					result.TimingDeltas = append(result.TimingDeltas, TimingDelta{
						UniqueID:  id,
						DurationA: math.Round(ma.ExecutionTime*100) / 100,
						DurationB: math.Round(mb.ExecutionTime*100) / 100,
						DeltaSec:  math.Round(delta*100) / 100,
						DeltaPct:  pct,
					})
				}
			}
		} else if !aOK && bOK && diffIsFailStatus(mb.Status) {
			// New model that immediately failed
			result.NewlyFailed = append(result.NewlyFailed, ModelChange{
				UniqueID: id, StatusA: "(not in run A)", StatusB: mb.Status,
			})
		}
	}

	// Sort timing deltas by abs delta descending
	sort.Slice(result.TimingDeltas, func(i, j int) bool {
		return math.Abs(result.TimingDeltas[i].DeltaSec) > math.Abs(result.TimingDeltas[j].DeltaSec)
	})

	result.Summary.NewlyFailedCount = len(result.NewlyFailed)
	result.Summary.NewlyPassedCount = len(result.NewlyPassed)
	result.Summary.TimingDeltaCount = len(result.TimingDeltas)
	return result
}

// pp:data-source live
func newNovelArtifactsDiffCmd(flags *rootFlags) *cobra.Command {
	var flagArtifact string
	var flagAccountID string
	var flagTimingThreshold float64

	cmd := &cobra.Command{
		Use:   "diff <run_a> <run_b>",
		Short: "Diff run_results.json between two runs to show which models newly failed or changed timing.",
		Long: `Fetch run_results.json for two dbt Cloud runs and diff them.

Reports:
  - Models that newly failed (passed/success in run A, error/fail in run B)
  - Models that newly passed (error/fail in run A, passed/success in run B)
  - Models with a large execution time change (> --timing-threshold seconds)

This command is read-only — it fetches artifact data from the live API.`,
		Example: `  dbt-cloud-pp-cli artifacts diff 12345 12346
  dbt-cloud-pp-cli artifacts diff 12345 12346 --json
  dbt-cloud-pp-cli artifacts diff 12345 12346 --timing-threshold 30`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				runA, runB := "<run_a>", "<run_b>"
				if len(args) > 0 {
					runA = args[0]
				}
				if len(args) > 1 {
					runB = args[1]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would diff %s vs %s artifact %s\n", runA, runB, flagArtifact)
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("two run IDs are required\nUsage: %s", cmd.UseLine()))
			}
			runA, runB := args[0], args[1]

			accountID := config.AccountID(flagAccountID)
			if accountID == "" {
				return usageErr(fmt.Errorf("account_id is required: pass --account-id or set DBT_CLOUD_ACCOUNT_ID"))
			}

			// Verify env: print intent, no network
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would diff runs %s and %s\n", runA, runB)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Helper to fetch artifact for a run
			fetchArtifact := func(runID string) (map[string]ModelResult, error) {
				path := fmt.Sprintf("/api/v2/accounts/%s/runs/%s/artifacts/%s", accountID, runID, flagArtifact)
				// dbt Cloud artifact endpoints return JSON but may reject the default
				// Accept: application/json header (some return 406). Use */* to match
				// whatever content type the server returns.
				raw, err := c.GetWithHeaders(ctx, path, nil, map[string]string{
					"Accept": "*/*",
				})
				if err != nil {
					return nil, fmt.Errorf("fetching artifact for run %s: %w", runID, err)
				}
				// run_results.json has a top-level "results" array
				var runResults struct {
					Results []struct {
						UniqueID      string  `json:"unique_id"`
						Status        string  `json:"status"`
						ExecutionTime float64 `json:"execution_time"`
					} `json:"results"`
				}
				if err := json.Unmarshal(raw, &runResults); err != nil {
					return nil, fmt.Errorf("parsing run_results.json for run %s: %w", runID, err)
				}
				m := make(map[string]ModelResult, len(runResults.Results))
				for _, r := range runResults.Results {
					m[r.UniqueID] = ModelResult{
						UniqueID:      r.UniqueID,
						Status:        r.Status,
						ExecutionTime: r.ExecutionTime,
					}
				}
				return m, nil
			}

			modelsA, err := fetchArtifact(runA)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			modelsB, err := fetchArtifact(runB)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			diff := computeArtifactsDiff(runA, runB, modelsA, modelsB, flagTimingThreshold)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), diff, flags)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Artifact diff: run %s → run %s (%s)\n\n", runA, runB, flagArtifact)
			fmt.Fprintf(cmd.OutOrStdout(), "Models in A: %d  Models in B: %d\n\n", diff.Summary.TotalModelsA, diff.Summary.TotalModelsB)

			if len(diff.NewlyFailed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Newly FAILED (%d):\n", len(diff.NewlyFailed))
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintf(tw, "  MODEL\tSTATUS_A\tSTATUS_B\n")
				for _, m := range diff.NewlyFailed {
					fmt.Fprintf(tw, "  %s\t%s\t%s\n", m.UniqueID, m.StatusA, m.StatusB)
				}
				tw.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "No newly failed models.\n\n")
			}

			if len(diff.NewlyPassed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Newly PASSED (%d):\n", len(diff.NewlyPassed))
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintf(tw, "  MODEL\tSTATUS_A\tSTATUS_B\n")
				for _, m := range diff.NewlyPassed {
					fmt.Fprintf(tw, "  %s\t%s\t%s\n", m.UniqueID, m.StatusA, m.StatusB)
				}
				tw.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(diff.TimingDeltas) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Large timing deltas (%d, threshold %.0fs):\n", len(diff.TimingDeltas), flagTimingThreshold)
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintf(tw, "  MODEL\tA_SEC\tB_SEC\tDELTA_SEC\tDELTA_PCT\n")
				for _, d := range diff.TimingDeltas {
					fmt.Fprintf(tw, "  %s\t%.1f\t%.1f\t%+.1f\t%+.1f%%\n",
						d.UniqueID, d.DurationA, d.DurationB, d.DeltaSec, d.DeltaPct)
				}
				tw.Flush()
				fmt.Fprintln(cmd.OutOrStdout())
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&flagArtifact, "artifact", "run_results.json", "Artifact file to diff (default: run_results.json)")
	cmd.Flags().StringVar(&flagAccountID, "account-id", "", "dbt Cloud account ID (default: DBT_CLOUD_ACCOUNT_ID env var)")
	cmd.Flags().Float64Var(&flagTimingThreshold, "timing-threshold", 10.0, "Minimum execution time delta in seconds to report (default 10)")
	return cmd
}
