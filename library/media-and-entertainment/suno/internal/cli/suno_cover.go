// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `cover` — produce a cover of an existing clip. POST /api/generate/v2-web/
// with create_mode "cover" and cover_clip_id set. Upstream requires a title.
// Captcha-gated like generate.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSunoCoverCmd(flags *rootFlags) *cobra.Command {
	var (
		title       string
		tags        string
		model       string
		token       string
		noCaptcha   bool
		wait        bool
		downloadDir string
		workspace   string
	)

	cmd := &cobra.Command{
		Use:   "cover <clip_id>",
		Short: "Create a cover of an existing clip (--title required, captcha-gated)",
		Long: `Create a cover (re-interpretation) of an existing clip in a new style.

--title is required by the upstream cover endpoint.

Captcha-gated: when Suno's hCaptcha gate trips, the CLI auto-solves it with a
dedicated piloted-Chrome profile (see 'auth captcha'). Use --captcha-profile to
pick an account, or pass a pre-solved --token to skip the browser.`,
		Example:     `  suno-pp-cli generate cover 550e8400-e29b-41d4-a716-446655440000 --title "Acoustic Version" --tags "acoustic" --token <hc>`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/api/generate/v2-web/"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a clip_id is required"))
			}
			clipID := args[0]

			if title == "" {
				return usageErr(fmt.Errorf("--title is required for cover"))
			}

			mv, err := resolveModel(model, sunoGenerateModels, sunoGenerateModelOrder)
			if err != nil {
				return err
			}

			// Optimistic captcha: attempt without a token; runGenerationFlow
			// surfaces captchaRequiredError only if Suno actually challenges it.
			if dryRunOK(flags) {
				return nil
			}

			body := buildGenerateBody(generateInput{
				createMode:  "cover",
				mv:          mv,
				title:       title,
				tags:        tags,
				token:       token,
				coverClipID: clipID,
			})
			return runGenerationFlow(cmd, flags, body, wait, downloadDir, workspace, noCaptcha)
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "Cover title (required)")
	cmd.Flags().StringVar(&tags, "tags", "", "Style tags for the cover (comma-separated)")
	cmd.Flags().StringVar(&model, "model", defaultGenerateModel, "Model: v5.5, v5, v4.5+, v4.5, v4, v3.5, v3, v2")
	cmd.Flags().StringVar(&token, "token", "", "hCaptcha token (required unless --no-captcha)")
	cmd.Flags().BoolVar(&noCaptcha, "no-captcha", false, "Bypass the hCaptcha requirement")
	cmd.Flags().StringVar(&captchaProfileFlag, "captcha-profile", "", "Solver profile (Chrome account) to use when the gate trips")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace (project) ID to add the generated clip(s) to")
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll until generation completes")
	cmd.Flags().StringVar(&downloadDir, "download", "", "Download finished clips to this directory (implies --wait)")
	return cmd
}
