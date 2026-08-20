package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		prompt       string
		modelsCSV    string
		duration     int
		resolution   string
		aspectRatio  string
		projectID    string
		wait         bool
		downloadDir  string
		pollInterval time.Duration
		timeout      time.Duration
	)
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Run one prompt across multiple models in parallel for side-by-side A/B testing",
		Long: `Submit the same prompt to N OpenArt models in parallel, optionally wait
for completion, and return a side-by-side report (model, status, cost,
duration, URL, thumbnail).

The cost column comes from the local model catalog; URLs come from the
real /suite/api/resources/<id> polling for each submission.`,
		Example: `  # Compare three video models on the same prompt
  openart-pp-cli compare --prompt "neon koi swimming through clouds" --models seedance2,kling2-6,grok-imagine --duration 5

  # Wait for completion and download all winners
  openart-pp-cli compare --prompt "..." --models seedance2,grok --wait --download ./compare/`,
		Annotations: map[string]string{
			// Spends credits — not read-only.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" || modelsCSV == "" {
				return cmd.Help()
			}

			modelsList, err := resolveModelList(modelsCSV)
			if err != nil {
				return err
			}
			if len(modelsList) < 2 {
				return fmt.Errorf("--models must list at least 2 models (got %d)", len(modelsList))
			}

			// Per-model preflight: image-family models have
			// DurationMin/Max=0 and use PixelResolutions; video models use
			// Resolutions. Treat each family on its own terms so image
			// models don't get rejected with "outside range (0-0)" or
			// "resolution not supported" against a nil slice.
			plans := make([]submitPlan, 0, len(modelsList))
			totalEstimate := 0
			for _, m := range modelsList {
				isImage := m.Family == openartmodels.FamilyImage
				d := duration
				if !isImage {
					if d <= 0 {
						d = (m.DurationMinSec + m.DurationMaxSec) / 2
						if d == 0 {
							d = 5
						}
					}
					if d < m.DurationMinSec || d > m.DurationMaxSec {
						return fmt.Errorf("--duration %ds outside %s range (%d-%d)", d, m.DisplayName, m.DurationMinSec, m.DurationMaxSec)
					}
				} else {
					d = 0
				}
				// PATCH(compare-resolution-opt-in): only enforce the resolution
				// preflight when the user explicitly passed --resolution. The
				// default "720p" is a sensible video default but image-family
				// PixelResolutions are pixel shapes ("1024x1024"), so the
				// implicit "720p" rejected every image model from `compare
				// --models seedance2,nano-banana` even though the flag was
				// never passed. Mirrors the same fix in models_novel.go.
				// Greptile P1 on PR #554.
				if cmd.Flags().Changed("resolution") && resolution != "" {
					supported := m.Resolutions
					if isImage {
						supported = m.PixelResolutions
					}
					if len(supported) > 0 && !modelSupports(supported, resolution) {
						return fmt.Errorf("resolution %q not supported by %s; supported: %s", resolution, m.DisplayName, strings.Join(supported, ", "))
					}
				}
				est := m.EstimateCredits(d, 1, resolution)
				totalEstimate += est
				plans = append(plans, submitPlan{Model: m, Duration: d, EstimateCredits: est})
			}

			if cliutil.IsVerifyEnv() || flags.dryRun {
				out := map[string]any{
					"would_compare":          true,
					"prompt":                 prompt,
					"models":                 modelsCSV,
					"resolution":             resolution,
					"per_model_plans":        plans,
					"total_estimate_credits": totalEstimate,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if projectID == "" {
				resolved, derr := resolveDefaultProject(c)
				if derr != nil {
					return fmt.Errorf("resolve default project: %w", derr)
				}
				projectID = resolved
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "compare: %d models, total estimate %d credits\n", len(plans), totalEstimate)

			// Submit in parallel.
			type submitResult struct {
				plan      submitPlan
				historyID string
				resID     string
				err       error
			}
			submits := make([]submitResult, len(plans))
			var wg sync.WaitGroup
			for i := range plans {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					p := plans[i]
					isImage := p.Model.Family == openartmodels.FamilyImage
					formType := openartmodels.FormText2Video
					var body map[string]any
					if isImage {
						formType = openartmodels.FormText2Image
						imgAspect := aspectRatio
						if imgAspect == "" {
							imgAspect = "1:1"
						}
						body = map[string]any{
							"prompt":           prompt,
							"model":            p.Model.Slug,
							"projectId":        projectID,
							"folderId":         nil,
							"imageCount":       1,
							"aspectRatio":      imgAspect,
							"visualReferences": []string{},
						}
						// Only forward resolution when the user explicitly set it:
						// the flag default ("720p") is a video format string the
						// image endpoint rejects. Mirrors the cost.go gating.
						if cmd.Flags().Changed("resolution") && resolution != "" {
							body["resolution"] = resolution
						}
					} else {
						vidAspect := aspectRatio
						if vidAspect == "" {
							vidAspect = "16:9"
						}
						vidRes := resolution
						if vidRes == "" {
							vidRes = "720p"
						}
						body = map[string]any{
							"prompt":            prompt,
							"model":             p.Model.Slug,
							"projectId":         projectID,
							"folderId":          nil,
							"videoCount":        1,
							"duration":          p.Duration,
							"aspectRatio":       vidAspect,
							"resolution":        vidRes,
							"autoEnhancePrompt": false,
							"enableUnlimited":   true,
						}
					}
					capability := p.Model.Capability(formType)
					path := "/forms/creations/" + url.PathEscape(capability)
					raw, status, err := c.Post(path, body)
					if err != nil {
						submits[i] = submitResult{plan: p, err: err}
						return
					}
					if status >= 400 {
						submits[i] = submitResult{plan: p, err: fmt.Errorf("HTTP %d: %s", status, summariseBody(raw))}
						return
					}
					var s struct {
						HistoryID   string   `json:"historyId"`
						ResourceIDs []string `json:"resourceIds"`
					}
					if err := json.Unmarshal(raw, &s); err != nil {
						submits[i] = submitResult{plan: p, err: err}
						return
					}
					if len(s.ResourceIDs) == 0 {
						submits[i] = submitResult{plan: p, err: fmt.Errorf("empty resourceIds")}
						return
					}
					submits[i] = submitResult{plan: p, historyID: s.HistoryID, resID: s.ResourceIDs[0]}
				}(i)
			}
			wg.Wait()

			// Build report.
			report := make([]map[string]any, 0, len(submits))
			waitable := make([]string, 0, len(submits))
			waitableIdx := map[string]int{}
			for i, s := range submits {
				row := map[string]any{
					"model":            s.plan.Model.Slug,
					"display_name":     s.plan.Model.DisplayName,
					"vendor":           s.plan.Model.Vendor,
					"duration":         s.plan.Duration,
					"resolution":       resolution,
					"estimate_credits": s.plan.EstimateCredits,
				}
				if s.err != nil {
					row["status"] = "submit-failed"
					row["error"] = s.err.Error()
				} else {
					row["status"] = "submitted"
					row["history_id"] = s.historyID
					row["resource_id"] = s.resID
					waitable = append(waitable, s.resID)
					waitableIdx[s.resID] = i
				}
				report = append(report, row)
			}

			result := map[string]any{
				"prompt":                 prompt,
				"models":                 modelsCSV,
				"resolution":             resolution,
				"total_estimate_credits": totalEstimate,
				"results":                report,
			}

			if !wait || len(waitable) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			completions, _ := waitForResources(ctx, c, waitable, pollInterval, cmd.ErrOrStderr())
			for _, comp := range completions {
				if i, ok := waitableIdx[comp.ResourceID]; ok {
					row := report[i]
					row["status"] = comp.Status
					row["url"] = comp.URL
					row["thumbnail_url"] = comp.ThumbnailURL
					row["duration_actual_sec"] = comp.DurationSec
				}
			}

			if downloadDir != "" {
				downloads, _ := downloadCompletions(ctx, downloadDir, completions, cmd.ErrOrStderr())
				result["downloads"] = downloads
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Text prompt (sent to every model)")
	cmd.Flags().StringVar(&modelsCSV, "models", "", "Comma-separated model slugs or shorthands (e.g. seedance2,kling2-6,grok)")
	cmd.Flags().IntVar(&duration, "duration", 0, "Duration in seconds (0 = use each model's midpoint)")
	cmd.Flags().StringVar(&resolution, "resolution", "720p", "Output resolution")
	cmd.Flags().StringVar(&aspectRatio, "aspect-ratio", "16:9", "Aspect ratio")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID (default: workspace's default)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until each generation completes")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Directory to write completed videos to (implies --wait)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "Polling interval while waiting")
	cmd.Flags().DurationVar(&timeout, "timeout", 25*time.Minute, "Max time to wait for all completions")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if downloadDir != "" {
			wait = true
		}
		return nil
	}
	return cmd
}

type submitPlan struct {
	Model           *openartmodels.Model `json:"-"`
	Duration        int                  `json:"duration"`
	EstimateCredits int                  `json:"estimate_credits"`
}

// MarshalJSON keeps the JSON output focused; the embedded *Model is
// dereferenced to the slug + display name.
func (p submitPlan) MarshalJSON() ([]byte, error) {
	if p.Model == nil {
		return json.Marshal(map[string]any{
			"model":            "",
			"duration":         p.Duration,
			"estimate_credits": p.EstimateCredits,
		})
	}
	return json.Marshal(map[string]any{
		"model":            p.Model.Slug,
		"display_name":     p.Model.DisplayName,
		"duration":         p.Duration,
		"estimate_credits": p.EstimateCredits,
	})
}

func resolveModelList(csv string) ([]*openartmodels.Model, error) {
	if csv == "" {
		return nil, fmt.Errorf("models list is empty")
	}
	parts := strings.Split(csv, ",")
	out := make([]*openartmodels.Model, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		m := openartmodels.Resolve(name)
		if m == nil {
			return nil, fmt.Errorf("unknown model %q", name)
		}
		out = append(out, m)
	}
	return out, nil
}
