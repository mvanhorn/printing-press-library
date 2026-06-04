package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type budgetReport struct {
	Model         string  `json:"model"`
	HTTPStatus    int     `json:"http_status"`
	Verdict       string  `json:"verdict"` // ok | exhausted | unknown
	RateLimited   bool    `json:"rate_limited"`
	ErrorMessage  string  `json:"error_message,omitempty"`
	UpgradeHint   string  `json:"upgrade_hint,omitempty"`
	LatencyMs     int     `json:"latency_ms"`
	PromptTokens  int     `json:"prompt_tokens,omitempty"`
	ProbedAt      time.Time `json:"probed_at"`
}

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	var model string
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Probe Ollama Cloud weekly-cap quota (free + paid tiers); surface 429 before workflows blow up",
		Long: strings.TrimSpace(`
Sends a 1-token chat probe to detect weekly cap exhaustion. Parses the 429
prose ("you have reached your weekly usage limit, upgrade for higher limits")
and emits a structured verdict so agents can pre-flight quota before launching
a long session. The same 429 prose appears on free AND paid tiers (paid =
higher cap, not unlimited; verified 2026-05-24).

Verdicts: ok | exhausted | unknown. exhausted surfaces the upgrade URL so the
caller has an actionable next step.
`),
		Example: "  ollama-cloud-pp-cli budget --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"model":    model,
				"messages": []map[string]any{{"role": "user", "content": "ping"}},
				"stream":   false,
				"options":  map[string]any{"num_predict": 1, "temperature": 0},
			}
			r := budgetReport{Model: model, ProbedAt: time.Now().UTC(), Verdict: "unknown"}
			start := time.Now()
			raw, status, perr := c.Post(ctx, "/api/chat", body)
			r.LatencyMs = int(time.Since(start) / time.Millisecond)
			r.HTTPStatus = status
			// The shared client surfaces post-retry 429s as a wrapped error
			// rather than status=429, so cross-check the error text for the
			// upstream prose. False positives are unlikely — Ollama's 429
			// envelope is distinctive.
			errText := ""
			if perr != nil {
				errText = perr.Error()
			}
			weeklyCapHit := strings.Contains(errText, "HTTP 429") ||
				strings.Contains(errText, "weekly usage limit") ||
				strings.Contains(strings.ToLower(string(raw)), "weekly usage limit")
			switch {
			case status == 429 || weeklyCapHit:
				r.Verdict = "exhausted"
				r.RateLimited = true
				r.HTTPStatus = 429
				if r.ErrorMessage == "" {
					if errText != "" {
						r.ErrorMessage = errText
					} else {
						r.ErrorMessage = string(raw)
					}
				}
				r.UpgradeHint = "https://ollama.com/upgrade"
			case perr != nil:
				r.ErrorMessage = errText
			case status >= 200 && status < 300:
				r.Verdict = "ok"
			default:
				r.ErrorMessage = fmt.Sprintf("HTTP %d: %s", status, truncate(string(raw), 200))
			}
			b, _ := json.MarshalIndent(r, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
			if r.Verdict == "exhausted" {
				return apiErr(fmt.Errorf("ollama-cloud free-tier exhausted; upgrade at %s", r.UpgradeHint))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", "gpt-oss:20b", "Model to probe (use a cheap/fast one)")
	return cmd
}
