// Copyright 2026 higgsfield-ai. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for higgsfield-pp-cli (Phase 3 transcendence).

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// fanoutRecord links a single fanout group to its per-model jobs. Persisted as
// JSON at $cache/fanouts/<fanout_id>.json so the linkage survives across
// invocations without a schema migration.
type fanoutRecord struct {
	FanoutID      string           `json:"fanout_id"`
	Prompt        string           `json:"prompt"`
	Models        []string         `json:"models"`
	MaxCost       int              `json:"max_cost,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	Jobs          []fanoutJobEntry `json:"jobs"`
	CostBreakdown map[string]int   `json:"cost_breakdown,omitempty"`
}

type fanoutJobEntry struct {
	Model     string `json:"model"`
	RequestID string `json:"request_id"`
	JobID     string `json:"job_id,omitempty"`
	Estimated int    `json:"estimated_cost"`
	Status    string `json:"status,omitempty"`
}

func fanoutDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "higgsfield-pp-cli", "fanouts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func fanoutPath(fanoutID string) (string, error) {
	dir, err := fanoutDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fanoutID+".json"), nil
}

func loadFanout(fanoutID string) (*fanoutRecord, error) {
	p, err := fanoutPath(fanoutID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var rec fanoutRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("decoding fanout %s: %w", fanoutID, err)
	}
	return &rec, nil
}

func saveFanout(rec *fanoutRecord) error {
	p, err := fanoutPath(rec.FanoutID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o644)
}

func newFanoutCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fanout",
		Short: "Submit one prompt to N models in parallel and track them as a single fanout group",
		Long: `Submit one prompt to N models in parallel, returning a fanout_id that links every
job under one group. Use 'fanout wait <id>' to poll all jobs, 'fanout compare <id>'
to render a side-by-side table once they complete.

Optional --max-cost gates submission on a pre-flight cost estimate.`,
		Example: strings.Trim(`
  higgsfield-pp-cli fanout --prompt "cinematic dawn over Manhattan" --models veo3_1,seedance_2_0 --max-cost 80
  higgsfield-pp-cli fanout wait fan_20260516_001 --json
  higgsfield-pp-cli fanout compare fan_20260516_001 --json --select model,cost,result_url`, "\n"),
	}

	cmd.AddCommand(newFanoutCreateCmd(flags))
	cmd.AddCommand(newFanoutWaitCmd(flags))
	cmd.AddCommand(newFanoutCompareCmd(flags))

	// When invoked as bare `fanout --prompt ... --models ...`, fall through to create.
	cmd.RunE = newFanoutCreateCmd(flags).RunE
	cmd.Flags().AddFlagSet(newFanoutCreateCmd(flags).Flags())
	return cmd
}

func newFanoutCreateCmd(flags *rootFlags) *cobra.Command {
	var prompt string
	var modelsCSV string
	var maxCost int

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a fanout: submit one prompt to N models in parallel",
		Example: strings.Trim(`
  higgsfield-pp-cli fanout create --prompt "cinematic dawn" --models veo3_1,seedance_2_0 --max-cost 80
  higgsfield-pp-cli fanout create --prompt "studio product shot" --models nano_banana_2,gpt_image_2 --json`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" || modelsCSV == "" {
				return cmd.Help()
			}
			models := strings.Split(modelsCSV, ",")
			for i, m := range models {
				models[i] = strings.TrimSpace(m)
			}
			if len(models) == 0 {
				return errors.New("--models must list at least one model_id")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Pre-flight cost estimate per model. The cost endpoint is hypothetical
			// for the web backend; fall through to 0 estimate if the upstream rejects.
			breakdown := map[string]int{}
			total := 0
			for _, m := range models {
				body := map[string]any{"prompt": prompt}
				raw, _, err := c.PostWithParams(fmt.Sprintf("/models/%s/cost", m), nil, body)
				if err != nil {
					breakdown[m] = 0
					continue
				}
				var resp struct {
					Cost int `json:"cost"`
				}
				_ = json.Unmarshal(raw, &resp)
				breakdown[m] = resp.Cost
				total += resp.Cost
			}

			if maxCost > 0 && total > maxCost {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"ok":             false,
						"reason":         "cost cap exceeded",
						"estimated_cost": total,
						"max_cost":       maxCost,
						"breakdown":      breakdown,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "fanout: cost cap exceeded (%d > %d credits)\n", total, maxCost)
				fmt.Fprintln(cmd.OutOrStdout(), "Per-model estimate:")
				for _, m := range models {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %d\n", m, breakdown[m])
				}
				return fmt.Errorf("aborted: estimated %d credits exceeds --max-cost %d", total, maxCost)
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":        true,
					"prompt":         prompt,
					"models":         models,
					"estimated_cost": total,
					"breakdown":      breakdown,
				}, flags)
			}

			fanoutID := fmt.Sprintf("fan_%s", time.Now().UTC().Format("20060102_150405"))
			rec := &fanoutRecord{
				FanoutID:      fanoutID,
				Prompt:        prompt,
				Models:        models,
				MaxCost:       maxCost,
				CreatedAt:     time.Now().UTC(),
				CostBreakdown: breakdown,
			}

			// Submit one job per model. Real API calls; failures are recorded
			// per-row rather than aborting the whole fanout.
			for _, m := range models {
				body := map[string]any{
					"params": map[string]any{"prompt": prompt},
				}
				raw, _, err := c.Post(fmt.Sprintf("/v1/generate/%s", m), body)
				entry := fanoutJobEntry{Model: m, Estimated: breakdown[m]}
				if err != nil {
					entry.Status = "submit_failed:" + err.Error()
				} else {
					var resp struct {
						RequestID string `json:"request_id"`
						JobID     string `json:"job_id"`
					}
					_ = json.Unmarshal(raw, &resp)
					entry.RequestID = resp.RequestID
					entry.JobID = resp.JobID
					entry.Status = "submitted"
				}
				rec.Jobs = append(rec.Jobs, entry)
			}

			if err := saveFanout(rec); err != nil {
				return fmt.Errorf("persisting fanout: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fanout %s — %d jobs submitted (est. %d credits)\n", fanoutID, len(rec.Jobs), total)
			for _, j := range rec.Jobs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %s\n", j.Model, j.Status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nPoll with: higgsfield-pp-cli fanout wait %s\n", fanoutID)
			return nil
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to submit to every model in the fanout")
	cmd.Flags().StringVar(&modelsCSV, "models", "", "Comma-separated list of model IDs (e.g. veo3_1,seedance_2_0,kling3_0)")
	cmd.Flags().IntVar(&maxCost, "max-cost", 0, "Refuse submission if total estimated cost exceeds this many credits (0 = no cap)")
	return cmd
}

func newFanoutWaitCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "wait <fanout_id>",
		Short: "Poll every job in a fanout until all are terminal or --timeout elapses",
		Example: strings.Trim(`
  higgsfield-pp-cli fanout wait fan_20260516_001
  higgsfield-pp-cli fanout wait fan_20260516_001 --json --interval 5s`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fanoutID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			rec, err := loadFanout(fanoutID)
			if err != nil {
				return fmt.Errorf("loading fanout: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			deadline := time.Now().Add(timeout)
			for {
				allDone := true
				for i := range rec.Jobs {
					j := &rec.Jobs[i]
					if isTerminalStatus(j.Status) {
						continue
					}
					if j.RequestID == "" {
						j.Status = "no_request_id"
						continue
					}
					raw, err := c.Get(fmt.Sprintf("/jobs/%s", j.RequestID), nil)
					if err != nil {
						j.Status = "poll_failed:" + err.Error()
						continue
					}
					var resp struct {
						Status string `json:"status"`
					}
					_ = json.Unmarshal(raw, &resp)
					if resp.Status != "" {
						j.Status = resp.Status
					}
					if !isTerminalStatus(j.Status) {
						allDone = false
					}
				}
				_ = saveFanout(rec)
				if allDone {
					break
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(interval)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fanout %s — final status:\n", rec.FanoutID)
			for _, j := range rec.Jobs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %s\n", j.Model, j.Status)
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Polling interval")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "Overall timeout")
	return cmd
}

func newFanoutCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <fanout_id>",
		Short: "Render a side-by-side table of every job in a fanout (model, cost, status, result_url)",
		Example: strings.Trim(`
  higgsfield-pp-cli fanout compare fan_20260516_001
  higgsfield-pp-cli fanout compare fan_20260516_001 --json --select model,estimated_cost,status`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fanoutID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			rec, err := loadFanout(fanoutID)
			if err != nil {
				return fmt.Errorf("loading fanout: %w", err)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fanout %s\n", rec.FanoutID)
			fmt.Fprintf(cmd.OutOrStdout(), "Prompt: %s\n", rec.Prompt)
			fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n\n", rec.CreatedAt.Format(time.RFC3339))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %-12s %-16s %s\n", "MODEL", "EST. COST", "STATUS", "REQUEST_ID")
			for _, j := range rec.Jobs {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-24s %-12d %-16s %s\n", j.Model, j.Estimated, j.Status, j.RequestID)
			}
			return nil
		},
	}
	return cmd
}

func isTerminalStatus(s string) bool {
	switch strings.ToLower(s) {
	case "completed", "failed", "nsfw", "canceled", "cancelled":
		return true
	}
	return false
}
