// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared logic for the SculptOK transcendence commands. Kept in
// one file (separate from the per-command files) so the workflow engine, cost
// table, and store/client helpers survive `generate --force` regeneration even
// if the thin command files are re-stubbed.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/sculptok"
	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/store"
)

// drawSpec describes one draw kind for the generate workflow.
type drawSpec struct {
	kind string // depthmap | stl | threed | restore
	path string // api-open sub-path
}

var drawSpecs = map[string]drawSpec{
	"depthmap": {kind: "depthmap", path: "/draw/prompt"},
	"stl":      {kind: "stl", path: "/draw/stl/prompt"},
	"threed":   {kind: "threed", path: "/draw/3d/prompt"},
	"restore":  {kind: "restore", path: "/draw/hd/prompt"},
}

// imageField returns the body key the given draw kind uses for the image URL.
// STL uses snake_case image_url; every other kind uses imageUrl.
func imageField(kind string) string {
	if kind == "stl" {
		return "image_url"
	}
	return "imageUrl"
}

// drawCost returns the documented credit cost for a draw.
func drawCost(kind, style, drawHD string) int {
	switch kind {
	case "depthmap":
		if style == "pro" {
			if drawHD == "4k" {
				return 30
			}
			return 15
		}
		return 10
	case "restore":
		return 2
	case "threed":
		return 10
	case "stl":
		return 3
	}
	return 0
}

// newSculptokClient builds the envelope-aware sibling client from config.
func newSculptokClient(flags *rootFlags) (*sculptok.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	return sculptok.New(cfg, flags.timeout, flags.rateLimit), nil
}

// resolveDBPath returns the store path, preferring an explicit --db override.
func resolveDBPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if v := os.Getenv("SCULPTOK_DB"); v != "" {
		return v
	}
	return store.DefaultPath()
}

// isImageURL reports whether s is already a remote image URL (vs a local path).
func isImageURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// generateOpts carries the per-invocation knobs for the workflow.
type generateOpts struct {
	kind         string
	image        string            // local path or remote URL
	body         map[string]any    // kind-specific draw params (image field added by workflow)
	restoreFirst bool              // run bg-removal+HD first, feed cleaned image into the draw
	noWait       bool              // submit only; do not poll
	pollInterval time.Duration     // poll cadence
	outDir       string            // download result files here when set
	style        string            // for cost preflight (depthmap)
	drawHD       string            // for cost preflight (depthmap)
	db           string            // store path override
	extraParams  map[string]string // recorded into the persisted job params
}

// generateResult is the JSON-friendly result of a generate run.
type generateResult struct {
	Kind        string   `json:"kind"`
	PromptID    string   `json:"promptId"`
	Status      string   `json:"status"`
	ImageURL    string   `json:"imageUrl"`
	CreditCost  int      `json:"creditCost"`
	Results     []string `json:"results"`
	Downloaded  []string `json:"downloaded,omitempty"`
	RestoredURL string   `json:"restoredUrl,omitempty"`
}

