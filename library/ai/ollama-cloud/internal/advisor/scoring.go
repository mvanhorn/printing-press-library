package advisor

import (
	"fmt"
	"strings"
)

func scoreModel(m Model, f Features, req Request) Candidate {
	c := Candidate{Model: m, ModelID: m.ID, Components: map[string]float64{}}

	if reason := hardFilter(m, f, req); reason != "" {
		c.Filtered = true
		c.FilterReason = reason
		c.Why = reason
		return c
	}

	c.Components["task_hint"] = scoreTaskHint(m, req.TaskHint)
	c.Components["ctx_fit"] = scoreCtxFit(m, f.InputTokens+256)
	c.Components["cost"] = scoreCost(m)
	c.Components["latency"] = scoreLatency(m, req.MaxLatencyMs)
	c.Components["coding"] = scoreCoding(m, f)
	c.Components["tool_boost"] = scoreToolBoost(m, f)

	// Fixed component order; map iteration is randomized in Go which produces
	// non-deterministic floating-point summation orders and bit-level different
	// scores. Keep this slice sorted alphabetically.
	weighted := []struct {
		key string
		w   float64
	}{
		{"coding", 0.14},
		{"cost", 0.10},
		{"ctx_fit", 0.22},
		{"latency", 0.14},
		{"task_hint", 0.32},
		{"tool_boost", 0.08},
	}
	var total, sumW float64
	for _, ww := range weighted {
		total += ww.w * c.Components[ww.key]
		sumW += ww.w
	}
	c.Score = total / sumW
	c.Why = composeWhy(m, f, req, c)
	return c
}

func hardFilter(m Model, f Features, req Request) string {
	if req.RequireTools && !m.SupportsTools {
		return "lacks tool-use support"
	}
	if f.HasVisionInput && !m.SupportsVision {
		return "vision input requires multimodal model"
	}
	estIn := f.InputTokens + 256
	if m.CtxWindow > 0 && estIn > m.CtxWindow {
		return fmt.Sprintf("prompt (~%d tok) exceeds ctx_window (%d)", estIn, m.CtxWindow)
	}
	if req.MaxLatencyMs > 0 && m.LatencyP50Ms > req.MaxLatencyMs {
		return fmt.Sprintf("p50 latency %dms exceeds --max-latency-ms %d", m.LatencyP50Ms, req.MaxLatencyMs)
	}
	if req.BudgetRemainingUSD > 0 {
		est := estimateCostUSD(m, f.InputTokens, req.ExpectedOutputTokens)
		if est > req.BudgetRemainingUSD {
			return fmt.Sprintf("est cost $%.4f exceeds budget $%.4f", est, req.BudgetRemainingUSD)
		}
	}
	return ""
}

func scoreTaskHint(m Model, hint string) float64 {
	if hint == "" {
		return 0.5
	}
	for _, s := range m.Strengths {
		if s == hint {
			return 1.0
		}
	}
	switch hint {
	case "coding":
		for _, s := range m.Strengths {
			if s == "coding" || s == "agentic" {
				return 0.9
			}
		}
	case "long-context":
		if m.CtxWindow >= 131072 {
			return 1.0
		}
		if m.CtxWindow >= 65536 {
			return 0.7
		}
		return 0.2
	case "cheap":
		for _, s := range m.Strengths {
			if s == "cheap" || s == "fast" {
				return 1.0
			}
		}
		return 0.2
	case "vision":
		if m.SupportsVision {
			return 1.0
		}
		return 0.0
	}
	return 0.3
}

func scoreCtxFit(m Model, estIn int) float64 {
	if m.CtxWindow <= 0 {
		return 0.5
	}
	if estIn >= m.CtxWindow {
		return 0
	}
	ratio := float64(estIn) / float64(m.CtxWindow)
	switch {
	case ratio < 0.05:
		return 0.7
	case ratio < 0.4:
		return 1.0
	case ratio < 0.6:
		return 0.85
	case ratio < 0.8:
		return 0.6
	default:
		return 0.3
	}
}

func scoreCost(m Model) float64 {
	if m.PriceInPer1M == 0 && m.PriceOutPer1M == 0 {
		return 1.0
	}
	total := m.PriceInPer1M + m.PriceOutPer1M
	if total <= 0.5 {
		return 1.0
	}
	if total >= 20 {
		return 0.1
	}
	return 1.0 - (total-0.5)/20
}

func scoreLatency(m Model, maxMs int) float64 {
	if m.LatencyP50Ms <= 0 {
		return 0.5
	}
	target := 3000
	if maxMs > 0 {
		target = maxMs
	}
	if m.LatencyP50Ms <= target/2 {
		return 1.0
	}
	if m.LatencyP50Ms <= target {
		return 0.7
	}
	return 0.3
}

func scoreCoding(m Model, f Features) float64 {
	if f.CodeFenceDensity < 0.05 {
		return 0.5
	}
	for _, s := range m.Strengths {
		if s == "coding" || s == "agentic" {
			return 1.0
		}
	}
	return 0.3
}

func scoreToolBoost(m Model, f Features) float64 {
	if f.ToolUseMentions <= 0 {
		return 0.5
	}
	if m.SupportsTools {
		return 1.0
	}
	return 0.1
}

func composeWhy(m Model, f Features, req Request, c Candidate) string {
	parts := []string{}
	if req.TaskHint != "" {
		parts = append(parts, fmt.Sprintf("task-hint=%s match=%.0f%%", req.TaskHint, c.Components["task_hint"]*100))
	}
	parts = append(parts, fmt.Sprintf("ctx %d/%d", f.InputTokens, m.CtxWindow))
	parts = append(parts, fmt.Sprintf("p50 %dms", m.LatencyP50Ms))
	if f.CodeFenceDensity >= 0.05 {
		parts = append(parts, fmt.Sprintf("code-fence density %.2f", f.CodeFenceDensity))
	}
	if f.ToolUseMentions > 0 && m.SupportsTools {
		parts = append(parts, "tools-capable")
	}
	if f.HasVisionInput && m.SupportsVision {
		parts = append(parts, "vision-capable")
	}
	if len(m.Strengths) > 0 {
		parts = append(parts, "strengths=["+strings.Join(m.Strengths, ",")+"]")
	}
	return strings.Join(parts, "; ")
}

func estimateCostUSD(m Model, inputTok, outputTok int) float64 {
	return (float64(inputTok)*m.PriceInPer1M + float64(outputTok)*m.PriceOutPer1M) / 1_000_000
}
