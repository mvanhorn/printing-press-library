// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

// Conservative heuristic constants used to size credit estimates from local
// dataset/voice metadata. The API does not expose a usage-pricing endpoint
// today; these per-unit rates are derived from published price tiers and
// rounded up so the estimate trends pessimistic rather than optimistic.
const (
	creditsPerFineTuneFile = 100
	creditsPerLocalizeLang = 500
)

// usageEstimate is the JSON shape returned by `usage estimate`.
type usageEstimate struct {
	Operation        string         `json:"operation"`
	Inputs           map[string]any `json:"inputs"`
	EstimatedCredits int            `json:"estimated_credits"`
	Basis            string         `json:"basis"`
}

// newUsageEstimateCmd registers `usage estimate <operation>`.
func newUsageEstimateCmd(flags *rootFlags) *cobra.Command {
	var datasetID string
	var voiceID string
	var languages int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "estimate [operation]",
		Short:       "Estimate credit cost for a planned fine-tune or localize operation",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Estimate credit cost for a planned fine-tune or localize operation
from local catalog metadata (no API call required).

Estimates use conservative per-unit heuristics (rounded up from published
price tiers) until Cartesia exposes a real usage-pricing endpoint:

  fine-tune: 100 credits per dataset file
  localize:  500 credits per target language

The returned 'basis' field documents the rate used so callers can audit
the math.`,
		Example: `  cartesia-pp-cli usage estimate fine-tune --dataset ds_abc --json
  cartesia-pp-cli usage estimate localize --voice voice_xyz --languages 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			op := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}

			switch op {
			case "fine-tune":
				if datasetID == "" {
					return fmt.Errorf("--dataset is required for fine-tune estimate")
				}
				files, err := countDatasetFiles(dbPath, datasetID)
				if err != nil {
					return err
				}
				return flags.printJSON(cmd, usageEstimate{
					Operation:        op,
					Inputs:           map[string]any{"dataset": datasetID, "file_count": files},
					EstimatedCredits: files * creditsPerFineTuneFile,
					Basis:            fmt.Sprintf("%d credits per dataset file (conservative heuristic; see --help)", creditsPerFineTuneFile),
				})
			case "localize":
				if voiceID == "" {
					return fmt.Errorf("--voice is required for localize estimate")
				}
				if languages <= 0 {
					languages = 1
				}
				return flags.printJSON(cmd, usageEstimate{
					Operation:        op,
					Inputs:           map[string]any{"voice": voiceID, "languages": languages},
					EstimatedCredits: languages * creditsPerLocalizeLang,
					Basis:            fmt.Sprintf("%d credits per target language (conservative heuristic; see --help)", creditsPerLocalizeLang),
				})
			default:
				return fmt.Errorf("unknown operation %q: expected fine-tune or localize", op)
			}
		},
	}

	cmd.Flags().StringVar(&datasetID, "dataset", "", "Dataset ID (for fine-tune)")
	cmd.Flags().StringVar(&voiceID, "voice", "", "Voice ID (for localize)")
	cmd.Flags().IntVar(&languages, "languages", 1, "Number of target languages (for localize)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// countDatasetFiles attempts to count file rows for a dataset. Returns 0
// (with no error) when the dataset isn't synced locally — estimate is a
// best-effort heuristic, not a hard gate.
func countDatasetFiles(dbPath, datasetID string) (int, error) {
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return 0, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	var n int
	err = db.DB().QueryRow(`SELECT COUNT(*) FROM files WHERE datasets_id = ?`, datasetID).Scan(&n)
	if err != nil {
		return 0, nil
	}
	return n, nil
}
