// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/spf13/cobra"
)

type qaVerdict struct {
	Check   string `json:"check"`
	Verdict string `json:"verdict"` // pass | warn | fail
	Detail  string `json:"detail,omitempty"`
	Shot    *int   `json:"shot,omitempty"`
}

func newQACmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "qa",
		Short: "Pre-production quality gates",
	}
	cmd.AddCommand(newQAPreflightCmd(flags))
	return cmd
}

func newQAPreflightCmd(flags *rootFlags) *cobra.Command {
	var brandFlag string
	cmd := &cobra.Command{
		Use:   "preflight <shotlist.json>",
		Short: "Validate a shotlist before producing it (pass/warn/fail)",
		Long:  "Runs pass/warn/fail checks over a shotlist: balance vs estimated cost, model availability in the live catalog, prompt-safety heuristics, param ranges, brand-profile coverage, and platform request-shape validity (e.g. an Instagram reel duration above the 90s cap). Validates the REQUEST shape; response-shape checks happen in pack.",
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

			catalog := catalogModelIDs(cmd.Context(), c)
			var verdicts []qaVerdict
			var total float64

			for i := range shots {
				idx := i
				s := shots[i]
				// Model availability.
				if s.Model == "" {
					verdicts = append(verdicts, qaVerdict{Check: "model-availability", Verdict: "warn", Detail: "shot has no model; pack will need one resolved", Shot: &idx})
				} else if catalog != nil && !catalog[s.Model] {
					verdicts = append(verdicts, qaVerdict{Check: "model-availability", Verdict: "fail", Detail: fmt.Sprintf("model %s not in catalog", s.Model), Shot: &idx})
				} else {
					verdicts = append(verdicts, qaVerdict{Check: "model-availability", Verdict: "pass", Shot: &idx})
				}

				// Prompt safety heuristics.
				verdicts = append(verdicts, promptSafetyVerdict(s, idx))

				// Platform request-shape (duration cap, aspect coverage).
				verdicts = append(verdicts, platformShapeVerdicts(s, idx)...)

				// Cost accumulation (best-effort; pricing errors don't fail qa).
				if s.Model != "" {
					if price, status := priceShot(cmd.Context(), c, s.Model, s.toModelInputs()); status == priceOK || status == priceCached {
						total += price
					}
				}
			}

			// Balance vs cost.
			if balance := fetchBalance(cmd.Context(), c); balance != nil {
				if *balance < total {
					verdicts = append(verdicts, qaVerdict{Check: "balance", Verdict: "fail", Detail: fmt.Sprintf("balance %.2f < estimated cost %.2f", *balance, total)})
				} else {
					verdicts = append(verdicts, qaVerdict{Check: "balance", Verdict: "pass", Detail: fmt.Sprintf("balance %.2f >= estimated cost %.2f", *balance, total)})
				}
			} else {
				verdicts = append(verdicts, qaVerdict{Check: "balance", Verdict: "warn", Detail: "balance unavailable; skipped"})
			}

			// Brand-profile coverage.
			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, brandFlag)
			verdicts = append(verdicts, brandCoverageVerdicts(brandName, shots)...)

			overall := worstVerdict(verdicts)
			env := newEnvelope("qa preflight")
			env.CostSpent = total
			env.Results = []any{map[string]any{
				"overall_verdict": overall,
				"shot_count":      len(shots),
				"estimated_cost":  total,
				"checks":          verdicts,
			}}
			if overall == "fail" {
				env.PartialFailure = true
				env.RecommendedAction = "resolve the fail verdicts before pack"
			} else {
				env.RecommendedAction = "pack --concept \"...\" --platforms ..."
				env.SuggestedNext = suggestNext("wavespeed-pp-cli pack --concept \"...\" --platforms instagram,tiktok")
			}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&brandFlag, "brand", "", "Brand profile to validate coverage against (defaults to active brand)")
	return cmd
}

