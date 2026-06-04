// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/spf13/cobra"
)

func newPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Plan creative work: turn briefs into shotlists, pick models, estimate cost",
		Long:  "Planning commands that shape work before producing it. brief-to-shotlist turns a free-text brief into a structured shotlist; model-pick recommends a model for an intent; cost-estimate prices a shotlist against your balance.",
	}
	cmd.AddCommand(
		newPlanBriefToShotlistCmd(flags),
		newPlanModelPickCmd(flags),
		newPlanCostEstimateCmd(flags),
	)
	return cmd
}

type planBriefFlags struct {
	prompt       string
	fromFile     string
	platforms    []string
	aspects      []string
	planner      string
	plannerModel string
	brand        string
}

func newPlanBriefToShotlistCmd(flags *rootFlags) *cobra.Command {
	var pf planBriefFlags
	cmd := &cobra.Command{
		Use:   "brief-to-shotlist",
		Short: "Turn a creative brief into a structured shotlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			brief := strings.TrimSpace(pf.prompt)
			if pf.fromFile != "" {
				raw, err := os.ReadFile(pf.fromFile)
				if err != nil {
					return usageErr(fmt.Errorf("reading --from-file: %w", err))
				}
				brief = strings.TrimSpace(string(raw))
			}
			if brief == "" {
				return usageErr(fmt.Errorf("a brief is required via --prompt or --from-file"))
			}

			planner := strings.ToLower(strings.TrimSpace(pf.planner))
			if planner == "" {
				planner = "auto"
			}

			shots, plannerUsed, warnings, err := planBrief(cmd.Context(), flags, planner, pf, brief)
			if err != nil {
				return err
			}

			// Brand merge.
			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, pf.brand)
			if brandName != "" {
				_, body, berr := loadBrandProfile(brandName)
				if berr == nil {
					for i := range shots {
						shots[i] = mergeBrandIntoShot(shots[i], brandName, body)
					}
				} else {
					warnings = append(warnings, fmt.Sprintf("brand %q not loaded: %v", brandName, berr))
				}
			}

			env := newEnvelope("plan brief-to-shotlist")
			env.PlannerUsed = plannerUsed
			env.Warnings = warnings
			for _, s := range shots {
				env.Results = append(env.Results, s)
			}
			env.RecommendedAction = "plan cost-estimate <shotlist.json>"
			env.SuggestedNext = suggestNext(
				"wavespeed-pp-cli plan cost-estimate shotlist.json",
				"wavespeed-pp-cli qa preflight shotlist.json",
				"wavespeed-pp-cli pack --concept \"...\" --platforms instagram,tiktok",
			)
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&pf.prompt, "prompt", "", "Free-text creative brief")
	cmd.Flags().StringVar(&pf.fromFile, "from-file", "", "Read the brief from a file")
	cmd.Flags().StringSliceVar(&pf.platforms, "platforms", nil, "Target platforms (instagram,tiktok,facebook,x)")
	cmd.Flags().StringSliceVar(&pf.aspects, "aspects", nil, "Explicit aspect ratios (16:9,1:1,9:16,4:5)")
	cmd.Flags().StringVar(&pf.planner, "planner", "auto", "Planner mode: deterministic|llm|auto")
	cmd.Flags().StringVar(&pf.plannerModel, "planner-model", "", "Model ID for the LLM planner (llm/auto fallback)")
	cmd.Flags().StringVar(&pf.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	return cmd
}

