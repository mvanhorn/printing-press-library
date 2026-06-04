// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

type aspectsFlags struct {
	platforms []string
	aspects   []string
	model     string
	prompt    string
	outpaint  bool
	noRecord  bool
	outDir    string
	brand     string
}

func newAspectsCmd(flags *rootFlags) *cobra.Command {
	var af aspectsFlags
	cmd := &cobra.Command{
		Use:   "aspects <image>",
		Short: "Re-frame one image into standard aspect ratios",
		Long:  "Produce platform aspect ratios from a single source image. Uses outpaint/extend when the model supports it; otherwise falls back to a re-render with an anchored prompt and warns.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if af.outDir == "" {
				af.outDir = "aspects"
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, af.brand)
			var body brandProfileBody
			if brandName != "" {
				if _, b, e := loadBrandProfile(brandName); e == nil {
					body = b
				}
			}
			model := af.model
			if model == "" && len(body.Models) > 0 {
				model = body.Models[0]
			}

			targets := aspectTargets(af)
			if len(targets) == 0 {
				return usageErr(fmt.Errorf("pass --platforms and/or --aspects to choose target ratios"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			env := newEnvelope("aspects")
			useOutpaint := (af.outpaint || modelSupportsOutpaint(model)) && looksLikeURL(source)
			if !useOutpaint {
				env.Warnings = append(env.Warnings, "model does not support outpaint (or source is a local path); re-rendering with an anchored prompt")
			}

			if flags.dryRun {
				env.DryRun = true
				for i, a := range targets {
					env.Results = append(env.Results, map[string]any{"target": i, "aspect_ratio": a, "model": model, "mode": outpaintMode(useOutpaint)})
				}
				return emitEnvelope(cmd.OutOrStdout(), env)
			}

			recordEnabled := shouldRecord(project, true, af.noRecord)
			ctx := cmd.Context()
			anchor := af.prompt
			if anchor == "" {
				anchor = "re-frame the source composition, preserving subject and brand styling"
			}
			for i, aspect := range targets {
				inputs := map[string]any{"aspect_ratio": aspect}
				if useOutpaint {
					inputs["image"] = source
					inputs["prompt"] = anchor
				} else {
					inputs["prompt"] = anchor
				}
				spec := filepath.Join(af.outDir, fmt.Sprintf("aspect-%s.{ext}", strings.ReplaceAll(aspect, ":", "x")))
				res, err := submitAndAwait(ctx, c, submitRequest{
					modelID: model, inputs: inputs, estimatePrice: true, priceBestEffort: true,
					wait: true, waitTimeout: 5 * time.Minute, pollInitial: 2 * time.Second,
					download: true, downloadSpec: spec,
				})
				oc := shotOutcome{Shot: Shot{Prompt: anchor, Model: model, AspectRatio: aspect}, Files: []string{}}
				if err != nil {
					oc.Err = err.Error()
				} else {
					oc.Cost = extractCostFromPricing(res.Pricing)
					oc.ContentHash = hashContent(res.Result)
					for _, d := range res.Downloads {
						oc.Files = append(oc.Files, d.Path)
					}
					if res.Failed {
						oc.Err = fmt.Sprintf("prediction failed with status %q", res.Status)
					}
				}
				env.CostSpent += oc.Cost
				env.Results = append(env.Results, map[string]any{"target": i, "aspect_ratio": aspect, "mode": outpaintMode(useOutpaint), "outcome": oc})
				if recordEnabled && oc.Err == "" {
					g := store.Generation{ID: newGenerationID(), Command: "aspects", ModelID: model, Prompt: anchor, BrandName: brandName, AspectRatio: aspect, Cost: oc.Cost, ContentHash: oc.ContentHash, Status: "completed"}
					if len(oc.Files) > 0 {
						g.Path = oc.Files[0]
					}
					if rerr := recordGeneration(g); rerr != nil {
						env.LibraryRecordErrors = append(env.LibraryRecordErrors, rerr.Error())
					}
				}
			}
			env.RecommendedAction = "review re-framed assets; pack them for posting"
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringSliceVar(&af.platforms, "platforms", nil, "Platforms whose default aspect ratios to target")
	cmd.Flags().StringSliceVar(&af.aspects, "aspects", nil, "Explicit aspect ratios to target")
	cmd.Flags().StringVar(&af.model, "model", "", "Model ID (defaults to brand's first model)")
	cmd.Flags().StringVar(&af.prompt, "prompt", "", "Anchor prompt for re-render fallback")
	cmd.Flags().BoolVar(&af.outpaint, "outpaint", false, "Force outpaint/extend mode")
	cmd.Flags().BoolVar(&af.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().StringVar(&af.outDir, "out-dir", "aspects", "Directory for outputs")
	cmd.Flags().StringVar(&af.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	return cmd
}

func aspectTargets(af aspectsFlags) []string {
	seen := map[string]bool{}
	var out []string
	add := func(a string) {
		a = strings.TrimSpace(a)
		if a == "" || seen[a] {
			return
		}
		seen[a] = true
		out = append(out, a)
	}
	for _, a := range af.aspects {
		add(a)
	}
	for _, p := range normalizePlatforms(af.platforms) {
		if f, ok := defaultFormatFor(p); ok {
			add(f.AspectRatio)
		}
	}
	return out
}

func modelSupportsOutpaint(model string) bool {
	m := strings.ToLower(model)
	for _, kw := range []string{"outpaint", "inpaint", "fill", "extend", "uncrop"} {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

func outpaintMode(useOutpaint bool) string {
	if useOutpaint {
		return "outpaint"
	}
	return "re-render"
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
