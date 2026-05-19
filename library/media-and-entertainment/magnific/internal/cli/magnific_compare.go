package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificCompareCmd(flags *rootFlags) *cobra.Command {
	var modelsCSV string
	var aspect string
	var timeoutStr string
	cmd := &cobra.Command{
		Use:   "compare <prompt>",
		Short: "Fan one prompt to N Magnific models in parallel, return a cost+latency manifest",
		Long: `compare submits the same prompt to each named model concurrently,
records every dispatch in the local magnific_tasks ledger, and (when the
endpoint synchronously returns a task_id) collects the initial response
shape for each. Each row also reports the curated credit cost so an agent
can score winners by cost-per-result.

Models must be in the curated registry — use 'magnific-pp-cli models list'
to see available slugs. The compare command does NOT block waiting for
generation completion; pipe the returned task_ids into 'task wait' if you
need the final outputs.`,
		Example: "  magnific-pp-cli compare \"a cinematic Tokyo street at golden hour\" --models mystic,flux-2-pro,seedream-v4-5 --aspect 16:9 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			prompt := strings.TrimSpace(strings.Join(args, " "))
			if prompt == "" {
				return usageErr(fmt.Errorf("prompt is required"))
			}
			modelsCSV = strings.TrimSpace(modelsCSV)
			if modelsCSV == "" {
				return usageErr(fmt.Errorf("--models is required (comma-separated slugs)"))
			}
			slugs := splitTrim(modelsCSV)
			if len(slugs) < 2 {
				return usageErr(fmt.Errorf("--models needs at least 2 slugs for a bake-off"))
			}
			for _, s := range slugs {
				if lookupModel(s) == nil {
					return notFoundErr(fmt.Errorf("model %q not in curated registry (run `magnific-pp-cli models list`)", s))
				}
			}
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return usageErr(fmt.Errorf("--timeout %q: %w", timeoutStr, err))
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type result struct {
				Model        string          `json:"model"`
				Family       string          `json:"family"`
				Endpoint     string          `json:"endpoint"`
				CreditCost   float64         `json:"credit_cost"`
				DispatchedAt string          `json:"dispatched_at"`
				LatencyMS    int64           `json:"dispatch_latency_ms"`
				HTTPStatus   int             `json:"http_status"`
				TaskID       string          `json:"task_id,omitempty"`
				Status       string          `json:"status,omitempty"`
				Error        string          `json:"error,omitempty"`
				Response     json.RawMessage `json:"response,omitempty"`
			}

			results := make([]result, len(slugs))
			var wg sync.WaitGroup
			for i, slug := range slugs {
				wg.Add(1)
				go func(i int, slug string) {
					defer wg.Done()
					m := lookupModel(slug)
					body := map[string]any{"prompt": prompt}
					if aspect != "" {
						body["aspect_ratio"] = aspect
					}
					start := time.Now()
					resp, status, err := postOnce(ctx, c, m.Endpoint, body)
					r := result{
						Model:        m.Slug,
						Family:       m.Family,
						Endpoint:     m.Endpoint,
						CreditCost:   m.CreditCost,
						DispatchedAt: start.UTC().Format(time.RFC3339Nano),
						LatencyMS:    time.Since(start).Milliseconds(),
						HTTPStatus:   status,
					}
					if err != nil {
						r.Error = err.Error()
					} else {
						r.TaskID = extractTaskID(resp)
						r.Status = extractTaskStatus(resp)
						r.Response = json.RawMessage(resp)
						// Record dispatch in local ledger.
						_, _ = db.DB().ExecContext(ctx,
							`INSERT OR REPLACE INTO magnific_tasks(task_id, model, endpoint, status, prompt, credit_cost)
							 VALUES (?, ?, ?, COALESCE(?,'IN_PROGRESS'), ?, ?)`,
							r.TaskID, m.Slug, m.Endpoint, r.Status, prompt, m.CreditCost)
						_, _ = db.DB().ExecContext(ctx,
							`INSERT INTO magnific_prompts(prompt, model, task_id) VALUES (?, ?, ?)`,
							prompt, m.Slug, r.TaskID)
					}
					results[i] = r
				}(i, slug)
			}
			wg.Wait()

			var totalCost float64
			for _, r := range results {
				if r.Error == "" {
					totalCost += r.CreditCost
				}
			}
			out := map[string]any{
				"prompt":         prompt,
				"aspect_ratio":   aspect,
				"models":         slugs,
				"total_credits":  totalCost,
				"results":        results,
				"completed_note": "task_ids are submitted; use `magnific-pp-cli task wait <task_id>` for final outputs",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&modelsCSV, "models", "", "Comma-separated model slugs (e.g. mystic,flux-2-pro,seedream-v4-5)")
	cmd.Flags().StringVar(&aspect, "aspect", "", "Aspect ratio passed to each model (e.g. 16:9, 1:1)")
	cmd.Flags().StringVar(&timeoutStr, "timeout", "60s", "Total dispatch timeout")
	return cmd
}

func splitTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// postOnce wraps Client.Post so we can stub it out in tests if ever needed.
func postOnce(ctx context.Context, c *client.Client, path string, body any) (json.RawMessage, int, error) {
	_ = ctx // client carries its own timeout via the underlying http.Client
	return c.Post(path, body)
}
