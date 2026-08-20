package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/ollama-cloud/internal/advisor"
	"github.com/spf13/cobra"
)

type replayRow struct {
	AdvisedAt      time.Time `json:"advised_at"`
	PromptHash     string    `json:"prompt_hash"`
	TaskHint       string    `json:"task_hint,omitempty"`
	Recommended    string    `json:"recommended"`
	ActualChosen   string    `json:"actual_chosen,omitempty"`
	DivergenceFlag bool      `json:"divergence,omitempty"`
}

func newAdviseReplayCmd(flags *rootFlags) *cobra.Command {
	var (
		logPath     string
		since       string
		limit       int
		divergeOnly bool
		dryRun      bool
	)
	cmd := &cobra.Command{
		Use:   "advise-replay",
		Short: "Replay advisor recommendations and report divergence vs actual-chosen models",
		Long: strings.TrimSpace(`
Reads the advisor JSONL log and surfaces every row's recommended model.
Divergence between recommended and actual_chosen (when populated by a
downstream consumer reporting back) indicates the advisor needs recalibration.

Each row's prompt is NOT stored in the log (privacy + atomic-append limits).
That means this command CANNOT score response quality against a judge LLM —
it can only report which prompts diverged from the recommendation. A
prompt-corpus sidecar that would unlock judge-LLM scoring is a future
addition; until then this command is divergence-reporting only.
`),
		Example: strings.Trim(`
  ollama-cloud-pp-cli advise-replay --since 7d
  ollama-cloud-pp-cli advise-replay --since 30d --diverge-only
  ollama-cloud-pp-cli advise-replay --since 7d --dry-run
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := logPath
			if path == "" {
				path = advisor.DefaultLogPath()
			}
			cutoff, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			rows, err := readAdvisorLog(path, cutoff)
			if err != nil {
				return apiErr(err)
			}

			// Tally divergence across the whole window first so the rate is
			// meaningful regardless of --diverge-only (which filters the displayed
			// rows) or --limit (which truncates them). Counting only the emitted
			// rows made divergence_pct trivially 100% under --diverge-only.
			divergeCount := 0
			for _, e := range rows {
				if e.ActualChosen != "" && e.ActualChosen != e.Recommended {
					divergeCount++
				}
			}

			out := make([]replayRow, 0, len(rows))
			for _, e := range rows {
				diverged := e.ActualChosen != "" && e.ActualChosen != e.Recommended
				if divergeOnly && !diverged {
					continue
				}
				out = append(out, replayRow{
					AdvisedAt:      e.AdvisedAt,
					PromptHash:     e.PromptHash,
					TaskHint:       e.TaskHint,
					Recommended:    e.Recommended,
					ActualChosen:   e.ActualChosen,
					DivergenceFlag: diverged,
				})
				if limit > 0 && len(out) >= limit {
					break
				}
			}

			envelope := map[string]any{
				"log_path":         path,
				"since":            since,
				"total_rows":       len(rows),
				"emitted":          len(out),
				"divergence_count": divergeCount,
				"divergence_pct":   percent(divergeCount, len(rows)),
				"dry_run":          dryRun,
				"rows":             out,
				"computed_at":      time.Now().UTC(),
			}
			b, _ := json.MarshalIndent(envelope, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&logPath, "log", "", "Override advisor log path")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: 7d, 24h, 1h, all")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap result rows (0 = no cap)")
	cmd.Flags().BoolVar(&divergeOnly, "diverge-only", false, "Only emit rows where actual_chosen differs from recommended")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Reserved for future judge-corpus path; currently a no-op on the divergence-reporting path")
	return cmd
}

func percent(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n*100) / float64(d)
}
