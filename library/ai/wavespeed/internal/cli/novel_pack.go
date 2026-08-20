// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

type packFlags struct {
	concept     string
	platforms   []string
	aspects     []string
	concurrency int
	maxCost     float64
	onFailure   string
	noRecord    bool
	history     bool
	clean       bool
	strictVideo bool
	model       string
	brand       string
	seed        int64
	outDir      string
}

// shotOutcome is the per-shot result of a pack run.
type shotOutcome struct {
	Shot        Shot     `json:"shot"`
	Platform    string   `json:"platform,omitempty"`
	Format      string   `json:"format,omitempty"`
	Files       []string `json:"files"`
	Manifest    string   `json:"manifest,omitempty"`
	Cost        float64  `json:"cost"`
	ContentHash string   `json:"content_hash,omitempty"`
	Dimensions  string   `json:"dimensions,omitempty"`
	SeedUsed    *int64   `json:"seed_used,omitempty"`
	Skipped     bool     `json:"skipped,omitempty"`
	Warning     string   `json:"warning,omitempty"`
	Err         string   `json:"error,omitempty"`
}

// platformManifest is the contract a downstream social-posting tool consumes.
type platformManifest struct {
	Platform       string   `json:"platform"`
	Format         string   `json:"format"`
	Dimensions     string   `json:"dimensions"`
	DurationCapSec int      `json:"duration_cap_sec"`
	CaptionHint    string   `json:"caption_hint"`
	HashtagSlots   int      `json:"hashtag_slots"`
	Files          []string `json:"files"`
	Model          string   `json:"model"`
	Cost           float64  `json:"cost"`
	Brand          string   `json:"brand,omitempty"`
	SeedUsed       *int64   `json:"seed_used,omitempty"`
	ContentHash    string   `json:"content_hash,omitempty"`
	// Assets carries per-asset detail when a platform has multiple aspect
	// variants in one manifest. Single-asset packs also populate the top-level
	// Dimensions/SeedUsed/ContentHash for back-compat.
	Assets []manifestAsset `json:"assets,omitempty"`
}