// planBrief selects the planner per the hybrid contract and returns the
// shotlist, the planner actually used, and any warnings.
func planBrief(ctx context.Context, flags *rootFlags, planner string, pf planBriefFlags, brief string) ([]Shot, string, []string, error) {
	var warnings []string
	hasCues := len(pf.platforms) > 0 || len(pf.aspects) > 0 || briefHasCues(brief)

	useLLM := false
	switch planner {
	case "deterministic":
		useLLM = false
	case "llm":
		useLLM = true
	case "auto":
		useLLM = !hasCues
	default:
		return nil, "", nil, usageErr(fmt.Errorf("invalid --planner %q (want deterministic|llm|auto)", pf.planner))
	}

	if useLLM {
		model := strings.TrimSpace(pf.plannerModel)
		if model == "" {
			warnings = append(warnings, "no --planner-model set; used deterministic parser")
			return parseBriefToShots(brief, pf.platforms, pf.aspects), "fallback-parser", warnings, nil
		}
		c, err := flags.newClient()
		if err != nil {
			return nil, "", nil, err
		}
		shots, lerr := llmBriefToShots(ctx, c, model, brief, false)
		if lerr != nil {
			// Retry once with a stricter schema reminder.
			shots, lerr = llmBriefToShots(ctx, c, model, brief, true)
		}
		if lerr != nil {
			if planner == "llm" {
				// Explicit llm with no usable cues and no parseable output.
				if !hasCues {
					return nil, "", nil, usageErr(fmt.Errorf("LLM planner produced no valid shots and the brief has no platform/aspect cues to parse: %v", lerr))
				}
			}
			warnings = append(warnings, fmt.Sprintf("LLM planner failed (%v); fell back to parser", lerr))
			return parseBriefToShots(brief, pf.platforms, pf.aspects), "fallback-parser", warnings, nil
		}
		return shots, "llm:" + model, warnings, nil
	}

	return parseBriefToShots(brief, pf.platforms, pf.aspects), "parser", warnings, nil
}

// briefHasCues reports whether the free text names a known platform or an
// aspect ratio, which lets the deterministic parser run with confidence.
func briefHasCues(brief string) bool {
	lower := strings.ToLower(brief)
	for _, p := range knownPlatforms() {
		if strings.Contains(lower, p) {
			return true
		}
	}
	for _, a := range []string{"16:9", "9:16", "1:1", "4:5"} {
		if strings.Contains(lower, a) {
			return true
		}
	}
	return false
}

// parseBriefToShots is the deterministic planner. It expands the brief across
// requested platforms (or platforms named in the text), or across explicit
// aspect ratios, or to a single shot when neither is present.
func parseBriefToShots(brief string, platforms, aspects []string) []Shot {
	platforms = normalizePlatforms(platforms)
	if len(platforms) == 0 {
		platforms = detectPlatforms(brief)
	}

	var shots []Shot
	switch {
	case len(platforms) > 0:
		// Mirror buildPackShots' platform×aspect cross-product so a planned
		// shotlist matches exactly what pack will produce — otherwise
		// cost-estimate and qa preflight operate on an undercounted list.
		for _, p := range platforms {
			fmtDesc, ok := defaultFormatFor(p)
			format := ""
			defAspect := ""
			if ok {
				format = fmtDesc.Format
				defAspect = fmtDesc.AspectRatio
			}
			if len(aspects) > 0 {
				for _, a := range aspects {
					shots = append(shots, Shot{
						Concept:     truncate(brief, 80),
						Prompt:      brief,
						Platform:    p,
						Format:      format,
						AspectRatio: a,
					})
				}
			} else {
				shots = append(shots, Shot{
					Concept:     truncate(brief, 80),
					Prompt:      brief,
					Platform:    p,
					Format:      format,
					AspectRatio: defAspect,
				})
			}
		}
	case len(aspects) > 0:
		for _, a := range aspects {
			shots = append(shots, Shot{Concept: truncate(brief, 80), Prompt: brief, AspectRatio: a})
		}
	default:
		shots = append(shots, Shot{Concept: truncate(brief, 80), Prompt: brief})
	}
	return shots
}

