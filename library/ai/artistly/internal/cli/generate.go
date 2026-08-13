// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `generate` submits an image generation to Artistly's
// POST /ai/{feature}/store endpoint (CSRF-protected, returns 302=queued), and
// with --wait polls /fetch-personal-designs until the design renders, then
// --download saves the CDN images. This is the headline command; batch and redo
// reuse runGeneration below.
//
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelGenerateCmd(flags *rootFlags) *cobra.Command {
	var s genSettings
	var presetName string
	var prompt string
	var wait bool
	var download string
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "generate [prompt]",
		Short: "Submit an image generation, optionally wait for it to render and download it.",
		Long: trimLong(`
Generate an image from a text prompt via Artistly. Pass the prompt as a
positional argument or with --prompt.

By default the generation is queued and the command returns immediately. Add
--wait to block until the image renders (polling, since completion is pushed
over a WebSocket the CLI cannot see), and --download <dir> to save the finished
images to disk. Apply a saved preset with --preset.

All Artistly generators share one endpoint; use --tool to target a non-default
generator feature (default: image-designer-v6).`),
		Example: trimLong(`
  artistly-pp-cli generate "a watercolor fox in a misty forest" --wait --download ./out
  artistly-pp-cli generate "logo for a coffee shop" --preset house-style --json`),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 && prompt == "" {
				prompt = args[0]
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would generate %q (tool=%s)\n", prompt, firstNonEmpty(s.Tool, defaultTool))
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would generate %q\n", prompt)
				return nil
			}
			if prompt == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a prompt is required (positional arg or --prompt)"))
			}

			settings, err := mergePresetAndFlags(cmd, presetName, s)
			if err != nil {
				return err
			}

			// Use the command context directly: generation with --wait can run
			// far longer than the short root --timeout, and runGeneration bounds
			// its own poll by --wait-timeout. Individual HTTP calls remain
			// bounded by the client's per-request timeout.
			ctx := cmd.Context()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			designs, err := runGeneration(ctx, c, prompt, settings, wait, download, waitTimeout, progressWriter(cmd, flags))
			if err != nil {
				// On a mid-render timeout runGeneration returns the designs that
				// did finish (already downloaded if --download was set); surface
				// them before the non-zero exit so completed work isn't hidden.
				if len(designs) > 0 {
					_ = emitGenerationResult(cmd, flags, prompt, designs, wait)
				}
				return classifyAPIError(err, flags)
			}
			return emitGenerationResult(cmd, flags, prompt, designs, wait)
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt text (alternative to the positional argument)")
	cmd.Flags().StringVar(&presetName, "preset", "", "Apply a saved settings preset (see: artistly-pp-cli preset)")
	cmd.Flags().StringVar(&s.Tool, "tool", "", "Generator feature slug (default image-designer-v6)")
	cmd.Flags().StringVar(&s.Style, "style", "", "Style label/slug to apply")
	cmd.Flags().StringVar(&s.Negative, "negative-prompt", "", "Negative prompt (what to avoid)")
	cmd.Flags().StringVar(&s.Aspect, "aspect-ratio", "", "Aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3 (default 1:1)")
	cmd.Flags().StringVar(&s.Quality, "quality", "", "Quality: fast or highQuality (default fast)")
	cmd.Flags().IntVar(&s.Quantity, "quantity", 0, "Number of images to generate (default 1)")
	cmd.Flags().IntVar(&s.Seed, "seed", 0, "Seed (0 = random)")
	cmd.Flags().IntVar(&s.CheckpointID, "checkpoint-id", 0, "Checkpoint (model) id")
	cmd.Flags().IntVar(&s.FolderID, "folder-id", 0, "Folder id to place results in")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the generation renders")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download finished images into (implies --wait)")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "Maximum time to wait when --wait/--download is set")
	return cmd
}