// runGenerate executes upload -> (optional restore) -> submit -> poll -> persist
// -> (optional download) for one image, returning the structured result.
func runGenerate(ctx context.Context, c *sculptok.Client, st *store.Store, opts generateOpts) (*generateResult, error) {
	spec, ok := drawSpecs[opts.kind]
	if !ok {
		return nil, fmt.Errorf("unknown draw kind %q", opts.kind)
	}

	// Resolve the source image to a SculptOK-hosted URL.
	imageURL := opts.image
	if !isImageURL(opts.image) {
		if _, err := os.Stat(opts.image); err != nil {
			return nil, fmt.Errorf("image not found: %s", opts.image)
		}
		uploaded, err := c.Upload(ctx, opts.image)
		if err != nil {
			return nil, fmt.Errorf("uploading image: %w", err)
		}
		imageURL = uploaded
	}

	res := &generateResult{Kind: opts.kind, ImageURL: imageURL}

	// Optional pre-process: background removal + HD restore, then feed the
	// cleaned image into the main draw.
	if opts.restoreFirst && opts.kind != "restore" {
		restoreBody := map[string]any{"imageUrl": imageURL, "hdFix": "true"}
		rid, err := c.Submit(ctx, drawSpecs["restore"].path, restoreBody)
		if err != nil {
			return nil, fmt.Errorf("restore submit: %w", err)
		}
		// The pre-pass is a billed draw of its own. Persist it under its own
		// promptId — reconcile matches credit-event remarks against stored
		// prompt IDs, so an unstored pre-pass reads as an unmatched charge.
		// Recorded right after submit (before polling) so an interrupted poll
		// still leaves the spend accounted for.
		restoreJob := store.Job{
			PromptID:   rid,
			Kind:       "restore",
			Status:     "submitted",
			ImageURL:   imageURL,
			Params:     `{"hdFix":"true","prePass":true}`,
			CreditCost: drawCost("restore", "", ""),
			CreatedAt:  nowStamp(),
		}
		if st != nil {
			_ = st.UpsertJob(ctx, restoreJob)
		}
		rs, err := c.Poll(ctx, rid, opts.pollInterval, nil)
		if err != nil {
			return nil, fmt.Errorf("restore poll: %w", err)
		}
		if len(rs.ImgRecords) > 0 {
			imageURL = rs.ImgRecords[0]
			res.RestoredURL = imageURL
		}
		if st != nil {
			restoreResults, _ := json.Marshal(rs.ImgRecords)
			restoreJob.Status = "completed"
			restoreJob.ResultURLs = string(restoreResults)
			_ = st.UpsertJob(ctx, restoreJob)
		}
	}

	// Build the draw body with the correct image field for this kind.
	body := map[string]any{}
	for k, v := range opts.body {
		body[k] = v
	}
	body[imageField(opts.kind)] = imageURL

	promptID, err := c.Submit(ctx, spec.path, body)
	if err != nil {
		return nil, err
	}
	res.PromptID = promptID
	res.CreditCost = drawCost(opts.kind, opts.style, opts.drawHD)

	// Persist the job immediately (status submitted) so credits spent are
	// never lost even if polling is interrupted.
	paramsJSON, _ := json.Marshal(mergeParams(opts))
	if st != nil {
		_ = st.UpsertJob(ctx, store.Job{
			PromptID:   promptID,
			Kind:       opts.kind,
			Status:     "submitted",
			ImageURL:   res.ImageURL,
			Params:     string(paramsJSON),
			ResultURLs: "[]",
			CreditCost: res.CreditCost,
			CreatedAt:  nowStamp(),
		})
	}

	if opts.noWait {
		res.Status = "submitted"
		return res, nil
	}

	status, err := c.Poll(ctx, promptID, opts.pollInterval, nil)
	if err != nil {
		res.Status = "pending"
		return res, err
	}
	res.Status = "completed"
	res.Results = status.ImgRecords

	if st != nil {
		resultsJSON, _ := json.Marshal(status.ImgRecords)
		_ = st.UpsertJob(ctx, store.Job{
			PromptID:   promptID,
			Kind:       opts.kind,
			Status:     "completed",
			ImageURL:   res.ImageURL,
			Params:     string(paramsJSON),
			ResultURLs: string(resultsJSON),
			CreditCost: res.CreditCost,
			CreatedAt:  nowStamp(),
		})
	}

	if opts.outDir != "" {
		downloaded, err := downloadResults(ctx, status.ImgRecords, opts.outDir, opts.kind, promptID)
		if err != nil {
			return res, fmt.Errorf("downloading results: %w", err)
		}
		res.Downloaded = downloaded
	}
	return res, nil
}