func normalizePlatforms(platforms []string) []string {
	out := []string{}
	for _, p := range platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func detectPlatforms(brief string) []string {
	lower := strings.ToLower(brief)
	var found []string
	for _, p := range knownPlatforms() {
		if strings.Contains(lower, p) {
			found = append(found, p)
		}
	}
	return found
}

// llmBriefToShots asks a text model to emit a JSON array of shots. strict adds
// a schema reminder for the retry.
func llmBriefToShots(ctx context.Context, c *client.Client, model, brief string, strict bool) ([]Shot, error) {
	instruction := "You are a creative shot planner. Read the brief and output ONLY a JSON array of shots. " +
		"Each shot is an object with fields: concept, prompt, platform, aspect_ratio. Output nothing but the JSON array."
	if strict {
		instruction += " Your previous output did not parse. Output MUST be a valid JSON array matching that schema and nothing else."
	}
	res, err := submitAndAwait(ctx, c, submitRequest{
		modelID:     model,
		inputs:      map[string]any{"prompt": instruction + "\n\nBrief: " + brief},
		wait:        true,
		waitTimeout: 60 * time.Second,
		pollInitial: 500 * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	if res.Failed {
		return nil, fmt.Errorf("planner prediction failed with status %q", res.Status)
	}
	text := extractTextOutput(res.Result)
	shots, err := parseShotsFromText(text)
	if err != nil {
		return nil, err
	}
	if len(shots) == 0 {
		return nil, fmt.Errorf("planner returned no shots")
	}
	return shots, nil
}

// extractTextOutput pulls a text string out of a prediction result. WaveSpeed
// text shapes vary; probe outputs[], output, text, and content.
func extractTextOutput(result json.RawMessage) string {
	obj := decodeObject(unwrapWaveSpeedData(result))
	if outs, ok := obj["outputs"].([]any); ok {
		for _, o := range outs {
			if s, ok := o.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	for _, key := range []string{"output", "text", "content", "result"} {
		if s, ok := obj[key].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// parseShotsFromText extracts the first JSON array in text and decodes it as
// []Shot, validating that each shot has a prompt.
func parseShotsFromText(text string) ([]Shot, error) {
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end < 0 || end <= start {
		return nil, fmt.Errorf("no JSON array found in planner output")
	}
	var shots []Shot
	if err := json.Unmarshal([]byte(text[start:end+1]), &shots); err != nil {
		return nil, fmt.Errorf("planner output is not a valid shot array: %w", err)
	}
	for i := range shots {
		if strings.TrimSpace(shots[i].Prompt) == "" {
			return nil, fmt.Errorf("planner shot %d is missing a prompt", i)
		}
	}
	return shots, nil
}

// --- model-pick ---------------------------------------------------------

func newPlanModelPickCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model-pick <intent>",
		Short: "Recommend a model for an intent from the live catalog",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			intent := strings.ToLower(strings.Join(args, " "))
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			models, err := c.GetNoCache(cmd.Context(), "/models", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			pick, rationale := pickModelForIntent(models, intent)
			if pick == "" {
				return notFoundErr(fmt.Errorf("no catalog model matched intent %q", intent))
			}
			env := newEnvelope("plan model-pick")
			env.Results = []any{map[string]any{
				"intent":    intent,
				"model":     pick,
				"rationale": rationale,
			}}
			env.RecommendedAction = "run " + pick
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	return cmd
}

// pickModelForIntent does a keyword match over the catalog. Video intents
// prefer video models; otherwise the first image-capable model.
func pickModelForIntent(models json.RawMessage, intent string) (string, string) {
	wantVideo := strings.Contains(intent, "video") || strings.Contains(intent, "reel") || strings.Contains(intent, "motion")
	arr := decodeModelArray(models)
	var firstImage string
	for _, m := range arr {
		id, _ := m["model_id"].(string)
		typ, _ := m["type"].(string)
		desc, _ := m["description"].(string)
		blob := strings.ToLower(id + " " + typ + " " + desc)
		if id == "" {
			continue
		}
		if wantVideo && strings.Contains(blob, "video") {
			return id, "video intent matched a video-capable model (type/description)"
		}
		if !wantVideo && firstImage == "" && (strings.Contains(blob, "image") || strings.Contains(blob, "flux") || strings.Contains(blob, "sdxl") || strings.Contains(blob, "text-to-image")) {
			firstImage = id
		}
	}
	if !wantVideo && firstImage != "" {
		return firstImage, "image intent matched an image-capable model"
	}
	// Fall back to the first model in the catalog.
	for _, m := range arr {
		if id, _ := m["model_id"].(string); id != "" {
			return id, "no strong keyword match; returning the first catalog model"
		}
	}
	return "", ""
}

func decodeModelArray(models json.RawMessage) []map[string]any {
	body := unwrapWaveSpeedData(models)
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr
	}
	// Some shapes wrap under data already unwrapped above; try object form.
	return nil
}

// --- cost-estimate ------------------------------------------------------

func newPlanCostEstimateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost-estimate <shotlist.json>",
		Short: "Price a shotlist against the live pricing API and your balance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "-"
			if len(args) == 1 {
				path = args[0]
			}
			shots, err := readShotlist(path)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var (
				total       float64
				warnings    []string
				breakdown   []any
				hardFailure error
			)
			for i, s := range shots {
				model := s.Model
				if model == "" {
					warnings = append(warnings, fmt.Sprintf("shot %d has no model; priced as 0", i))
					breakdown = append(breakdown, map[string]any{"shot": i, "model": "", "cost": 0.0, "priced": false})
					continue
				}
				price, status := priceShot(cmd.Context(), c, model, s.toModelInputs())
				switch status {
				case priceOK:
					total += price
					breakdown = append(breakdown, map[string]any{"shot": i, "model": model, "cost": price, "priced": true})
				case priceUnavailable:
					warnings = append(warnings, fmt.Sprintf("shot %d: pricing unavailable for %s", i, model))
					breakdown = append(breakdown, map[string]any{"shot": i, "model": model, "cost": 0.0, "priced": false})
				case priceCached:
					total += price
					warnings = append(warnings, fmt.Sprintf("shot %d: used cached pricing for %s", i, model))
					breakdown = append(breakdown, map[string]any{"shot": i, "model": model, "cost": price, "priced": true, "cached": true})
				case priceError:
					hardFailure = fmt.Errorf("pricing failed for model %s and no cached price is available", model)
				}
			}
			if hardFailure != nil {
				return apiErr(hardFailure)
			}

			balance := fetchBalance(cmd.Context(), c)
			env := newEnvelope("plan cost-estimate")
			env.Warnings = warnings
			env.CostSpent = total
			env.BalanceAfter = balance
			result := map[string]any{
				"total_cost":   total,
				"currency":     "account credits (WaveSpeed credit unit)",
				"per_shot":     breakdown,
				"shot_count":   len(shots),
			}
			if balance != nil {
				result["balance"] = *balance
				result["sufficient"] = *balance >= total
			}
			env.Results = []any{result}
			env.RecommendedAction = "qa preflight <shotlist.json>"
			env.SuggestedNext = suggestNext("wavespeed-pp-cli qa preflight shotlist.json")
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	return cmd
}

type priceStatus int

const (
	priceOK priceStatus = iota
	priceUnavailable
	priceCached
	priceError
)

// priceShot queries /model/pricing for one model+inputs. On a server error it
// falls back to cached pricing from the archive DB; with no cache it reports
// priceError so the caller can hard-fail.
func priceShot(ctx context.Context, c *client.Client, model string, inputs map[string]any) (float64, priceStatus) {
	pricing, _, err := c.PostQueryWithParams(ctx, "/model/pricing", nil, map[string]any{
		"model_id": model,
		"inputs":   inputs,
	})
	if err != nil {
		if cached, ok := cachedModelPrice(model); ok {
			return cached, priceCached
		}
		return 0, priceError
	}
	cost := extractCostFromPricing(pricing)
	if cost == 0 {
		return 0, priceUnavailable
	}
	return cost, priceOK
}

// cachedModelPrice reads a previously-synced price from the archive DB's
// model_pricing table. Best-effort: any failure returns ok=false.
func cachedModelPrice(model string) (float64, bool) {
	s, err := openArchiveReadOnly()
	if err != nil {
		return 0, false
	}
	defer s.Close()
	var data string
	err = s.DB().QueryRow(`SELECT data FROM model_pricing WHERE id = ? OR id = ?`, model, "model_pricing:"+model).Scan(&data)
	if err != nil {
		return 0, false
	}
	price := extractCostFromPricing(json.RawMessage(data))
	if price == 0 {
		return 0, false
	}
	return price, true
}
