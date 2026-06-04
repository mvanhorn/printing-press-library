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

type restyleFlags struct {
	brand    string
	style    string
	model    string
	noRecord bool
	outDir   string
}

func newRestyleCmd(flags *rootFlags) *cobra.Command {
	var rf restyleFlags
	cmd := &cobra.Command{
		Use:   "restyle <image>",
		Short: "Apply a brand or style to an existing asset",
		Long:  "Re-style an existing asset using a brand profile or an explicit style (img2img with a style prompt, or a style-transfer model when one is available). Fails clearly when no style-capable model can be resolved.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if rf.outDir == "" {
				rf.outDir = "restyle"
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, rf.brand)
			var body brandProfileBody
			if brandName != "" {
				if _, b, e := loadBrandProfile(brandName); e == nil {
					body = b
				}
			}

			model := rf.model
			if model == "" && len(body.Models) > 0 {
				model = body.Models[0]
			}
			if model == "" {
				return notFoundErr(fmt.Errorf("no style-capable model resolved; pass --model or apply a brand with models. style transfer is unavailable without a model"))
			}

			stylePrompt := buildStylePrompt(rf.style, body)
			if stylePrompt == "" {
				return usageErr(fmt.Errorf("nothing to restyle with; pass --style or use a brand with style anchors"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			env := newEnvelope("restyle")
			if flags.dryRun {
				env.DryRun = true
				env.Results = []any{map[string]any{"source": source, "model": model, "style_prompt": stylePrompt}}
				return emitEnvelope(cmd.OutOrStdout(), env)
			}

			inputs := map[string]any{"prompt": stylePrompt}
			if looksLikeURL(source) {
				inputs["image"] = source
			} else {
				env.Warnings = append(env.Warnings, "source is a local path, not a URL; img2img needs an uploaded image URL — upload it first or pass a URL")
			}
			spec := filepath.Join(rf.outDir, "restyled.{ext}")
			res, err := submitAndAwait(cmd.Context(), c, submitRequest{
				modelID: model, inputs: inputs, estimatePrice: true, priceBestEffort: true,
				wait: true, waitTimeout: 5 * time.Minute, pollInitial: 2 * time.Second,
				download: true, downloadSpec: spec,
			})
			oc := shotOutcome{Shot: Shot{Prompt: stylePrompt, Model: model}, Files: []string{}}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			oc.Cost = extractCostFromPricing(res.Pricing)
			oc.ContentHash = hashContent(res.Result)
			for _, d := range res.Downloads {
				oc.Files = append(oc.Files, d.Path)
			}
			if res.Failed {
				oc.Err = fmt.Sprintf("prediction failed with status %q", res.Status)
				env.PartialFailure = true
			}
			env.CostSpent = oc.Cost
			env.Results = []any{oc}

			if shouldRecord(project, true, rf.noRecord) && oc.Err == "" {
				g := store.Generation{ID: newGenerationID(), Command: "restyle", ModelID: model, Prompt: stylePrompt, BrandName: brandName, Cost: oc.Cost, ContentHash: oc.ContentHash, Status: "completed"}
				if len(oc.Files) > 0 {
					g.Path = oc.Files[0]
				}
				if rerr := recordGeneration(g); rerr != nil {
					env.LibraryRecordErrors = append(env.LibraryRecordErrors, rerr.Error())
				}
			}
			env.RecommendedAction = "pack the restyled asset for posting"
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&rf.brand, "brand", "", "Brand profile whose style to apply (defaults to active brand)")
	cmd.Flags().StringVar(&rf.style, "style", "", "Explicit style descriptor")
	cmd.Flags().StringVar(&rf.model, "model", "", "Model ID (defaults to brand's first model)")
	cmd.Flags().BoolVar(&rf.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().StringVar(&rf.outDir, "out-dir", "restyle", "Directory for outputs")
	return cmd
}

func buildStylePrompt(style string, body brandProfileBody) string {
	parts := []string{}
	if strings.TrimSpace(style) != "" {
		parts = append(parts, strings.TrimSpace(style))
	}
	parts = append(parts, body.StyleAnchors...)
	if len(body.Palette) > 0 {
		parts = append(parts, "palette: "+strings.Join(body.Palette, ", "))
	}
	return strings.Join(parts, ", ")
}
