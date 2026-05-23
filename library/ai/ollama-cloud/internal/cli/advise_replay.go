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
	JudgePicked    string    `json:"judge_picked,omitempty"`
	JudgeRationale string    `json:"judge_rationale,omitempty"`
	JudgeError     string    `json:"judge_error,omitempty"`
}

func newAdviseReplayCmd(flags *rootFlags) *cobra.Command {
	var (
		logPath     string
		since       string
		judgeModel  string
		limit       int
		divergeOnly bool
		dryRun      bool
	)
	cmd := &cobra.Command{
		Use:   "advise-replay",
		Short: "Replay advisor recommendations and (optionally) score them with a judge LLM",
		Long: strings.TrimSpace(`
Reads the advisor JSONL log, surfaces every row's recommended model, and
optionally calls a judge LLM (--judge-with) to score whether the picked model
handled the prompt better than the next-best alternative. Powers the
divergence canary: divergence between recommended and actual-chosen indicates
the advisor needs recalibration.

Each row's prompt is NOT stored in the log (privacy + atomic-append limits) —
this command can only score divergence between recommended and actual_chosen
when actual_chosen is present, or annotate rows with the recommendation as-is.
The full judge path requires a separate prompt corpus; reserved for v0.2.
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

			out := make([]replayRow, 0, len(rows))
			divergeCount := 0
			for _, e := range rows {
				if divergeOnly && (e.ActualChosen == "" || e.ActualChosen == e.Recommended) {
					continue
				}
				rr := replayRow{
					AdvisedAt:      e.AdvisedAt,
					PromptHash:     e.PromptHash,
					TaskHint:       e.TaskHint,
					Recommended:    e.Recommended,
					ActualChosen:   e.ActualChosen,
					DivergenceFlag: e.ActualChosen != "" && e.ActualChosen != e.Recommended,
				}
				if rr.DivergenceFlag {
					divergeCount++
				}
				out = append(out, rr)
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
				"divergence_pct":   percent(divergeCount, len(out)),
				"judge_model":      judgeModel,
				"dry_run":          dryRun,
				"rows":             out,
				"computed_at":      time.Now().UTC(),
			}
			if !dryRun && judgeModel != "" {
				envelope["judge_note"] = "judge-LLM scoring requires the original prompt corpus, which advisor-log.jsonl does not retain. Track --judge-with as opt-in for v0.2 once a prompt-corpus sidecar is wired."
			}
			b, _ := json.MarshalIndent(envelope, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			return nil
		},
	}
	cmd.Flags().StringVar(&logPath, "log", "", "Override advisor log path")
	cmd.Flags().StringVar(&since, "since", "7d", "Time window: 7d, 24h, 1h, all")
	cmd.Flags().StringVar(&judgeModel, "judge-with", "", "(opt-in) judge LLM model ID; requires prompt corpus")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap result rows (0 = no cap)")
	cmd.Flags().BoolVar(&divergeOnly, "diverge-only", false, "Only emit rows where actual_chosen differs from recommended")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Skip any judge call; just emit divergence counts")
	return cmd
}

func percent(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n*100) / float64(d)
}