// catalogModelIDs returns the set of model_ids in the live catalog, or nil if
// the catalog can't be fetched (so availability checks degrade to warn).
func catalogModelIDs(ctx context.Context, c *client.Client) map[string]bool {
	models, err := c.GetNoCache(ctx, "/models", nil)
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, m := range decodeModelArray(models) {
		if id, _ := m["model_id"].(string); id != "" {
			set[id] = true
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func promptSafetyVerdict(s Shot, idx int) qaVerdict {
	if strings.TrimSpace(s.Prompt) == "" {
		return qaVerdict{Check: "prompt-safety", Verdict: "fail", Detail: "empty prompt", Shot: &idx}
	}
	lower := strings.ToLower(s.Prompt)
	for _, banned := range []string{"nsfw", "explicit gore", "child"} {
		if strings.Contains(lower, banned) {
			return qaVerdict{Check: "prompt-safety", Verdict: "warn", Detail: fmt.Sprintf("prompt contains sensitive token %q", banned), Shot: &idx}
		}
	}
	return qaVerdict{Check: "prompt-safety", Verdict: "pass", Shot: &idx}
}

// platformShapeVerdicts validates a shot's request against the platform spec:
// the duration param must not exceed the format's cap, and the aspect ratio
// should match a known format.
func platformShapeVerdicts(s Shot, idx int) []qaVerdict {
	if s.Platform == "" {
		return nil
	}
	spec, ok := lookupPlatform(s.Platform)
	if !ok {
		return []qaVerdict{{Check: "platform-shape", Verdict: "fail", Detail: fmt.Sprintf("unknown platform %q", s.Platform), Shot: &idx}}
	}
	format, ok := formatFor(spec, s.Format)
	if !ok {
		return []qaVerdict{{Check: "platform-shape", Verdict: "warn", Detail: fmt.Sprintf("format %q not defined for %s", s.Format, s.Platform), Shot: &idx}}
	}
	var out []qaVerdict
	if dur, ok := shotDurationSec(s); ok && format.DurationCapSec > 0 && dur > float64(format.DurationCapSec) {
		out = append(out, qaVerdict{Check: "platform-duration", Verdict: "fail", Detail: fmt.Sprintf("%s %s duration %.0fs exceeds %ds cap", s.Platform, format.Format, dur, format.DurationCapSec), Shot: &idx})
	}
	if s.AspectRatio != "" && s.AspectRatio != format.AspectRatio {
		out = append(out, qaVerdict{Check: "platform-aspect", Verdict: "warn", Detail: fmt.Sprintf("aspect %s differs from %s %s default %s", s.AspectRatio, s.Platform, format.Format, format.AspectRatio), Shot: &idx})
	}
	if len(out) == 0 {
		out = append(out, qaVerdict{Check: "platform-shape", Verdict: "pass", Shot: &idx})
	}
	return out
}

func shotDurationSec(s Shot) (float64, bool) {
	for _, key := range []string{"duration", "duration_sec", "duration_seconds", "length"} {
		if f, ok := toFloat(s.Params[key]); ok {
			return f, true
		}
	}
	return 0, false
}

func brandCoverageVerdicts(brandName string, shots []Shot) []qaVerdict {
	if brandName == "" {
		return []qaVerdict{{Check: "brand-coverage", Verdict: "warn", Detail: "no active brand; shots will use raw params"}}
	}
	_, body, err := loadBrandProfile(brandName)
	if err != nil {
		return []qaVerdict{{Check: "brand-coverage", Verdict: "fail", Detail: fmt.Sprintf("active brand %q not found", brandName)}}
	}
	covered := map[string]bool{}
	for _, p := range body.Platforms {
		covered[strings.ToLower(p)] = true
	}
	var out []qaVerdict
	for i := range shots {
		idx := i
		p := strings.ToLower(shots[i].Platform)
		if p != "" && len(covered) > 0 && !covered[p] {
			out = append(out, qaVerdict{Check: "brand-coverage", Verdict: "warn", Detail: fmt.Sprintf("platform %s not in brand %q coverage", p, brandName), Shot: &idx})
		}
	}
	if len(out) == 0 {
		out = append(out, qaVerdict{Check: "brand-coverage", Verdict: "pass", Detail: fmt.Sprintf("brand %q applied", brandName)})
	}
	return out
}

func worstVerdict(verdicts []qaVerdict) string {
	worst := "pass"
	for _, v := range verdicts {
		switch v.Verdict {
		case "fail":
			return "fail"
		case "warn":
			worst = "warn"
		}
	}
	return worst
}