func mergeParams(opts generateOpts) map[string]string {
	out := map[string]string{}
	for k, v := range opts.extraParams {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func nowStamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

// downloadResults fetches each result URL into outDir.
func downloadResults(ctx context.Context, urls []string, outDir, kind, promptID string) ([]string, error) {
	// #nosec G301 -- outDir is a user-chosen output directory for generated
	// assets; 0o755 lets the user read/share their own results as expected.
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var saved []string
	client := &http.Client{Timeout: 2 * time.Minute}
	for i, u := range urls {
		// Strip any query string / fragment before reading the extension so a
		// URL like ".../x.png?token=..." doesn't yield a bogus extension.
		cleanURL := u
		if idx := strings.IndexAny(cleanURL, "?#"); idx >= 0 {
			cleanURL = cleanURL[:idx]
		}
		ext := filepath.Ext(cleanURL)
		if ext == "" || len(ext) > 5 {
			ext = ".png"
		}
		name := fmt.Sprintf("%s-%s-%d%s", kind, shortID(promptID), i+1, ext)
		dest := filepath.Join(outDir, name)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return saved, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return saved, err
		}
		// Guard against writing a CDN error body (expired presigned URL 403,
		// 404, 5xx) into the output file and reporting it as a success.
		if resp.StatusCode >= 400 {
			_ = resp.Body.Close()
			return saved, fmt.Errorf("downloading %s: HTTP %d", u, resp.StatusCode)
		}
		// #nosec G304 -- dest is built from the user-chosen outDir and a
		// sanitized basename; writing generated assets to a user path is the
		// command's purpose.
		f, err := os.Create(dest)
		if err != nil {
			_ = resp.Body.Close()
			return saved, err
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			_ = f.Close()
			_ = resp.Body.Close()
			return saved, err
		}
		if err := f.Close(); err != nil {
			_ = resp.Body.Close()
			return saved, err
		}
		_ = resp.Body.Close()
		saved = append(saved, dest)
	}
	return saved, nil
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

// printGenerateResult renders the result respecting output flags.
func printGenerateResult(cmd *cobra.Command, flags *rootFlags, res *generateResult) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), res, flags)
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s draw %s (%s) — cost %d credits\n", res.Kind, res.PromptID, res.Status, res.CreditCost)
	if res.RestoredURL != "" {
		fmt.Fprintf(w, "  pre-processed (restore): %s\n", res.RestoredURL)
	}
	for i, u := range res.Results {
		fmt.Fprintf(w, "  result %d: %s\n", i+1, u)
	}
	for _, d := range res.Downloaded {
		fmt.Fprintf(w, "  saved: %s\n", d)
	}
	if res.Status == "submitted" {
		fmt.Fprintf(w, "  poll with: sculptok-pp-cli draw status --uuid %s\n", res.PromptID)
	}
	return nil
}

// openReadStore opens the local store read-only. ok is false (no error) when
// the store file does not exist yet.
func openReadStore(ctx context.Context, path string) (*store.Store, bool, error) {
	return store.OpenReadOnly(ctx, path)
}

// emptyMirror handles the "no local store yet" case for read-only local
// commands: it prints an empty machine result to stdout and a sync hint to
// stderr, returning nil (an empty cache is not an error).
func emptyMirror(cmd *cobra.Command, flags *rootFlags, dbPath string) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: sculptok-pp-cli sync --resources credits,drawings\n", dbPath)
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// genCmdConfig is the per-kind configuration the thin command files pass in.
type genCmdConfig struct {
	kind         string
	restoreFirst bool
	noWait       bool
	batchDir     string
	outDir       string
	db           string
	pollInterval time.Duration
	// body holds the kind-specific draw params (image field is added later).
	body map[string]any
	// recorded is the human-facing params snapshot persisted with the job.
	recorded map[string]string
	style    string // cost preflight (depthmap)
	drawHD   string // cost preflight (depthmap)
}

// imageExts are the input formats SculptOK accepts.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".bmp": true, ".exr": true,
}

