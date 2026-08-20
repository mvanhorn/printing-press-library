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

type composeFlags struct {
	steps    string
	prompt   string
	models   []string
	noRecord bool
	outDir   string
	brand    string
}

type composeStep struct {
	From  string
	To    string
	Model string
}

func newComposeCmd(flags *rootFlags) *cobra.Command {
	var cf composeFlags
	cmd := &cobra.Command{
		Use:   "compose",
		Short: "Run an explicit multi-step generation pipeline",
		Long:  "Chain steps (e.g. text->image,image->upscale,image->video), feeding each step's output into the next. A step failure rolls back later steps and records the completed steps.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(cf.prompt) == "" {
				return usageErr(fmt.Errorf("--prompt is required"))
			}
			if cf.outDir == "" {
				cf.outDir = "compose"
			}
			steps, err := parseComposeSteps(cf.steps, cf.models)
			if err != nil {
				return usageErr(err)
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, cf.brand)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			env := newEnvelope("compose")
			if flags.dryRun {
				env.DryRun = true
				for i, st := range steps {
					env.Results = append(env.Results, map[string]any{"step": i, "from": st.From, "to": st.To, "model": st.Model})
				}
				env.RecommendedAction = "drop --dry-run to run the pipeline"
				return emitEnvelope(cmd.OutOrStdout(), env)
			}

			recordEnabled := shouldRecord(project, true, cf.noRecord)
			ctx := cmd.Context()
			prevURL := ""
			var stepFailed bool
			for i, st := range steps {
				inputs := map[string]any{"prompt": cf.prompt}
				if i > 0 && prevURL != "" {
					inputs[stepInputKey(st.From)] = prevURL
				}
				spec := filepath.Join(cf.outDir, fmt.Sprintf("step-%02d-%s.{ext}", i, dirSafe(st.To)))
				res, err := submitAndAwait(ctx, c, submitRequest{
					modelID: st.Model, inputs: inputs, estimatePrice: true, priceBestEffort: true,
					wait: true, waitTimeout: 5 * time.Minute, pollInitial: 2 * time.Second,
					download: true, downloadSpec: spec,
				})
				oc := shotOutcome{Shot: Shot{Prompt: cf.prompt, Model: st.Model}, Files: []string{}}
				if err != nil {
					oc.Err = err.Error()
				} else {
					oc.Cost = extractCostFromPricing(res.Pricing)
					oc.ContentHash = hashContent(res.Result)
					for _, d := range res.Downloads {
						oc.Files = append(oc.Files, d.Path)
					}
					if urls := collectURLStrings(unwrapWaveSpeedData(res.Result)); len(urls) > 0 {
						prevURL = urls[0]
					}
					if res.Failed {
						oc.Err = fmt.Sprintf("prediction failed with status %q", res.Status)
					}
				}
				env.Results = append(env.Results, map[string]any{"step": i, "from": st.From, "to": st.To, "model": st.Model, "outcome": oc})
				env.CostSpent += oc.Cost

				if recordEnabled && oc.Err == "" {
					g := store.Generation{ID: newGenerationID(), Command: "compose", ModelID: st.Model, Prompt: cf.prompt, BrandName: brandName, Cost: oc.Cost, ContentHash: oc.ContentHash, Status: "completed"}
					if len(oc.Files) > 0 {
						g.Path = oc.Files[0]
					}
					if rerr := recordGeneration(g); rerr != nil {
						env.LibraryRecordErrors = append(env.LibraryRecordErrors, rerr.Error())
					}
				}

				if oc.Err != "" {
					stepFailed = true
					env.Warnings = append(env.Warnings, fmt.Sprintf("step %d (%s->%s) failed; rolled back %d later step(s)", i, st.From, st.To, len(steps)-i-1))
					break
				}
			}

			if stepFailed {
				env.PartialFailure = true
				env.RecommendedAction = "fix the failed step's model/inputs and re-run; completed steps are recorded"
				_ = emitEnvelope(cmd.OutOrStdout(), env)
				return partialFailureErr(fmt.Errorf("compose pipeline stopped at a failed step"))
			}
			env.RecommendedAction = "the final step output is your deliverable"
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&cf.steps, "steps", "text->image,image->video", "Pipeline steps, e.g. text->image,image->upscale,image->video")
	cmd.Flags().StringVar(&cf.prompt, "prompt", "", "Prompt for the first step")
	cmd.Flags().StringSliceVar(&cf.models, "models", nil, "Model per step, positional to --steps")
	cmd.Flags().BoolVar(&cf.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().StringVar(&cf.outDir, "out-dir", "compose", "Directory for step outputs")
	cmd.Flags().StringVar(&cf.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	return cmd
}

// parseComposeSteps parses "a->b,b->c" into ordered steps, aligning a model to
// each step positionally. When fewer models than steps are given, the last
// model is reused; an empty model list yields steps with empty models (which
// will fail at submission with a clear API error).
func parseComposeSteps(spec string, models []string) ([]composeStep, error) {
	parts := strings.Split(spec, ",")
	var steps []composeStep
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fromTo := strings.SplitN(p, "->", 2)
		if len(fromTo) != 2 {
			return nil, fmt.Errorf("invalid step %q (want from->to)", p)
		}
		steps = append(steps, composeStep{
			From: strings.TrimSpace(fromTo[0]),
			To:   strings.TrimSpace(fromTo[1]),
		})
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps parsed from %q", spec)
	}
	for i := range steps {
		switch {
		case i < len(models):
			steps[i].Model = strings.TrimSpace(models[i])
		case len(models) > 0:
			steps[i].Model = strings.TrimSpace(models[len(models)-1])
		}
	}
	return steps, nil
}

// stepInputKey maps a step's source type to the input field the next step
// expects the upstream URL under.
func stepInputKey(from string) string {
	switch strings.ToLower(from) {
	case "video":
		return "video"
	case "image", "upscale":
		return "image"
	default:
		return "image"
	}
}
