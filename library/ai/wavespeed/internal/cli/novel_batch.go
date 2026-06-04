// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

type batchFlags struct {
	from         string
	maxCost      float64
	failTolerant bool
	concurrency  int
	noRecord     bool
	outDir       string
	model        string
	brand        string
}

func newBatchCmd(flags *rootFlags) *cobra.Command {
	var bf batchFlags
	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Submit many prompts from a CSV or JSON file",
		Long:  "Submit N prompts from a CSV (prompt[,model][,platform] columns) or JSON shotlist. Honors a spend ceiling and fail-fast (default) or fail-tolerant semantics; completed generations are recorded to the library before any abort.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(bf.from) == "" {
				return usageErr(fmt.Errorf("--from <file.csv|file.json> is required"))
			}
			if bf.concurrency < 1 {
				bf.concurrency = 1
			}
			if bf.concurrency > 10 {
				bf.concurrency = 10
			}
			if bf.outDir == "" {
				bf.outDir = "batch"
			}
			shots, err := readBatchInput(bf.from)
			if err != nil {
				return usageErr(err)
			}
			if len(shots) == 0 {
				return usageErr(fmt.Errorf("no prompts found in %s", bf.from))
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, bf.brand)
			var body brandProfileBody
			if brandName != "" {
				if _, b, e := loadBrandProfile(brandName); e == nil {
					body = b
				}
			}
			for i := range shots {
				if shots[i].Model == "" && bf.model != "" {
					shots[i].Model = bf.model
				}
				if brandName != "" {
					shots[i] = mergeBrandIntoShot(shots[i], brandName, body)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				env := newEnvelope("batch")
				env.DryRun = true
				var total float64
				for i := range shots {
					var cost float64
					if shots[i].Model != "" {
						if p, st := priceShot(cmd.Context(), c, shots[i].Model, shots[i].toModelInputs()); st == priceOK || st == priceCached {
							cost = p
						}
					}
					total += cost
					env.Results = append(env.Results, map[string]any{"shot": i, "model": shots[i].Model, "prompt": shots[i].Prompt, "estimated_cost": cost})
				}
				env.CostSpent = total
				return emitEnvelope(cmd.OutOrStdout(), env)
			}

			return batchExecute(cmd, c, project, bf, shots, brandName)
		},
	}
	cmd.Flags().StringVar(&bf.from, "from", "", "CSV or JSON file of prompts/shots")
	cmd.Flags().Float64Var(&bf.maxCost, "max-cost", 0, "Spend ceiling across the batch")
	cmd.Flags().BoolVar(&bf.failTolerant, "fail-tolerant", false, "Continue after a shot fails (default is fail-fast)")
	cmd.Flags().Bool("fail-fast", true, "Abort the batch on the first failure (default)")
	cmd.Flags().IntVar(&bf.concurrency, "concurrency", 4, "Concurrent submissions (clamped 1-10)")
	cmd.Flags().BoolVar(&bf.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().StringVar(&bf.outDir, "out-dir", "batch", "Directory for downloaded outputs")
	cmd.Flags().StringVar(&bf.model, "model", "", "Default model for shots that don't specify one")
	cmd.Flags().StringVar(&bf.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	return cmd
}

// readBatchInput parses a JSON shotlist or a CSV with a prompt column.
func readBatchInput(path string) ([]Shot, error) {
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		return readShotlist(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	// Header detection: if the first row contains "prompt", treat as header.
	header := rows[0]
	startRow := 0
	col := map[string]int{}
	if containsFold(header, "prompt") {
		for i, h := range header {
			col[strings.ToLower(strings.TrimSpace(h))] = i
		}
		startRow = 1
	} else {
		col["prompt"] = 0
		if len(header) > 1 {
			col["model"] = 1
		}
		if len(header) > 2 {
			col["platform"] = 2
		}
	}
	var shots []Shot
	for _, row := range rows[startRow:] {
		get := func(k string) string {
			if idx, ok := col[k]; ok && idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}
		prompt := get("prompt")
		if prompt == "" {
			continue
		}
		shots = append(shots, Shot{Prompt: prompt, Model: get("model"), Platform: get("platform")})
	}
	return shots, nil
}

func containsFold(row []string, want string) bool {
	for _, s := range row {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}

func batchExecute(cmd *cobra.Command, c *client.Client, project wavespeedProjectConfig, bf batchFlags, shots []Shot, brandName string) error {
	ctx := cmd.Context()
	failFast := !bf.failTolerant
	recordEnabled := shouldRecord(project, true, bf.noRecord)

	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		spent      float64
		aborted    bool
		failure    bool
		recordErrs []string
		results    = make([]shotOutcome, len(shots))
		sem        = make(chan struct{}, bf.concurrency)
	)

	for i := range shots {
		sem <- struct{}{}
		mu.Lock()
		stop := aborted || (bf.maxCost > 0 && spent >= bf.maxCost)
		mu.Unlock()
		if stop {
			<-sem
			results[i] = shotOutcome{Shot: shots[i], Skipped: true, Err: "skipped: cost ceiling or prior failure"}
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			s := shots[i]
			spec := filepath.Join(bf.outDir, fmt.Sprintf("%03d.{ext}", i))
			res, err := submitAndAwait(ctx, c, submitRequest{
				modelID:       s.Model,
				inputs:        s.toModelInputs(),
				estimatePrice: true, priceBestEffort: true,
				wait:          true,
				waitTimeout:   5 * time.Minute,
				pollInitial:   2 * time.Second,
				download:      true,
				downloadSpec:  spec,
			})
			oc := shotOutcome{Shot: s, Files: []string{}}
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
			if bf.maxCost > 0 && spent >= bf.maxCost {
				aborted = true
			}
			if oc.Err != "" {
				failure = true
				if failFast {
					aborted = true
				}
			}
			results[i] = oc
			mu.Unlock()

			if recordEnabled && oc.Err == "" {
				g := store.Generation{
					ID: newGenerationID(), Command: "batch", ModelID: s.Model, Prompt: s.Prompt,
					PlatformTarget: s.Platform, BrandName: brandName, Cost: oc.Cost, ContentHash: oc.ContentHash, Status: "completed",
				}
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

	env := newEnvelope("batch")
	env.CostSpent = spent
	env.LibraryRecordErrors = recordErrs
	for i := range results {
		env.Results = append(env.Results, results[i])
	}
	costCeilingHit := bf.maxCost > 0 && spent >= bf.maxCost
	if failure || costCeilingHit {
		env.PartialFailure = true
		_ = emitEnvelope(cmd.OutOrStdout(), env)
		return partialFailureErr(fmt.Errorf("batch incomplete (failure=%v, cost-ceiling=%v, spent=%.2f)", failure, costCeilingHit, spent))
	}
	env.RecommendedAction = "wavespeed-pp-cli library list --since 1d"
	return emitEnvelope(cmd.OutOrStdout(), env)
}