// executeGenerate is the shared RunE body for every generate subcommand. It
// handles the help-only probe, the side-effect guard, single vs --batch, store
// persistence, credit preflight, and output.
func executeGenerate(cmd *cobra.Command, flags *rootFlags, args []string, cfg genCmdConfig) error {
	hasInput := len(args) > 0 || cfg.batchDir != ""
	if !hasInput && cmd.Flags().NFlag() == 0 {
		return cmd.Help()
	}
	if guardSideEffect(cmd, flags, cfg.kind, firstInput(args, cfg.batchDir)) {
		return nil
	}
	if !hasInput {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("provide an image path or URL (or --batch <dir>)"))
	}

	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	c, err := newSculptokClient(flags)
	if err != nil {
		return err
	}
	if !c.HasKey() {
		return usageErr(fmt.Errorf("no API key configured: set SCULPTOK_API_KEY or run 'sculptok-pp-cli auth set-token <key>'"))
	}

	st, err := store.Open(ctx, resolveDBPath(cfg.db))
	if err != nil {
		// The store is a convenience (job history); a failure to open it must
		// not block the actual generation.
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: local store unavailable (%v); job will not be recorded\n", err)
		st = nil
	} else {
		defer st.Close()
	}

	// Gather input images.
	var images []string
	if cfg.batchDir != "" {
		entries, err := os.ReadDir(cfg.batchDir)
		if err != nil {
			return fmt.Errorf("reading batch dir: %w", err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
				images = append(images, filepath.Join(cfg.batchDir, e.Name()))
			}
		}
		if len(images) == 0 {
			return fmt.Errorf("no images found in %s", cfg.batchDir)
		}
	} else {
		images = append(images, args[0])
	}

	// Credit preflight: warn (do not block) when balance is below the total.
	perCost := drawCost(cfg.kind, cfg.style, cfg.drawHD)
	if cfg.restoreFirst && cfg.kind != "restore" {
		perCost += drawCost("restore", "", "")
	}
	total := perCost * len(images)
	if balance, berr := c.Balance(ctx); berr == nil {
		if total > balance {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: this run needs ~%d credits but your balance is %d; some draws may fail\n", total, balance)
		}
	}

	results := make([]*generateResult, 0, len(images))
	failures := make([]map[string]string, 0)
	for _, img := range images {
		opts := generateOpts{
			kind:         cfg.kind,
			image:        img,
			body:         cfg.body,
			restoreFirst: cfg.restoreFirst,
			noWait:       cfg.noWait,
			pollInterval: cfg.pollInterval,
			outDir:       cfg.outDir,
			style:        cfg.style,
			drawHD:       cfg.drawHD,
			db:           cfg.db,
			extraParams:  cfg.recorded,
		}
		res, rerr := runGenerate(ctx, c, st, opts)
		if rerr != nil {
			if len(images) == 1 {
				return rerr
			}
			failures = append(failures, map[string]string{"image": img, "error": rerr.Error()})
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s failed: %v\n", img, rerr)
			continue
		}
		results = append(results, res)
	}

	if cfg.batchDir != "" {
		view := map[string]any{
			"kind":      cfg.kind,
			"submitted": len(results),
			"failed":    len(failures),
			"results":   results,
		}
		if len(failures) > 0 {
			view["fetch_failures"] = failures
		}
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
		// A batch where every draw failed must not exit 0 — a script checking
		// $? would otherwise read a total failure as success.
		if len(results) == 0 && len(failures) > 0 {
			return fmt.Errorf("all %d batch draws failed", len(failures))
		}
		return nil
	}
	if len(results) == 1 {
		return printGenerateResult(cmd, flags, results[0])
	}
	return nil
}

func firstInput(args []string, batchDir string) string {
	if len(args) > 0 {
		return args[0]
	}
	if batchDir != "" {
		return batchDir
	}
	return "<image>"
}

// guardSideEffect returns true and prints a plan line when the command should
// NOT actually spend credits — under --dry-run, the verify harness, or the
// live-dogfood matrix. Paid generation must never fire from a test harness.
func guardSideEffect(cmd *cobra.Command, flags *rootFlags, kind, image string) bool {
	if dryRunOK(flags) || cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would generate %s from %s (no credits spent)\n", kind, image)
		return true
	}
	return false
}
