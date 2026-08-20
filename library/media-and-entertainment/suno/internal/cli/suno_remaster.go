// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `remaster` — remaster an existing clip with a newer model. POST
// /api/generate/v2-web/ with create_mode "remaster", cover_clip_id set, and
// mv = the remaster model key. Captcha-gated like generate.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSunoRemasterCmd(flags *rootFlags) *cobra.Command {
	var (
		model       string
		token       string
		noCaptcha   bool
		wait        bool
		downloadDir string
		workspace   string
	)

	cmd := &cobra.Command{
		Use:   "remaster <clip_id>",
		Short: "Remaster an existing clip with a newer model (captcha-gated)",
		Long: `Remaster an existing clip, re-rendering it with a newer remaster-capable model.

Remaster models (--model -> wire key): v5.5 -> chirp-flounder, v5 -> chirp-carp,
v4.5+ -> chirp-bass.

Captcha-gated: when Suno's hCaptcha gate trips, the CLI auto-solves it with a
dedicated piloted-Chrome profile (see 'auth captcha'). Use --captcha-profile to
pick an account, or pass a pre-solved --token to skip the browser.`,
		Example:     `  suno-pp-cli generate remaster 550e8400-e29b-41d4-a716-446655440000 --model v5.5 --token <hc>`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/api/generate/v2-web/"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a clip_id is required"))
			}
			clipID := args[0]

			mv, err := resolveModel(model, sunoRemasterModels, sunoRemasterModelOrder)
			if err != nil {
				return err
			}

			// Optimistic captcha: attempt without a token; runGenerationFlow
			// surfaces captchaRequiredError only if Suno actually challenges it.
			if dryRunOK(flags) {
				return nil
			}

			body := buildGenerateBody(generateInput{
				createMode:  "remaster",
				mv:          mv,
				token:       token,
				coverClipID: clipID,
			})
			return runGenerationFlow(cmd, flags, body, wait, downloadDir, workspace, noCaptcha)
		},
	}

	cmd.Flags().StringVar(&model, "model", defaultGenerateModel, "Remaster model: v5.5, v5, v4.5+")
	cmd.Flags().StringVar(&token, "token", "", "hCaptcha token (required unless --no-captcha)")
	cmd.Flags().BoolVar(&noCaptcha, "no-captcha", false, "Bypass the hCaptcha requirement")
	cmd.Flags().StringVar(&captchaProfileFlag, "captcha-profile", "", "Solver profile (Chrome account) to use when the gate trips")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace (project) ID to add the generated clip(s) to")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until generation completes")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Download finished clips to this directory (implies --wait)")
	return cmd
}
