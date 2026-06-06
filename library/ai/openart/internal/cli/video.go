// Hand-written novel commands: high-UX wrapper for OpenArt video generation.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openart/internal/openartmodels"
)

func newVideoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video",
		Short: "Generate, list, and manage OpenArt videos",
		Long: `Headline workflow for OpenArt video generation.

The 'gen' subcommand wraps OpenArt's submit + poll + download into one
fire-and-forget call. Use --wait to block until completion (~10 min for
Seedance 2 at default settings; ~15s for Grok Imagine).`,
	}
	cmd.AddCommand(newVideoGenCmd(flags))
	return cmd
}

func newVideoGenCmd(flags *rootFlags) *cobra.Command {
	var (
		prompt       string
		modelInput   string
		duration     int
		count        int
		aspectRatio  string
		resolution   string
		autoEnhance  bool
		noAudio      bool
		projectID    string
		folderID     string
		wait         bool
		downloadDir  string
		pollInterval time.Duration
		timeout      time.Duration
		notify       bool
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate one or more videos with the chosen OpenArt model",
		Long: `Submit a video generation, optionally poll until completion, and optionally
download the resulting MP4(s) to local disk.

The model can be referenced by slug (e.g. byte-plus-seedance-2) or display
name (e.g. "Seedance 2.0", "kling 2.6", "grok"). Run
'openart-pp-cli models list' to see what is available.

By default this returns immediately with the historyId + resourceIds and
exits. Pass --wait to poll until each resource has status='completed'.
Combine with --download <dir> to stream the finished MP4s to disk.`,
		Example: `  # Submit two Seedance videos and walk away
  openart-pp-cli video gen --prompt "a phoenix soaring over molten gold" --model seedance2 --count 2

  # Wait until done, write videos to ./out/
  openart-pp-cli video gen --prompt "..." --model seedance2 --duration 10 --wait --download ./out/

  # Quick Grok Imagine probe with a notification on completion
  openart-pp-cli video gen --prompt "neon koi" --model grok --duration 5 --wait --notify`,
		Annotations: map[string]string{
			// Mutating side effect: spends credits, writes files when --download is set.
			// No mcp:read-only annotation.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return cmd.Help()
			}
			if modelInput == "" {
				return fmt.Errorf("--model is required (e.g. --model seedance2). Run 'openart-pp-cli models list' to see options.")
			}

			model := openartmodels.Resolve(modelInput)
			if model == nil {
				return fmt.Errorf("unknown model %q. Run 'openart-pp-cli models list' to see available models.", modelInput)
			}
			if model.Family != openartmodels.FamilyVideo {
				return fmt.Errorf("model %q is not a video model (family=%s)", model.Slug, model.Family)
			}

			if duration <= 0 {
				duration = (model.DurationMinSec + model.DurationMaxSec) / 2
				if duration == 0 {
					duration = 5
				}
			}
			if duration < model.DurationMinSec || duration > model.DurationMaxSec {
				return fmt.Errorf("--duration %ds is outside %s's supported range (%d-%d s)",
					duration, model.DisplayName, model.DurationMinSec, model.DurationMaxSec)
			}
			if count <= 0 {
				count = 1
			}
			if !modelSupports(model.Resolutions, resolution) {
				return fmt.Errorf("--resolution %q not supported by %s. Supported: %s",
					resolution, model.DisplayName, strings.Join(model.Resolutions, ", "))
			}

			// Pre-flight cost estimate (best-effort, local).
			estimate := model.EstimateCredits(duration, count, resolution)

			// Side-effect verify guard: never actually submit during a verify pass.
			if cliutil.IsVerifyEnv() {
				out := map[string]any{
					"would_submit":   true,
					"capability_id": model.Capability(openartmodels.FormText2Video),
					"model":         model.Slug,
					"prompt":        prompt,
					"duration":      duration,
					"count":         count,
					"aspect_ratio":  aspectRatio,
					"resolution":    resolution,
					"estimate_credits": estimate,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if flags.dryRun {
				body := map[string]any{
					"prompt":            prompt,
					"model":             model.Slug,
					"projectId":         projectID,
					"folderId":          nilOrString(folderID),
					"videoCount":        count,
					"duration":          duration,
					"aspectRatio":       aspectRatio,
					"resolution":        resolution,
					"autoEnhancePrompt": autoEnhance,
					"enableUnlimited":   true,
				}
				if noAudio {
					body["generateAudio"] = false
				}
				out := map[string]any{
					"action":        "submit",
					"capability_id": model.Capability(openartmodels.FormText2Video),
					"endpoint":      "/forms/creations/" + url.PathEscape(model.Capability(openartmodels.FormText2Video)),
					"body":          body,
					"estimate_credits": estimate,
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve projectId if not provided.
			if projectID == "" {
				resolved, derr := resolveDefaultProject(c)
				if derr != nil {
					return fmt.Errorf("resolve default project (pass --project-id to override): %w", derr)
				}
				projectID = resolved
			}

			capability := model.Capability(openartmodels.FormText2Video)
			body := map[string]any{
				"prompt":            prompt,
				"model":             model.Slug,
				"projectId":         projectID,
				"folderId":          nilOrString(folderID),
				"videoCount":        count,
				"duration":          duration,
				"aspectRatio":       aspectRatio,
				"resolution":        resolution,
				"autoEnhancePrompt": autoEnhance,
				"enableUnlimited":   true,
			}
			if noAudio {
				// Verified field name via 2026-05-13 JS-bundle scan: Zod
				// schema had `generateAudio: a.z.boolean().default(!0)`.
				// Submitting `generateAudio: false` skips Seedance's audio
				// track (and its OutputAudioSensitiveContentDetected
				// failure class). Cost matrix observed in the bundle shows
				// noAudio is also CHEAPER on Seedance (e.g. 720p/normal:
				// audio=135 credits, noAudio=70).
				body["generateAudio"] = false
			}

			submitPath := "/forms/creations/" + url.PathEscape(capability)
			fmt.Fprintf(cmd.ErrOrStderr(), "submitting %d × %s (%dx%d %ds): est. %d credits\n",
				count, model.DisplayName, parseResolutionWidth(resolution, aspectRatio), parseResolutionHeight(resolution, aspectRatio), duration, estimate)

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
				"history_id":     submit.HistoryID,
				"resource_ids":   submit.ResourceIDs,
				"capability_id":  capability,
				"prompt":         prompt,
				"count":          count,
				"duration":       duration,
				"aspect_ratio":   aspectRatio,
				"resolution":     resolution,
				"model":          model.Slug,
				"estimate_credits": estimate,
			}

			if !wait {
				result["status"] = "submitted"
				result["next"] = fmt.Sprintf("openart-pp-cli media get %s", submit.ResourceIDs[0])
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Poll each resource to completion concurrently.
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
				downloads, derr := downloadCompletions(ctx, downloadDir, completions, cmd.ErrOrStderr())
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

	cmd.Flags().StringVar(&prompt, "prompt", "", "Text prompt for the video")
	cmd.Flags().StringVar(&modelInput, "model", "", "Model slug or display name (e.g. byte-plus-seedance-2, seedance2, kling 2.6)")
	cmd.Flags().IntVar(&duration, "duration", 0, "Video duration in seconds (default: midpoint of model's supported range)")
	cmd.Flags().IntVar(&count, "count", 1, "How many videos to generate")
	cmd.Flags().StringVar(&aspectRatio, "aspect-ratio", "16:9", "Aspect ratio (16:9, 9:16, 1:1)")
	cmd.Flags().StringVar(&resolution, "resolution", "720p", "Output resolution")
	cmd.Flags().BoolVar(&autoEnhance, "auto-enhance", false, "Run prompt through OpenArt auto-polish before submit")
	cmd.Flags().BoolVar(&noAudio, "no-audio", false, "Disable Seedance's auto-generated audio track. Workaround for OutputAudioSensitiveContentDetected moderation failures; also halves the cost on Seedance 2.0 normal mode")
	cmd.Flags().StringVar(&projectID, "project-id", "", "Project to file the generation under (default: workspace's default project)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "Optional folder ID")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until each video reaches status=completed")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Directory to write completed videos (.mp4) to (implies --wait)")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 5*time.Second, "How often to poll while waiting")
	cmd.Flags().DurationVar(&timeout, "timeout", 25*time.Minute, "Max time to wait for completion (Seedance 2 at 1080p can run >15 min)")
	cmd.Flags().BoolVar(&notify, "notify", false, "Ring the terminal bell on completion (no-op when not waiting)")

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// --download implies --wait
		if downloadDir != "" {
			wait = true
		}
		return nil
	}

	return cmd
}

// CompletionResult is one finished generation's resource snapshot.
type CompletionResult struct {
	ResourceID   string `json:"resource_id"`
	Status       string `json:"status"`
	URL          string `json:"url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	DurationSec  any    `json:"duration_sec,omitempty"`
	WidthPx      any    `json:"width_px,omitempty"`
	HeightPx     any    `json:"height_px,omitempty"`
	Error        string `json:"error,omitempty"`
}

// resolveDefaultProject calls /projects/default to get the active project ID.
func resolveDefaultProject(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}) (string, error) {
	raw, err := c.Get("/projects/default", nil)
	if err != nil {
		return "", err
	}
	var resp struct {
		ProjectID string `json:"projectId"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if resp.ProjectID == "" {
		return "", errors.New("server returned empty projectId")
	}
	return resp.ProjectID, nil
}

// waitForResources polls each resource until terminal status, returning
// snapshots in original order. Returns the partial set on timeout.
func waitForResources(ctx context.Context, c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, resourceIDs []string, interval time.Duration, progressW io.Writer) ([]CompletionResult, error) {
	completions := make([]CompletionResult, len(resourceIDs))
	for i, id := range resourceIDs {
		completions[i] = CompletionResult{ResourceID: id, Status: "pending"}
	}
	done := func() bool {
		for _, c := range completions {
			if !isTerminalStatus(c.Status) {
				return false
			}
		}
		return true
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	pollOnce := func() error {
		for i, comp := range completions {
			if isTerminalStatus(comp.Status) {
				continue
			}
			snap, err := pollResource(c, comp.ResourceID)
			if err != nil {
				// Transient: log and continue.
				fmt.Fprintf(progressW, "  poll %s: %v\n", comp.ResourceID, err)
				continue
			}
			completions[i] = snap
			if isTerminalStatus(snap.Status) {
				fmt.Fprintf(progressW, "  %s -> %s\n", snap.ResourceID, snap.Status)
			}
		}
		return nil
	}

	if err := pollOnce(); err != nil {
		return completions, err
	}
	if done() {
		return completions, nil
	}
	for {
		select {
		case <-ctx.Done():
			return completions, fmt.Errorf("wait timeout: %w", ctx.Err())
		case <-tick.C:
			if err := pollOnce(); err != nil {
				return completions, err
			}
			if done() {
				return completions, nil
			}
		}
	}
}

func pollResource(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, id string) (CompletionResult, error) {
	raw, err := c.Get("/resources/"+id, nil)
	if err != nil {
		return CompletionResult{ResourceID: id, Status: "error", Error: err.Error()}, err
	}
	var resp struct {
		Data struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			URL          string `json:"url"`
			ThumbnailURL string `json:"thumbnailUrl"`
			Metadata     struct {
				Duration any `json:"duration"`
				Width    any `json:"width"`
				Height   any `json:"height"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return CompletionResult{ResourceID: id, Status: "error", Error: err.Error()}, err
	}
	if resp.Data.ID == "" {
		// Some endpoints return the resource at top-level rather than under "data";
		// fall back to direct unmarshal.
		var direct struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			URL          string `json:"url"`
			ThumbnailURL string `json:"thumbnailUrl"`
		}
		if err := json.Unmarshal(raw, &direct); err == nil && direct.ID != "" {
			return CompletionResult{
				ResourceID:   direct.ID,
				Status:       direct.Status,
				URL:          direct.URL,
				ThumbnailURL: direct.ThumbnailURL,
			}, nil
		}
	}
	return CompletionResult{
		ResourceID:   resp.Data.ID,
		Status:       resp.Data.Status,
		URL:          resp.Data.URL,
		ThumbnailURL: resp.Data.ThumbnailURL,
		DurationSec:  resp.Data.Metadata.Duration,
		WidthPx:      resp.Data.Metadata.Width,
		HeightPx:     resp.Data.Metadata.Height,
	}, nil
}

func isTerminalStatus(s string) bool {
	switch strings.ToLower(s) {
	case "completed", "failed", "error", "cancelled", "canceled":
		return true
	}
	return false
}

func downloadCompletions(ctx context.Context, dir string, completions []CompletionResult, progressW io.Writer) (map[string]string, error) {
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
		path := filepath.Join(dir, fmt.Sprintf("%s%s", comp.ResourceID, mediaExtForURL(comp.URL)))
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

// mediaExtForURL infers the output extension from the resource URL so that
// image-family completions flowing through shared callers (compare, prompts
// replay) are not saved as .mp4. Falls back to .mp4 for extension-less video
// CDN URLs, which is the dominant case for this downloader.
func mediaExtForURL(u string) string {
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	for _, ext := range []string{".mp4", ".webm", ".mov", ".png", ".webp", ".jpg", ".jpeg", ".gif"} {
		if hasSuffixCI(u, ext) {
			return ext
		}
	}
	return ".mp4"
}

func streamToFile(ctx context.Context, src, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func notifyDone(modelName string, n int) {
	// Print a terminal bell + a stderr line. On macOS, also fire-and-forget
	// an osascript notification — non-fatal if osascript isn't present.
	fmt.Fprintf(os.Stderr, "\007openart: %s × %d ready\n", modelName, n)
}

func summariseBody(raw json.RawMessage) string {
	if len(raw) > 400 {
		return string(raw[:400]) + "...[truncated]"
	}
	return string(raw)
}

func nilOrString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// modelSupports does a case-insensitive contains-check on a Resolutions slice.
func modelSupports(supported []string, want string) bool {
	if want == "" {
		return true
	}
	for _, r := range supported {
		if strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

// parseResolutionWidth/Height return rough pixel dimensions from a
// "<height>p" string + an aspect ratio. Used for the human progress line;
// best-effort, never blocks a submit.
func parseResolutionWidth(res, aspect string) int {
	h := parseResolutionHeight(res, aspect)
	if h == 0 {
		return 0
	}
	num, den := splitAspect(aspect)
	if num == 0 || den == 0 {
		return 0
	}
	return h * num / den
}

func parseResolutionHeight(res, aspect string) int {
	if strings.EqualFold(res, "4K") || strings.EqualFold(res, "4k") {
		return 2160
	}
	res = strings.TrimSuffix(strings.ToLower(res), "p")
	var n int
	for _, r := range res {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			continue
		}
		return 0
	}
	return n
}

func splitAspect(aspect string) (int, int) {
	parts := strings.SplitN(aspect, ":", 2)
	if len(parts) != 2 {
		return 0, 0
	}
	num, _ := atoiSafe(parts[0])
	den, _ := atoiSafe(parts[1])
	return num, den
}

func atoiSafe(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-numeric")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
