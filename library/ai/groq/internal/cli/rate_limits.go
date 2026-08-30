// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/groq/internal/config"
)

// rateLimitBudget is the per-model request/token budget Groq advertises via
// x-ratelimit-* response headers on inference endpoints. No API endpoint
// exposes "my remaining budget" directly; the headers on any inference
// response are the source of truth, so this command makes one minimal
// 1-token completion against the requested model to read them.
type rateLimitBudget struct {
	Model             string `json:"model"`
	LimitRequests     int    `json:"limit_requests"`
	RemainingRequests int    `json:"remaining_requests"`
	ResetRequests     string `json:"reset_requests,omitempty"`
	LimitTokens       int    `json:"limit_tokens"`
	RemainingTokens   int    `json:"remaining_tokens"`
	ResetTokens       string `json:"reset_tokens,omitempty"`
	RetryAfter        string `json:"retry_after,omitempty"`
	RequestedAt       string `json:"requested_at"`
}

func newNovelRateLimitsCmd(flags *rootFlags) *cobra.Command {
	var flagModel string

	cmd := &cobra.Command{
		Use:         "rate-limits",
		Short:       "See remaining per-model request/token budget from every API call, with reset windows.",
		Long:        "Inspect the per-model rate-limit budget Groq advertises on inference responses (RPM/RPD + TPM, with remaining counts and reset windows). Makes one minimal 1-token completion against the model to read the x-ratelimit-* headers — no other endpoint exposes remaining budget.",
		Example:     "  groq-pp-cli rate-limits --model openai/gpt-oss-20b --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rate-limits")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if !cfg.CredentialConfigured() {
				return usageErr(fmt.Errorf("GROQ_API_KEY is not set; export GROQ_API_KEY or run 'groq-pp-cli auth set-token <key>'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			model := flagModel
			if model == "" {
				model = "openai/gpt-oss-20b"
			}
			budget, err := fetchRateLimitBudget(ctx, cfg, model)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), budget, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Model: %s\n", budget.Model)
			fmt.Fprintf(cmd.OutOrStdout(), "  requests: %d remaining / %d limit (resets in %s)\n", budget.RemainingRequests, budget.LimitRequests, orDash(budget.ResetRequests))
			fmt.Fprintf(cmd.OutOrStdout(), "  tokens:   %d remaining / %d limit (resets in %s)\n", budget.RemainingTokens, budget.LimitTokens, orDash(budget.ResetTokens))
			if budget.RetryAfter != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  retry-after: %s\n", budget.RetryAfter)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  observed at: %s\n", budget.RequestedAt)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagModel, "model", "", "Model to inspect the rate-limit budget for (default: openai/gpt-oss-20b)")
	return cmd
}

// fetchRateLimitBudget performs a minimal chat completion via direct HTTP so
// the x-ratelimit-* response headers are readable (the typed client does not
// surface response headers to callers).
func fetchRateLimitBudget(ctx context.Context, cfg *config.Config, model string) (*rateLimitBudget, error) {
	payload, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "ping"}},
		"max_tokens": 1,
	})
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://api.groq.com"
	}
	url := base + "/openai/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building rate-limit probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.GroqApiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("probing rate-limit budget for %s: %w", model, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rate-limit probe for %s failed with HTTP %d: %s", model, resp.StatusCode, truncateString(string(body), 300))
	}

	h := resp.Header
	budget := &rateLimitBudget{
		Model:             model,
		LimitRequests:     headerInt(h, "X-RateLimit-Limit-Requests"),
		RemainingRequests: headerInt(h, "X-RateLimit-Remaining-Requests"),
		ResetRequests:     h.Get("X-RateLimit-Reset-Requests"),
		LimitTokens:       headerInt(h, "X-RateLimit-Limit-Tokens"),
		RemainingTokens:   headerInt(h, "X-RateLimit-Remaining-Tokens"),
		ResetTokens:       h.Get("X-RateLimit-Reset-Tokens"),
		RetryAfter:        h.Get("Retry-After"),
		RequestedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	return budget, nil
}

func headerInt(h http.Header, name string) int {
	v := h.Get(name)
	if v == "" {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0
	}
	return n
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
