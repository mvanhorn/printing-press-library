// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `edit upscale` and `edit bg-remove` drive Artistly's editor
// image-to-image tools (editor.artistly.ai). See editor.go for the transport,
// auto-mint auth, and result polling. Input can be an existing design (id/uuid),
// an image URL, or a local file path.
//
// pp:data-source live

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newEditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "edit",
		Short:       "Image-to-image edit tools: upscale and background removal",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEditUpscaleCmd(flags))
	cmd.AddCommand(newEditBgRemoveCmd(flags))
	return cmd
}

func newEditUpscaleCmd(flags *rootFlags) *cobra.Command {
	var face bool
	var wait bool
	var download string
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "upscale <design-id>",
		Short: "Upscale/enhance an image (existing design, URL, or local file).",
		Long: trimLong(`
Upscale an image via Artistly's editor. The input can be an existing design
(by id or uuid), an image URL, or a local file path. Add --face to enhance
faces. With --wait the command blocks until the result renders; --download saves
it. Creates a new design and consumes editor quota.`),
		Example:     "  artistly-pp-cli edit upscale <design-id> --wait --download ./out",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			faceQ := "no"
			if face {
				faceQ = "yes"
			}
			return runEditOp(cmd, flags, args, "upscale", "upscale", "/api/ai/upscaler",
				func(userID, dataURI string) map[string]any {
					return map[string]any{
						"baseImageBase64": dataURI,
						"face_quality":    faceQ,
						"tool_used":       "upscaler",
						"user_id":         userID,
					}
				}, wait, download, waitTimeout)
		},
	}
	cmd.Flags().BoolVar(&face, "face", false, "Enhance faces during upscale")
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the result renders")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download the result into")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "Maximum time to wait when --wait/--download is set")
	return cmd
}

func newEditBgRemoveCmd(flags *rootFlags) *cobra.Command {
	var wait bool
	var download string
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "bg-remove <design-id|image-url|file>",
		Short: "Remove an image's background (existing design, URL, or local file).",
		Long: trimLong(`
Remove the background from an image via Artistly's editor (Photoroom). The input
can be an existing design (by id or uuid), an image URL, or a local file path.
With --wait the command blocks until the result renders; --download saves it.
Creates a new design and consumes editor quota.`),
		Example:     "  artistly-pp-cli edit bg-remove ./product.png --wait --download ./out",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEditOp(cmd, flags, args, "background removal", "bg-remove", "/api/ai/replace-bg",
				func(userID, dataURI string) map[string]any {
					return map[string]any{
						"image":   dataURI,
						"mode":    "bg_remove",
						"user_id": userID,
					}
				}, wait, download, waitTimeout)
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Block until the result renders")
	cmd.Flags().StringVar(&download, "download", "", "Directory to download the result into")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 5*time.Minute, "Maximum time to wait when --wait/--download is set")
	return cmd
}

// runEditOp is the shared flow for editor edit commands: verify-friendly guards,
// auto-mint the editor token, resolve the input image, submit, and optionally
// wait + download.
func runEditOp(cmd *cobra.Command, flags *rootFlags, args []string, opName, tag, path string,
	bodyBuild func(userID, dataURI string) map[string]any, wait bool, download string, waitTimeout time.Duration) error {

	if len(args) == 0 && cmd.Flags().NFlag() == 0 {
		return cmd.Help()
	}
	if dryRunOK(flags) || cliutil.IsVerifyEnv() {
		return writeNoop(flags, "dry_run", "would run "+opName)
	}
	if len(args) < 1 {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("an input image is required (design id/uuid, image URL, or file path)"))
	}
	input := args[0]

	ctx := cmd.Context() // editor ops are async; poll self-bounds by --wait-timeout
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	token, err := mintEditorToken(ctx, c)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	userID, err := editorUserID(ctx, token)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	dataURI, err := imageToDataURI(ctx, c, input)
	if err != nil {
		return classifyAPIError(err, flags)
	}

	id, err := submitEditorOp(ctx, token, path, bodyBuild(userID, dataURI))
	if err != nil {
		return classifyAPIError(err, flags)
	}

	if !wait && download == "" {
		if flags.asJSON {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"op": opName, "id": id, "queued": true}, flags)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Queued %s (id %s). Add --wait to block for the result.\n", opName, id)
		return nil
	}

	images, err := pollEditorImage(ctx, token, id, waitTimeout, progressWriter(cmd, flags))
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var files []string
	if download != "" {
		files, err = downloadURLs(ctx, images, download, tag+"-"+id)
		if err != nil {
			return err
		}
	}
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"op": opName, "id": id, "images": images, "files": files}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s complete (id %s):\n", opName, id)
	for _, u := range images {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", u)
	}
	for _, f := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "  saved: %s\n", f)
	}
	return nil
}
