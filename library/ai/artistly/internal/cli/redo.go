// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `redo` resubmits a past design's exact settings (prompt, style,
// dimensions, seed) via the generate flow, with optional overrides. It reads the
// source design live from /fetch-personal-designs so no prior sync is required.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelRedoCmd(flags *rootFlags) *cobra.Command {
	var seed string
	var quantity int
	var promptAppend string
	var wait bool
	var download string
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "redo <design-id>",
		Short: "Resubmit a past design's settings (prompt, checkpoint, dimensions, seed) with optional overrides.",
		Long: trimLong(`
Re-run a previous design by its numeric id. The original prompt, negative
prompt, checkpoint, dimensions, aspect ratio, and seed are reused, so you get
more of the same. Override individual settings with --seed, --quantity, or
--prompt-append.

Use this to GENERATE new images from one existing design's settings. For many
new prompts use 'batch'. To merely re-download the original images use 'export'.`),
		Example:     "  artistly-pp-cli redo <design-id> --seed random --quantity 4 --wait --download ./out",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resubmit a past design")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would resubmit a past design")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a design id is required"))
			}
			id, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				return usageErr(fmt.Errorf("design id must be a number: %q", args[0]))
			}

			ctx := cmd.Context() // --wait can outlast root --timeout; poll self-bounds
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			designs, err := fetchPersonalDesigns(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var src *Design
			for i := range designs {
				if designs[i].ID == id {
					src = &designs[i]
					break
				}
			}
			if src == nil {
				return notFoundErr(fmt.Errorf("design %d not found in your recent designs", id))
			}

			s := genSettings{
				Style:        "",
				Negative:     src.NegativePrompt,
				Aspect:       src.AspectRatio,
				CheckpointID: src.CheckpointID,
				Quantity:     src.Quantity,
			}
			if cmd.Flags().Changed("quantity") {
				s.Quantity = quantity
			}
			// Seed: default reuse the source seed; "random" => 0.
			if cmd.Flags().Changed("seed") {
				if strings.EqualFold(seed, "random") {
					s.Seed = 0
				} else {
					n, serr := strconv.Atoi(seed)
					if serr != nil {
						return usageErr(fmt.Errorf("--seed must be a number or 'random'"))
					}
					s.Seed = n
				}
			} else if src.Seed != "" {
				if n, serr := strconv.Atoi(src.Seed); serr == nil {
					s.Seed = n
				}
			}

			prompt := src.PositivePrompt
			if promptAppend != "" {
				prompt = strings.TrimSpace(prompt + " " + promptAppend)
			}

			newDesigns, err := runGeneration(ctx, c, prompt, s, wait, download, waitTimeout, progressWriter(cmd, flags))
			if err != nil {
				// Surface any designs that finished before a mid-render timeout
				// (already downloaded if --download was set) before exiting non-zero.
				if len(newDesigns) > 0 {
					_ = emitGenerationResult(cmd, flags, prompt, newDesigns, wait)
				}
				return classifyAPIError(err, flags)
			}
			return emitGenerationResult(cmd, flags, prompt, newDesigns, wait)
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "", "Override seed: a number, or 'random'")
	cmd.Flags().IntVar(&quantity, "quantity", 0, "Override number of images")
	cmd.Flags().StringVar(&promptAppend, "prompt-append", "", "Append text to the original prompt")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the generation renders")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download finished images into")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "Maximum time to wait when --wait/--download is set")
	return cmd
}
