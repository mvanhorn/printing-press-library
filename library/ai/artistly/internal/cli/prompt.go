// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `prompt enhance` and `prompt extract` wrap Artistly's prompt
// tools. Both are Laravel flash-redirect endpoints (contracts verified live):
// enhance = POST /prompt/enhance {prompt} -> props.enhancedPrompt; extract =
// POST /prompt/extract {image: <url>} -> props.extractedPrompt. The result is
// flashed and read from the next page load (see runPromptTransform).
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newPromptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "prompt",
		Short:       "AI prompt tools: enhance a prompt, or extract a prompt from an image",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPromptEnhanceCmd(flags))
	cmd.AddCommand(newPromptExtractCmd(flags))
	return cmd
}

func newPromptEnhanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enhance <prompt>",
		Short: "Expand a short prompt into a rich, detailed one using Artistly's enhancer.",
		Long: trimLong(`
Send a prompt to Artistly's AI enhancer and print the expanded, more detailed
version. Useful as a pre-step before 'generate'. Reads the result from the
authenticated session; no image is generated and no quota is consumed.`),
		Example:     "  artistly-pp-cli prompt enhance \"a fox in a forest\"",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would enhance a prompt")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would enhance a prompt")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a prompt is required"))
			}
			prompt := strings.Join(args, " ")
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			enhanced, err := runPromptTransform(ctx, c, "/prompt/enhance", map[string]any{"prompt": prompt}, "enhancedPrompt")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"original": prompt, "enhanced": enhanced}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), enhanced)
			return nil
		},
	}
	return cmd
}

func newPromptExtractCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <design-id>",
		Short: "Extract a text prompt from an image (image-to-prompt).",
		Long: trimLong(`
Generate a descriptive prompt from an image. Pass an image URL directly, or a
design id/uuid from your library (its image URL is resolved automatically).
Reads the result from the authenticated session; no quota is consumed.`),
		Example:     "  artistly-pp-cli prompt extract <design-id>\n  artistly-pp-cli prompt extract https://cdn.artistly.ai/user-assets/...png",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would extract a prompt from an image")
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would extract a prompt from an image")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an image URL or design id/uuid is required"))
			}
			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			imageURL := strings.TrimSpace(args[0])
			if !strings.HasPrefix(imageURL, "http://") && !strings.HasPrefix(imageURL, "https://") {
				// Treat as a design id/uuid; resolve to its first real image URL.
				d, derr := findDesign(ctx, c, imageURL)
				if derr != nil {
					return classifyAPIError(derr, flags)
				}
				urls := designImageURLs(*d)
				if len(urls) == 0 {
					return apiErr(fmt.Errorf("design %q has no rendered image to extract from (status: %s)", imageURL, d.Status))
				}
				imageURL = urls[0]
			}

			extracted, err := runPromptTransform(ctx, c, "/prompt/extract", map[string]any{"image": imageURL}, "extractedPrompt")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"image": imageURL, "extracted": extracted}, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), extracted)
			return nil
		},
	}
	return cmd
}
