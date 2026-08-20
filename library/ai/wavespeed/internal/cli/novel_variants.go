// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

type variantsFlags struct {
	base        string
	prompt      string
	model       string
	vary        string
	count       int
	values      []string
	maxCost     float64
	concurrency int
	noRecord    bool
	outDir      string
	brand       string
}

func newVariantsCmd(flags *rootFlags) *cobra.Command {
	var vf variantsFlags
	cmd := &cobra.Command{
		Use:   "variants",
		Short: "Produce controlled variations of one base shot",
		Long:  "Sweep one dimension (seed, style, or model) off a base shot to produce comparable outputs with side-by-side metadata an agent can pick from.",
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := variantBaseShot(vf)
			if err != nil {
				return usageErr(err)
			}
			vary := strings.ToLower(strings.TrimSpace(vf.vary))
			if vary == "" {
				vary = "seed"
			}
			if vf.concurrency < 1 {
				vf.concurrency = 1
			}
			if vf.concurrency > 10 {
				vf.concurrency = 10
			}
			if vf.outDir == "" {
				vf.outDir = "variants"
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, vf.brand)
			if brandName != "" {
				if _, body, e := loadBrandProfile(brandName); e == nil {
					base = mergeBrandIntoShot(base, brandName, body)
				}
			}

			shots, err := buildVariantShots(base, vary, vf)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				env := newEnvelope("variants")
				env.DryRun = true
				for i, s := range shots {
					env.Results = append(env.Results, map[string]any{
						"variant": i, "vary": vary, "model": s.Model, "seed": s.Seed, "params": s.toModelInputs(),
					})
				}
				return emitEnvelope(cmd.OutOrStdout(), env)
			}

			return variantsExecute(cmd, c, project, vf, vary, shots, brandName)
		},
	}
	cmd.Flags().StringVar(&vf.base, "base", "", "Base shotlist file (first shot is the base)")
	cmd.Flags().StringVar(&vf.prompt, "prompt", "", "Base prompt (alternative to --base)")
	cmd.Flags().StringVar(&vf.model, "model", "", "Base model")
	cmd.Flags().StringVar(&vf.vary, "vary", "seed", "Dimension to sweep: seed|style|model")
	cmd.Flags().IntVar(&vf.count, "count", 4, "Number of variants (seed sweep)")
	cmd.Flags().StringSliceVar(&vf.values, "values", nil, "Explicit values for style/model sweeps")
	cmd.Flags().Float64Var(&vf.maxCost, "max-cost", 0, "Spend ceiling across variants")
	cmd.Flags().IntVar(&vf.concurrency, "concurrency", 4, "Concurrent submissions (clamped 1-10)")
	cmd.Flags().BoolVar(&vf.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().StringVar(&vf.outDir, "out-dir", "variants", "Directory for downloaded outputs")
	cmd.Flags().StringVar(&vf.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	return cmd
}

func variantBaseShot(vf variantsFlags) (Shot, error) {
	if vf.base != "" {
		shots, err := readShotlist(vf.base)
		if err != nil {
			return Shot{}, err
		}
		if len(shots) == 0 {
			return Shot{}, fmt.Errorf("base shotlist is empty")
		}
		base := shots[0]
		if vf.model != "" {
			base.Model = vf.model
		}
		return base, nil
	}
	if strings.TrimSpace(vf.prompt) == "" {
		return Shot{}, fmt.Errorf("provide --base <file> or --prompt")
	}
	return Shot{Prompt: vf.prompt, Model: vf.model}, nil
}

func buildVariantShots(base Shot, vary string, vf variantsFlags) ([]Shot, error) {
	var shots []Shot
	clone := func() Shot {
		c := base
		c.Params = map[string]any{}
		for k, v := range base.Params {
			c.Params[k] = v
		}
		return c
	}
	switch vary {
	case "seed":
		n := vf.count
		if n < 1 {
			n = 1
		}
		for i := 0; i < n; i++ {
			s := clone()
			seed := int64(i + 1)
			s.Seed = &seed
			shots = append(shots, s)
		}
	case "style":
		if len(vf.values) == 0 {
			return nil, fmt.Errorf("style sweep needs --values style1,style2,...")
		}
		for _, style := range vf.values {
			s := clone()
			s.Prompt = strings.TrimSpace(base.Prompt + ", " + style)
			shots = append(shots, s)
		}
	case "model":
		if len(vf.values) == 0 {
			return nil, fmt.Errorf("model sweep needs --values model1,model2,...")
		}
		for _, m := range vf.values {
			s := clone()
			s.Model = m
			shots = append(shots, s)
		}
	default:
		return nil, fmt.Errorf("invalid --vary %q (want seed|style|model)", vary)
	}
	return shots, nil
}

func variantsExecute(cmd *cobra.Command, c *client.Client, project wavespeedProjectConfig, vf variantsFlags, vary string, shots []Shot, brandName string) error {
	ctx := cmd.Context()
	recordEnabled := shouldRecord(project, true, vf.noRecord)
	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		spent      float64
		aborted    bool
		anyFailed  bool
		recordErrs []string
		results    = make([]shotOutcome, len(shots))
		sem        = make(chan struct{}, vf.concurrency)
	)

	for i := range shots {
		sem <- struct{}{}
		mu.Lock()
		stop := aborted || (vf.maxCost > 0 && spent >= vf.maxCost)
		mu.Unlock()
		if stop {
			<-sem
			results[i] = shotOutcome{Shot: shots[i], Skipped: true, Err: "skipped: cost ceiling"}
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			s := shots[i]
			spec := filepath.Join(vf.outDir, fmt.Sprintf("variant-%03d.{ext}", i))
			res, err := submitAndAwait(ctx, c, submitRequest{
				modelID: s.Model, inputs: s.toModelInputs(), estimatePrice: true, priceBestEffort: true,
				wait: true, waitTimeout: 5 * time.Minute, pollInitial: 2 * time.Second,
				download: true, downloadSpec: spec,
			})
			oc := shotOutcome{Shot: s, Files: []string{}, SeedUsed: s.Seed}
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
			mu.Lock()
			spent += oc.Cost
			if vf.maxCost > 0 && spent >= vf.maxCost {
				aborted = true
			}
			if oc.Err != "" {
				anyFailed = true
			}
			results[i] = oc
			mu.Unlock()

			if recordEnabled && oc.Err == "" {
				g := store.Generation{ID: newGenerationID(), Command: "variants", ModelID: s.Model, Prompt: s.Prompt, BrandName: brandName, Seed: oc.SeedUsed, Cost: oc.Cost, ContentHash: oc.ContentHash, Status: "completed"}
				if len(oc.Files) > 0 {
					g.Path = oc.Files[0]
				}
				if rerr := recordGeneration(g); rerr != nil {
					mu.Lock()
					recordErrs = append(recordErrs, rerr.Error())
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	env := newEnvelope("variants")
	env.CostSpent = spent
	env.LibraryRecordErrors = recordErrs
	for i := range results {
		env.Results = append(env.Results, map[string]any{"variant": i, "vary": vary, "outcome": results[i]})
	}
	if anyFailed {
		env.PartialFailure = true
		env.RecommendedAction = "one or more variants failed; surviving variants are recorded and available to compare"
	} else {
		env.RecommendedAction = "compare variants and pick one to scale via pack"
	}
	return emitEnvelope(cmd.OutOrStdout(), env)
}
