// Hand-written novel command: high-UX wrapper for OpenArt image generation.
// Mirrors video.go structure; reuses pollResource, waitForResources,
// downloadCompletions, streamToFile, resolveDefaultProject, nilOrString,
// summariseBody, and modelSupports from that file.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newImageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Generate, list, and manage OpenArt images",
		Long: `Headline workflow for OpenArt image generation.

The 'gen' subcommand wraps OpenArt's submit + poll + download into one
fire-and-forget call against any image model in the catalog. Verified
models (currently nano-banana) are safe to submit; experimental entries
print a warning before spending credits because their submit contract
has not been end-to-end verified.`,
	}
	cmd.AddCommand(newImageGenCmd(flags))
	return cmd
}

func newImageGenCmd(flags *rootFlags) *cobra.Command {
	var (
		prompt          string
		modelInput      string
		count           int
		aspectRatio     string
		resolution      string
		visualReferences []string
		projectID       string
		folderID        string
		wait            bool
		downloadDir     string
		pollInterval    time.Duration
		timeout         time.Duration
		notify          bool
		acceptExperimental bool
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate one or more images with the chosen OpenArt model",
		Long: `Submit an image generation, optionally poll until completion, and
optionally download the resulting PNG/WebP(s) to local disk.

Pick a model first. Run 'openart-pp-cli models list --family image' to
see slugs, costs, and the 'experimental' flag, then pass --model with
the slug (e.g. --model nano-banana for the verified default, or --model
nano-banana-pro for the higher-quality variant). The Nano Banana family
has two members: nano-banana (50 credits, verified) and nano-banana-pro
(120 credits, experimental). There is no nano-banana-2 — Pro is the
upgrade path. Experimental models print a warning and require
--accept-experimental because their submit contract has not been
individually verified end-to-end.

By default this returns immediately with the historyId + resourceIds.
Pass --wait to poll until each resource has status='completed'. Combine
with --download <dir> to stream the finished images to disk.`,
		Example: `  # Verified, cheapest path: nano-banana (50 credits per image)
  openart-pp-cli image gen --prompt "a phoenix soaring over molten gold canyons" --model nano-banana --count 2 --wait --download ./out/

  # Higher-quality Pro variant: nano-banana-pro (120 credits, opt-in)
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model nano-banana-pro --count 2 --accept-experimental --wait --download ./out/

  # Other experimental models follow the same pattern
  openart-pp-cli image gen --prompt "donkey on Mercer Island" --model gpt-image-2 --accept-experimental --wait --download ./out/

  # Square 1024x1024 with explicit aspect ratio
  openart-pp-cli image gen --prompt "neon koi" --model nano-banana --aspect-ratio 1:1 --resolution 1024x1024`,
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return cmd.Help()
			}
			if modelInput == "" {
				return fmt.Errorf("--model is required (e.g. --model nano-banana). Run 'openart-pp-cli models list --family image' to see options.")
			}

			model := openartmodels.Resolve(modelInput)
			if model == nil {
				return fmt.Errorf("unknown model %q. Run 'openart-pp-cli models list --family image' to see available image models.", modelInput)
			}
			if model.Family != openartmodels.FamilyImage {
				return fmt.Errorf("model %q is not an image model (family=%s). Use 'openart-pp-cli video gen' for video models.", model.Slug, model.Family)
			}
			if model.Experimental && !acceptExperimental {
				return fmt.Errorf("model %q is marked experimental — its submit contract has not been verified end-to-end against the live API and may 404 or require fields the catalog doesn't know about. Re-run with --accept-experimental to proceed (and the spend is on you if it fails).", model.Slug)
			}
			if count <= 0 {
				count = 1
			}
			if aspectRatio == "" {
				aspectRatio = "1:1"
			}
			if resolution != "" && len(model.PixelResolutions) > 0 && !modelSupports(model.PixelResolutions, resolution) {
				return fmt.Errorf("--resolution %q not supported by %s. Supported: %v", resolution, model.DisplayName, model.PixelResolutions)
			}

			// Pre-flight cost estimate (best-effort, local).
			estimate := model.EstimateCredits(0, count, resolution)

			if cliutil.IsVerifyEnv() {
				out := map[string]any{
					"would_submit":     true,
					"capability_id":    model.Capability(openartmodels.FormText2Image),
					"model":            model.Slug,
					"prompt":           prompt,
					"count":            count,
					"aspect_ratio":     aspectRatio,
					"resolution":       resolution,
					"experimental":     model.Experimental,
					"estimate_credits": estimate,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if flags.dryRun {
				body := map[string]any{
					"prompt":           prompt,
					"imageCount":       count,
					"aspectRatio":      aspectRatio,
					"visualReferences": visualReferences,
					"model":            model.Slug,
					"projectId":        projectID,
					"folderId":         nilOrString(folderID),
				}
				if resolution != "" {
					body["resolution"] = resolution
				}
				out := map[string]any{
					"action":           "submit",
					"capability_id":    model.Capability(openartmodels.FormText2Image),
					"endpoint":         "/forms/creations/" + url.PathEscape(model.Capability(openartmodels.FormText2Image)),
					"body":             body,
					"experimental":     model.Experimental,
					"estimate_credits": estimate,
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
					return fmt.Errorf("resolve default project (pass --project-id to override): %w", derr)
				}
				projectID = resolved
			}

			capability := model.Capability(openartmodels.FormText2Image)
			body := map[string]any{
				"prompt":           prompt,
				"imageCount":       count,
				"aspectRatio":      aspectRatio,
				"visualReferences": visualReferences,
				"model":            model.Slug,
				"projectId":        projectID,
				"folderId":         nilOrString(folderID),
			}
			if resolution != "" {
				body["resolution"] = resolution
			}
			if visualReferences == nil {
				body["visualReferences"] = []string{}
			}

			submitPath := "/forms/creations/" + url.PathEscape(capability)
			fmt.Fprintf(cmd.ErrOrStderr(), "submitting %d × %s (%s aspect): est. %d credits\n",
				count, model.DisplayName, aspectRatio, estimate)
			if model.Experimental {
				fmt.Fprintf(cmd.ErrOrStderr(), "  WARNING: %s is experimental — contract unverified, may 404 or behave unexpectedly\n", model.Slug)
			}

			raw, status, err := c.Post(submitPath, body)
			if err != nil {
				return fmt.Errorf("submit: %w", err)
			}
			if status >= 400 {
				return fmt.Errorf("submit failed (HTTP %d): %s", status, summariseBody(raw))
			}

			var submit struct {
				HistoryID   string   `json:"historyId"`
				ResourceIDs []string `json:"resourceIds"`
			}
			if err := json.Unmarshal(raw, &submit); err != nil {
				return fmt.Errorf("parse submit response: %w (body=%s)", err, summariseBody(raw))
			}
			if submit.HistoryID == "" || len(submit.ResourceIDs) == 0 {
				return fmt.Errorf("submit returned empty historyId/resourceIds: %s", summariseBody(raw))
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "submitted: history=%s resources=%v\n", submit.HistoryID, submit.ResourceIDs)

			result := map[string]any{
				"history_id":       submit.HistoryID,
				"resource_ids":     submit.ResourceIDs,
				"capability_id":    capability,
				"prompt":           prompt,
				"count":            count,
				"aspect_ratio":     aspectRatio,
				"resolution":       resolution,
				"model":            model.Slug,
				"experimental":     model.Experimental,
				"estimate_credits": estimate,
			}

			if !wait {
				result["status"] = "submitted"
				result["next"] = fmt.Sprintf("openart-pp-cli media get %s", submit.ResourceIDs[0])
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			completions, err := waitForResources(ctx, c, submit.ResourceIDs, pollInterval, cmd.ErrOrStderr())
			if err != nil {
				result["status"] = "timeout"
				result["error"] = err.Error()
				result["completions"] = completions
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			result["status"] = "completed"
			result["completions"] = completions

			if downloadDir != "" {
				downloads, derr := downloadImageCompletions(ctx, downloadDir, completions, cmd.ErrOrStderr())
				if derr != nil {
					result["download_error"] = derr.Error()
				}
				result["downloads"] = downloads
			}

			if notify {
				notifyDone(model.DisplayName, len(completions))
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Text prompt for the image")
	cmd.Flags().StringVar(&modelInput, "model", "", "Model slug or display name (e.g. nano-banana, gpt-image-2)")
	cmd.Flags().IntVar(&count, "count", 1, "How many images to generate")
	cmd.Flags().StringVar(&aspectRatio, "aspect-ratio", "1:1", "Aspect ratio (1:1, 16:9, 9:16, 3:4, 4:3)")
	cmd.Flags().StringVar(&resolution, "resolution", "", "Pixel resolution (e.g. 1024x1024, 1536x1024). Omit to use the model's default.")
	cmd.Flags().StringSliceVar(&visualReferences, "reference", nil, "Visual reference image URL or asset id (repeatable)")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project to file the generation under (default: workspace's default project)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Optional folder ID")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until each image reaches status=completed")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Directory to write completed images (.png/.webp) to (implies --wait)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 3*time.Second, "How often to poll while waiting")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Max time to wait for completion (image gens are usually <60s)")
	cmd.Flags().BoolVar(&notify, "notify", false, "Ring the terminal bell on completion (no-op when not waiting)")
	cmd.Flags().BoolVar(&acceptExperimental, "accept-experimental", false, "Required when the chosen model has Experimental=true (its submit contract has not been verified end-to-end)")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if downloadDir != "" {
			wait = true
		}
		return nil
	}

	return cmd
}

// downloadImageCompletions writes each completed image to disk. Distinct
// from downloadCompletions in video.go because the file extension comes
// from the resource URL or metadata.format rather than always being .mp4.
func downloadImageCompletions(ctx context.Context, dir string, completions []CompletionResult, progressW interface {
	Write(p []byte) (n int, err error)
}) (map[string]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := map[string]string{}
	var firstErr error
	for _, comp := range completions {
		if comp.URL == "" {
			fmt.Fprintf(progressW, "  skip download %s: no URL (status=%s)\n", comp.ResourceID, comp.Status)
			continue
		}
		ext := imageExtForURL(comp.URL)
		path := filepath.Join(dir, fmt.Sprintf("%s%s", comp.ResourceID, ext))
		if err := streamToFile(ctx, comp.URL, path); err != nil {
			fmt.Fprintf(progressW, "  download %s failed: %v\n", comp.ResourceID, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(progressW, "  downloaded %s -> %s\n", comp.ResourceID, path)
		out[comp.ResourceID] = path
	}
	return out, firstErr
}

func imageExtForURL(u string) string {
	for _, ext := range []string{".png", ".webp", ".jpg", ".jpeg"} {
		if hasSuffixCI(u, ext) {
			return ext
		}
	}
	return ".png"
}
