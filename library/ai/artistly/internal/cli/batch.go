// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `batch` generates from a file of prompts in one quota-aware
// run, submitting sequentially (pacing itself against the concurrent limit) and
// optionally waiting for and downloading each result. This is the headline
// leverage over the one-prompt-at-a-time web UI.
//
// pp:data-source live

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelBatchCmd(flags *rootFlags) *cobra.Command {
	var s genSettings
	var presetName string
	var wait bool
	var download string
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "batch <file>",
		Short: "Generate images from a file of prompts (one per line) in one quota-aware run.",
		Long: trimLong(`
Generate from a file of prompts — one prompt per line; blank lines and lines
starting with # are ignored. Shared settings (--style, --aspect-ratio,
--quality, --quantity, --preset, ...) apply to every prompt.

Prompts are submitted sequentially so the run paces itself against Artistly's
concurrent limit and the undocumented ~400/day cap. With --wait each generation
is awaited before the next; add --download <dir> to save finished images.

For a single prompt use 'generate'. To re-run one past design use 'redo'. To
download already-finished images use 'export'.`),
		Example:     "  artistly-pp-cli batch prompts.txt --preset house-style --wait --download ./out",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would batch-generate from a prompt file")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would batch-generate from a prompt file")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a prompt file is required"))
			}
			prompts, err := readPromptFile(args[0])
			if err != nil {
				return err
			}
			if len(prompts) == 0 {
				return usageErr(fmt.Errorf("no prompts found in %s", args[0]))
			}

			settings, err := mergePresetAndFlags(cmd, presetName, s)
			if err != nil {
				return err
			}

			ctx := cmd.Context() // --wait can outlast root --timeout; poll self-bounds
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Under live-dogfood, only exercise one prompt to keep cost bounded.
			if cliutil.IsDogfoodEnv() && len(prompts) > 1 {
				prompts = prompts[:1]
			}

			type batchResult struct {
				Prompt   string `json:"prompt"`
				Rendered int    `json:"rendered"`
				Error    string `json:"error,omitempty"`
			}
			results := make([]batchResult, 0, len(prompts))
			submitted := 0
			failures := 0
			for i, p := range prompts {
				if i > 0 {
					select { // pace submissions, but stay responsive to cancellation
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Second):
					}
				}
				designs, gerr := runGeneration(ctx, c, p, settings, wait, download, waitTimeout, progressWriter(cmd, flags))
				r := batchResult{Prompt: p, Rendered: len(designs)}
				if gerr != nil {
					r.Error = gerr.Error()
					failures++
				} else {
					submitted++
				}
				results = append(results, r)
				if !flags.asJSON {
					if gerr != nil {
						// On a mid-render timeout runGeneration returns the designs
						// that did finish (already downloaded if --download was set);
						// surface that count so partial progress isn't hidden, matching
						// generate/redo.
						if len(designs) > 0 {
							fmt.Fprintf(cmd.ErrOrStderr(), "  [%d/%d] FAILED %q after %d image(s) rendered: %v\n", i+1, len(prompts), truncate(p, 50), len(designs), gerr)
						} else {
							fmt.Fprintf(cmd.ErrOrStderr(), "  [%d/%d] FAILED %q: %v\n", i+1, len(prompts), truncate(p, 50), gerr)
						}
					} else if wait {
						fmt.Fprintf(cmd.OutOrStdout(), "  [%d/%d] rendered %d image(s) for %q\n", i+1, len(prompts), len(designs), truncate(p, 50))
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "  [%d/%d] queued %q\n", i+1, len(prompts), truncate(p, 50))
					}
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"total":     len(prompts),
					"submitted": submitted,
					"failed":    failures,
					"results":   results,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Batch complete: %d submitted, %d failed of %d prompts.\n", submitted, failures, len(prompts))
			if failures > 0 {
				return apiErr(fmt.Errorf("%d of %d generations failed", failures, len(prompts)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&presetName, "preset", "", "Apply a saved settings preset")
	cmd.Flags().StringVar(&s.Tool, "tool", "", "Generator feature slug (default image-designer-v6)")
	cmd.Flags().StringVar(&s.Style, "style", "", "Style label/slug to apply to every prompt")
	cmd.Flags().StringVar(&s.Negative, "negative-prompt", "", "Negative prompt for every prompt")
	cmd.Flags().StringVar(&s.Aspect, "aspect-ratio", "", "Aspect ratio: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3")
	cmd.Flags().StringVar(&s.Quality, "quality", "", "Quality: fast or highQuality")
	cmd.Flags().IntVar(&s.Quantity, "quantity", 0, "Images per prompt (default 1)")
	cmd.Flags().IntVar(&s.Seed, "seed", 0, "Seed for every prompt (0 = random)")
	cmd.Flags().IntVar(&s.CheckpointID, "checkpoint-id", 0, "Checkpoint (model) id")
	cmd.Flags().IntVar(&s.FolderID, "folder-id", 0, "Folder id to place results in")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for each generation to render before the next")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download finished images into (implies --wait)")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "Per-prompt wait timeout when --wait/--download is set")
	return cmd
}

// readPromptFile reads prompts from a file: one prompt per line, skipping blank
// lines and lines beginning with '#'.
func readPromptFile(path string) ([]string, error) {
	f, err := os.Open(path) // #nosec G304 -- path is the user's own --file prompt list; reading it is the command's purpose
	if err != nil {
		return nil, usageErr(fmt.Errorf("cannot read prompt file %s: %w", path, err))
	}
	defer f.Close()
	var prompts []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		prompts = append(prompts, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return prompts, nil
}