// manifestAsset is one produced file set within a platform manifest.
type manifestAsset struct {
	Files       []string `json:"files"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Dimensions  string   `json:"dimensions,omitempty"`
	ContentHash string   `json:"content_hash,omitempty"`
	SeedUsed    *int64   `json:"seed_used,omitempty"`
	Cost        float64  `json:"cost"`
}

func newPackCmd(flags *rootFlags) *cobra.Command {
	var pf packFlags
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Produce a multi-platform creative pack from one concept",
		Long:  "Generate a full creative pack for one concept across platforms and aspect ratios, writing post-ready files at stable packs/<slug>/<platform>/ paths plus a per-platform manifest a downstream posting tool consumes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(pf.concept) == "" {
				return usageErr(fmt.Errorf("--concept is required"))
			}
			if pf.concurrency < 1 {
				pf.concurrency = 1
			}
			if pf.concurrency > 10 {
				pf.concurrency = 10
			}
			if pf.onFailure == "" {
				pf.onFailure = "abort"
			}
			if pf.onFailure != "abort" && pf.onFailure != "continue" {
				return usageErr(fmt.Errorf("--on-failure must be abort or continue"))
			}
			if pf.outDir == "" {
				pf.outDir = "packs"
			}

			project, _ := loadWavespeedProjectConfig()
			brandName := resolveActiveBrand(project, pf.brand)
			var body brandProfileBody
			brandID := ""
			if brandName != "" {
				if prof, b, err := loadBrandProfile(brandName); err == nil {
					body = b
					brandID = prof.ID
				}
			}
			shots := buildPackShots(pf, brandName, body)
			if len(shots) == 0 {
				return usageErr(fmt.Errorf("no shots to produce; pass --platforms and/or --aspects"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Default max-cost from balance * 0.25 when not set.
			if !cmd.Flags().Changed("max-cost") {
				if bal := fetchBalance(cmd.Context(), c); bal != nil {
					pf.maxCost = *bal * 0.25
				}
			}

			slug := slugify(pf.concept)
			if flags.dryRun {
				return packDryRun(cmd, c, pf, slug, shots, brandName)
			}
			return packExecute(cmd, c, project, pf, slug, shots, brandName, brandID)
		},
	}
	cmd.Flags().StringVar(&pf.concept, "concept", "", "The single creative concept to expand into a pack")
	cmd.Flags().StringSliceVar(&pf.platforms, "platforms", nil, "Target platforms (instagram,tiktok,facebook,x)")
	cmd.Flags().StringSliceVar(&pf.aspects, "aspects", nil, "Explicit aspect ratios, composable with --platforms")
	cmd.Flags().IntVar(&pf.concurrency, "concurrency", 4, "Concurrent submissions (clamped 1-10)")
	cmd.Flags().Float64Var(&pf.maxCost, "max-cost", 0, "Spend ceiling (default: balance * 0.25)")
	cmd.Flags().StringVar(&pf.onFailure, "on-failure", "abort", "On shot failure: abort|continue")
	cmd.Flags().BoolVar(&pf.noRecord, "no-record", false, "Do not record generations to the library")
	cmd.Flags().BoolVar(&pf.history, "history", false, "Also write a timestamped snapshot under packs/<slug>-history/<ts>/")
	cmd.Flags().BoolVar(&pf.clean, "clean", false, "Delete leftover files in each platform dir before writing")
	cmd.Flags().BoolVar(&pf.strictVideo, "strict-video", false, "Validate video duration/dims with ffprobe if installed")
	cmd.Flags().StringVar(&pf.model, "model", "", "Model ID for all shots (overrides brand default)")
	cmd.Flags().StringVar(&pf.brand, "brand", "", "Brand profile to merge (defaults to active brand)")
	cmd.Flags().Int64Var(&pf.seed, "seed", 0, "Seed for reproducible content (model must support it)")
	cmd.Flags().StringVar(&pf.outDir, "out-dir", "packs", "Base directory for pack output")
	return cmd
}

// buildPackShots expands the concept across platforms and aspect ratios.
func buildPackShots(pf packFlags, brandName string, body brandProfileBody) []Shot {
	platforms := normalizePlatforms(pf.platforms)
	aspects := pf.aspects
	mk := func(platform, format, aspect string) Shot {
		s := Shot{Concept: pf.concept, Prompt: pf.concept, Platform: platform, Format: format, AspectRatio: aspect, Model: pf.model}
		if pf.seed != 0 {
			sd := pf.seed
			s.Seed = &sd
		}
		if brandName != "" {
			s = mergeBrandIntoShot(s, brandName, body)
		}
		if aspect != "" {
			if s.Params == nil {
				s.Params = map[string]any{}
			}
			if _, ok := s.Params["aspect_ratio"]; !ok {
				s.Params["aspect_ratio"] = aspect
			}
		}
		return s
	}
	var shots []Shot
	switch {
	case len(platforms) > 0:
		for _, p := range platforms {
			f, _ := defaultFormatFor(p)
			if len(aspects) > 0 {
				for _, a := range aspects {
					shots = append(shots, mk(p, f.Format, a))
				}
			} else {
				shots = append(shots, mk(p, f.Format, f.AspectRatio))
			}
		}
	case len(aspects) > 0:
		for _, a := range aspects {
			shots = append(shots, mk("", "", a))
		}
	default:
		shots = append(shots, mk("", "", ""))
	}
	return shots
}

func packDryRun(cmd *cobra.Command, c *client.Client, pf packFlags, slug string, shots []Shot, brandName string) error {
	env := newEnvelope("pack")
	env.DryRun = true
	var total float64
	for i := range shots {
		s := shots[i]
		platformDir := filepath.Join(pf.outDir, slug, dirSafe(s.Platform))
		var cost float64
		if s.Model != "" {
			if price, status := priceShot(cmd.Context(), c, s.Model, s.toModelInputs()); status == priceOK || status == priceCached {
				cost = price
			}
		}
		total += cost
		env.Results = append(env.Results, map[string]any{
			"shot":           i,
			"platform":       s.Platform,
			"format":         s.Format,
			"aspect_ratio":   s.AspectRatio,
			"model":          s.Model,
			"brand":          s.Brand,
			"planned_dir":    platformDir,
			"merged_params":  s.toModelInputs(),
			"estimated_cost": cost,
		})
	}
	env.CostSpent = total
	env.RecommendedAction = "drop --dry-run to produce the pack"
	return emitEnvelope(cmd.OutOrStdout(), env)
}

func packExecute(cmd *cobra.Command, c *client.Client, project wavespeedProjectConfig, pf packFlags, slug string, shots []Shot, brandName, brandID string) error {
	ctx := cmd.Context()

	// --clean runs ONCE per unique platform dir before any goroutine launches.
	// Cleaning inside produceShot would race: two shots sharing a platform dir
	// (e.g. multiple aspects for one platform) could delete each other's
	// freshly-written output.
	if pf.clean {
		cleaned := map[string]bool{}
		for i := range shots {
			dir := filepath.Join(pf.outDir, slug, dirSafe(shots[i].Platform))
			if cleaned[dir] {
				continue
			}
			cleaned[dir] = true
			if entries, err := os.ReadDir(dir); err == nil {
				for _, e := range entries {
					_ = os.Remove(filepath.Join(dir, e.Name()))
				}
			}
		}
	}

	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		spent    float64
		aborted  bool
		failure  bool
		outcomes = make([]shotOutcome, len(shots))
		sem      = make(chan struct{}, pf.concurrency)
	)

	recordEnabled := shouldRecord(project, true, pf.noRecord)
	var recordErrs []string

	for i := range shots {
		// Acquire the concurrency slot BEFORE the stop check so the check sees
		// the spend/abort state left by the shot that just freed the slot. With
		// concurrency 1 this makes the ceiling exact; at higher concurrency it
		// is necessarily approximate (in-flight shots may overshoot), which is
		// acceptable — completed shots are recorded before any abort.
		sem <- struct{}{}
		mu.Lock()
		stop := aborted || (pf.maxCost > 0 && spent >= pf.maxCost)
		mu.Unlock()
		if stop {
			<-sem
			outcomes[i] = shotOutcome{Shot: shots[i], Platform: shots[i].Platform, Skipped: true, Err: "skipped: cost ceiling or prior failure"}
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			oc := produceShot(ctx, c, pf, slug, shots[i])

			mu.Lock()
			spent += oc.Cost
			if pf.maxCost > 0 && spent >= pf.maxCost {
				aborted = true
			}
			if oc.Err != "" && pf.onFailure == "abort" {
				aborted = true
				failure = true
			} else if oc.Err != "" {
				failure = true
			}
			outcomes[i] = oc
			mu.Unlock()

			// Record completed shots regardless of later abort. A record
			// failure is logged and never fails the generation.
			if recordEnabled && oc.Err == "" {
				if rerr := recordPackShot(oc, shots[i], brandName, brandID); rerr != nil {
					mu.Lock()
					recordErrs = append(recordErrs, rerr.Error())
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()

	// Write per-platform manifests for produced platforms.
	manifests := writePlatformManifests(pf, slug, shots, outcomes)

	env := newEnvelope("pack")
	env.CostSpent = spent
	env.BalanceAfter = fetchBalance(ctx, c)
	env.LibraryRecordErrors = recordErrs
	costCeilingHit := pf.maxCost > 0 && spent >= pf.maxCost
	for i := range outcomes {
		env.Results = append(env.Results, outcomes[i])
		if outcomes[i].Warning != "" {
			env.Warnings = append(env.Warnings, outcomes[i].Warning)
		}
	}
	env.Manifests = manifests

	if failure || costCeilingHit {
		env.PartialFailure = true
		env.RecommendedAction = "review skipped/failed shots; re-run with --on-failure continue or a higher --max-cost"
		_ = emitEnvelope(cmd.OutOrStdout(), env)
		if costCeilingHit {
			return partialFailureErr(fmt.Errorf("pack stopped at cost ceiling %.2f (spent %.2f)", pf.maxCost, spent))
		}
		return partialFailureErr(fmt.Errorf("one or more shots failed with --on-failure=%s", pf.onFailure))
	}

	env.RecommendedAction = "post the pack via your social-posting tool using the per-platform manifests"
	env.SuggestedNext = suggestNext(
		fmt.Sprintf("cat %s/%s/<platform>/manifest.json", pf.outDir, slug),
		"wavespeed-pp-cli library list --since 1d",
	)
	return emitEnvelope(cmd.OutOrStdout(), env)
}

// produceShot submits one shot, polls, downloads into the platform dir, and
// validates image dimensions. Returns a shotOutcome (never panics).
func produceShot(ctx context.Context, c *client.Client, pf packFlags, slug string, s Shot) shotOutcome {
	oc := shotOutcome{Shot: s, Platform: s.Platform, Format: s.Format, Files: []string{}, SeedUsed: s.Seed}
	platformDir := filepath.Join(pf.outDir, slug, dirSafe(s.Platform))

	// --clean is handled once per platform dir in packExecute before launch.

	fileBase := s.Format
	if fileBase == "" {
		fileBase = "asset"
	}
	// When multiple aspect ratios target the same platform, the format alone
	// (e.g. "feed") collides — every shot would write feed.{ext}. Disambiguate
	// by aspect so each asset gets a distinct path. The common single-aspect
	// case keeps the clean "feed.png" name.
	if len(pf.aspects) > 0 && s.AspectRatio != "" {
		fileBase += "-" + strings.ReplaceAll(s.AspectRatio, ":", "x")
	}
	spec := filepath.Join(platformDir, fileBase+".{ext}")

	res, err := submitAndAwait(ctx, c, submitRequest{
		modelID:      s.Model,
		inputs:       s.toModelInputs(),
		estimatePrice: true, priceBestEffort: true,
		wait:         true,
		waitTimeout:  5 * time.Minute,
		pollInitial:  2 * time.Second,
		download:     true,
		downloadSpec: spec,
	})
	if err != nil {
		oc.Err = err.Error()
		return oc
	}
	oc.Cost = extractCostFromPricing(res.Pricing)
	oc.ContentHash = hashContent(res.Result)
	if res.Failed {
		oc.Err = fmt.Sprintf("prediction failed with status %q", res.Status)
		return oc
	}
	for _, d := range res.Downloads {
		oc.Files = append(oc.Files, d.Path)
	}
	// Inspection-light image-dimension validation.
	if dims, warn := validateImageDims(oc.Files, s); dims != "" {
		oc.Dimensions = dims
		oc.Warning = warn
	}
	return oc
}

// validateImageDims reads PNG/JPEG header dimensions and compares to the
// expected platform format size. Mismatch → advisory warning; the actual dims
// are recorded. Returns dims string ("WxH") and a warning if mismatched.
func validateImageDims(files []string, s Shot) (string, string) {
	expectW, expectH := 0, 0
	if spec, ok := lookupPlatform(s.Platform); ok {
		if f, ok := formatFor(spec, s.Format); ok && !f.IsVideo {
			expectW, expectH = f.Width, f.Height
		}
	}
	for _, p := range files {
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err != nil {
			continue
		}
		dims := fmt.Sprintf("%dx%d", cfg.Width, cfg.Height)
		if expectW > 0 && (cfg.Width != expectW || cfg.Height != expectH) {
			return dims, fmt.Sprintf("%s: model returned %s, expected %dx%d", filepath.Base(p), dims, expectW, expectH)
		}
		return dims, ""
	}
	return "", ""
}

// writePlatformManifests writes one manifest.json per produced platform dir,
// grouping all of that platform's shots (e.g. multiple aspect variants) into a
// single manifest so a later aspect doesn't clobber an earlier one's manifest.
// Returns the list of manifest paths written.
func writePlatformManifests(pf packFlags, slug string, shots []Shot, outcomes []shotOutcome) []string {
	type entry struct {
		shot Shot
		oc   shotOutcome
	}
	byPlatform := map[string][]entry{}
	order := []string{}
	for i := range outcomes {
		oc := outcomes[i]
		if oc.Skipped || oc.Err != "" || len(oc.Files) == 0 {
			continue
		}
		p := shots[i].Platform
		if _, seen := byPlatform[p]; !seen {
			order = append(order, p)
		}
		byPlatform[p] = append(byPlatform[p], entry{shot: shots[i], oc: oc})
	}

	written := []string{}
	for _, p := range order {
		entries := byPlatform[p]
		spec, _ := lookupPlatform(p)
		f, _ := formatFor(spec, entries[0].shot.Format)
		man := platformManifest{
			Platform:       p,
			Format:         f.Format,
			DurationCapSec: f.DurationCapSec,
			CaptionHint:    f.CaptionHint,
			HashtagSlots:   f.HashtagSlots,
			Model:          entries[0].shot.Model,
			Brand:          entries[0].shot.Brand,
		}
		for _, e := range entries {
			man.Files = append(man.Files, e.oc.Files...)
			man.Cost += e.oc.Cost
			man.Assets = append(man.Assets, manifestAsset{
				Files:       e.oc.Files,
				AspectRatio: e.shot.AspectRatio,
				Dimensions:  e.oc.Dimensions,
				ContentHash: e.oc.ContentHash,
				SeedUsed:    e.oc.SeedUsed,
				Cost:        e.oc.Cost,
			})
		}
		// For the single-asset common case, surface the asset's details at the
		// top level too (back-compat with one-shot-per-platform consumers).
		if len(entries) == 1 {
			man.Dimensions = entries[0].oc.Dimensions
			man.ContentHash = entries[0].oc.ContentHash
			man.SeedUsed = entries[0].oc.SeedUsed
		}
		dir := filepath.Join(pf.outDir, slug, dirSafe(p))
		path := filepath.Join(dir, "manifest.json")
		if writeJSONFile(path, man) == nil {
			written = append(written, path)
		}
		if pf.history {
			ts := time.Now().UTC().Format("20060102-150405")
			histDir := filepath.Join(pf.outDir, slug+"-history", ts, dirSafe(p))
			_ = writeJSONFile(filepath.Join(histDir, "manifest.json"), man)
		}
	}
	return written
}

func recordPackShot(oc shotOutcome, s Shot, brandName, brandID string) error {
	params, _ := json.Marshal(s.toModelInputs())
	g := store.Generation{
		ID:             newGenerationID(),
		Command:        "pack",
		BrandName:      brandName,
		BrandProfileID: brandID,
		PlatformTarget: s.Platform,
		ModelID:        s.Model,
		Prompt:         s.Prompt,
		AspectRatio:    s.AspectRatio,
		Seed:           oc.SeedUsed,
		Cost:           oc.Cost,
		ContentHash:    oc.ContentHash,
		Status:         "completed",
		Params:         json.RawMessage(params),
	}
	if len(oc.Files) > 0 {
		g.Path = oc.Files[0]
	}
	return recordGeneration(g)
}

func writeJSONFile(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "pack"
	}
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

// dirSafe returns a directory-safe platform segment, defaulting to "general"
// for shots with no platform.
func dirSafe(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return "general"
	}
	return slugify(platform)
}