// runGeneration submits one prompt and, when wait/download is requested, blocks
// for the rendered designs and downloads them. Shared by generate, batch, redo.
func runGeneration(ctx context.Context, c *client.Client, prompt string, s genSettings, wait bool, download string, timeout time.Duration, progress io.Writer) ([]Design, error) {
	quantity := s.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	quality := firstNonEmpty(s.Quality, "fast")
	aspect, width, height, err := resolveAspect(s.Aspect)
	if err != nil {
		return nil, err
	}
	checkpointID := s.CheckpointID
	if checkpointID == 0 {
		checkpointID = 1
	}
	var folderID *int
	if s.FolderID != 0 {
		folderID = &s.FolderID
	}

	// Under live-dogfood, keep the matrix cheap: one image, no blocking wait.
	if cliutil.IsDogfoodEnv() {
		quantity = 1
		wait = false
		download = ""
	}

	body := generationBody(prompt, s.Negative, s.Style, checkpointID, width, height, quantity, s.Seed, aspect, quality, folderID)

	// Cursor: highest existing design id before we submit.
	before, err := fetchPersonalDesigns(ctx, c)
	if err != nil {
		return nil, err
	}
	afterID := maxDesignID(before)

	if err := submitGeneration(ctx, c, s.Tool, body); err != nil {
		return nil, err
	}

	if !wait && download == "" {
		return nil, nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	// The poll is inherently longer than a single request; bound it by the
	// caller's --wait-timeout rather than the short root --timeout. Each HTTP
	// call inside still carries the client's per-request timeout.
	pollCtx, cancelPoll := context.WithTimeout(ctx, timeout)
	defer cancelPoll()
	designs, err := pollForNewDesigns(pollCtx, c, afterID, quantity, timeout, progress)
	if err != nil && len(designs) == 0 {
		return nil, err
	}
	if download != "" {
		for _, d := range designs {
			if _, derr := downloadDesign(ctx, d, download, "{prompt}-{id}"); derr != nil {
				return designs, derr
			}
		}
	}
	return designs, err
}

func emitGenerationResult(cmd *cobra.Command, flags *rootFlags, prompt string, designs []Design, waited bool) error {
	if flags.asJSON {
		result := map[string]any{"prompt": prompt, "queued": true}
		if waited {
			result["designs"] = designs
			result["rendered"] = len(designs)
		}
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if !waited {
		fmt.Fprintf(cmd.OutOrStdout(), "Queued generation for %q. Run 'artistly-pp-cli designs list' to see it, or add --wait next time.\n", prompt)
		return nil
	}
	if len(designs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Generation queued but did not finish before the timeout. Check 'artistly-pp-cli designs list'.")
		return nil
	}
	for _, d := range designs {
		fmt.Fprintf(cmd.OutOrStdout(), "Rendered design %d (%s):\n", d.ID, d.UUID)
		for _, u := range designImageURLs(d) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", u)
		}
	}
	return nil
}

// mergePresetAndFlags loads a preset (if named) as the base, then overlays any
// flags the user explicitly set on this invocation.
func mergePresetAndFlags(cmd *cobra.Command, presetName string, flagVals genSettings) (genSettings, error) {
	base := genSettings{}
	if presetName != "" {
		p, err := loadPreset(presetName)
		if err != nil {
			return base, err
		}
		base = p
	}
	f := cmd.Flags()
	if f.Changed("tool") {
		base.Tool = flagVals.Tool
	}
	if f.Changed("style") {
		base.Style = flagVals.Style
	}
	if f.Changed("negative-prompt") {
		base.Negative = flagVals.Negative
	}
	if f.Changed("aspect-ratio") {
		base.Aspect = flagVals.Aspect
	}
	if f.Changed("quality") {
		base.Quality = flagVals.Quality
	}
	if f.Changed("quantity") {
		base.Quantity = flagVals.Quantity
	}
	if f.Changed("seed") {
		base.Seed = flagVals.Seed
	}
	if f.Changed("checkpoint-id") {
		base.CheckpointID = flagVals.CheckpointID
	}
	if f.Changed("folder-id") {
		base.FolderID = flagVals.FolderID
	}
	return base, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func trimLong(s string) string {
	// Trim only leading/trailing newlines, preserving example indentation.
	for len(s) > 0 && (s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

func progressWriter(cmd *cobra.Command, flags *rootFlags) io.Writer {
	if flags.asJSON || flags.quiet {
		return nil
	}
	return cmd.ErrOrStderr()
}
