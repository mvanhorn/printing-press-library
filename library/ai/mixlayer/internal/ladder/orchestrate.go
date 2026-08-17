// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package ladder

import (
	"context"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/mixapi"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/pricing"
)

type Result struct {
	Model            string  `json:"model"`
	Answer           string  `json:"answer,omitempty"`
	Reasoning        string  `json:"reasoning,omitempty"`
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
	LatencyMS        int64   `json:"latency_ms,omitempty"`
	Error            string  `json:"error,omitempty"`
}

func Rungs(spec string) []string {
	if strings.TrimSpace(spec) == "" || strings.EqualFold(spec, "all") {
		return append([]string(nil), pricing.DefaultRungs...)
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func AskAcross(ctx context.Context, c *client.Client, prompt string, rungs []string, reasoning bool, seed *int64) []Result {
	if len(rungs) == 0 {
		rungs = pricing.DefaultRungs
	}
	results := make([]Result, len(rungs))
	var wg sync.WaitGroup
	for i, model := range rungs {
		wg.Add(1)
		go func(i int, model string) {
			defer wg.Done()
			thinking := reasoning
			req := mixapi.ChatRequest{
				Model:    model,
				Messages: []mixapi.Message{{Role: "user", Content: prompt}},
				Thinking: &thinking,
				Seed:     seed,
			}
			res, err := mixapi.Chat(ctx, c, req)
			results[i].Model = model
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			results[i].Answer = res.Answer
			results[i].Reasoning = res.Reasoning
			results[i].PromptTokens = res.PromptTokens
			results[i].CompletionTokens = res.CompletionTokens
			results[i].TotalTokens = res.TotalTokens
			results[i].LatencyMS = res.LatencyMS
			results[i].CostUSD = pricing.Estimate(model, res.PromptTokens, res.CompletionTokens)
		}(i, model)
	}
	wg.Wait()
	return results
}

func FirstConfident(results []Result) string {
	for _, r := range results {
		if r.Error == "" && strings.TrimSpace(r.Answer) != "" {
			return r.Model
		}
	}
	return ""
}
